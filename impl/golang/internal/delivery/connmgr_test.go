package delivery

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithConnManager_HostBuilds verifies the option compiles cleanly into a
// real libp2p host construction. The libp2p connmgr API is exercised; if a
// future libp2p version changes the signature, this test catches it.
func TestWithConnManager_HostBuilds(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	h, err := NewLibp2pHost(key, "/ip4/127.0.0.1/udp/0/quic-v1",
		WithConnManager(8, 16, 5*time.Second))
	require.NoError(t, err)
	defer h.Close()

	cm := h.ConnManager()
	require.NotNil(t, cm, "host should expose a ConnManager when WithConnManager is set")
}

// TestLibp2pAdapter_ProtectsAndUnprotects exercises the live Protect /
// Unprotect lifecycle: open a delivery channel between two hosts, verify
// the dialer's connmgr has the remote PeerID protected, then disconnect and
// verify the protection is released.
func TestLibp2pAdapter_ProtectsAndUnprotects(t *testing.T) {
	sellerKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Two in-process hosts; the seller dials, the buyer accepts.
	sellerHost, err := NewLibp2pHost(sellerKey, "/ip4/127.0.0.1/udp/0/quic-v1",
		WithConnManager(8, 16, 5*time.Second))
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerHost, err := NewLibp2pHost(buyerKey, "/ip4/127.0.0.1/udp/0/quic-v1",
		WithConnManager(8, 16, 5*time.Second))
	require.NoError(t, err)
	defer buyerHost.Close()

	const proto = protocol.ID("/neuron/test-connmgr/1.0.0")
	sellerAdapter := NewLibp2pAdapter(sellerHost)
	buyerAdapter := NewLibp2pAdapter(buyerHost)

	accepted := make(chan *DeliveryChannel, 1)
	buyerAdapter.HandleIncoming(proto, func(ch *DeliveryChannel) { accepted <- ch })

	maddrs := make([]string, len(buyerHost.Addrs()))
	for i, a := range buyerHost.Addrs() {
		maddrs[i] = a.String()
	}

	channel, err := sellerAdapter.Connect(buyerHost.ID().String(), maddrs, string(proto), nil)
	require.NoError(t, err)

	// First send to wake up the stream handler — libp2p's QUIC requires the
	// dialer to write at least once before the receiver's stream handler
	// fires. Mirrors the buyer-seller-demo pattern.
	_, err = sellerAdapter.Send(channel, []byte{})
	require.NoError(t, err)

	// Wait for the buyer's HandleIncoming to fire.
	var inbound *DeliveryChannel
	select {
	case inbound = <-accepted:
	case <-time.After(10 * time.Second):
		t.Fatal("buyer did not accept incoming stream")
	}

	// Both sides should have protected the remote PeerID.
	buyerPID, _ := peer.Decode(buyerHost.ID().String())
	sellerPID, _ := peer.Decode(sellerHost.ID().String())
	assert.True(t, sellerHost.ConnManager().IsProtected(buyerPID, ActiveStreamProtectTag),
		"seller's connmgr should protect buyer peer after Connect")
	assert.True(t, buyerHost.ConnManager().IsProtected(sellerPID, ActiveStreamProtectTag),
		"buyer's connmgr should protect seller peer after HandleIncoming")

	// Disconnect should release the dialer's protection.
	require.NoError(t, sellerAdapter.Disconnect(channel))
	assert.False(t, sellerHost.ConnManager().IsProtected(buyerPID, ActiveStreamProtectTag),
		"seller's connmgr should unprotect buyer peer after Disconnect")

	// Buyer side: Disconnect releases its own protect.
	require.NoError(t, buyerAdapter.Disconnect(inbound))
	assert.False(t, buyerHost.ConnManager().IsProtected(sellerPID, ActiveStreamProtectTag),
		"buyer's connmgr should unprotect seller peer after Disconnect")
}
