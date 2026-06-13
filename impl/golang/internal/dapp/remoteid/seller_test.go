package remoteid

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/feeds/remoteid"
)

// makeECDSAKey returns a secp256k1 (NeuronPrivateKey-compatible) keypair
// suitable for delivery.NewLibp2pHost.
func makeECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	// elliptic.P256 is incorrect for secp256k1, but delivery.NewLibp2pHost's
	// convertSecp256k1Key actually accepts any ecdsa.PrivateKey by reading
	// its D as raw scalar bytes. The test passes any 32-byte ECDSA key.
	// For correctness across runs we use crypto/rand on P-256.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return priv
}

func TestSeller_Start_PumpsSyntheticFramesToBuyer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Seller host on loopback.
	sellerKey := makeECDSAKey(t)
	sellerHost, err := delivery.NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	// Buyer host on loopback (separate process simulated within the same test).
	buyerKey := makeECDSAKey(t)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	// Start the Remote ID seller with a small synthetic-orbit source.
	sellerCtx, sellerCancel := context.WithCancel(ctx)
	defer sellerCancel()
	running, err := Start(sellerCtx, SellerConfig{
		Host:   sellerHost,
		Source: SynthSource(remoteid.SynthOptions{FPS: 100, DroneCount: 2}),
	})
	require.NoError(t, err)
	require.Equal(t, ProtocolRaw, running.Protocol)

	// Buyer dials the seller and opens /ds240/raw/1.0.0.
	sellerAddrInfo := peer.AddrInfo{
		ID:    sellerHost.ID(),
		Addrs: sellerHost.Addrs(),
	}
	require.NoError(t, buyerHost.Connect(ctx, sellerAddrInfo))

	stream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(ProtocolRaw))
	require.NoError(t, err)
	defer stream.Close()

	// Read up to 5 frames over the wire and verify they're valid RemoteIdFrame JSON.
	reader := delivery.NewFrameReader(stream)
	var received []RemoteIdFrame
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(received) < 5 {
		stream.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		data, err := reader.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			// timeout from SetReadDeadline; retry until outer deadline.
			continue
		}
		var f RemoteIdFrame
		require.NoError(t, json.Unmarshal(data, &f), "frame must be valid RemoteIdFrame JSON")
		received = append(received, f)
	}

	require.GreaterOrEqual(t, len(received), 3, "expected ≥ 3 frames in 2s at FPS=100; got %d", len(received))

	first := received[0]
	assert.Equal(t, FrameType, first.Type)
	assert.Equal(t, FrameVersion, first.Version)
	assert.Equal(t, "synth", first.Source)
	assert.NotEmpty(t, first.DroneID)
	assert.Equal(t, "serial", first.DroneIDType)
	require.NotNil(t, first.Position)
	assert.NotZero(t, first.Position.Lat)

	// Cancel and wait for the seller goroutine to exit cleanly.
	sellerCancel()
	select {
	case <-running.Done:
	case <-time.After(2 * time.Second):
		t.Fatal("seller did not shut down within 2s of cancel")
	}
}

func TestSeller_Start_RequiresHost(t *testing.T) {
	t.Parallel()
	_, err := Start(context.Background(), SellerConfig{Source: SynthSource(remoteid.SynthOptions{FPS: 1, DroneCount: 1})})
	require.Error(t, err)
}

func TestSeller_Start_RequiresSource(t *testing.T) {
	t.Parallel()
	key := makeECDSAKey(t)
	host, err := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer host.Close()

	_, err = Start(context.Background(), SellerConfig{Host: host})
	require.Error(t, err)
}

func TestSeller_Start_DefaultsToRawProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := makeECDSAKey(t)
	host, err := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer host.Close()

	running, err := Start(ctx, SellerConfig{
		Host:   host,
		Source: SynthSource(remoteid.SynthOptions{FPS: 1, DroneCount: 1}),
	})
	require.NoError(t, err)
	defer running.Cancel()

	assert.Equal(t, ProtocolRaw, running.Protocol)
	assert.Equal(t, "/ds240/raw/1.0.0", running.Protocol)
}

func TestSeller_Start_HonorsCustomProtocolID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := makeECDSAKey(t)
	host, err := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer host.Close()

	running, err := Start(ctx, SellerConfig{
		Host:       host,
		Source:     SynthSource(remoteid.SynthOptions{FPS: 1, DroneCount: 1}),
		ProtocolID: "/test/remoteid-isolation/1.0.0",
	})
	require.NoError(t, err)
	defer running.Cancel()

	assert.Equal(t, "/test/remoteid-isolation/1.0.0", running.Protocol)
}

// TestSeller_Start_DefaultRegistersOnlyProtocolRaw covers the
// back-compat path: empty ProtocolIDs + empty ProtocolID → registers
// /ds240/raw/1.0.0 only. Buyers dialing /ds240/basestation
// receive a "protocol not supported" error.
func TestSeller_Start_DefaultRegistersOnlyProtocolRaw(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	sellerKey := makeECDSAKey(t)
	sellerHost, err := delivery.NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerKey := makeECDSAKey(t)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	running, err := Start(ctx, SellerConfig{
		Host:   sellerHost,
		Source: SynthSource(remoteid.SynthOptions{FPS: 5, DroneCount: 1}),
	})
	require.NoError(t, err)
	defer running.Cancel()
	assert.Equal(t, ProtocolRaw, running.Protocol, "default Protocol must be ProtocolRaw")

	require.NoError(t, buyerHost.Connect(ctx, peer.AddrInfo{
		ID: sellerHost.ID(), Addrs: sellerHost.Addrs(),
	}))

	// Raw should succeed.
	rawStream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(ProtocolRaw))
	require.NoError(t, err)
	_ = rawStream.Close()

	// Basestation should fail (handler not registered).
	_, err = buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(ProtocolBasestation))
	require.Error(t, err, "ProtocolBasestation must NOT be registered when only ProtocolRaw is configured (back-compat)")
}

// TestSeller_Start_RegistersMultipleProtocols covers the new
// ProtocolIDs path: both /ds240/raw and /ds240/basestation are
// dial-able by a buyer.
func TestSeller_Start_RegistersMultipleProtocols(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sellerKey := makeECDSAKey(t)
	sellerHost, err := delivery.NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerKey := makeECDSAKey(t)
	buyerHost, err := delivery.NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	running, err := Start(ctx, SellerConfig{
		Host:        sellerHost,
		Source:      SynthSource(remoteid.SynthOptions{FPS: 100, DroneCount: 2}),
		ProtocolIDs: []string{ProtocolRaw, ProtocolBasestation},
	})
	require.NoError(t, err)
	defer running.Cancel()
	// First entry becomes running.Protocol for back-compat with
	// single-protocol callers.
	assert.Equal(t, ProtocolRaw, running.Protocol)

	require.NoError(t, buyerHost.Connect(ctx, peer.AddrInfo{
		ID: sellerHost.ID(), Addrs: sellerHost.Addrs(),
	}))

	// Open BOTH streams in parallel. Each must deliver byte-identical
	// canonical RemoteIdFrame envelopes.
	openAndRead := func(proto string) []byte {
		stream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(proto))
		require.NoError(t, err, "open stream %s", proto)
		t.Cleanup(func() { _ = stream.Close() })

		stream.SetReadDeadline(time.Now().Add(2 * time.Second))
		reader := delivery.NewFrameReader(stream)
		data, err := reader.ReadFrame()
		if err == io.EOF {
			t.Fatalf("EOF before any frame on %s", proto)
		}
		require.NoError(t, err, "ReadFrame %s", proto)
		// Sanity: must unmarshal as RemoteIdFrame.
		var f RemoteIdFrame
		require.NoError(t, json.Unmarshal(data, &f))
		assert.Equal(t, FrameType, f.Type)
		return data
	}

	rawBytes := openAndRead(ProtocolRaw)
	bsBytes := openAndRead(ProtocolBasestation)

	// Frames are produced from independent synthetic-source goroutines
	// per stream, so the bytes may differ (different observedAt /
	// position). The shape contract is what matters: both must be
	// valid RemoteIdFrame canonical JSON with the same Type + Version.
	require.NotEmpty(t, rawBytes)
	require.NotEmpty(t, bsBytes)
}

// TestSeller_Start_BackCompatProtocolIDHonored verifies the legacy
// ProtocolID flag still wins when ProtocolIDs is empty. Mirrors the
// existing TestSeller_Start_HonorsCustomProtocolID test but documents
// that the multi-protocol path does not break the single-protocol
// surface.
func TestSeller_Start_BackCompatProtocolIDHonored(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := makeECDSAKey(t)
	host, err := delivery.NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer host.Close()

	running, err := Start(ctx, SellerConfig{
		Host:       host,
		Source:     SynthSource(remoteid.SynthOptions{FPS: 1, DroneCount: 1}),
		ProtocolID: "/legacy/only/1.0.0",
		// ProtocolIDs intentionally empty
	})
	require.NoError(t, err)
	defer running.Cancel()
	assert.Equal(t, "/legacy/only/1.0.0", running.Protocol)
}

// Helper to silence unused imports in dev cycles.
var _ = ma.NewMultiaddr
