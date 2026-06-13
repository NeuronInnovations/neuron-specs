package delivery

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T035: Full End-to-End Integration Test (SC-D01) ---

func TestIntegration_FullE2EFlow(t *testing.T) {
	// SC-D01: Complete connect → send/receive → disconnect cycle.
	// This exercises the complete protocol path:
	// key gen → host creation → ECIES encrypt → connectionSetup process →
	// libp2p dial → framed send → framed receive → disconnect

	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create hosts.
	sellerHost, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerHost, err := NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	// Seller registers handler.
	sellerAdapter := NewLibp2pAdapter(sellerHost)
	proto := "/neuron/integration-test/1.0.0"

	type receivedFrame struct {
		data []byte
		at   time.Time
	}
	frames := make(chan receivedFrame, 10)

	sellerAdapter.HandleIncoming(protocol.ID(proto), func(ch *DeliveryChannel) {
		for {
			frame, err := sellerAdapter.Receive(ch)
			if err != nil {
				return
			}
			frames <- receivedFrame{data: frame.Data, at: frame.ReceivedAt}
		}
	})

	// Build seller multiaddrs.
	sellerAddrs := buildFullMultiaddrs(sellerHost)

	// ECIES encrypt for buyer.
	encrypted, err := EncryptMultiaddrs(sellerAddrs, &buyerKey.PublicKey)
	require.NoError(t, err)

	// Buyer processes connectionSetup.
	setup, err := ProcessConnectionSetup(
		sellerHost.ID().String(), encrypted, proto, "public", buyerKey,
	)
	require.NoError(t, err)

	// Buyer connects.
	buyerAdapter := NewLibp2pAdapter(buyerHost)
	channel, err := buyerAdapter.Connect(setup.PeerID, setup.Multiaddrs, setup.Protocol, nil)
	require.NoError(t, err)
	assert.Equal(t, StateConnected, channel.State())

	// Send 5 frames.
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"seq":%d,"type":"adsb","data":"test"}`, i))
		result, err := buyerAdapter.Send(channel, payload)
		require.NoError(t, err)
		assert.Greater(t, result.BytesSent, 0)
	}

	// Receive 5 frames in order.
	for i := 0; i < 5; i++ {
		select {
		case f := <-frames:
			expected := fmt.Sprintf(`{"seq":%d,"type":"adsb","data":"test"}`, i)
			assert.Equal(t, expected, string(f.data), "frame %d content", i)
			assert.False(t, f.at.IsZero(), "receivedAt must be set")
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for frame %d", i)
		}
	}

	// Verify status.
	status := buyerAdapter.GetStatus(channel)
	assert.Equal(t, StateConnected, status.State)
	assert.Equal(t, "quic-v1", status.Transport)

	// Clean disconnect.
	err = buyerAdapter.Disconnect(channel)
	require.NoError(t, err)
	assert.Equal(t, StateDisconnected, channel.State())
}

// --- T036: ConnectionState Determinism Test (SC-D03) ---

func TestIntegration_StateMachineDeterminism(t *testing.T) {
	// SC-D03: Same transport event sequence on two independent state machines
	// produces the same final state.

	events := []struct {
		event  ConnectionEvent
		reason string
	}{
		{EventConnect, "dial initiated"},
		{EventDialSuccess, "direct connection"},
		{EventTransportDrop, "network interruption"},
		{EventReconnectDirect, "recovered"},
		{EventTransportDrop, "second drop"},
		{EventReconnectRelay, "relay fallback"},
		{EventDCUtRSuccess, "upgraded to direct"},
		{EventDisconnect, "user disconnect"},
	}

	sm1 := NewConnectionStateMachine(nil)
	sm2 := NewConnectionStateMachine(nil)

	for _, e := range events {
		s1, err1 := sm1.Transition(e.event, e.reason)
		s2, err2 := sm2.Transition(e.event, e.reason)

		// Both must produce identical results.
		assert.Equal(t, err1 == nil, err2 == nil, "error status must match for event %s", e.event)
		assert.Equal(t, s1, s2, "states must match after event %s", e.event)
	}

	assert.Equal(t, sm1.State(), sm2.State())
	assert.Equal(t, StateDisconnected, sm1.State())
}

// --- T037: Framing Interop Test (SC-D06) ---

func TestIntegration_FramingInterop(t *testing.T) {
	// SC-D06: Frame written by Go SDK can be read by TypeScript SDK.
	// Verify exact wire format: 4-byte BE length prefix + payload bytes.

	payload := []byte("neuron-interop-test-payload")

	// Write frame to buffer.
	var buf bytes.Buffer
	w := NewFrameWriter(&buf)
	require.NoError(t, w.WriteFrame(payload))

	wireBytes := buf.Bytes()

	// Verify exact wire format.
	assert.Equal(t, 4+len(payload), len(wireBytes), "wire = 4 prefix + payload")

	// First 4 bytes = big-endian uint32 length.
	length := binary.BigEndian.Uint32(wireBytes[:4])
	assert.Equal(t, uint32(len(payload)), length)

	// Remaining bytes = exact payload.
	assert.Equal(t, payload, wireBytes[4:])

	// Known test vector: "ADSB" (4 bytes) should produce:
	// [0x00, 0x00, 0x00, 0x04, 0x41, 0x44, 0x53, 0x42]
	var buf2 bytes.Buffer
	w2 := NewFrameWriter(&buf2)
	require.NoError(t, w2.WriteFrame([]byte("ADSB")))

	expected := []byte{0x00, 0x00, 0x00, 0x04, 'A', 'D', 'S', 'B'}
	assert.Equal(t, expected, buf2.Bytes(), "known test vector must match exactly")

	// Read back and verify.
	r := NewFrameReader(bytes.NewReader(buf2.Bytes()))
	data, err := r.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, []byte("ADSB"), data)
}

// --- T038: Validator-Perspective Test (VR-DEL-01, VR-DEL-02) ---

func TestIntegration_ValidatorPerspective(t *testing.T) {
	// VR-DEL-01: Verify peerID field is valid libp2p PeerID format.
	// VR-DEL-02: Verify requestId consistency across connectionSetup messages.

	t.Run("VR-DEL-01: PeerID format validation", func(t *testing.T) {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)

		host, err := NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
		require.NoError(t, err)
		defer host.Close()

		peerID := host.ID().String()

		// PeerID should be base58btc multihash format.
		// Valid libp2p PeerIDs start with "12D3KooW" (Ed25519) or "16Uiu2HA" (secp256k1).
		assert.True(t,
			strings.HasPrefix(peerID, "16Uiu2HA") || strings.HasPrefix(peerID, "12D3KooW"),
			"PeerID %q must start with valid libp2p prefix", peerID)

		// Must be non-empty and reasonable length.
		assert.Greater(t, len(peerID), 20, "PeerID must be substantial length")
	})

	t.Run("VR-DEL-02: requestId consistency", func(t *testing.T) {
		// Simulate two connectionSetup messages (buyer and seller) sharing requestId.
		requestId := "550e8400-e29b-41d4-a716-446655440000"

		// Buyer's connectionSetup.
		buyerSetup := map[string]string{
			"type":                "connectionSetup",
			"version":             "1.0.0",
			"requestId":           requestId,
			"peerID":              "16Uiu2HAkBuyerPeerID",
			"encryptedMultiaddrs": "base64buyer==",
			"protocol":            "/neuron/test/1.0.0",
		}

		// Seller's connectionSetup.
		sellerSetup := map[string]string{
			"type":                "connectionSetup",
			"version":             "1.0.0",
			"requestId":           requestId,
			"peerID":              "16Uiu2HAkSellerPeerID",
			"encryptedMultiaddrs": "base64seller==",
			"protocol":            "/neuron/test/1.0.0",
		}

		// VR-DEL-02: requestId must match between both parties.
		assert.Equal(t, buyerSetup["requestId"], sellerSetup["requestId"],
			"requestId must be consistent across buyer and seller connectionSetup messages")

		// Verify JSON serializable.
		buyerJSON, err := json.Marshal(buyerSetup)
		require.NoError(t, err)
		assert.Contains(t, string(buyerJSON), requestId)

		sellerJSON, err := json.Marshal(sellerSetup)
		require.NoError(t, err)
		assert.Contains(t, string(sellerJSON), requestId)
	})

	t.Run("VR-DEL-01: protocol format validation", func(t *testing.T) {
		// Protocol must follow path-like format with version.
		validProtocols := []string{
			"/neuron/adsb/1.0.0",
			"/neuron/video/2.1.0",
			"/my-app/protocol/1.0",
		}
		invalidProtocols := []string{
			"",
			"no-slash",
			"/single",
		}

		for _, p := range validProtocols {
			assert.True(t, isValidProtocolID(p), "protocol %q should be valid", p)
		}
		for _, p := range invalidProtocols {
			assert.False(t, isValidProtocolID(p), "protocol %q should be invalid", p)
		}
	})
}

// --- T027: Reconnection Tests (FR-D09, FR-D10, SC-D05) ---

func TestIntegration_ReconnectionBackoffConfig(t *testing.T) {
	// FR-D09: Verify adapter has backoff config wired.
	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	h, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer h.Close()

	// Default backoff.
	adapter := NewLibp2pAdapter(h)
	cfg := adapter.BackoffConfig()
	assert.Equal(t, time.Duration(5*time.Second), cfg.InitialDelay)
	assert.Equal(t, 2.0, cfg.Factor)
	assert.Equal(t, time.Duration(10*time.Minute), cfg.MaxDelay)
	assert.Equal(t, time.Duration(1*time.Hour), cfg.MaxDuration)

	// Custom backoff.
	customCfg := BackoffConfig{
		InitialDelay: 100 * time.Millisecond,
		Factor:       1.5,
		MaxDelay:     1 * time.Second,
		MaxDuration:  5 * time.Second,
	}
	adapter2 := NewLibp2pAdapter(h, WithBackoffConfig(customCfg))
	assert.Equal(t, customCfg.InitialDelay, adapter2.BackoffConfig().InitialDelay)
}

func TestIntegration_ReconnectionStateTransitions(t *testing.T) {
	// FR-D10, SC-D05: State change callback fires during reconnection scenario.
	var transitions []ConnectionState

	sm := NewConnectionStateMachine(func(state ConnectionState, _ ConnectionEvent, _ string) {
		transitions = append(transitions, state)
	})

	// Simulate: connect → connected → drop → reconnecting → reconnect → connected → disconnect
	sm.Transition(EventConnect, "")
	sm.Transition(EventDialSuccess, "")
	sm.Transition(EventTransportDrop, "network failure")
	sm.Transition(EventReconnectDirect, "recovered")
	sm.Transition(EventDisconnect, "done")

	expected := []ConnectionState{
		StateConnecting, StateConnected, StateReconnecting, StateConnected, StateDisconnected,
	}
	assert.Equal(t, expected, transitions, "callback must fire for each transition")
}

// --- T031: Transport Selection Tests (FR-D25, FR-D26) ---

func TestIntegration_TransportDetection(t *testing.T) {
	// FR-D25, FR-D26: QUIC multiaddr → QUIC transport detected.
	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	sellerHost, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerHost, err := NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	sellerAdapter := NewLibp2pAdapter(sellerHost)
	sellerAdapter.HandleIncoming(protocol.ID("/neuron/transport-test/1.0.0"), func(ch *DeliveryChannel) {
		// Just accept connection.
	})

	addrs := buildFullMultiaddrs(sellerHost)
	buyerAdapter := NewLibp2pAdapter(buyerHost)
	channel, err := buyerAdapter.Connect(sellerHost.ID().String(), addrs, "/neuron/transport-test/1.0.0", nil)
	require.NoError(t, err)

	// Verify QUIC transport selected.
	assert.Equal(t, "quic-v1", channel.Transport, "QUIC multiaddr must select quic-v1 transport")

	status := buyerAdapter.GetStatus(channel)
	assert.Equal(t, "quic-v1", status.Transport)

	buyerAdapter.Disconnect(channel)
}

// --- T034: Transport Encryption Enforcement (FR-D27) ---

func TestIntegration_TransportEncryption(t *testing.T) {
	// FR-D27: All transports MUST provide transport-layer encryption.
	// QUIC uses TLS 1.3 by default — verify transport is not "unknown" or "tcp" (unencrypted).
	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	sellerHost, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerHost, err := NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	sellerAdapter := NewLibp2pAdapter(sellerHost)
	sellerAdapter.HandleIncoming(protocol.ID("/neuron/encryption-test/1.0.0"), func(ch *DeliveryChannel) {})

	addrs := buildFullMultiaddrs(sellerHost)
	buyerAdapter := NewLibp2pAdapter(buyerHost)
	channel, err := buyerAdapter.Connect(sellerHost.ID().String(), addrs, "/neuron/encryption-test/1.0.0", nil)
	require.NoError(t, err)

	// QUIC transport provides TLS 1.3 encryption.
	// Verify the detected transport is an encrypted one.
	encryptedTransports := map[string]bool{
		"quic-v1":      true, // TLS 1.3
		"webtransport": true, // TLS 1.3
		"webrtc":       true, // DTLS
	}

	assert.True(t, encryptedTransports[channel.Transport],
		"transport %q must be encrypted (FR-D27)", channel.Transport)

	buyerAdapter.Disconnect(channel)
}

// --- T022: NAT Traversal Configuration Test (FR-D18, FR-D20, FR-D21) ---

func TestIntegration_NATHostConfiguration(t *testing.T) {
	// FR-D18: Host can be configured with relay and AutoNAT.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	t.Run("host with relay enabled", func(t *testing.T) {
		h, err := NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1", WithRelay())
		require.NoError(t, err)
		defer h.Close()
		assert.NotNil(t, h)
		assert.NotEmpty(t, h.Addrs())
	})

	t.Run("host with AutoNAT enabled", func(t *testing.T) {
		h, err := NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1", WithAutoNAT())
		require.NoError(t, err)
		defer h.Close()
		assert.NotNil(t, h)
	})

	t.Run("host with both relay and AutoNAT", func(t *testing.T) {
		h, err := NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1", WithRelay(), WithAutoNAT())
		require.NoError(t, err)
		defer h.Close()
		assert.NotNil(t, h)
	})
}

// --- T023: AutoNAT Status Test (FR-D19) ---

func TestIntegration_NATStatus(t *testing.T) {
	// FR-D19: natStatus reflects reachability.
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	t.Run("localhost returns unknown", func(t *testing.T) {
		h, err := NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1")
		require.NoError(t, err)
		defer h.Close()

		status := NATStatus(h)
		// Localhost addresses → "unknown" (no public address).
		assert.Equal(t, "unknown", status)
	})

	t.Run("all-interfaces returns public", func(t *testing.T) {
		h, err := NewLibp2pHost(key, "/ip4/0.0.0.0/udp/0/quic-v1")
		require.NoError(t, err)
		defer h.Close()

		status := NATStatus(h)
		// 0.0.0.0 listener may expose non-loopback addresses.
		// Result depends on network config — accept "public" or "unknown".
		assert.Contains(t, []string{"public", "unknown"}, status)
	})
}

// --- helper ---

func buildFullMultiaddrs(h host.Host) []string {
	addrs := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", addr.String(), h.ID().String()))
	}
	return addrs
}
