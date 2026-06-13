package browserprofile

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Server holds the seller-side state needed to drive the Tier B browser
// profile 4-message flow. Construct with NewServer, register handlers on the
// libp2p host with RegisterHandlers.
type Server struct {
	h                   host.Host
	sellerKey           *keylib.NeuronPrivateKey
	asset               *LoadedAsset
	escrow              *MockEscrow
	advertisedMultiaddr string
	// Logger is called for each protocol-level event. Defaults to
	// fmt.Println on the seller's stdout so `tail -f /var/log/wt-seller.log`
	// on the VPS shows the 4-message exchange. Override for tests.
	Logger func(format string, args ...any)
}

// NewServer wires the seller state. `advertisedMultiaddr` is embedded inside
// the ECIES-encrypted connectionSetup so the buyer learns the seller's
// multiaddr; in the WebTransport spike the seller dials BACK on the existing
// libp2p connection, so the value is informational. Passing an empty string
// is valid — the browser decrypts an empty multiaddr array and falls through
// to the bootstrap multiaddr.
func NewServer(h host.Host, sellerKey *keylib.NeuronPrivateKey, asset *LoadedAsset, escrow *MockEscrow, advertisedMultiaddr string) *Server {
	return &Server{
		h:                   h,
		sellerKey:           sellerKey,
		asset:               asset,
		escrow:              escrow,
		advertisedMultiaddr: advertisedMultiaddr,
	}
}

// RegisterHandlers installs the control stream handler on the host. The data
// stream is initiated by the seller (not inbound), so no handler is needed
// for DataProtocolID on this side.
func (s *Server) RegisterHandlers() {
	s.h.SetStreamHandler(protocol.ID(ControlProtocolID), s.HandleControl)
}

// HandleControl implements the seller's half of the Tier 1 4-message flow.
//
// Protocol lifecycle on one inbound control stream:
//  1. Read first frame -> parse as TopicMessage envelope -> payload.type == serviceRequest
//  2. Propose escrow with a freshly minted agreementHash
//  3. Emit signed paymentDetails envelope
//  4. ECIES-encrypt advertisedMultiaddr for buyerPubKeyHex -> connectionSetup
//  5. Open a separate data stream back to the buyer via host.NewStream,
//     send asset (frame 0 metadata + chunks), close the data stream
//  6. Read next frame on the control stream -> invoiceAck -> escrow.Release
//  7. Close the control stream
//
// Any unexpected envelope type aborts the handler. Signature verification on
// inbound envelopes is intentionally omitted (Tier 1 seller has the same
// posture: it trusts the control-stream direction because the transport is
// Noise-authenticated and the browser is a stateless ephemeral session).
func (s *Server) HandleControl(stream network.Stream) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer func() { _ = stream.Close() }()

	remotePeer := stream.Conn().RemotePeer()
	s.logf("[wt-seller] control stream opened by %s", remotePeer)

	reader := delivery.NewFrameReader(stream)
	writer := delivery.NewFrameWriter(stream)

	var nextSeq uint64 = 1
	emit := func(payload any) error {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		ts := uint64(time.Now().UnixNano())
		seq := nextSeq
		nextSeq++
		env, err := topic.NewTopicMessage(s.sellerKey, ts, seq, payloadBytes)
		if err != nil {
			return fmt.Errorf("sign envelope: %w", err)
		}
		envJSON, err := env.ToJSON()
		if err != nil {
			return fmt.Errorf("marshal envelope: %w", err)
		}
		return writer.WriteFrame(envJSON)
	}

	// ------------------------------------------------------------------
	// Message 1 (inbound): serviceRequest
	// ------------------------------------------------------------------
	req, err := readServiceRequest(reader)
	if err != nil {
		s.logf("[wt-seller] serviceRequest parse failed: %v", err)
		_ = stream.Reset()
		return
	}
	s.logf("[wt-seller] serviceRequest from %s service=%s", req.BuyerAddress, req.Service)

	// Fresh agreementHash: "0x" + 32 random hex bytes (mirrors TS hex32()).
	var rnd [32]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		s.logf("[wt-seller] rand: %v", err)
		return
	}
	agreementHash := "0x" + hex.EncodeToString(rnd[:])
	if err := s.escrow.Propose(agreementHash, "1", s.asset.Metadata.Sha256Hex); err != nil {
		s.logf("[wt-seller] escrow propose: %v", err)
		return
	}
	s.logf("[wt-seller] escrow proposed %s -> price=1atto sha=%s",
		shortHash(agreementHash), s.asset.Metadata.Sha256Hex)

	// ------------------------------------------------------------------
	// Message 2 (outbound): paymentDetails
	// ------------------------------------------------------------------
	if err := emit(PaymentDetailsPayload{
		Type:             TypePaymentDetails,
		AgreementHash:    agreementHash,
		PriceAtto:        "1",
		InvoiceSha256Hex: s.asset.Metadata.Sha256Hex,
	}); err != nil {
		s.logf("[wt-seller] emit paymentDetails: %v", err)
		return
	}
	s.logf("[wt-seller] -> paymentDetails")

	// ------------------------------------------------------------------
	// Message 3 (outbound): connectionSetup (ECIES encrypt multiaddrs)
	// ------------------------------------------------------------------
	buyerPubKey, err := decompressBuyerPubKey(req.BuyerPubKeyHex)
	if err != nil {
		s.logf("[wt-seller] decompress buyerPubKey: %v", err)
		return
	}

	multiaddrs := []string{}
	if s.advertisedMultiaddr != "" {
		multiaddrs = append(multiaddrs, s.advertisedMultiaddr)
	}
	encrypted, err := delivery.EncryptMultiaddrs(multiaddrs, buyerPubKey)
	if err != nil {
		s.logf("[wt-seller] ECIES encrypt: %v", err)
		return
	}

	if err := emit(ConnectionSetupPayload{
		Type:                TypeConnectionSetup,
		RecipientEVMAddress: req.BuyerAddress,
		EncryptedMultiaddrs: encrypted,
		StreamProtocol:      DataProtocolID,
	}); err != nil {
		s.logf("[wt-seller] emit connectionSetup: %v", err)
		return
	}
	s.logf("[wt-seller] -> connectionSetup (encrypted %d multiaddrs)", len(multiaddrs))

	// ------------------------------------------------------------------
	// Open data stream back to the buyer, send file.
	// ------------------------------------------------------------------
	if err := s.sendFileTo(ctx, remotePeer); err != nil {
		s.logf("[wt-seller] data stream error: %v", err)
		return
	}

	// ------------------------------------------------------------------
	// Message 4 (inbound, best-effort): invoiceAck -> release escrow.
	//
	// Tier 1 fires `invoiceAck` and immediately tears the browser's libp2p
	// transport down. libp2p@3's stream.send() is fire-and-forget (queues
	// into an internal buffer; does not await a QUIC-level ACK), and the
	// subsequent session close races against the flush. In the WebTransport
	// path the seller often observes a *webtransport.SessionError instead
	// of the frame.
	//
	// Semantically the flow is complete once the data stream closed without
	// error — the buyer has the bytes and the SHA-256 on frame 0. So we
	// attempt a bounded read for the ack (to log it when it does arrive),
	// then release the escrow unconditionally. This matches the Tier 1
	// mock-escrow semantics; real on-chain settlement is out of scope for
	// v1 / v2a.
	// ------------------------------------------------------------------
	_ = stream.SetReadDeadline(time.Now().Add(2 * time.Second))
	if ack, err := readInvoiceAck(reader); err == nil {
		s.logf("[wt-seller] <- invoiceAck receivedSha=%s", ack.ReceivedSha256Hex)
	} else {
		s.logf("[wt-seller] invoiceAck not observed (%v); releasing escrow based on successful asset delivery", err)
	}

	if err := s.escrow.Release(agreementHash); err != nil {
		s.logf("[wt-seller] escrow release: %v", err)
		return
	}
	s.logf("[wt-seller] escrow %s -> released", shortHash(agreementHash))
}

// sendFileTo opens a data stream back to the buyer and writes the asset.
func (s *Server) sendFileTo(ctx context.Context, buyer peer.ID) error {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	dataStream, err := s.h.NewStream(dialCtx, buyer, protocol.ID(DataProtocolID))
	if err != nil {
		return fmt.Errorf("open data stream to %s: %w", buyer, err)
	}
	s.logf("[wt-seller] data stream opened to %s", buyer)
	defer func() {
		// Mirror Tier 1: 250 ms grace for the OS to drain, then close.
		time.Sleep(250 * time.Millisecond)
		_ = dataStream.Close()
		s.logf("[wt-seller] data stream closed")
	}()

	if err := SendAsset(dataStream, s.asset); err != nil {
		return fmt.Errorf("send asset: %w", err)
	}
	s.logf("[wt-seller] asset sent: %d bytes, sha=%s",
		s.asset.Metadata.SizeBytes, s.asset.Metadata.Sha256Hex)
	return nil
}

// readServiceRequest waits for one control-stream frame, parses it as a
// signed TopicMessage, and extracts a ServiceRequestPayload. Matches the
// parsing contract of TS seller-flow.ts parseEnvelopeFromJson.
func readServiceRequest(reader *delivery.FrameReader) (*ServiceRequestPayload, error) {
	env, err := readEnvelope(reader)
	if err != nil {
		return nil, err
	}
	var req ServiceRequestPayload
	if err := json.Unmarshal(env.Payload(), &req); err != nil {
		return nil, fmt.Errorf("unmarshal serviceRequest: %w", err)
	}
	if req.Type != TypeServiceRequest {
		return nil, fmt.Errorf("expected %q, got %q", TypeServiceRequest, req.Type)
	}
	return &req, nil
}

// readInvoiceAck waits for the terminal control-stream frame.
func readInvoiceAck(reader *delivery.FrameReader) (*InvoiceAckPayload, error) {
	env, err := readEnvelope(reader)
	if err != nil {
		return nil, err
	}
	var ack InvoiceAckPayload
	if err := json.Unmarshal(env.Payload(), &ack); err != nil {
		return nil, fmt.Errorf("unmarshal invoiceAck: %w", err)
	}
	if ack.Type != TypeInvoiceAck {
		return nil, fmt.Errorf("expected %q, got %q", TypeInvoiceAck, ack.Type)
	}
	return &ack, nil
}

// readEnvelope reads one length-prefixed frame and parses it as a canonical
// TopicMessage. Keep-alive frames (empty payload) are filtered by the
// FrameReader.
func readEnvelope(reader *delivery.FrameReader) (topic.TopicMessage, error) {
	frame, err := reader.ReadFrame()
	if err != nil {
		if err == io.EOF {
			return topic.TopicMessage{}, fmt.Errorf("stream closed before envelope")
		}
		return topic.TopicMessage{}, fmt.Errorf("read frame: %w (type=%T)", err, err)
	}
	env, err := topic.TopicMessageFromJSON(frame)
	if err != nil {
		return topic.TopicMessage{}, fmt.Errorf("parse envelope: %w", err)
	}
	return env, nil
}

// decompressBuyerPubKey converts a 66-hex-char compressed secp256k1 pubkey
// (as produced by session.ts compressedPublicKeyHex) into *ecdsa.PublicKey
// suitable for delivery.EncryptMultiaddrs.
//
// The TS hex has no 0x prefix, but accept either for robustness.
func decompressBuyerPubKey(hexStr string) (*ecdsa.PublicKey, error) {
	clean := hexStr
	if len(clean) >= 2 && (clean[:2] == "0x" || clean[:2] == "0X") {
		clean = clean[2:]
	}
	raw, err := hex.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("buyerPubKey hex: %w", err)
	}
	if len(raw) != 33 {
		return nil, fmt.Errorf("buyerPubKey must be 33 bytes (compressed), got %d", len(raw))
	}
	pub, err := crypto.DecompressPubkey(raw)
	if err != nil {
		return nil, fmt.Errorf("decompress secp256k1 pubkey: %w", err)
	}
	return pub, nil
}

// logf invokes Server.Logger if set, else prints to stdout with a newline.
func (s *Server) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}

// shortHash returns the first 10 and last 4 hex chars of a 0x-prefixed hash
// for friendlier logs.
func shortHash(h string) string {
	if len(h) <= 14 {
		return h
	}
	return h[:10] + "…" + h[len(h)-4:]
}
