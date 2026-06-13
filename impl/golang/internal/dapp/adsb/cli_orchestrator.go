package adsb

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

// SellerCLIOptions is the seller CLI's flag-shaped view.
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

// RunSellerCLISession runs ONE ServiceRequest end-to-end through the seller
// orchestrator.
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

// BuyerCLIOptions is the buyer CLI's flag-shaped view.
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

	BuyerHost  host.Host
	FrameLimit uint64

	// OnFrameReceived is invoked per NormalizedTrack decoded off the stream.
	OnFrameReceived func(track NormalizedTrack) error

	DialOverride     func(ctx context.Context, info peer.AddrInfo, proto string) (io.ReadWriteCloser, error)
	ServiceRequest   *payment.ServiceRequest
	DialAddrOverride peer.AddrInfo
}

// RunBuyerCLISession runs the full buyer-side lifecycle: negotiation → dial →
// frame loop → settlement.
func RunBuyerCLISession(ctx context.Context, opts BuyerCLIOptions) (BuyerSessionResult, error) {
	if opts.BuyerHost == nil && opts.DialOverride == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunBuyerCLISession: BuyerHost (or DialOverride) required")
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
		return partial, fmt.Errorf("adsb.RunBuyerCLISession: %w", err)
	}
	if partial.Discovery == nil {
		return partial, errors.New("adsb.RunBuyerCLISession: RunBuyerSession returned without ConnectionSetup")
	}

	stream, err := opts.dialSellerStream(ctx, partial)
	if err != nil {
		return partial, fmt.Errorf("adsb.RunBuyerCLISession: dial: %w", err)
	}
	defer stream.Close()
	logger.Printf("[buyer] dialed seller; reading up to %d frames", opts.FrameLimit)

	if err := opts.readFrames(ctx, stream); err != nil {
		return partial, fmt.Errorf("adsb.RunBuyerCLISession: frame loop: %w", err)
	}
	_ = stream.Close()
	logger.Printf("[buyer] frame loop ended; entering FinaliseBuyerSession")

	finalSeq := uint64(3)
	if opts.Mode == CommerceModeRegistrationOnly {
		finalSeq = 2
	}
	return FinaliseBuyerSession(ctx, sess, partial, finalSeq)
}

func (opts *BuyerCLIOptions) dialSellerStream(ctx context.Context, partial BuyerSessionResult) (io.ReadWriteCloser, error) {
	addrInfo := partial.SellerAddr
	if opts.DialAddrOverride.ID != "" {
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

func (opts *BuyerCLIOptions) readFrames(ctx context.Context, stream io.ReadWriteCloser) error {
	reader := delivery.NewFrameReader(stream)
	var n uint64
	for opts.FrameLimit == 0 || n < opts.FrameLimit {
		if ctx.Err() != nil {
			return nil
		}
		data, err := reader.ReadFrame()
		if err != nil {
			return nil
		}
		var f NormalizedTrack
		if err := json.Unmarshal(data, &f); err != nil {
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
