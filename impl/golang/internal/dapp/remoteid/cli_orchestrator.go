package remoteid

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/neuron-sdk/neuron-go-sdk/internal/delivery"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// SellerCLIOptions is the seller CLI's flag-shaped view of the underlying
// SellerSessionOptions. The bridge is intentionally thin so the CLI keeps
// no knowledge of the orchestrator's internals.
type SellerCLIOptions struct {
	Key           *keylib.NeuronPrivateKey
	Adapter       topic.TopicAdapter
	SellerStdIn   topic.TopicRef
	Descriptor    ServiceDescriptor
	Host          host.Host
	Escrow        payment.EscrowAdapter
	EscrowBinding string
	Mode          string
	Logger        *log.Logger
	FrameSummary  func() (frameCount, firstSentAt, lastSentAt uint64)
}

// RunSellerCLISession runs ONE ServiceRequest end-to-end through the
// Stage 2b orchestrator. Returns when the lifecycle reaches COMPLETED
// (or TERMINATED on abort). Callers loop this in a goroutine to handle
// multiple buyers (Stage 4+).
func RunSellerCLISession(ctx context.Context, opts SellerCLIOptions) (SellerSessionResult, error) {
	return RunSellerSession(ctx, SellerSessionOptions{
		Key:           opts.Key,
		Adapter:       opts.Adapter,
		SellerStdIn:   opts.SellerStdIn,
		Descriptor:    opts.Descriptor,
		Host:          opts.Host,
		Escrow:        opts.Escrow,
		EscrowBinding: opts.EscrowBinding,
		Mode:          opts.Mode,
		Logger:        opts.Logger,
		FrameSummary:  opts.FrameSummary,
	})
}

// BuyerCLIOptions is the buyer CLI's flag-shaped view of the underlying
// BuyerSessionOptions plus the libp2p dial inputs the CLI needs to
// consume frames before settlement. RunBuyerCLISession owns the full
// shape: 008 negotiation → dial → frame loop → settlement.
type BuyerCLIOptions struct {
	Key                  *keylib.NeuronPrivateKey
	EcdsaPriv            *ecdsa.PrivateKey
	Adapter              topic.TopicAdapter
	SellerStdIn          topic.TopicRef
	BuyerStdIn           topic.TopicRef
	RequestID            string
	ExpectedSellerPeerID string
	Mode                 string
	Escrow               payment.EscrowAdapter
	EscrowBinding        string
	SellerEVM            string
	Logger               *log.Logger

	// BuyerHost is the libp2p host the buyer uses to dial the seller
	// after ECIES-decrypting the multiaddrs in ConnectionSetup. The
	// caller owns the host's lifetime (typically `defer host.Close()`).
	BuyerHost host.Host

	// FrameLimit > 0 fires ServiceStop after N frames received from the
	// seller's libp2p stream. 0 = wait for ctx cancellation.
	FrameLimit uint64

	// OnFrameReceived, if non-nil, is invoked per RemoteIdFrame after
	// it is decoded off the stream. The CLI uses this to emit the
	// tagged JSONL envelope to its sink. When nil, frames are decoded
	// and discarded (useful in unit tests).
	OnFrameReceived func(frame RemoteIdFrame) error

	// DialOverride, when non-nil, replaces the default libp2p dial path.
	// Tests use this to short-circuit dialing when there is no live
	// seller host (e.g. asserting only that the orchestrator branched
	// to dial). Production CLIs leave it nil and rely on BuyerHost.
	DialOverride func(ctx context.Context, info peer.AddrInfo, proto string) (io.ReadWriteCloser, error)

	// ServiceRequest, when non-nil, overrides the ServiceRequest the
	// orchestrator would otherwise auto-build. Stage 3A uses this to
	// pass the operator-supplied --pricing-amount through the wire.
	ServiceRequest *payment.ServiceRequest

	// DialAddrOverride, when ID != "", replaces the seller AddrInfo the
	// orchestrator derived from the ECIES-decrypted ConnectionSetup
	// multiaddrs. Stage 3B uses this to dial via registry-discovered
	// multiaddrs (so the operator never needs --seller-multiaddr). The
	// orchestrator still validates that PeerID matches discovery; the
	// dial just uses these multiaddrs.
	DialAddrOverride peer.AddrInfo
}

// RunBuyerCLISession runs the full buyer-side lifecycle:
//
//   1. RunBuyerSession   — publish serviceRequest, receive serviceResponse,
//                          fund escrow, receive connectionSetup
//   2. Dial seller host  — using the ECIES-decrypted multiaddr
//   3. Read frames       — up to FrameLimit, invoking OnFrameReceived
//   4. FinaliseBuyerSession — publish serviceStop, receive invoice,
//                             approveRelease, publish invoiceAck
//
// Returns the BuyerSessionResult with FinalAction = "approved" on the
// happy path.
func RunBuyerCLISession(ctx context.Context, opts BuyerCLIOptions) (BuyerSessionResult, error) {
	if opts.BuyerHost == nil && opts.DialOverride == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunBuyerCLISession: BuyerHost (or DialOverride) required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(discardWriter{}, "", 0)
	}

	sess := BuyerSessionOptions{
		Key:                  opts.Key,
		EcdsaPriv:            opts.EcdsaPriv,
		Adapter:              opts.Adapter,
		SellerStdIn:          opts.SellerStdIn,
		BuyerStdIn:           opts.BuyerStdIn,
		RequestID:            opts.RequestID,
		ExpectedSellerPeerID: opts.ExpectedSellerPeerID,
		Mode:                 opts.Mode,
		Escrow:               opts.Escrow,
		EscrowBinding:        opts.EscrowBinding,
		SellerEVM:            opts.SellerEVM,
		Logger:               logger,
		ServiceRequest:       opts.ServiceRequest,
	}

	partial, err := RunBuyerSession(ctx, sess)
	if err != nil {
		return partial, fmt.Errorf("remoteid.RunBuyerCLISession: %w", err)
	}
	if partial.Discovery == nil {
		return partial, errors.New("remoteid.RunBuyerCLISession: RunBuyerSession returned without ConnectionSetup")
	}

	stream, err := opts.dialSellerStream(ctx, partial)
	if err != nil {
		return partial, fmt.Errorf("remoteid.RunBuyerCLISession: dial: %w", err)
	}
	defer stream.Close()
	logger.Printf("[buyer] dialed seller; reading up to %d frames", opts.FrameLimit)

	if err := opts.readFrames(ctx, stream); err != nil {
		return partial, fmt.Errorf("remoteid.RunBuyerCLISession: frame loop: %w", err)
	}
	_ = stream.Close()
	logger.Printf("[buyer] frame loop ended; entering FinaliseBuyerSession")

	// Sequence numbers continue from where RunBuyerSession left off.
	// Full-commerce flow uses seq 1 (ServiceRequest) and seq 2
	// (EscrowCreated); FinaliseBuyerSession starts at 3.
	finalSeq := uint64(3)
	if opts.Mode == CommerceModeRegistrationOnly {
		// Registration-only: only seq 1 was used (no EscrowCreated).
		finalSeq = 2
	}
	return FinaliseBuyerSession(ctx, sess, partial, finalSeq)
}

// dialSellerStream opens the canonical raw stream on the seller. Uses the
// DialOverride if set; otherwise dials via opts.BuyerHost. The decrypted
// seller AddrInfo arrives via RunBuyerSession → BuyerSessionResult.SellerAddr.
// When opts.DialAddrOverride.ID != "" (Stage 3B), the override replaces
// the orchestrator-derived AddrInfo so the dial uses registry-discovered
// multiaddrs.
func (opts *BuyerCLIOptions) dialSellerStream(ctx context.Context, partial BuyerSessionResult) (io.ReadWriteCloser, error) {
	addrInfo := partial.SellerAddr
	if opts.DialAddrOverride.ID != "" {
		// Stage 3B: registry-derived multiaddrs replace the ECIES-decrypted
		// ones. Sanity-check the PeerID matches before swapping; both come
		// from the same registered identity (PeerID in AgentURI vs PeerID
		// embedded in ConnectionSetup) so this is defence-in-depth.
		if partial.SellerAddr.ID != "" && partial.SellerAddr.ID != opts.DialAddrOverride.ID {
			return nil, fmt.Errorf("dialSellerStream: registry PeerID %s != ConnectionSetup PeerID %s",
				opts.DialAddrOverride.ID, partial.SellerAddr.ID)
		}
		addrInfo = opts.DialAddrOverride
	}
	if addrInfo.ID == "" {
		return nil, errors.New("BuyerSessionResult.SellerAddr.ID is empty; RunBuyerSession should have populated it")
	}
	streamProto := partial.Discovery.Streams[0].ProtocolID
	if opts.DialOverride != nil {
		return opts.DialOverride(ctx, addrInfo, streamProto)
	}
	if err := opts.BuyerHost.Connect(ctx, addrInfo); err != nil {
		return nil, fmt.Errorf("host.Connect: %w", err)
	}
	stream, err := opts.BuyerHost.NewStream(ctx, addrInfo.ID, protocol.ID(streamProto))
	if err != nil {
		return nil, fmt.Errorf("host.NewStream %s: %w", streamProto, err)
	}
	return stream, nil
}

// readFrames consumes RemoteIdFrame payloads off the libp2p stream and
// invokes OnFrameReceived per frame. Stops when FrameLimit is reached
// (if > 0), the context is cancelled, or the stream EOFs.
func (opts *BuyerCLIOptions) readFrames(ctx context.Context, stream io.ReadWriteCloser) error {
	reader := delivery.NewFrameReader(stream)
	var n uint64
	for opts.FrameLimit == 0 || n < opts.FrameLimit {
		if ctx.Err() != nil {
			return nil
		}
		data, err := reader.ReadFrame()
		if err != nil {
			// EOF is expected when the seller closes; treat as stream-end.
			return nil
		}
		var f RemoteIdFrame
		if err := json.Unmarshal(data, &f); err != nil {
			// Malformed frames are informational per FR-R07 — skip.
			continue
		}
		if opts.OnFrameReceived != nil {
			if err := opts.OnFrameReceived(f); err != nil {
				return err
			}
		}
		n++
	}
	return nil
}

