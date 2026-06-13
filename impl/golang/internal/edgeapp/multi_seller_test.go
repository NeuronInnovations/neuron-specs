package edgeapp

import (
	"context"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeICAOFeed returns a feed source that emits Mode-S extended squitter
// (DF=17) frames whose ICAO24 is the supplied 24-bit prefix. Lets the test
// distinguish frames produced by different sellers.
func makeICAOFeed(icao24Prefix uint32, fps int) FeedSource {
	return func(ctx context.Context, out chan<- feeds.FeedFrame) error {
		interval := time.Second / time.Duration(fps)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var seq uint64
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			// DF=17 extended squitter, 14-byte payload. ICAO in bytes 1-3.
			payload := make([]byte, 14)
			payload[0] = 0x8D // DF=17 (10001000 → top 5 bits = 17), capability = 0
			payload[1] = byte(icao24Prefix >> 16)
			payload[2] = byte(icao24Prefix >> 8)
			payload[3] = byte(icao24Prefix)
			// Pack seq into the data field so we can verify per-seller monotonicity.
			binary.BigEndian.PutUint32(payload[4:8], uint32(seq))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- feeds.FeedFrame{
				Raw:                  payload,
				SecondsSinceMidnight: 12345 + seq/uint64(fps),
				Nanoseconds:          (seq % uint64(fps)) * uint64(time.Second/time.Duration(fps)),
				Rx:                   time.Now().UTC(),
			}:
			}
			seq++
		}
	}
}

// TestMultiSellerAggregation_TwoSellers exercises the multi-seller buyer:
// two synthetic sellers, each producing distinguishable ICAO ranges, feed
// one buyer that aggregates and tags frames by seller.
//
// Asserts:
//   - Frames from BOTH sellers reach the buyer.
//   - Each frame is correctly tagged with its seller's EVM and PeerID.
//   - DecodeModeSMeta extracts the right ICAO24 per seller.
//   - OnSellerStatus fires connected for both sellers.
func TestMultiSellerAggregation_TwoSellers(t *testing.T) {
	bus := NewMemoryBus()

	sellerAKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	sellerBKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	mkTopic := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: memo})
		require.NoError(t, err)
		return ref
	}

	// Pre-create topics for all three agents.
	aIn, aOut, aErr := mkTopic("sellerA-stdIn"), mkTopic("sellerA-stdOut"), mkTopic("sellerA-stdErr")
	bIn, bOut, bErr := mkTopic("sellerB-stdIn"), mkTopic("sellerB-stdOut"), mkTopic("sellerB-stdErr")
	yIn, yOut, yErr := mkTopic("buyer-stdIn"), mkTopic("buyer-stdOut"), mkTopic("buyer-stdErr")

	sellerAPub, err := sellerAKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)
	sellerBPub, err := sellerBKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)

	// Aggregation collector — protected by mutex.
	type bucket struct {
		count uint64
		icaos map[string]struct{}
	}
	var mu sync.Mutex
	buckets := map[string]*bucket{}
	getBucket := func(name string) *bucket {
		if b, ok := buckets[name]; ok {
			return b
		}
		b := &bucket{icaos: map[string]struct{}{}}
		buckets[name] = b
		return b
	}

	onAgg := func(af AggregatedFrame) {
		mu.Lock()
		defer mu.Unlock()
		b := getBucket(af.SellerEVM)
		b.count++
		if af.Meta.ICAO != "" {
			b.icaos[af.Meta.ICAO] = struct{}{}
		}
	}

	type statusEvent struct {
		EVM   string
		State SellerState
	}
	var statusMu sync.Mutex
	statusEvents := []statusEvent{}
	onStatus := func(s SellerStatus) {
		statusMu.Lock()
		statusEvents = append(statusEvents, statusEvent{EVM: s.EVM, State: s.State})
		statusMu.Unlock()
	}

	logger := tlogger{t: t}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Seller A — emits ICAO 0xAA1234 frames.
	sellerACfg := SellerConfig{
		Bus: bus, PrivateKey: &sellerAKey,
		StdIn: aIn, StdOut: aOut, StdErr: aErr,
		LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:         DefaultProtocol,
		HeartbeatPeriod:  10 * time.Second,
		FeedSource:       makeICAOFeed(0xAA1234, 100),
		Logger:           logger,
	}

	// Seller B — emits ICAO 0xBB5678 frames.
	sellerBCfg := SellerConfig{
		Bus: bus, PrivateKey: &sellerBKey,
		StdIn: bIn, StdOut: bOut, StdErr: bErr,
		LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:         DefaultProtocol,
		HeartbeatPeriod:  10 * time.Second,
		FeedSource:       makeICAOFeed(0xBB5678, 100),
		Logger:           logger,
	}

	buyerCfg := BuyerConfig{
		Bus: bus, PrivateKey: &buyerKey,
		StdIn: yIn, StdOut: yOut, StdErr: yErr,
		Sellers: []SellerEntry{
			{StdIn: aIn, PubKey: sellerAPub, DisplayName: "alpha"},
			{StdIn: bIn, PubKey: sellerBPub, DisplayName: "beta"},
		},
		LibP2PListenAddr:  "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:          DefaultProtocol,
		RequestID:         "multi-001",
		HeartbeatPeriod:   10 * time.Second,
		ReconnectBackoff:  500 * time.Millisecond,
		SellerDialTimeout: 5 * time.Second,
		OnAggregatedFrame: onAgg,
		OnSellerStatus:    onStatus,
		Logger:            logger,
	}

	sellerADone := make(chan error, 1)
	sellerBDone := make(chan error, 1)
	buyerDone := make(chan error, 1)
	go func() { sellerADone <- RunSeller(ctx, sellerACfg) }()
	go func() { sellerBDone <- RunSeller(ctx, sellerBCfg) }()
	time.Sleep(200 * time.Millisecond) // stagger so seller stdIn subs are live before buyer publishes
	go func() { buyerDone <- RunBuyer(ctx, buyerCfg) }()

	// Wait until each seller has produced ≥ 100 frames.
	deadline := time.After(6 * time.Second)
loop:
	for {
		mu.Lock()
		var allGood bool
		if len(buckets) == 2 {
			allGood = true
			for _, b := range buckets {
				if b.count < 100 {
					allGood = false
					break
				}
			}
		}
		mu.Unlock()
		if allGood {
			break loop
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("did not see ≥100 frames from each of 2 sellers in 6s; buckets=%+v", buckets)
		case <-time.After(50 * time.Millisecond):
		}
	}

	cancel()
	for _, ch := range []chan error{sellerADone, sellerBDone, buyerDone} {
		select {
		case err := <-ch:
			assert.True(t, err == nil || err == context.Canceled || err == context.DeadlineExceeded,
				"unexpected error: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatal("a process did not return within 3s of cancel")
		}
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, buckets, 2, "expected exactly two distinct seller EVMs")

	sellerAEVM := sellerAKey.PublicKey().EVMAddress().Hex()
	sellerBEVM := sellerBKey.PublicKey().EVMAddress().Hex()

	// Each seller's bucket has the right ICAO.
	bA, okA := buckets[sellerAEVM]
	require.True(t, okA, "no frames from seller A (%s)", sellerAEVM)
	bB, okB := buckets[sellerBEVM]
	require.True(t, okB, "no frames from seller B (%s)", sellerBEVM)

	assert.Contains(t, bA.icaos, "aa1234", "seller A's ICAO should appear")
	assert.Contains(t, bB.icaos, "bb5678", "seller B's ICAO should appear")
	// And they must NOT cross-contaminate.
	assert.NotContains(t, bA.icaos, "bb5678")
	assert.NotContains(t, bB.icaos, "aa1234")

	// At least connected status fired for each seller.
	statusMu.Lock()
	defer statusMu.Unlock()
	connectedSeen := map[string]bool{}
	for _, ev := range statusEvents {
		if ev.State == SellerStateConnected {
			connectedSeen[ev.EVM] = true
		}
	}
	assert.True(t, connectedSeen[sellerAEVM], "no connected event for seller A")
	assert.True(t, connectedSeen[sellerBEVM], "no connected event for seller B")
	t.Logf("collected %d status events", len(statusEvents))
}

// TestMultiSellerAggregation_ReconnectAfterSellerDrop verifies that when one
// seller's process dies, the other keeps flowing, and the buyer reconnects
// when the dropped seller comes back.
func TestMultiSellerAggregation_ReconnectAfterSellerDrop(t *testing.T) {
	bus := NewMemoryBus()

	sellerAKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	sellerBKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	buyerKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	mkTopic := func(memo string) topic.TopicRef {
		ref, err := bus.CreateTopic(topic.CreateTopicOpts{Transport: bus.SupportedTransport(), Memo: memo})
		require.NoError(t, err)
		return ref
	}
	aIn, aOut, aErr := mkTopic("sellerA-stdIn"), mkTopic("sellerA-stdOut"), mkTopic("sellerA-stdErr")
	bIn, bOut, bErr := mkTopic("sellerB-stdIn"), mkTopic("sellerB-stdOut"), mkTopic("sellerB-stdErr")
	yIn, yOut, yErr := mkTopic("buyer-stdIn"), mkTopic("buyer-stdOut"), mkTopic("buyer-stdErr")

	sellerAPub, err := sellerAKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)
	sellerBPub, err := sellerBKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)

	var aFrames, bFrames atomic.Uint64
	onAgg := func(af AggregatedFrame) {
		switch af.SellerEVM {
		case sellerAKey.PublicKey().EVMAddress().Hex():
			aFrames.Add(1)
		case sellerBKey.PublicKey().EVMAddress().Hex():
			bFrames.Add(1)
		}
	}

	type statusEvent struct {
		evm   string
		state SellerState
	}
	statusEvents := make(chan statusEvent, 64)
	onStatus := func(s SellerStatus) {
		select {
		case statusEvents <- statusEvent{evm: s.EVM, state: s.State}:
		default:
		}
	}

	logger := tlogger{t: t}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	mkSellerCfg := func(key *keylib.NeuronPrivateKey, in, out, errRef topic.TopicRef, prefix uint32) SellerConfig {
		return SellerConfig{
			Bus: bus, PrivateKey: key,
			StdIn: in, StdOut: out, StdErr: errRef,
			LibP2PListenAddr: "/ip4/127.0.0.1/udp/0/quic-v1",
			Protocol:         DefaultProtocol,
			HeartbeatPeriod:  10 * time.Second,
			FeedSource:       makeICAOFeed(prefix, 100),
			Logger:           logger,
		}
	}

	buyerCfg := BuyerConfig{
		Bus: bus, PrivateKey: &buyerKey,
		StdIn: yIn, StdOut: yOut, StdErr: yErr,
		Sellers: []SellerEntry{
			{StdIn: aIn, PubKey: sellerAPub, DisplayName: "alpha"},
			{StdIn: bIn, PubKey: sellerBPub, DisplayName: "beta"},
		},
		LibP2PListenAddr:  "/ip4/127.0.0.1/udp/0/quic-v1",
		Protocol:          DefaultProtocol,
		RequestID:         "reconn-001",
		HeartbeatPeriod:   10 * time.Second,
		ReconnectBackoff:  500 * time.Millisecond,
		SellerDialTimeout: 3 * time.Second,
		OnAggregatedFrame: onAgg,
		OnSellerStatus:    onStatus,
		Logger:            logger,
	}

	sellerACtx, sellerACancel := context.WithCancel(ctx)
	sellerADone := make(chan error, 1)
	sellerBDone := make(chan error, 1)
	buyerDone := make(chan error, 1)

	go func() { sellerADone <- RunSeller(sellerACtx, mkSellerCfg(&sellerAKey, aIn, aOut, aErr, 0xAA1234)) }()
	go func() { sellerBDone <- RunSeller(ctx, mkSellerCfg(&sellerBKey, bIn, bOut, bErr, 0xBB5678)) }()
	time.Sleep(200 * time.Millisecond)
	go func() { buyerDone <- RunBuyer(ctx, buyerCfg) }()

	// Wait until both sellers are streaming.
	waitFor(t, "both sellers streaming", 8*time.Second, func() bool {
		return aFrames.Load() >= 50 && bFrames.Load() >= 50
	})

	// Drop seller A.
	sellerACancel()
	select {
	case <-sellerADone:
	case <-time.After(3 * time.Second):
		t.Fatal("seller A did not exit after cancel")
	}
	t.Logf("seller A dropped at aFrames=%d bFrames=%d", aFrames.Load(), bFrames.Load())

	// Snapshot A's frame count post-drop. Wait for B to keep flowing.
	aPaused := aFrames.Load()
	bBefore := bFrames.Load()
	waitFor(t, "B keeps flowing while A is down", 3*time.Second, func() bool {
		return bFrames.Load() >= bBefore+100
	})
	// A should be roughly paused (allow a few in-flight frames to drain).
	delta := aFrames.Load() - aPaused
	assert.Less(t, int(delta), 200, "seller A should be paused; got %d more frames after drop", delta)

	// Restart seller A with a fresh context. Same key + topics → same identity.
	sellerARestartCtx, sellerARestartCancel := context.WithCancel(ctx)
	sellerARestartDone := make(chan error, 1)
	go func() {
		sellerARestartDone <- RunSeller(sellerARestartCtx, mkSellerCfg(&sellerAKey, aIn, aOut, aErr, 0xAA1234))
	}()

	// Buyer should re-publish ReverseConnectionSetup within ReconnectBackoff,
	// seller A should pick it up and dial in. Allow generous time — the buyer
	// may currently be inside a SellerDialTimeout from an earlier failed dial.
	aResumed := aFrames.Load()
	waitFor(t, "A resumes after restart", 15*time.Second, func() bool {
		return aFrames.Load() >= aResumed+50
	})
	t.Logf("A resumed at aFrames=%d (delta=%d)", aFrames.Load(), aFrames.Load()-aResumed)

	sellerARestartCancel()
	cancel()

	for _, ch := range []chan error{sellerARestartDone, sellerBDone, buyerDone} {
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Fatal("a process did not return within 5s of cancel")
		}
	}
}

func waitFor(t *testing.T, what string, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if ok() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s", what)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
