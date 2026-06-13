package browserprofile_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/browserprofile"
	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// TestBrowserProfile_FullBuyFlow_InProcess verifies the Tier B 4-message
// flow end-to-end over TCP libp2p (no WebTransport — we're exercising the
// application-layer protocol, not the transport).
//
// It spins up TWO in-process libp2p hosts on random 127.0.0.1 TCP ports:
//
//	seller — registers browserprofile.Server.HandleControl
//	buyer  — registers an ad-hoc DATA_PROTOCOL_ID handler and replays the
//	         exact 4-message sequence the browser-client/buyer-flow.ts
//	         sends in production
//
// On success the test asserts:
//   - the buyer receives the JPEG bytes byte-exactly
//   - the buyer-computed SHA-256 matches asset.Metadata.Sha256Hex
//   - the seller's mock escrow state is EscrowReleased for the agreementHash
//     carried in paymentDetails
func TestBrowserProfile_FullBuyFlow_InProcess(t *testing.T) {
	// --------------------------------------------------------------
	// Asset fixture — a small synthetic PNG-ish blob is fine; the
	// metadata.contentType just has to be "image/jpeg" or "image/png"
	// per file-metadata.ts ALLOWED_CONTENT_TYPES.
	// --------------------------------------------------------------
	payload := make([]byte, 4096)
	_, err := rand.Read(payload)
	require.NoError(t, err)

	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "fixture.jpg")
	require.NoError(t, os.WriteFile(assetPath, payload, 0o644))

	asset, err := browserprofile.LoadAsset(assetPath, "image/jpeg")
	require.NoError(t, err)

	// Sanity-check the SHA-256 the seller declares matches what the buyer
	// will compute (no NEURON_TAMPER in effect for this test).
	expectedSha := sha256.Sum256(payload)
	require.Equal(t, hex.EncodeToString(expectedSha[:]), asset.Metadata.Sha256Hex)

	// --------------------------------------------------------------
	// Seller host.
	// --------------------------------------------------------------
	sellerPriv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
	require.NoError(t, err)
	sellerNeuronKey := mustNeuronKey(t, sellerPriv)

	sellerHost, err := libp2p.New(
		libp2p.Identity(sellerPriv),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sellerHost.Close() })

	escrow := browserprofile.NewMockEscrow()
	server := browserprofile.NewServer(sellerHost, &sellerNeuronKey, asset, escrow, "")
	// Capture logs into the test output so failures are debuggable.
	server.Logger = func(format string, args ...any) {
		t.Logf(format, args...)
	}
	server.RegisterHandlers()

	// --------------------------------------------------------------
	// Buyer host.
	// --------------------------------------------------------------
	buyerPriv, _, err := libp2pcrypto.GenerateKeyPairWithReader(libp2pcrypto.Secp256k1, 0, rand.Reader)
	require.NoError(t, err)
	buyerNeuronKey := mustNeuronKey(t, buyerPriv)
	buyerAddress := buyerNeuronKey.PublicKey().EVMAddress().Hex()
	buyerPubKeyHex := hex.EncodeToString(buyerNeuronKey.PublicKey().Compressed())

	buyerHost, err := libp2p.New(
		libp2p.Identity(buyerPriv),
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = buyerHost.Close() })

	// Buyer registers the DATA_PROTOCOL_ID handler BEFORE sending
	// serviceRequest — matches buyer-flow.ts:128.
	dataStreamCh := make(chan network.Stream, 1)
	buyerHost.SetStreamHandler(protocol.ID(browserprofile.DataProtocolID), func(stream network.Stream) {
		dataStreamCh <- stream
	})

	// Dial the seller over /p2p/<seller> via the TCP multiaddr.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sellerAddrs := sellerHost.Addrs()
	require.NotEmpty(t, sellerAddrs)
	sellerAddr := sellerAddrs[0].Encapsulate(mustMultiaddr(t, "/p2p/"+sellerHost.ID().String()))
	info, err := peer.AddrInfoFromP2pAddr(sellerAddr)
	require.NoError(t, err)
	require.NoError(t, buyerHost.Connect(ctx, *info))

	controlStream, err := buyerHost.NewStream(ctx, sellerHost.ID(), protocol.ID(browserprofile.ControlProtocolID))
	require.NoError(t, err)
	defer func() { _ = controlStream.Close() }()

	reader := delivery.NewFrameReader(controlStream)
	writer := delivery.NewFrameWriter(controlStream)

	// --------------------------------------------------------------
	// Buyer -> seller: serviceRequest
	// --------------------------------------------------------------
	var buyerSeq uint64 = 1
	emit := func(payload any) error {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		ts := uint64(time.Now().UnixNano())
		seq := buyerSeq
		buyerSeq++
		env, err := topic.NewTopicMessage(&buyerNeuronKey, ts, seq, payloadBytes)
		if err != nil {
			return err
		}
		envJSON, err := env.ToJSON()
		if err != nil {
			return err
		}
		return writer.WriteFrame(envJSON)
	}

	require.NoError(t, emit(browserprofile.ServiceRequestPayload{
		Type:           browserprofile.TypeServiceRequest,
		Service:        browserprofile.ServiceName,
		BuyerAddress:   buyerAddress,
		BuyerPubKeyHex: buyerPubKeyHex,
	}))

	// --------------------------------------------------------------
	// Seller -> buyer: paymentDetails
	// --------------------------------------------------------------
	paymentEnv := mustReadEnvelope(t, reader)
	var paymentPayload browserprofile.PaymentDetailsPayload
	require.NoError(t, json.Unmarshal(paymentEnv.Payload(), &paymentPayload))
	assert.Equal(t, browserprofile.TypePaymentDetails, paymentPayload.Type)
	assert.Equal(t, "1", paymentPayload.PriceAtto)
	assert.Equal(t, asset.Metadata.Sha256Hex, paymentPayload.InvoiceSha256Hex)
	assert.Regexp(t, `^0x[0-9a-f]{64}$`, paymentPayload.AgreementHash)

	// --------------------------------------------------------------
	// Seller -> buyer: connectionSetup
	// --------------------------------------------------------------
	setupEnv := mustReadEnvelope(t, reader)
	var setupPayload browserprofile.ConnectionSetupPayload
	require.NoError(t, json.Unmarshal(setupEnv.Payload(), &setupPayload))
	assert.Equal(t, browserprofile.TypeConnectionSetup, setupPayload.Type)
	assert.Equal(t, buyerAddress, setupPayload.RecipientEVMAddress)
	assert.Equal(t, browserprofile.DataProtocolID, setupPayload.StreamProtocol)

	// Buyer decrypts ECIES multiaddrs — must succeed (byte-compat check).
	buyerEC := mustBuyerECDSAPriv(t, buyerNeuronKey)
	_, err = delivery.DecryptMultiaddrs(setupPayload.EncryptedMultiaddrs, buyerEC)
	require.NoError(t, err, "buyer failed to decrypt seller's connectionSetup (ECIES byte-compat regression)")

	// --------------------------------------------------------------
	// Seller opens data stream back -> buyer receives frame 0 + chunks.
	// --------------------------------------------------------------
	select {
	case dataStream := <-dataStreamCh:
		received, recvSha, err := receiveFile(dataStream)
		require.NoError(t, err)
		assert.Equal(t, asset.Metadata.Sha256Hex, recvSha)
		assert.Equal(t, payload, received)
	case <-time.After(10 * time.Second):
		t.Fatal("seller did not open data stream within 10s")
	}

	// --------------------------------------------------------------
	// Buyer -> seller: invoiceAck -> escrow released.
	// --------------------------------------------------------------
	require.NoError(t, emit(browserprofile.InvoiceAckPayload{
		Type:              browserprofile.TypeInvoiceAck,
		ReceivedSha256Hex: asset.Metadata.Sha256Hex,
	}))

	// The seller processes invoiceAck async; poll the escrow state for a
	// couple of seconds to ride out any scheduling delay.
	var releasedState browserprofile.EscrowState
	for range 50 {
		if got := escrow.State(paymentPayload.AgreementHash); got == browserprofile.EscrowReleased {
			releasedState = got
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.Equal(t, browserprofile.EscrowReleased, releasedState)
}

// mustNeuronKey extracts the raw scalar from a libp2p secp256k1 key and wraps
// it in a keylib.NeuronPrivateKey.
func mustNeuronKey(t *testing.T, priv libp2pcrypto.PrivKey) keylib.NeuronPrivateKey {
	t.Helper()
	raw, err := priv.Raw()
	require.NoError(t, err)
	nkey, err := keylib.NeuronPrivateKeyFromBytes(raw)
	require.NoError(t, err)
	return nkey
}

// mustMultiaddr parses a multiaddr or fails the test.
func mustMultiaddr(t *testing.T, s string) multiaddr.Multiaddr {
	t.Helper()
	ma, err := multiaddr.NewMultiaddr(s)
	require.NoError(t, err)
	return ma
}

// mustBuyerECDSAPriv converts a keylib NeuronPrivateKey into the
// *ecdsa.PrivateKey shape required by delivery.DecryptMultiaddrs.
func mustBuyerECDSAPriv(t *testing.T, key keylib.NeuronPrivateKey) *ecdsa.PrivateKey {
	t.Helper()
	ec, err := key.ToBlockchainKey()
	require.NoError(t, err)
	return ec
}

// mustReadEnvelope reads one length-prefixed frame from the control stream
// and parses it as a canonical TopicMessage.
func mustReadEnvelope(t *testing.T, reader *delivery.FrameReader) topic.TopicMessage {
	t.Helper()
	frame, err := reader.ReadFrame()
	require.NoError(t, err)
	env, err := topic.TopicMessageFromJSON(frame)
	require.NoError(t, err)
	return env
}

// receiveFile implements the buyer half of the data-plane protocol: reads
// frame 0 as JSON metadata, accumulates frames until total == sizeBytes,
// computes SHA-256, returns (bytes, shaHex). Matches file-receive.ts.
func receiveFile(stream io.Reader) ([]byte, string, error) {
	reader := delivery.NewFrameReader(stream)
	metaBytes, err := reader.ReadFrame()
	if err != nil {
		return nil, "", err
	}
	var meta browserprofile.FileMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, "", err
	}
	buf := make([]byte, 0, meta.SizeBytes)
	for len(buf) < meta.SizeBytes {
		chunk, err := reader.ReadFrame()
		if err != nil {
			return nil, "", err
		}
		buf = append(buf, chunk...)
	}
	sum := sha256.Sum256(buf)
	return buf, hex.EncodeToString(sum[:]), nil
}
