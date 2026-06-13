package edgeapp

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// makeHeartbeatDelivery constructs a signed TopicMessage envelope wrapping
// a spec-005 heartbeat payload, then bundles it with a consensus timestamp
// (in **seconds** to match the deadline scale spec 005 expects when
// observer arithmetic crosses the topic boundary).
func makeHeartbeatDelivery(
	t *testing.T,
	key *keylib.NeuronPrivateKey,
	deadline, senderClock, seq, consensusSec uint64,
) topic.MessageDelivery {
	t.Helper()
	payload := health.BuildHeartbeatPayload(deadline, health.RoleSeller)
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	msg, err := topic.NewTopicMessage(key, senderClock, seq, data)
	require.NoError(t, err)
	return topic.MessageDelivery{
		Message:            msg,
		ConsensusTimestamp: consensusSec,
		BackendSequence:    seq,
	}
}

// fakeClock is a goroutine-safe time source the watchdog can be steered
// against. Tests advance time explicitly with .Advance.
type fakeClock struct {
	nanos atomic.Int64
}

func (c *fakeClock) Now() time.Time {
	return time.Unix(0, c.nanos.Load()).UTC()
}
func (c *fakeClock) Set(t time.Time) {
	c.nanos.Store(t.UnixNano())
}
func (c *fakeClock) Advance(d time.Duration) {
	c.nanos.Add(int64(d))
}

func newKey(t *testing.T) (*keylib.NeuronPrivateKey, string) {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &k, k.PublicKey().EVMAddress().Hex()
}

// runTracker spins up a tracker with the given input channel and returns
// the events output channel + cancel + done-signal. Tests own the lifecycle.
func runTracker(
	ctx context.Context,
	sellerEVM string,
	clock *fakeClock,
	in chan topic.MessageDelivery,
) (<-chan LivenessEvent, chan error) {
	events := make(chan LivenessEvent, 8)
	tr := &LivenessTracker{
		SellerEVM:    sellerEVM,
		Deliveries:   in,
		PollInterval: 50 * time.Millisecond,
		Now:          clock.Now,
		Events:       events,
	}
	done := make(chan error, 1)
	go func() { done <- tr.Run(ctx) }()
	return events, done
}

func TestLivenessTracker_HappyPath_NoEventsWhileAlive(t *testing.T) {
	clock := &fakeClock{}
	clock.Set(time.Unix(1_700_000_000, 0))

	key, evm := newKey(t)
	in := make(chan topic.MessageDelivery, 8)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runTracker(ctx, evm, clock, in)

	// Three back-to-back heartbeats, each declaring deadline = consensus + 60.
	// Seller stays ALIVE throughout.
	for i := uint64(1); i <= 3; i++ {
		consensus := uint64(clock.Now().Unix())
		in <- makeHeartbeatDelivery(t, key, consensus+60, consensus, i, consensus)
		clock.Advance(1 * time.Second)
		time.Sleep(120 * time.Millisecond)
	}

	cancel()
	require.NoError(t, <-done)

	count := 0
	for range events {
		count++
	}
	assert.Equal(t, 0, count, "no LivenessEvent expected during steady-ALIVE")
}

func TestLivenessTracker_DeadlinePasses_EmitsSuspectThenDead(t *testing.T) {
	clock := &fakeClock{}
	clock.Set(time.Unix(1_700_000_000, 0))

	key, evm := newKey(t)
	in := make(chan topic.MessageDelivery, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, done := runTracker(ctx, evm, clock, in)

	// HB1: deadline = consensus + 30s.
	startSec := uint64(clock.Now().Unix())
	in <- makeHeartbeatDelivery(t, key, startSec+30, startSec, 1, startSec)
	time.Sleep(120 * time.Millisecond)

	// Step time past deadline+grace ⇒ SUSPECT.
	clock.Set(time.Unix(int64(startSec)+30+int64(health.GracePeriod)+1, 0))

	var ev LivenessEvent
	select {
	case ev = <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("expected SUSPECT event within 2s of deadline+grace boundary")
	}
	assert.Equal(t, health.StateSuspect, ev.State)
	assert.Equal(t, evm, ev.SellerEVM)
	assert.Equal(t, startSec+30, ev.LastDeadline)

	// Step further: + suspect-to-dead ⇒ DEAD.
	clock.Set(time.Unix(int64(startSec)+30+int64(health.GracePeriod)+int64(health.SuspectToDead)+1, 0))

	select {
	case ev = <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("expected DEAD event")
	}
	assert.Equal(t, health.StateDead, ev.State)

	cancel()
	require.NoError(t, <-done)
}

func TestLivenessTracker_RecoveryEmitsAlive(t *testing.T) {
	clock := &fakeClock{}
	clock.Set(time.Unix(1_700_000_000, 0))

	key, evm := newKey(t)
	in := make(chan topic.MessageDelivery, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, done := runTracker(ctx, evm, clock, in)

	// HB1 — deadline +30 ⇒ ALIVE.
	startSec := uint64(clock.Now().Unix())
	in <- makeHeartbeatDelivery(t, key, startSec+30, startSec, 1, startSec)
	time.Sleep(120 * time.Millisecond)

	// Force into SUSPECT.
	clock.Set(time.Unix(int64(startSec)+30+int64(health.GracePeriod)+1, 0))
	select {
	case ev := <-events:
		assert.Equal(t, health.StateSuspect, ev.State)
	case <-time.After(2 * time.Second):
		t.Fatal("expected SUSPECT event")
	}

	// Recovery heartbeat: a fresh deadline at the new clock time.
	recSec := uint64(clock.Now().Unix())
	in <- makeHeartbeatDelivery(t, key, recSec+60, recSec, 2, recSec)

	select {
	case ev := <-events:
		assert.Equal(t, health.StateAlive, ev.State, "recovery from SUSPECT must emit ALIVE")
	case <-time.After(2 * time.Second):
		t.Fatal("expected ALIVE recovery event")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestLivenessTracker_ShutdownSentinelEmitsOffline(t *testing.T) {
	clock := &fakeClock{}
	clock.Set(time.Unix(1_700_000_000, 0))

	key, evm := newKey(t)
	in := make(chan topic.MessageDelivery, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, done := runTracker(ctx, evm, clock, in)

	// First a normal heartbeat to seed ALIVE state.
	now := uint64(clock.Now().Unix())
	in <- makeHeartbeatDelivery(t, key, now+60, now, 1, now)
	time.Sleep(80 * time.Millisecond)

	// Shutdown sentinel — V-OBS-04 bypasses delta checks.
	in <- makeHeartbeatDelivery(t, key, health.ShutdownSentinel, now+1, 2, now+1)

	select {
	case ev := <-events:
		assert.Equal(t, health.StateOffline, ev.State, "ShutdownSentinel ⇒ OFFLINE")
		assert.Equal(t, evm, ev.SellerEVM)
	case <-time.After(2 * time.Second):
		t.Fatal("expected OFFLINE event after shutdown sentinel")
	}

	cancel()
	require.NoError(t, <-done)
}

func TestLivenessTracker_ClosingDeliveriesStopsRun(t *testing.T) {
	clock := &fakeClock{}
	clock.Set(time.Unix(1_700_000_000, 0))

	_, evm := newKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, done := runTracker(ctx, evm, clock, in)

	close(in)

	select {
	case err := <-done:
		require.NoError(t, err, "closing Deliveries must terminate Run cleanly")
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Deliveries was closed")
	}
}

func TestLivenessTracker_RejectsInvalidConfig(t *testing.T) {
	cases := map[string]LivenessTracker{
		"missing SellerEVM": {
			Deliveries: make(chan topic.MessageDelivery), Events: make(chan LivenessEvent),
		},
		"missing Deliveries": {
			SellerEVM: "0xabc", Events: make(chan LivenessEvent),
		},
		"missing Events": {
			SellerEVM: "0xabc", Deliveries: make(chan topic.MessageDelivery),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := tc
			err := tr.Run(context.Background())
			assert.Error(t, err)
		})
	}
}

func TestLivenessTracker_DropsHeartbeatsFromOtherSenders(t *testing.T) {
	clock := &fakeClock{}
	clock.Set(time.Unix(1_700_000_000, 0))

	target, targetEVM := newKey(t)
	other, _ := newKey(t)
	in := make(chan topic.MessageDelivery, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, done := runTracker(ctx, targetEVM, clock, in)

	// Both target and `other` publish on the same channel. Tracker is
	// configured for `target` only — `other`'s heartbeats keep their own
	// LivenessRecord ALIVE in the wrapped HeartbeatObserver, but the
	// tracker only reads target's record when evaluating state.
	now := uint64(clock.Now().Unix())
	in <- makeHeartbeatDelivery(t, target, now+30, now, 1, now)    // target ALIVE
	in <- makeHeartbeatDelivery(t, other, now+3600, now, 1, now)   // other generous

	time.Sleep(120 * time.Millisecond)

	// Step past target's deadline+grace. Target ⇒ SUSPECT despite other's
	// recent fresh heartbeat.
	clock.Set(time.Unix(int64(now)+30+int64(health.GracePeriod)+1, 0))

	select {
	case ev := <-events:
		assert.Equal(t, health.StateSuspect, ev.State)
		assert.Equal(t, targetEVM, ev.SellerEVM)
	case <-time.After(2 * time.Second):
		t.Fatal("expected SUSPECT event for target only")
	}

	cancel()
	require.NoError(t, <-done)
}
