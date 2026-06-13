package remoteid

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Test_FR_R02_OnWireStreamsCatalog asserts the streams[] catalog rides
// the wire inside ConnectionSetup (Stage 2 advance over Stage 1's
// descriptor-only field).
func Test_FR_R02_OnWireStreamsCatalog(t *testing.T) {
	t.Run("publishes-streams-on-wire", TestPublishConnectionSetup_StreamsOnWire)
	t.Run("rejects-empty-streams", TestPublishConnectionSetup_RejectsEmptyStreams)
}

// Test_FR_D15_ECIESMultiaddrDecryption asserts the buyer can decrypt the
// seller's multiaddrs using its secp256k1 private key.
func Test_FR_D15_ECIESMultiaddrDecryption(t *testing.T) {
	t.Run("buyer-decrypts-and-resolves-addrinfo", TestReceiveConnectionSetup_DecryptsAndResolvesAddrInfo)
}

// Test_FR_R01_ConnectionSetupPeerIDBinding asserts the registry-derived
// PeerID gates dial: a mismatch surfaces as PeerIDMismatchError and the
// buyer refuses to proceed.
func Test_FR_R01_ConnectionSetupPeerIDBinding(t *testing.T) {
	t.Run("envelope-peerid-mismatch-aborts", TestReceiveConnectionSetup_EnvelopePeerIDMismatchAborts)
	t.Run("sender-pubkey-roundtrip", TestExtractSenderECDSAPublicKey_RoundTrip)
}

// --- Test scaffolding ---

type connSetupRecv struct {
	setup *payment.ConnectionSetup
	info  peer.AddrInfo
	err   error
}

// runConnSetupReceiver starts a goroutine that calls ReceiveConnectionSetup
// and returns a channel for the result. Tests use this to subscribe BEFORE
// publishing (MemoryTopicAdapter only delivers messages published after
// subscribe).
func runConnSetupReceiver(ctx context.Context, adapter topic.TopicAdapter, ref topic.TopicRef,
	requestID, expectedPeerID string, recipientPriv *ecdsa.PrivateKey,
) <-chan connSetupRecv {
	ch := make(chan connSetupRecv, 1)
	go func() {
		setup, info, err := ReceiveConnectionSetup(ctx, adapter, ref, requestID, expectedPeerID, recipientPriv)
		ch <- connSetupRecv{setup: setup, info: info, err: err}
	}()
	return ch
}

func mustECDSAKeys(t *testing.T, key *keylib.NeuronPrivateKey) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := key.ToBlockchainKey()
	require.NoError(t, err)
	pub, err := key.PublicKey().ToBlockchainKey()
	require.NoError(t, err)
	return priv, pub
}

// --- Underlying tests ---

func TestPublishConnectionSetup_StreamsOnWire(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerPriv, buyerPub := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })

	adapter, buyerStdIn, _ := setupMemoryTopic(t, "conn-streams")
	resultCh := runConnSetupReceiver(ctx, adapter, buyerStdIn, "req-streams",
		sellerHost.ID().String(), buyerPriv)
	time.Sleep(20 * time.Millisecond)

	streams := []payment.StreamCatalogEntry{
		{Name: "raw", ProtocolID: ProtocolRaw, Direction: payment.StreamDirectionSeller, Schema: SchemaURL},
	}
	setup, err := PublishConnectionSetup(ctx, adapter, buyerStdIn, sellerKey, 1,
		"req-streams", sellerHost, buyerPub, streams)
	require.NoError(t, err)
	assert.Equal(t, payment.PayloadConnectionSetup, setup.Type)
	require.Len(t, setup.Streams, 1, "streams[] MUST appear on wire (FR-R02)")
	assert.Equal(t, ProtocolRaw, setup.Streams[0].ProtocolID)
	assert.NotEmpty(t, setup.EncryptedMultiaddrs, "FR-D11 ECIES output non-empty")

	r := <-resultCh
	require.NoError(t, r.err)
	require.NotNil(t, r.setup)
	assert.Equal(t, sellerHost.ID().String(), r.setup.PeerID)
	require.Len(t, r.setup.Streams, 1)
	assert.Equal(t, ProtocolRaw, r.setup.Streams[0].ProtocolID)
}

func TestPublishConnectionSetup_RejectsEmptyStreams(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerPriv, _ := mustECDSAKeys(t, sellerKey)
	_, buyerPub := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })

	adapter, ref, _ := setupMemoryTopic(t, "conn-empty-streams")
	_, err = PublishConnectionSetup(ctx, adapter, ref, sellerKey, 1, "req", sellerHost, buyerPub, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "StreamCatalogEntry required")
}

func TestReceiveConnectionSetup_DecryptsAndResolvesAddrInfo(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerPriv, buyerPub := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })

	adapter, buyerStdIn, _ := setupMemoryTopic(t, "conn-decrypt")
	streams := []payment.StreamCatalogEntry{
		{Name: "raw", ProtocolID: ProtocolRaw, Direction: payment.StreamDirectionSeller, Schema: SchemaURL},
	}

	resultCh := runConnSetupReceiver(ctx, adapter, buyerStdIn, "req-dec",
		sellerHost.ID().String(), buyerPriv)
	time.Sleep(20 * time.Millisecond)

	_, err = PublishConnectionSetup(ctx, adapter, buyerStdIn, sellerKey, 1, "req-dec",
		sellerHost, buyerPub, streams)
	require.NoError(t, err)

	r := <-resultCh
	require.NoError(t, r.err)
	require.NotNil(t, r.setup)
	assert.Equal(t, sellerHost.ID().String(), r.setup.PeerID)
	assert.Equal(t, sellerHost.ID(), r.info.ID, "decrypted addrInfo.ID equals seller's PeerID")
	assert.NotEmpty(t, r.info.Addrs, "decrypted multiaddrs non-empty")
}

func TestReceiveConnectionSetup_EnvelopePeerIDMismatchAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	sellerKey := newTestKey(t)
	buyerKey := newTestKey(t)
	sellerPriv, _ := mustECDSAKeys(t, sellerKey)
	buyerPriv, buyerPub := mustECDSAKeys(t, buyerKey)

	sellerHost, err := delivery.NewLibp2pHost(sellerPriv, "/ip4/127.0.0.1/udp/0/quic-v1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })

	adapter, buyerStdIn, _ := setupMemoryTopic(t, "conn-pid-mismatch")

	// Buyer expects a DIFFERENT seller PeerID than the one actually publishing.
	otherKey := newTestKey(t)
	otherPeer, err := otherKey.PublicKey().PeerID()
	require.NoError(t, err)
	expectedPeerID := otherPeer.String()
	require.NotEqual(t, sellerHost.ID().String(), expectedPeerID)

	resultCh := runConnSetupReceiver(ctx, adapter, buyerStdIn, "req-mismatch",
		expectedPeerID, buyerPriv)
	time.Sleep(20 * time.Millisecond)

	streams := []payment.StreamCatalogEntry{
		{Name: "raw", ProtocolID: ProtocolRaw, Direction: payment.StreamDirectionSeller, Schema: SchemaURL},
	}
	_, err = PublishConnectionSetup(ctx, adapter, buyerStdIn, sellerKey, 1, "req-mismatch",
		sellerHost, buyerPub, streams)
	require.NoError(t, err)

	r := <-resultCh
	require.Error(t, r.err)
	var mismatch *PeerIDMismatchError
	require.True(t, errors.As(r.err, &mismatch), "expected PeerIDMismatchError, got %T: %v", r.err, r.err)
	assert.Equal(t, "connectionSetup.peerID", mismatch.Source)
	assert.Equal(t, expectedPeerID, mismatch.Expected)
	assert.Equal(t, sellerHost.ID().String(), mismatch.Observed)
}

func TestExtractSenderECDSAPublicKey_RoundTrip(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	senderKey := newTestKey(t)
	adapter, ref, _ := setupMemoryTopic(t, "extract-pubkey")

	type recvResult struct {
		msg topic.TopicMessage
		err error
	}
	resultCh := make(chan recvResult, 1)
	go func() {
		_, msg, err := ReceiveServiceRequest(ctx, adapter, ref)
		resultCh <- recvResult{msg, err}
	}()
	time.Sleep(20 * time.Millisecond)
	req, err := BuildServiceRequest(ServiceRequestOptions{RequestID: "round", BuyerStdIn: "x"})
	require.NoError(t, err)
	_, err = PublishPayload(ctx, adapter, ref, senderKey, 1, req)
	require.NoError(t, err)

	r := <-resultCh
	require.NoError(t, r.err)

	got, err := ExtractSenderECDSAPublicKey(r.msg)
	require.NoError(t, err)
	expected, err := senderKey.PublicKey().ToBlockchainKey()
	require.NoError(t, err)
	assert.Equal(t, expected.X.Bytes(), got.X.Bytes(), "ecrecover'd pubkey X matches signer")
	assert.Equal(t, expected.Y.Bytes(), got.Y.Bytes(), "ecrecover'd pubkey Y matches signer")
}
