package delivery

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

// TestIntegration_RelayReservation exercises the full relay ladder on loopback:
// a public relay node (EnableRelayService + EnableNATService + ForceReachabilityPublic)
// and two "private" peers (NewLibp2pHost with WithRelay + WithForcedReachability(Private)).
// The test proves relay reservations succeed and that a buyer can reach a seller
// through a manually-constructed /p2p-circuit multiaddr.
//
// Note on autorelay address advertisement: libp2p autorelay deliberately does
// not rewrite host.Addrs() to include /p2p-circuit entries when the relay's
// listen set is loopback-only (see cleanupAddressSet in
// p2p/host/autorelay/addrsplosion.go — it drops anything that isn't
// manet.IsPublicAddr). In production (relay on a public IP) autorelay emits
// the circuit addrs automatically, and the buyer-seller demo consumes them
// through encryptedMultiaddrs as usual. This hermetic test substitutes by
// dialing a manually-constructed /p2p-circuit multiaddr, which exercises the
// same server-side code path and is the tightest hermetic proof available.
//
// Skipped in -short mode; reservation + relayed dial takes a few seconds.
//
// FR-D18, FR-D20: Circuit Relay v2 + autorelay with static relays.
// FR-D21: relayed connection ready for DCUtR upgrade.
func TestIntegration_RelayReservation(t *testing.T) {
	if testing.Short() {
		t.Skip("relay integration test skipped in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// 1. Relay node: public reachability, relay service, AutoNAT v2 server.
	relayPriv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
	require.NoError(t, err)
	relayHost, err := libp2p.New(
		libp2p.Identity(relayPriv),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"),
		libp2p.ForceReachabilityPublic(),
		libp2p.EnableRelayService(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
	)
	require.NoError(t, err)
	defer relayHost.Close()

	relayAddrs := relayHost.Addrs()
	require.NotEmpty(t, relayAddrs, "relay should have at least one listen address")

	// Build the relay multiaddr strings clients will consume.
	relayMultiaddrs := make([]string, 0, len(relayAddrs))
	for _, a := range relayAddrs {
		relayMultiaddrs = append(relayMultiaddrs, a.String()+"/p2p/"+relayHost.ID().String())
	}

	// 2. Two private peers, each configured via our NewLibp2pHost with
	//    WithRelay (autorelay + AutoNAT v2 + hole punching + UPnP) and
	//    WithForcedReachability(Private) so autorelay triggers immediately.
	sellerKey, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	sellerHost, err := NewLibp2pHost(
		sellerKey,
		"/ip4/127.0.0.1/udp/0/quic-v1",
		WithRelay(relayMultiaddrs...),
		WithForcedReachability(network.ReachabilityPrivate),
	)
	require.NoError(t, err)
	defer sellerHost.Close()

	buyerKey, err := ethcrypto.GenerateKey()
	require.NoError(t, err)
	buyerHost, err := NewLibp2pHost(
		buyerKey,
		"/ip4/127.0.0.1/udp/0/quic-v1",
		WithRelay(relayMultiaddrs...),
		WithForcedReachability(network.ReachabilityPrivate),
	)
	require.NoError(t, err)
	defer buyerHost.Close()

	// 3. Seller reserves a relay slot. circuitv2.Reserve performs the full
	//    reservation handshake and returns when the relay has accepted the
	//    reservation. This is the same call autorelay makes internally; using
	//    it directly here skips the addr-advertisement layer that loopback
	//    filtering would block (see test comment above).
	relayInfo := peer.AddrInfo{ID: relayHost.ID(), Addrs: relayAddrs}
	require.NoError(t, sellerHost.Connect(ctx, relayInfo))
	rsvp, err := client.Reserve(ctx, sellerHost, relayInfo)
	require.NoError(t, err, "seller should obtain a relay reservation from the relay")
	require.NotNil(t, rsvp)
	// rsvp.Addrs is intentionally not asserted: the relay strips non-public
	// addresses from the reservation response (see `cleanupAddressSet`), so on
	// loopback it can legitimately be empty even though the reservation is
	// active. The subsequent circuit dial is the real proof.

	// 4. Buyer opens a delivery stream to the seller through the relay via a
	//    manually-constructed /p2p-circuit multiaddr. If this succeeds, the
	//    relay has an active reservation for the seller AND is willing to route
	//    data — the full reservation + circuit routing flow works end to end.
	relayBase := relayAddrs[0].String()
	circuitStr := fmt.Sprintf("%s/p2p/%s/p2p-circuit", relayBase, relayHost.ID())
	circuitAddr, err := ma.NewMultiaddr(circuitStr)
	require.NoError(t, err)
	require.NoError(t, buyerHost.Connect(ctx, relayInfo),
		"buyer should first connect to the relay so the circuit dial can piggyback")

	sellerAdapter := NewLibp2pAdapter(sellerHost)
	sellerAdapter.HandleIncoming(protocol.ID("/neuron/relay-test/1.0.0"), func(ch *DeliveryChannel) {})

	buyerAdapter := NewLibp2pAdapter(buyerHost)
	channel, err := buyerAdapter.Connect(sellerHost.ID().String(), []string{circuitAddr.String()}, "/neuron/relay-test/1.0.0", nil)
	require.NoError(t, err, "buyer should reach seller via the relay-assisted /p2p-circuit multiaddr")
	require.True(t, channel.Path.Limited, "relay-backed stream must be marked limited")
	require.True(t, strings.Contains(channel.Path.RemoteMultiaddr, "/p2p-circuit"),
		"relay-backed stream must expose a /p2p-circuit remote multiaddr")

	// 5. Confirm the buyer has a working connection back to the seller.
	conns := buyerHost.Network().ConnsToPeer(sellerHost.ID())
	require.NotEmpty(t, conns, "buyer should have at least one connection to seller")
	require.NoError(t, buyerAdapter.Disconnect(channel))
}

// TestIntegration_ReachabilityTracker verifies that the tracker consumes
// ForceReachabilityPrivate's event-bus emission. Confirms FR-D19 path.
func TestIntegration_ReachabilityTracker(t *testing.T) {
	if testing.Short() {
		t.Skip("reachability tracker test skipped in short mode")
	}

	key, err := ethcrypto.GenerateKey()
	require.NoError(t, err)

	h, err := NewLibp2pHost(
		key,
		"/ip4/127.0.0.1/udp/0/quic-v1",
		WithForcedReachability(network.ReachabilityPrivate),
		WithAutoNAT(),
	)
	require.NoError(t, err)
	defer h.Close()

	tracker, err := NewReachabilityTracker(h)
	require.NoError(t, err)
	defer tracker.Close()

	// ForceReachabilityPrivate emits on the event bus during host init.
	// Give the tracker goroutine a moment to consume it.
	deadline := time.Now().Add(5 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		status = tracker.Status()
		if status == "private" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Equal(t, "private", status, "tracker should observe forced-private reachability")
}
