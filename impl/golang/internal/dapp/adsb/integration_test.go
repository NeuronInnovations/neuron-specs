package adsb

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	sbs "github.com/neuron-sdk/neuron-go-sdk/internal/feeds/sbs"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// testWriter pipes log output into t.Logf for easy debugging.
type testWriter struct {
	t      *testing.T
	prefix string
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.t.Logf("[%s] %s", w.prefix, strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// Test helpers — duplicated locally (test packages don't share).

func newTestKey(t *testing.T) *keylib.NeuronPrivateKey {
	t.Helper()
	k, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	return &k
}

func mustECDSAKeys(t *testing.T, key *keylib.NeuronPrivateKey) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := key.ToBlockchainKey()
	require.NoError(t, err)
	pub, err := key.PublicKey().ToBlockchainKey()
	require.NoError(t, err)
	return priv, pub
}

func makeECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return priv
}

// TestPhase2_BasestationFixtureVerticalSlice — seller backed by replay
// source of the SBS-1 fixture, buyer dials and opens
// /jetvision/basestation/1.0.0, asserts that NormalizedTrack frames flow.
// Mirrors remoteid TestPhase2_FixtureVerticalSlice with the ADS-B substitutions.
func TestPhase2_BasestationFixtureVerticalSlice(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	sellerKey := makeECDSAKey(t)
	sellerHost, err := delivery.NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	running, err := Start(ctx, SellerConfig{
		Host: sellerHost,
		Source: BaseStationReplaySource(
			"../../feeds/sbs/testdata/vanilla-jv.sbs",
			sbs.ReplayOptions{FrameInterval: 5 * time.Millisecond, Loop: true},
		),
	})
	require.NoError(t, err)
	defer running.Cancel()

	buyerKey := makeECDSAKey(t)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	require.NoError(t, buyerHost.Connect(ctx, peer.AddrInfo{
		ID:    sellerHost.ID(),
		Addrs: sellerHost.Addrs(),
	}))

	stream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(ProtocolBaseStation))
	require.NoError(t, err)
	defer stream.Close()

	reader := delivery.NewFrameReader(stream)
	deadline := time.Now().Add(3 * time.Second)
	var captured int
	for captured < 4 && time.Now().Before(deadline) {
		_ = stream.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		data, err := reader.ReadFrame()
		if err != nil {
			continue
		}
		var nt NormalizedTrack
		require.NoError(t, json.Unmarshal(data, &nt))
		assert.Equal(t, FrameType, nt.Type)
		assert.Equal(t, FrameVersion, nt.Version)
		assert.Equal(t, SourceAdsb, nt.Source)
		assert.Equal(t, EntityTypeAircraft, nt.EntityType)
		assert.NotEmpty(t, nt.EntityID)
		require.NotNil(t, nt.Position, "fixture rows carry lat/lon → position must be present")
		captured++
	}
	require.GreaterOrEqual(t, captured, 4, "expected >=4 NormalizedTrack frames within 3s")
}

// TestFullCommerceLifecycle drives the full 008 lifecycle for an ADS-B
// seller + buyer over memory adapter + memory escrow. Mirrors remoteid's
// TestFullCommerceLifecycle.
func TestFullCommerceLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()

	SetReceiveTraceLogger(log.New(&testWriter{t: t, prefix: "recv"}, "", 0))
	t.Cleanup(func() { SetReceiveTraceLogger(nil) })

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stdin"})
	require.NoError(t, err)
	sellerStdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stdout"})
	require.NoError(t, err)
	buyerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-buyer-stdin"})
	require.NoError(t, err)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerEcdsaPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerEcdsaPriv, _ := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:   sellerKey,
		ChainID:    296,
		FeedSource: FeedSourceSynthetic,
	})
	require.NoError(t, err)

	running, err := Start(ctx, SellerConfig{
		Host:   sellerHost,
		Source: BaseStationSynthSource(sbs.SynthOptions{Aircraft: 1, Fps: 50}),
	})
	require.NoError(t, err)
	defer running.Cancel()

	type sellerResult struct {
		res SellerSessionResult
		err error
	}
	sellerResultCh := make(chan sellerResult, 1)
	sellerLogger := log.New(&testWriter{t: t, prefix: "seller"}, "", 0)
	go func() {
		res, err := RunSellerSession(ctx, SellerSessionOptions{
			Key:         sellerKey,
			Adapter:     bus,
			SellerStdIn: sellerStdIn,
			Descriptor:  descriptor,
			Host:        sellerHost,
			Escrow:      escrow,
			Mode:        CommerceModeFull,
			Logger:      sellerLogger,
			FrameSummary: func() (uint64, uint64, uint64) {
				return 8, 1, 2
			},
		})
		sellerResultCh <- sellerResult{res, err}
	}()

	buyerLogger := log.New(&testWriter{t: t, prefix: "buyer"}, "", 0)
	buyerOpts := BuyerSessionOptions{
		Key:                  buyerKey,
		EcdsaPriv:            buyerEcdsaPriv,
		Adapter:              bus,
		SellerStdIn:          sellerStdIn,
		BuyerStdIn:           buyerStdIn,
		RequestID:            "adsb-req-int-1",
		ExpectedSellerPeerID: sellerHost.ID().String(),
		Mode:                 CommerceModeFull,
		Escrow:               escrow,
		SellerEVM:            sellerKey.PublicKey().EVMAddress().Hex(),
		Logger:               buyerLogger,
	}

	time.Sleep(50 * time.Millisecond)

	partial, err := RunBuyerSession(ctx, buyerOpts)
	require.NoError(t, err)
	require.NotNil(t, partial.Discovery, "buyer must receive ConnectionSetup")
	require.NotEmpty(t, partial.Discovery.Streams, "FR-A02 streams[] catalog on wire")
	assert.Equal(t, ProtocolBaseStation, partial.Discovery.Streams[0].ProtocolID)

	_ = sellerStdOut
	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()
	require.NoError(t, buyerHost.Connect(ctx, peer.AddrInfo{
		ID:    sellerHost.ID(),
		Addrs: sellerHost.Addrs(),
	}))
	stream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(partial.Discovery.Streams[0].ProtocolID))
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close() })

	reader := delivery.NewFrameReader(stream)
	for i := 0; i < 4; i++ {
		_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))
		data, err := reader.ReadFrame()
		require.NoError(t, err, "buyer should be able to read frame %d", i)
		require.NotEmpty(t, data)
		var nt NormalizedTrack
		require.NoError(t, json.Unmarshal(data, &nt))
		assert.Equal(t, FrameType, nt.Type)
		assert.Equal(t, SourceAdsb, nt.Source)
		assert.Equal(t, EntityTypeAircraft, nt.EntityType)
	}
	_ = stream.Close()

	final, err := FinaliseBuyerSession(ctx, buyerOpts, partial, 3)
	require.NoError(t, err)
	assert.Equal(t, "approved", final.FinalAction)

	select {
	case sr := <-sellerResultCh:
		require.NoError(t, sr.err, "seller session must succeed")
		assert.Equal(t, payment.StateCompleted, sr.res.FinalState,
			"full-commerce lifecycle ends in COMPLETED")
		assert.NotEmpty(t, sr.res.EvidenceHash)
	case <-time.After(5 * time.Second):
		t.Fatal("seller session did not complete within 5s")
	}
}

func TestRunFullCommerceFlow_InvokesOutputCallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	bus := topic.NewMemoryTopicAdapter()
	bus.SetSubscriberBuffer(64)
	escrow := payment.NewMemoryEscrow()
	contract := registry.NewMemoryRegistryContract()

	sellerStdIn, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stdin"})
	require.NoError(t, err)
	sellerStdOut, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stdout"})
	require.NoError(t, err)
	sellerStdErr, err := bus.CreateTopic(topic.CreateTopicOpts{Memo: "adsb-seller-stderr"})
	require.NoError(t, err)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerEcdsaPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerEcdsaPriv, _ := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()
	require.NotEmpty(t, sellerHost.Addrs())

	descriptor, err := BuildServiceDescriptor(DescriptorOptions{
		ChildKey:     sellerKey,
		ChainID:      296,
		FeedSource:   FeedSourceSynthetic,
		CommerceMode: CommerceModeFull,
		TopicConfig: map[string]map[string]any{
			"stdIn":  {"topicId": sellerStdIn.Locator()},
			"stdOut": {"topicId": sellerStdOut.Locator()},
			"stdErr": {"topicId": sellerStdErr.Locator()},
		},
	})
	require.NoError(t, err)

	contract.SetPendingOwner(common.BytesToAddress(sellerKey.PublicKey().EVMAddress().Bytes()))
	registryAddr, err := keylib.EVMAddressFromHex("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28")
	require.NoError(t, err)
	_, err = RegisterSeller(ctx, sellerKey, descriptor, registryAddr, 296, contract)
	require.NoError(t, err)

	running, err := Start(ctx, SellerConfig{
		Host:   sellerHost,
		Source: BaseStationSynthSource(sbs.SynthOptions{Aircraft: 1, Fps: 50}),
	})
	require.NoError(t, err)
	defer running.Cancel()

	type sellerResult struct {
		res SellerSessionResult
		err error
	}
	sellerResultCh := make(chan sellerResult, 1)
	go func() {
		res, err := RunSellerSession(ctx, SellerSessionOptions{
			Key:         sellerKey,
			Adapter:     bus,
			SellerStdIn: sellerStdIn,
			Descriptor:  descriptor,
			Host:        sellerHost,
			Escrow:      escrow,
			Mode:        CommerceModeFull,
			Logger:      log.New(&testWriter{t: t, prefix: "seller"}, "", 0),
			FrameSummary: func() (uint64, uint64, uint64) {
				return 4, 1, 1
			},
		})
		sellerResultCh <- sellerResult{res, err}
	}()

	buyerHost, err := delivery.NewLibp2pHost(buyerEcdsaPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	var callbackCount int
	var callbackPeer peer.ID
	final, err := RunFullCommerceFlow(ctx, FullCommerceFlowOpts{
		Logger:           log.New(&testWriter{t: t, prefix: "buyer"}, "", 0),
		Key:              buyerKey,
		EcdsaPriv:        buyerEcdsaPriv,
		BuyerHost:        buyerHost,
		Adapter:          bus,
		Escrow:           escrow,
		EscrowBinding:    "memory",
		Contract:         contract,
		RegistryAddress:  registryAddr,
		ChainID:          296,
		SellerEVM:        sellerKey.PublicKey().EVMAddress(),
		PricingAmount:    "1",
		FrameLimit:       3,
		SellerMaOverride: sellerHost.Addrs()[0].String() + "/p2p/" + sellerHost.ID().String(),
		AllowMaOverride:  true,
	}, func(track NormalizedTrack, sellerPID peer.ID) error {
		callbackCount++
		callbackPeer = sellerPID
		assert.Equal(t, FrameType, track.Type)
		assert.Equal(t, SourceAdsb, track.Source)
		assert.Equal(t, EntityTypeAircraft, track.EntityType)
		assert.NotEmpty(t, track.EntityID)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "approved", final.FinalAction)
	assert.GreaterOrEqual(t, callbackCount, 3)
	assert.Equal(t, sellerHost.ID(), callbackPeer)

	select {
	case sr := <-sellerResultCh:
		require.NoError(t, sr.err)
		assert.Equal(t, payment.StateCompleted, sr.res.FinalState)
	case <-time.After(5 * time.Second):
		t.Fatal("seller session did not complete")
	}
}
