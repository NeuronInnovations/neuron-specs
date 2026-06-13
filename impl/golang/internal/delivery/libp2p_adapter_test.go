package delivery

import (
	"crypto/ecdsa"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase A: Localhost Direct-Connect E2E ---

func TestLibp2pAdapter_LocalhostE2E(t *testing.T) {
	// SC-D01: Full connect → send/receive → disconnect cycle.
	// Two in-process libp2p hosts on localhost QUIC.

	// 1. Generate two secp256k1 keys (buyer + seller).
	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// 2. Create two in-process hosts on localhost.
	sellerHost, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerHost, err := NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	t.Logf("[1/8] Seller started: PeerID=%s, Addrs=%v", sellerHost.ID(), sellerHost.Addrs())
	t.Logf("[2/8] Buyer started:  PeerID=%s, Addrs=%v", buyerHost.ID(), buyerHost.Addrs())

	// 3. Seller: register stream handler.
	sellerAdapter := NewLibp2pAdapter(sellerHost)
	protocolID := "/neuron/test/1.0.0"

	received := make(chan []byte, 1)
	sellerAdapter.HandleIncoming("/neuron/test/1.0.0", func(ch *DeliveryChannel) {
		frame, err := sellerAdapter.Receive(ch)
		if err != nil {
			t.Logf("Seller receive error: %v", err)
			return
		}
		received <- frame.Data
	})
	t.Logf("[3/8] Seller registered stream handler for protocol %s", protocolID)

	// 4. Build seller's full multiaddrs (addr + /p2p/peerID).
	sellerAddrs := make([]string, 0, len(sellerHost.Addrs()))
	for _, addr := range sellerHost.Addrs() {
		full := fmt.Sprintf("%s/p2p/%s", addr.String(), sellerHost.ID().String())
		sellerAddrs = append(sellerAddrs, full)
	}

	// 5. Seller encrypts their multiaddrs FOR the buyer (using buyer's public key).
	//    In real flow: seller sends connectionSetup on topic, buyer decrypts.
	//    Here we simulate: encrypt with buyer's pubkey, buyer decrypts with buyer's privkey.
	encrypted, err := EncryptMultiaddrs(sellerAddrs, &buyerKey.PublicKey)
	require.NoError(t, err)
	t.Logf("[4/8] ECIES: Encrypted seller multiaddrs -> %d bytes base64 ciphertext", len(encrypted))

	// 6. Buyer: process connectionSetup (decrypt with buyer's key + validate).
	setup, err := ProcessConnectionSetup(
		sellerHost.ID().String(),
		encrypted,
		protocolID,
		"public",
		buyerKey,
	)
	require.NoError(t, err)
	assert.Equal(t, sellerHost.ID().String(), setup.PeerID)
	assert.NotEmpty(t, setup.Multiaddrs)
	t.Logf("[5/8] Buyer decrypted connectionSetup: %d multiaddr(s), PeerID verified", len(setup.Multiaddrs))

	// 7. Buyer: connect to seller.
	buyerAdapter := NewLibp2pAdapter(buyerHost)
	channel, err := buyerAdapter.Connect(setup.PeerID, setup.Multiaddrs, setup.Protocol, nil)
	require.NoError(t, err)
	assert.Equal(t, StateConnected, channel.State())
	assert.Equal(t, "quic-v1", channel.Transport)
	assert.False(t, channel.Path.Limited, "localhost direct stream must not be relay-limited")
	assert.NotEmpty(t, channel.Path.RemoteMultiaddr)
	assert.False(t, strings.Contains(channel.Path.RemoteMultiaddr, "/p2p-circuit"))
	t.Logf("[6/8] Buyer connected to seller: state=%s, transport=%s", channel.State(), channel.Transport)

	// 8. Buyer: send data frame.
	payload := []byte("hello seller from buyer")
	result, err := buyerAdapter.Send(channel, payload)
	require.NoError(t, err)
	assert.Greater(t, result.BytesSent, 0)
	t.Logf("[7/8] Buyer sent %d bytes: %q", result.BytesSent, string(payload))

	// 9. Seller: receive data frame.
	select {
	case data := <-received:
		assert.Equal(t, payload, data)
		t.Logf("[8/8] Seller received: %q -- exact match", string(data))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for seller to receive data")
	}

	// 10. Clean disconnect.
	err = buyerAdapter.Disconnect(channel)
	require.NoError(t, err)
	assert.Equal(t, StateDisconnected, channel.State())
	t.Logf("[OK] Clean disconnect: state=%s", channel.State())
}

func TestLibp2pAdapter_PeerIDVerification(t *testing.T) {
	// FR-D28, SC-D07: Connect with mismatched PeerID → PeerIDMismatch error.
	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	fakeKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	sellerHost, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerHost, err := NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	// Seller registers handler.
	sellerAdapter := NewLibp2pAdapter(sellerHost)
	sellerAdapter.HandleIncoming("/neuron/test/1.0.0", func(ch *DeliveryChannel) {})

	// Build seller's multiaddrs.
	sellerAddrs := make([]string, 0)
	for _, addr := range sellerHost.Addrs() {
		sellerAddrs = append(sellerAddrs, fmt.Sprintf("%s/p2p/%s", addr.String(), sellerHost.ID()))
	}

	// Generate a fake PeerID from different key.
	fakePeerID, err := peerIDFromECDSA(fakeKey)
	require.NoError(t, err)

	// Buyer tries to connect using fake PeerID but real seller addrs.
	// This should fail because the actual remote peer will be seller, not fake.
	buyerAdapter := NewLibp2pAdapter(buyerHost)
	_, err = buyerAdapter.Connect(fakePeerID, sellerAddrs, "/neuron/test/1.0.0", nil)

	// The connection should fail — either DialFailed (can't find fake peer at seller's addr)
	// or PeerIDMismatch if connection succeeds but IDs don't match.
	require.Error(t, err, "connecting with wrong PeerID must fail")
	t.Logf("Expected error: %v", err)
}

func TestLibp2pAdapter_MultipleFrames(t *testing.T) {
	// SC-D06: Multiple frames delivered in order.
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

	frameCount := 10
	receivedFrames := make(chan []byte, frameCount)
	sellerAdapter.HandleIncoming("/neuron/test/1.0.0", func(ch *DeliveryChannel) {
		for i := 0; i < frameCount; i++ {
			frame, err := sellerAdapter.Receive(ch)
			if err != nil {
				return
			}
			receivedFrames <- frame.Data
		}
	})

	// Build multiaddrs.
	sellerAddrs := make([]string, 0)
	for _, addr := range sellerHost.Addrs() {
		sellerAddrs = append(sellerAddrs, fmt.Sprintf("%s/p2p/%s", addr.String(), sellerHost.ID()))
	}

	buyerAdapter := NewLibp2pAdapter(buyerHost)
	channel, err := buyerAdapter.Connect(sellerHost.ID().String(), sellerAddrs, "/neuron/test/1.0.0", nil)
	require.NoError(t, err)

	// Send 10 frames.
	for i := 0; i < frameCount; i++ {
		data := []byte(fmt.Sprintf("frame-%d", i))
		_, err := buyerAdapter.Send(channel, data)
		require.NoError(t, err)
	}

	// Receive all 10 in order.
	for i := 0; i < frameCount; i++ {
		select {
		case data := <-receivedFrames:
			expected := fmt.Sprintf("frame-%d", i)
			assert.Equal(t, expected, string(data), "frame %d must match", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for frame %d", i)
		}
	}

	buyerAdapter.Disconnect(channel)
}

// --- helper ---

func peerIDFromECDSA(key *ecdsa.PrivateKey) (string, error) {
	libp2pPriv, err := libp2pcrypto.UnmarshalSecp256k1PrivateKey(key.D.Bytes())
	if err != nil {
		return "", err
	}
	pid, err := peer.IDFromPrivateKey(libp2pPriv)
	if err != nil {
		return "", err
	}
	return pid.String(), nil
}
