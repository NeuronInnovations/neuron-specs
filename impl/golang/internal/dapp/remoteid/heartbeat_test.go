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
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Test_FR_R15_HeartbeatFeedSource — observer sees feedSource capability.
func Test_FR_R15_HeartbeatFeedSource(t *testing.T) {
	t.Run("advertises-replay", TestHeartbeatLoop_AdvertisesFeedSourceReplay)
	t.Run("advertises-synthetic", TestHeartbeatLoop_AdvertisesFeedSourceSynthetic)
}

// Test_FR_P58_HeartbeatCommerceMode — observer sees commerceMode capability.
func Test_FR_P58_HeartbeatCommerceMode(t *testing.T) {
	t.Run("advertises-full", TestHeartbeatLoop_AdvertisesCommerceModeFull)
	t.Run("advertises-registration-only", TestHeartbeatLoop_AdvertisesCommerceModeRegistrationOnly)
}

// Test_FR_F_02_HeartbeatProfile — observer sees profile id capability.
func Test_FR_F_02_HeartbeatProfile(t *testing.T) {
	t.Run("advertises-r1-profile", TestHeartbeatLoop_AdvertisesProfileR1)
}

// --- Underlying tests ---

func runHeartbeatLoopAndCaptureOne(t *testing.T, descriptor ServiceDescriptor) health.Capabilities {
	t.Helper()
	return runHeartbeatLoopWithOptsAndCaptureOne(t, func(opts *HeartbeatLoopOptions) {
		opts.Descriptor = descriptor
	})
}

// runHeartbeatLoopWithOptsAndCaptureOne lets tests adjust HeartbeatLoopOptions
// before starting the loop. The helper supplies safe defaults for the
// Stage-3C-required SellerEVM / SellerPeerID derived from the test key so
// existing tests don't need to redeclare them.
func runHeartbeatLoopWithOptsAndCaptureOne(t *testing.T, mutate func(*HeartbeatLoopOptions)) health.Capabilities {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	key := newTestKey(t)
	adapter, stdOutRef, _ := setupMemoryTopic(t, "hb-"+t.Name())

	// Derive test-default identity values from the key. PeerID derivation
	// always succeeds for a freshly-created secp256k1 key.
	peerID, err := key.PublicKey().PeerID()
	require.NoError(t, err)
	sellerEVM := key.PublicKey().EVMAddress().Hex()

	// Subscribe BEFORE the loop publishes the first heartbeat.
	subCh, err := adapter.Subscribe(ctx, stdOutRef, topic.SubscribeOpts{})
	require.NoError(t, err)

	opts := HeartbeatLoopOptions{
		Key:          key,
		StdOutRef:    stdOutRef,
		Adapter:      adapter,
		Interval:     50 * time.Millisecond,
		Logger:       log.New(&bytes.Buffer{}, "", 0),
		SellerEVM:    sellerEVM,
		SellerPeerID: peerID.String(),
	}
	if mutate != nil {
		mutate(&opts)
	}

	loop, err := StartHeartbeatLoop(ctx, opts)
	require.NoError(t, err)

	select {
	case delivery, ok := <-subCh:
		require.True(t, ok, "subscribe channel closed before first heartbeat")
		var payload health.HeartbeatPayload
		require.NoError(t, json.Unmarshal(delivery.Message.Payload(), &payload))
		require.NotNil(t, payload.Capabilities, "Stage 2 heartbeat MUST carry capabilities")
		cancel()
		<-loop.Done
		return *payload.Capabilities
	case <-time.After(time.Second):
		t.Fatal("no heartbeat received within 1s")
		return health.Capabilities{}
	}
}

func TestHeartbeatLoop_AdvertisesFeedSourceReplay(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopAndCaptureOne(t, ServiceDescriptor{
		FeedSource:   FeedSourceReplay,
		CommerceMode: CommerceModeFull,
		ProfileID:    ProfileR1,
	})
	assert.Equal(t, FeedSourceReplay, caps.FeedSource)
}

func TestHeartbeatLoop_AdvertisesFeedSourceSynthetic(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopAndCaptureOne(t, ServiceDescriptor{
		FeedSource:   FeedSourceSynthetic,
		CommerceMode: CommerceModeFull,
		ProfileID:    ProfileR1,
	})
	assert.Equal(t, FeedSourceSynthetic, caps.FeedSource)
}

func TestHeartbeatLoop_AdvertisesCommerceModeFull(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopAndCaptureOne(t, ServiceDescriptor{
		FeedSource:   FeedSourceLive,
		CommerceMode: CommerceModeFull,
		ProfileID:    ProfileR1,
	})
	assert.Equal(t, CommerceModeFull, caps.CommerceMode)
}

func TestHeartbeatLoop_AdvertisesCommerceModeRegistrationOnly(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopAndCaptureOne(t, ServiceDescriptor{
		FeedSource:   FeedSourceLive,
		CommerceMode: CommerceModeRegistrationOnly,
		ProfileID:    ProfileR1,
	})
	assert.Equal(t, CommerceModeRegistrationOnly, caps.CommerceMode)
}

func TestHeartbeatLoop_AdvertisesProfileR1(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopAndCaptureOne(t, ServiceDescriptor{
		FeedSource:   FeedSourceLive,
		CommerceMode: CommerceModeFull,
		ProfileID:    ProfileR1,
	})
	assert.Equal(t, ProfileR1, caps.Profile)
}

func TestHeartbeatLoop_RequiresKey(t *testing.T) {
	t.Parallel()
	_, err := StartHeartbeatLoop(context.Background(), HeartbeatLoopOptions{
		Adapter: topic.NewMemoryTopicAdapter(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Key required")
}

// --- Stage 3C (FR-R21): operational disclosure on the heartbeat wire ---

// Test_FR_R21_HeartbeatOperationalDisclosure groups the Stage 3C
// FR-R21 seller-side coverage so the FR row in CONFORMANCE.md resolves
// to a single wrapper name.
func Test_FR_R21_HeartbeatOperationalDisclosure(t *testing.T) {
	t.Run("advertises-identity", TestHeartbeatLoop_AdvertisesOperationalIdentity)
	t.Run("advertises-backends", TestHeartbeatLoop_AdvertisesBackends)
	t.Run("advertises-degraded-true", TestHeartbeatLoop_AdvertisesDegradedTrue)
	t.Run("requires-seller-evm-and-peer-id", TestHeartbeatLoop_RequiresSellerEVMAndPeerID)
}

func TestHeartbeatLoop_AdvertisesOperationalIdentity(t *testing.T) {
	t.Parallel()
	const sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	caps := runHeartbeatLoopWithOptsAndCaptureOne(t, func(o *HeartbeatLoopOptions) {
		o.Descriptor = ServiceDescriptor{
			FeedSource:   FeedSourceLive,
			CommerceMode: CommerceModeFull,
			ProfileID:    ProfileR1,
		}
		o.AgentURISha256 = sha
	})
	require.NotNil(t, caps.Operational, "Stage 3C: every heartbeat MUST carry Operational")
	op := caps.Operational
	assert.Equal(t, CommerceServiceName, op.ServiceName, "default ServiceName must be remote-id (FR-R21)")
	assert.NotEmpty(t, op.SellerEVM, "SellerEVM threaded through by test helper")
	assert.NotEmpty(t, op.SellerPeerID, "SellerPeerID threaded through by test helper")
	assert.Equal(t, sha, op.AgentURISha256)
}

func TestHeartbeatLoop_AdvertisesBackends(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopWithOptsAndCaptureOne(t, func(o *HeartbeatLoopOptions) {
		o.Descriptor = ServiceDescriptor{
			FeedSource:   FeedSourceLive,
			CommerceMode: CommerceModeFull,
			ProfileID:    ProfileR1,
		}
		o.TopicBackend = "memory"
		o.EscrowBackend = "memory"
	})
	require.NotNil(t, caps.Operational)
	assert.Equal(t, "memory", caps.Operational.TopicBackend)
	assert.Equal(t, "memory", caps.Operational.EscrowBackend)
}

func TestHeartbeatLoop_AdvertisesDegradedTrue(t *testing.T) {
	t.Parallel()
	caps := runHeartbeatLoopWithOptsAndCaptureOne(t, func(o *HeartbeatLoopOptions) {
		o.Descriptor = ServiceDescriptor{
			FeedSource:   FeedSourceLive,
			CommerceMode: CommerceModeFull,
			ProfileID:    ProfileR1,
		}
		o.DegradedFunc = func() bool { return true }
	})
	require.NotNil(t, caps.Operational)
	assert.True(t, caps.Operational.Degraded, "DegradedFunc=true must surface on the wire")
}

func TestHeartbeatLoop_RequiresSellerEVMAndPeerID(t *testing.T) {
	t.Parallel()
	key := newTestKey(t)
	adapter, stdOutRef, _ := setupMemoryTopic(t, "hb-"+t.Name())

	// Missing SellerEVM → error.
	_, err := StartHeartbeatLoop(context.Background(), HeartbeatLoopOptions{
		Key:          key,
		StdOutRef:    stdOutRef,
		Adapter:      adapter,
		Interval:     50 * time.Millisecond,
		SellerPeerID: "12D3KooWExample",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SellerEVM required")

	// Missing SellerPeerID → error.
	_, err = StartHeartbeatLoop(context.Background(), HeartbeatLoopOptions{
		Key:       key,
		StdOutRef: stdOutRef,
		Adapter:   adapter,
		Interval:  50 * time.Millisecond,
		SellerEVM: "0xABCDEF",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SellerPeerID required")
}

// TestHeartbeatLoop_DegradedFlapsPerTick confirms DegradedFunc is
// re-evaluated per tick (not cached at StartHeartbeatLoop time). Captures
// two consecutive heartbeats and asserts alternating degraded values.
func TestHeartbeatLoop_DegradedFlapsPerTick(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	key := newTestKey(t)
	adapter, stdOutRef, _ := setupMemoryTopic(t, "hb-flap-"+t.Name())
	peerID, err := key.PublicKey().PeerID()
	require.NoError(t, err)

	subCh, err := adapter.Subscribe(ctx, stdOutRef, topic.SubscribeOpts{})
	require.NoError(t, err)

	var tick uint64
	loop, err := StartHeartbeatLoop(ctx, HeartbeatLoopOptions{
		Key:          key,
		StdOutRef:    stdOutRef,
		Adapter:      adapter,
		Descriptor:   ServiceDescriptor{FeedSource: FeedSourceLive, CommerceMode: CommerceModeFull, ProfileID: ProfileR1},
		Interval:     50 * time.Millisecond,
		Logger:       log.New(&bytes.Buffer{}, "", 0),
		SellerEVM:    key.PublicKey().EVMAddress().Hex(),
		SellerPeerID: peerID.String(),
		// Alternate: first call returns 1 → odd → false; second call returns 2 → even → true.
		DegradedFunc: func() bool {
			return (atomic.AddUint64(&tick, 1) % 2) == 0
		},
	})
	require.NoError(t, err)
	defer func() {
		cancel()
		<-loop.Done
	}()

	collected := make([]bool, 0, 2)
	for len(collected) < 2 {
		select {
		case delivery, ok := <-subCh:
			require.True(t, ok)
			var payload health.HeartbeatPayload
			require.NoError(t, json.Unmarshal(delivery.Message.Payload(), &payload))
			require.NotNil(t, payload.Capabilities)
			require.NotNil(t, payload.Capabilities.Operational)
			collected = append(collected, payload.Capabilities.Operational.Degraded)
		case <-time.After(2 * time.Second):
			t.Fatalf("only collected %d/2 heartbeats", len(collected))
		}
	}
	assert.NotEqual(t, collected[0], collected[1],
		"two consecutive heartbeats with alternating DegradedFunc must produce alternating degraded values")
}

