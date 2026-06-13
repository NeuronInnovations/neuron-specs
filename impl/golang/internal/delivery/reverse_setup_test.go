package delivery

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildReverseAndConnect_RoundTrip exercises the inverse of the forward
// negotiate-bridge round-trip:
//
//   - The BUYER builds a ReverseConnectionSetup containing its own (reachable)
//     multiaddrs encrypted to the seller's pub key.
//   - The SELLER decrypts and recovers the buyer's multiaddrs and PeerID.
//
// We do NOT actually dial — that would require both hosts running. We only
// verify the encrypt/decrypt symmetry and that the addresses round-trip
// through the ECIES path correctly.
func TestBuildReverseAndConnect_RoundTrip(t *testing.T) {
	sellerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Buyer is the dialee — its host is the one with reachable listen addrs.
	buyerHost, err := NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	proto := "/neuron/edge-feed/1.0.0"

	setup, err := BuildReverseConnectionSetup("req-rev-001", buyerHost, proto, &sellerECDSA.PublicKey)
	require.NoError(t, err)

	// Envelope shape — same as forward direction.
	assert.Equal(t, "connectionSetup", setup.Type)
	assert.Equal(t, "1.0.0", setup.Version)
	assert.Equal(t, "req-rev-001", setup.RequestID)
	assert.Equal(t, buyerHost.ID().String(), setup.PeerID, "PeerID should be the dialee's (buyer)")
	assert.NotEmpty(t, setup.EncryptedMultiaddrs)
	assert.Equal(t, proto, setup.Protocol)

	// Seller side: decrypt with seller's private key.
	result, err := ProcessConnectionSetup(
		setup.PeerID, setup.EncryptedMultiaddrs, setup.Protocol, setup.NATStatus, sellerECDSA,
	)
	require.NoError(t, err)
	assert.Equal(t, buyerHost.ID().String(), result.PeerID)
	assert.NotEmpty(t, result.Multiaddrs)

	// Decrypted multiaddrs should match the buyer's host listen addrs.
	want := make([]string, 0, len(buyerHost.Addrs()))
	for _, a := range buyerHost.Addrs() {
		want = append(want, a.String())
	}
	assert.ElementsMatch(t, want, result.Multiaddrs)
}

// TestReverseSetup_WrongDialerCannotDecrypt confirms ECIES isolation: a
// ReverseConnectionSetup encrypted to seller A's key cannot be decrypted by
// seller B (a different ECDSA key).
func TestReverseSetup_WrongDialerCannotDecrypt(t *testing.T) {
	sellerA, err := crypto.GenerateKey()
	require.NoError(t, err)
	sellerB, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	buyerHost, err := NewLibp2pHost(buyerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer buyerHost.Close()

	setup, err := BuildReverseConnectionSetup("req-iso", buyerHost,
		"/neuron/edge-feed/1.0.0", &sellerA.PublicKey)
	require.NoError(t, err)

	// Seller B (wrong key) — must fail to decrypt.
	_, err = ProcessConnectionSetup(
		setup.PeerID, setup.EncryptedMultiaddrs, setup.Protocol, setup.NATStatus, sellerB,
	)
	assert.Error(t, err)
}
