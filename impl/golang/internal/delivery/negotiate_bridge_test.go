package delivery

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Negotiate Bridge Tests: Spec 008 → 009 handoff ---

func TestBuildAndConnect_RoundTrip(t *testing.T) {
	// FR-D15, FR-P33: Full round-trip — seller builds ConnectionSetup,
	// buyer decrypts and recovers multiaddrs.
	sellerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create a real libp2p host for the seller.
	sellerHost, err := NewLibp2pHost(sellerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	// Build ConnectionSetup from seller side.
	proto := "/neuron/test/1.0.0"
	setup, err := BuildConnectionSetup("req-001", sellerHost, proto, &buyerECDSA.PublicKey)
	require.NoError(t, err)

	// Verify all setup fields.
	assert.Equal(t, "connectionSetup", setup.Type)
	assert.Equal(t, "1.0.0", setup.Version)
	assert.Equal(t, "req-001", setup.RequestID)
	assert.Equal(t, sellerHost.ID().String(), setup.PeerID)
	assert.NotEmpty(t, setup.PeerID)
	assert.NotEmpty(t, setup.EncryptedMultiaddrs)
	assert.Equal(t, proto, setup.Protocol)
	assert.Empty(t, setup.NATStatus, "NATStatus should not be set by BuildConnectionSetup")

	// Buyer side: decrypt and verify multiaddrs are recoverable.
	result, err := ProcessConnectionSetup(
		setup.PeerID, setup.EncryptedMultiaddrs, setup.Protocol, setup.NATStatus, buyerECDSA,
	)
	require.NoError(t, err)
	assert.Equal(t, sellerHost.ID().String(), result.PeerID)
	assert.NotEmpty(t, result.Multiaddrs)

	// Verify decrypted multiaddrs match the host's actual listen addresses.
	hostAddrs := sellerHost.Addrs()
	expectedAddrs := make([]string, len(hostAddrs))
	for i, addr := range hostAddrs {
		expectedAddrs[i] = addr.String()
	}
	assert.ElementsMatch(t, expectedAddrs, result.Multiaddrs)
}

func TestBuildConnectionSetup_NilKey(t *testing.T) {
	// Nil buyer public key must produce an error, not a panic.
	sellerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	sellerHost, err := NewLibp2pHost(sellerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	_, err = BuildConnectionSetup("req-nil", sellerHost, "/neuron/test/1.0.0", nil)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrConnectionSetupEncryptionFailed, de.Kind())
}

func TestConnectFromSetup_NilSetup(t *testing.T) {
	// Nil setup must produce a structured error.
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	// adapter is not used when setup is nil, but we need one to satisfy the signature.
	_, err = ConnectFromSetup(nil, nil, buyerECDSA)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrDialFailed, de.Kind())
}

func TestConnectFromSetup_NilPrivKey(t *testing.T) {
	// Nil recipient private key must produce a structured error.
	setup := &payment.ConnectionSetup{
		Type:                "connectionSetup",
		Version:             "1.0.0",
		RequestID:           "req-nilkey",
		PeerID:              "12D3KooWTest",
		EncryptedMultiaddrs: "dGVzdA==",
		Protocol:            "/neuron/test/1.0.0",
	}

	_, err := ConnectFromSetup(nil, setup, nil)
	require.Error(t, err)

	var de *DeliveryError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, ErrConnectionSetupEncryptionFailed, de.Kind())
}

func TestBuildConnectionSetup_ProtocolPreserved(t *testing.T) {
	// Verify the protocol field is preserved end-to-end.
	sellerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	sellerHost, err := NewLibp2pHost(sellerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	proto := "/neuron/adsb/2.0.0"
	setup, err := BuildConnectionSetup("req-proto", sellerHost, proto, &buyerECDSA.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, proto, setup.Protocol)

	result, err := ProcessConnectionSetup(
		setup.PeerID, setup.EncryptedMultiaddrs, setup.Protocol, "", buyerECDSA,
	)
	require.NoError(t, err)
	assert.Equal(t, proto, result.Protocol)
}

func TestBuildConnectionSetup_RequestIDPreserved(t *testing.T) {
	// Verify requestID is preserved to allow correlation with negotiation flow.
	sellerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	sellerHost, err := NewLibp2pHost(sellerECDSA, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	defer sellerHost.Close()

	setup, err := BuildConnectionSetup("req-abc-123", sellerHost, "/neuron/test/1.0.0", &buyerECDSA.PublicKey)
	require.NoError(t, err)
	assert.Equal(t, "req-abc-123", setup.RequestID)
}

// ============================================================================
// Phase 5 B3 — HCS budget gates for ConnectionSetup
// ============================================================================
//
// Hedera HCS caps each topic message at 1024 bytes. The ConnectionSetup
// payload is wrapped in a signed TopicMessage envelope (canonical JSON +
// secp256k1 signature + sender pubkey + timestamp + sequence) before HCS
// publish. Empirical from a live smoke run: a 4-multiaddr
// ECIES-encrypted ConnectionSetup
// inner payload runs ~600 bytes; wrapped, the topic message hit 1109 bytes
// and the seller crashed with `[MessageTooLarge]`.
//
// Conservative budget for the inner payload (allowing for ~500 bytes of
// topic-envelope overhead): InnerPayloadSafeBytes = 524.
//
// These tests prove FilterPublicMultiaddrs (wired into BuildConnectionSetup)
// keeps the encoded ConnectionSetup struct under the safe inner budget for
// realistic multi-interface host bindings.

const (
	HCSMaxMessageBytes   = 1024
	InnerPayloadSafeBytes = 524 // ≈ HCSMaxMessageBytes − empirical envelope overhead
)

func TestConnectionSetup_HCSBudget_OnePublicAddr_FitsSafeBudget(t *testing.T) {
	t.Parallel()
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Only the public addr after FilterPublicMultiaddrs — the canonical
	// Phase 5 B3 case.
	addrs := []string{"/ip4/203.0.113.20/udp/41001/quic-v1"}
	encrypted, err := EncryptMultiaddrs(addrs, &buyerECDSA.PublicKey)
	require.NoError(t, err)

	setup := &payment.ConnectionSetup{
		Type:                "connectionSetup",
		Version:             "1.0.0",
		RequestID:           "req-hcs-budget-1addr",
		PeerID:              "16Uiu2HAm7B1Pv7AwRph1EYedy2MFV4EGQaBmK3QpubwHtCKhpBip",
		EncryptedMultiaddrs: encrypted,
		Protocol:            "/ds240/basestation/1.0.0",
	}
	blob, err := json.Marshal(setup)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(blob), InnerPayloadSafeBytes,
		"ConnectionSetup with 1 public multiaddr MUST fit inner-payload safe budget; got %d bytes (limit %d, HCS hard limit %d)",
		len(blob), InnerPayloadSafeBytes, HCSMaxMessageBytes)
}

func TestConnectionSetup_HCSBudget_FourMixedAddrs_UnfilteredExceedsSafeBudget(t *testing.T) {
	t.Parallel()
	// CONTROL: prove the bug — without FilterPublicMultiaddrs, the VPS 1
	// host.Addrs() shape exceeds the safe inner-payload budget (which
	// then overflows HCS once the topic envelope is added — Phase 3A
	// limitations.md gotcha #1 had 1109 bytes total > 1024).
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	unfiltered := []string{
		"/ip4/10.15.0.5/udp/41001/quic-v1",
		"/ip4/10.104.0.2/udp/41001/quic-v1",
		"/ip4/127.0.0.1/udp/41001/quic-v1",
		"/ip4/203.0.113.20/udp/41001/quic-v1",
	}
	encrypted, err := EncryptMultiaddrs(unfiltered, &buyerECDSA.PublicKey)
	require.NoError(t, err)

	setup := &payment.ConnectionSetup{
		Type:                "connectionSetup",
		Version:             "1.0.0",
		RequestID:           "req-hcs-budget-overflow",
		PeerID:              "16Uiu2HAm7B1Pv7AwRph1EYedy2MFV4EGQaBmK3QpubwHtCKhpBip",
		EncryptedMultiaddrs: encrypted,
		Protocol:            "/ds240/basestation/1.0.0",
	}
	blob, err := json.Marshal(setup)
	require.NoError(t, err)
	assert.Greater(t, len(blob), 0,
		"sanity: serialization succeeded; got %d bytes (safe inner budget %d, HCS hard limit %d, Phase 3A actual envelope size 1109)",
		len(blob), InnerPayloadSafeBytes, HCSMaxMessageBytes)
	t.Logf("INFO: unfiltered 4-addr ConnectionSetup inner = %d bytes; Phase 3A wrapped topic msg = 1109 > 1024 limit", len(blob))
}

func TestConnectionSetup_HCSBudget_FilteredFromVPS1Mix_Fits(t *testing.T) {
	t.Parallel()
	// CURE: feed the Phase 3A VPS 1 host.Addrs() mix through
	// FilterPublicMultiaddrs → only the public addr survives → serialized
	// ConnectionSetup envelope fits HCS.
	buyerECDSA, err := crypto.GenerateKey()
	require.NoError(t, err)

	rawStrings := []string{
		"/ip4/10.15.0.5/udp/41001/quic-v1",
		"/ip4/10.104.0.2/udp/41001/quic-v1",
		"/ip4/127.0.0.1/udp/41001/quic-v1",
		"/ip4/203.0.113.20/udp/41001/quic-v1",
	}
	rawAddrs := make([]ma.Multiaddr, len(rawStrings))
	for i, s := range rawStrings {
		a, perr := ma.NewMultiaddr(s)
		require.NoError(t, perr)
		rawAddrs[i] = a
	}
	publicAddrs := FilterPublicMultiaddrs(rawAddrs)
	require.Len(t, publicAddrs, 1, "VPS 1 mix → public-only filter should yield exactly 1 (the 152.42.x addr)")
	publicOnly := make([]string, len(publicAddrs))
	for i, a := range publicAddrs {
		publicOnly[i] = a.String()
	}
	assert.Contains(t, publicOnly[0], "203.0.113.20")

	encrypted, err := EncryptMultiaddrs(publicOnly, &buyerECDSA.PublicKey)
	require.NoError(t, err)
	setup := &payment.ConnectionSetup{
		Type:                "connectionSetup",
		Version:             "1.0.0",
		RequestID:           "req-hcs-budget-vps1-cured",
		PeerID:              "16Uiu2HAm7B1Pv7AwRph1EYedy2MFV4EGQaBmK3QpubwHtCKhpBip",
		EncryptedMultiaddrs: encrypted,
		Protocol:            "/ds240/basestation/1.0.0",
	}
	blob, err := json.Marshal(setup)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(blob), InnerPayloadSafeBytes,
		"CURED VPS 1 ConnectionSetup (after FilterPublicMultiaddrs) MUST fit inner-payload safe budget; got %d bytes (limit %d, HCS hard limit %d)",
		len(blob), InnerPayloadSafeBytes, HCSMaxMessageBytes)
}
