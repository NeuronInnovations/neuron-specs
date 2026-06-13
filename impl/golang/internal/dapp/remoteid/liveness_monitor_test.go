package remoteid

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Test_FR_R21_LivenessDisclosureRoundTrip groups the Stage 3C buyer-side
// FR-R21 coverage so the CONFORMANCE.md row resolves to a single wrapper.
func Test_FR_R21_LivenessDisclosureRoundTrip(t *testing.T) {
	t.Run("healthy-first-heartbeat", TestMonitor_Healthy_FirstHeartbeat)
	t.Run("healthy-to-stale", TestMonitor_Healthy_To_Stale)
	t.Run("stale-to-offline-on-silence", TestMonitor_Stale_To_Offline_OnSilence)
	t.Run("offline-graceful-sentinel", TestMonitor_Offline_GracefulSentinel)
	t.Run("degraded-while-alive", TestMonitor_Degraded_While_Alive)
	t.Run("degraded-to-stale-to-offline", TestMonitor_Degraded_To_Stale_To_Offline)
	t.Run("recovery-offline-to-healthy", TestMonitor_Recovery_OfflineToHealthy)
	t.Run("drops-invalid-heartbeats", TestMonitor_DropsInvalidHeartbeats)
	t.Run("agenturisha256-match-silent", TestMonitor_AgentURISha256_MatchSilent)
	t.Run("agenturisha256-mismatch-logs-warning", TestMonitor_AgentURISha256_MismatchLogsWarning)
}

// --- Test helpers (mirrored from internal/edgeapp/liveness_test.go) ---

type fakeMonitorClock struct {
	nanos atomic.Int64
}

func (c *fakeMonitorClock) Now() time.Time {
	return time.Unix(0, c.nanos.Load()).UTC()
}
func (c *fakeMonitorClock) Set(t time.Time) {
	c.nanos.Store(t.UnixNano())
}
func (c *fakeMonitorClock) Advance(d time.Duration) {
	c.nanos.Add(int64(d))
}

func newMonitorKey(t *testing.T) (*keylib.NeuronPrivateKey, string) {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &k, k.PublicKey().EVMAddress().Hex()
}

// makeMonitorDelivery builds a signed heartbeat delivery carrying an
// optional Operational sub-struct. Pass nil for ops to omit Operational
// entirely (Stage 2-shaped heartbeat).
func makeMonitorDelivery(
	t *testing.T,
	key *keylib.NeuronPrivateKey,
	deadline, senderClock, seq, consensusSec uint64,
	ops *health.OperationalCapabilities,
) topic.MessageDelivery {
	t.Helper()
	caps := &health.Capabilities{
		Protocols: []health.ProtocolID{health.ProtocolID(ProtocolRaw)},
	}
	if ops != nil {
		caps.Operational = ops
	}
	payload := health.BuildHeartbeatPayload(deadline, health.RoleSeller,
		health.WithCapabilities(caps))
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

// runMonitor spins up a monitor against the given input channel and
// returns the events channel + done. Tests own the lifecycle.
func runMonitor(
	ctx context.Context,
	sellerEVM string,
	clock *fakeMonitorClock,
	in chan topic.MessageDelivery,
	logger *log.Logger,
	expectedSha string,
) (<-chan RemoteIdLivenessEvent, chan error) {
	events := make(chan RemoteIdLivenessEvent, 8)
	m := &RemoteIdLivenessMonitor{
		SellerEVM:              sellerEVM,
		Deliveries:             in,
		PollInterval:           50 * time.Millisecond,
		Now:                    clock.Now,
		Logger:                 logger,
		Events:                 events,
		ExpectedAgentURISha256: expectedSha,
	}
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()
	return events, done
}

// drainAllEvents collects every event off the channel until it closes,
// returning the slice. Caller must ensure the producer terminates first
// (e.g. by cancelling the monitor's ctx).
func drainAllEvents(ch <-chan RemoteIdLivenessEvent) []RemoteIdLivenessEvent {
	var out []RemoteIdLivenessEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

// --- Tests ---

// TestMonitor_Healthy_FirstHeartbeat — one valid heartbeat with
// degraded=false produces Healthy(Alive).
func TestMonitor_Healthy_FirstHeartbeat(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName, Degraded: false})
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.Len(t, all, 1, "exactly one Healthy event expected")
	assert.Equal(t, LivenessHealthy, all[0].State)
	assert.Equal(t, health.StateAlive, all[0].Spec005)
	assert.False(t, all[0].Degraded)
}

// TestMonitor_Healthy_To_Stale — after the deadline + GracePeriod
// elapses without a new heartbeat, the monitor emits Stale (spec005=Suspect).
func TestMonitor_Healthy_To_Stale(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName})
	time.Sleep(120 * time.Millisecond)

	// Advance past deadline + GracePeriod (60 + 30 = 90s + 1s slack).
	clock.Advance(91 * time.Second)
	time.Sleep(150 * time.Millisecond) // let the ticker fire

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.GreaterOrEqual(t, len(all), 2, "expected Healthy then Stale; got %d", len(all))
	assert.Equal(t, LivenessHealthy, all[0].State)
	assert.Equal(t, LivenessStale, all[len(all)-1].State,
		"final emitted state must be Stale after deadline+grace expires")
}

// TestMonitor_Stale_To_Offline_OnSilence — past deadline + grace +
// SuspectToDead → Offline (spec005=Dead).
func TestMonitor_Stale_To_Offline_OnSilence(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName})
	time.Sleep(120 * time.Millisecond)

	// Cross deadline + grace + SuspectToDead (60 + 30 + 120 = 210s + slack).
	clock.Advance(211 * time.Second)
	time.Sleep(150 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.GreaterOrEqual(t, len(all), 2)
	final := all[len(all)-1]
	assert.Equal(t, LivenessOffline, final.State,
		"final emitted state must be Offline once SuspectToDead window closes")
	assert.True(t, final.Spec005 == health.StateDead || final.Spec005 == health.StateOffline,
		"final spec005 must be Dead or Offline; got %s", final.Spec005)
}

// TestMonitor_Offline_GracefulSentinel — a heartbeat with deadline=0
// (shutdown sentinel) takes the seller straight to Offline.
func TestMonitor_Offline_GracefulSentinel(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	// First a healthy heartbeat so we have a transition baseline.
	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName})
	time.Sleep(120 * time.Millisecond)

	// Now publish the shutdown sentinel (deadline=0).
	consensus2 := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, 0, consensus2, 2, consensus2,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName})
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.GreaterOrEqual(t, len(all), 2)
	final := all[len(all)-1]
	assert.Equal(t, LivenessOffline, final.State)
	assert.Equal(t, health.StateOffline, final.Spec005,
		"shutdown sentinel must produce spec005=Offline (not Dead)")
}

// TestMonitor_Degraded_While_Alive — a heartbeat with degraded=true
// within deadline produces Degraded (not Healthy).
func TestMonitor_Degraded_While_Alive(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName, Degraded: true})
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.Len(t, all, 1)
	assert.Equal(t, LivenessDegraded, all[0].State)
	assert.Equal(t, health.StateAlive, all[0].Spec005,
		"degraded surfaces only when underlying spec005 is Alive")
	assert.True(t, all[0].Degraded)
}

// TestMonitor_Degraded_To_Stale_To_Offline — degraded then silence →
// Degraded → Stale → Offline.
func TestMonitor_Degraded_To_Stale_To_Offline(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName, Degraded: true})
	time.Sleep(120 * time.Millisecond)

	clock.Advance(91 * time.Second)
	time.Sleep(150 * time.Millisecond)

	clock.Advance(120 * time.Second)
	time.Sleep(150 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.GreaterOrEqual(t, len(all), 3, "expected Degraded → Stale → Offline; got %d", len(all))
	assert.Equal(t, LivenessDegraded, all[0].State)
	// Match the FIRST occurrence of each later state, since the ticker may
	// emit multiple Offline events while time advances under it.
	assert.Equal(t, LivenessStale, all[1].State)
	assert.Equal(t, LivenessOffline, all[len(all)-1].State)
}

// TestMonitor_Recovery_OfflineToHealthy — after Offline, a fresh
// degraded=false heartbeat transitions back to Healthy.
func TestMonitor_Recovery_OfflineToHealthy(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, nil, "")

	// First heartbeat → Healthy.
	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName})
	time.Sleep(120 * time.Millisecond)

	// Advance enough to reach Offline (deadline + grace + SuspectToDead).
	clock.Advance(220 * time.Second)
	time.Sleep(150 * time.Millisecond)

	// Recovery heartbeat — clock has moved 220s; deadline must be in the future.
	consensus2 := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus2+60, consensus2, 2, consensus2,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName, Degraded: false})
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.GreaterOrEqual(t, len(all), 3)
	assert.Equal(t, LivenessHealthy, all[len(all)-1].State,
		"final transition after recovery must be Healthy")
}

// TestMonitor_DropsInvalidHeartbeats — a delivery with a tampered
// payload fails validation; no event is emitted and the logger records
// a drop.
func TestMonitor_DropsInvalidHeartbeats(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, logger, "")

	// Build a valid delivery then corrupt the payload bytes so
	// signature verification fails.
	consensus := uint64(clock.Now().Unix())
	d := makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{ServiceName: CommerceServiceName})
	// Replace message's payload via a fresh TopicMessage signed by a
	// DIFFERENT key — the senderAddress derived from signature recovery
	// will then point at the wrong address.
	otherKey, _ := newMonitorKey(t)
	tampered, err := topic.NewTopicMessage(otherKey, consensus, 1, d.Message.Payload())
	require.NoError(t, err)
	d.Message = tampered
	in <- d
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	// observer.ProcessDelivery may either drop the heartbeat (no event)
	// OR succeed and register the OTHER signer's address (no transition
	// on m.SellerEVM since we never received a valid heartbeat for it).
	// Either way, no event for our target seller.
	for _, ev := range all {
		assert.NotEqual(t, evm, ev.SellerEVM,
			"target seller must not emit events for tampered heartbeats")
	}
	// Optional: assert no Healthy event got through for our seller.
	for _, ev := range all {
		if ev.SellerEVM == evm {
			t.Errorf("unexpected event for target seller after tamper: %+v", ev)
		}
	}
}

// TestMonitor_AgentURISha256_MatchSilent — agentURISha256 advertised
// matches discovery.AgentURISha256 → no mismatch log; Healthy still
// emits.
func TestMonitor_AgentURISha256_MatchSilent(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	const sha = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, logger, sha)

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{
			ServiceName:    CommerceServiceName,
			AgentURISha256: sha,
		})
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.Len(t, all, 1)
	assert.Equal(t, LivenessHealthy, all[0].State)
	assert.NotContains(t, logBuf.String(), "evidence-grade mismatch",
		"matching agentURISha256 must NOT trigger the mismatch warning")
}

// TestMonitor_AgentURISha256_MismatchLogsWarning — advertised SHA disagrees
// with the buyer-captured value; emit a single mismatch log line; state
// transition is still Healthy (no consequence beyond logging).
func TestMonitor_AgentURISha256_MismatchLogsWarning(t *testing.T) {
	clock := &fakeMonitorClock{}
	clock.Set(time.Unix(1_700_000_000, 0))
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 4)
	var logBuf bytes.Buffer
	logger := log.New(&logBuf, "", 0)

	const sellerSha = "1111111111111111111111111111111111111111111111111111111111111111"
	const buyerSha = "2222222222222222222222222222222222222222222222222222222222222222"

	ctx, cancel := context.WithCancel(context.Background())
	events, done := runMonitor(ctx, evm, clock, in, logger, buyerSha)

	consensus := uint64(clock.Now().Unix())
	in <- makeMonitorDelivery(t, key, consensus+60, consensus, 1, consensus,
		&health.OperationalCapabilities{
			ServiceName:    CommerceServiceName,
			AgentURISha256: sellerSha,
		})
	time.Sleep(120 * time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	all := drainAllEvents(events)
	require.Len(t, all, 1, "Healthy still emits despite SHA mismatch")
	assert.Equal(t, LivenessHealthy, all[0].State)

	logs := logBuf.String()
	assert.Contains(t, logs, "evidence-grade mismatch",
		"SHA mismatch must produce a warn-only log line")
	assert.Contains(t, logs, "registered="+buyerSha)
	assert.Contains(t, logs, "advertised="+sellerSha)
}

// TestMonitor_RaceSafe_RealTicker — under -race, 200ms PollInterval,
// 10 deliveries + concurrent ticks. Must produce zero data race AND
// terminate cleanly on ctx cancel.
func TestMonitor_RaceSafe_RealTicker(t *testing.T) {
	t.Parallel()
	key, evm := newMonitorKey(t)
	in := make(chan topic.MessageDelivery, 16)
	events := make(chan RemoteIdLivenessEvent, 64)
	logger := log.New(&bytes.Buffer{}, "", 0)
	m := &RemoteIdLivenessMonitor{
		SellerEVM:    evm,
		Deliveries:   in,
		PollInterval: 25 * time.Millisecond,
		Logger:       logger,
		Events:       events,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()

	// Drain events concurrently to avoid blocking emit().
	drained := make(chan int, 1)
	go func() {
		count := 0
		for range events {
			count++
		}
		drained <- count
	}()

	// Send 10 deliveries with monotonically-advancing time so the ticker
	// fires concurrently with deliveries.
	base := uint64(time.Now().Unix())
	for i := uint64(0); i < 10; i++ {
		consensus := base + i
		in <- makeMonitorDelivery(t, key, consensus+60, consensus, i+1, consensus,
			&health.OperationalCapabilities{ServiceName: CommerceServiceName})
		time.Sleep(15 * time.Millisecond)
	}

	time.Sleep(60 * time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	<-drained
}

// TestMonitor_RejectsInvalidConfig covers the validate() error paths.
func TestMonitor_RejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		m    *RemoteIdLivenessMonitor
		want string
	}{
		{
			name: "missing-sellerEVM",
			m: &RemoteIdLivenessMonitor{
				Deliveries: make(chan topic.MessageDelivery),
				Events:     make(chan RemoteIdLivenessEvent),
			},
			want: "SellerEVM required",
		},
		{
			name: "missing-deliveries",
			m: &RemoteIdLivenessMonitor{
				SellerEVM: "0xabc",
				Events:    make(chan RemoteIdLivenessEvent),
			},
			want: "Deliveries channel required",
		},
		{
			name: "missing-events",
			m: &RemoteIdLivenessMonitor{
				SellerEVM:  "0xabc",
				Deliveries: make(chan topic.MessageDelivery),
			},
			want: "Events channel required",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.m.Run(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
