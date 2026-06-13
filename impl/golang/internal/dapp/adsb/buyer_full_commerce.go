package adsb

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// FullCommerceFlowOpts bundles caller-supplied infrastructure for one
// Spec 008 buyer-side commerce session. The caller (typically
// cmd/multistream-buyer) owns the lifecycle of BuyerHost, Adapter,
// Escrow, and Contract — RunFullCommerceFlow does NOT close them. This is
// what enables multistream-buyer to share ONE libp2p host across N
// concurrent commerce sessions.
type FullCommerceFlowOpts struct {
	Logger               *log.Logger
	Key                  *keylib.NeuronPrivateKey
	EcdsaPriv            *ecdsa.PrivateKey
	BuyerHost            host.Host                 // SHARED across sessions in multistream
	Adapter              topic.TopicAdapter        // pre-built (HCS or memory)
	Escrow               payment.EscrowAdapter     // pre-built (may be mutex-wrapped at caller scope)
	EscrowBinding        string                    // "memory" | "evm-escrow"
	Contract             registry.RegistryContract // pre-built; no per-session rebuild
	RegistryAddress      keylib.EVMAddress
	ChainID              uint64
	SellerEVM            keylib.EVMAddress
	PricingAmount        string
	FrameLimit           uint64
	SellerMaOverride     string        // empty = no override (use registry-derived multiaddr)
	AllowMaOverride      bool          // false = ignore SellerMaOverride
	LivenessPollInterval time.Duration // 0 = default
}

// RunFullCommerceFlow drives the buyer's Spec 008 commerce flow end-to-end:
//
//	discover seller → resolve dial addrinfo → liveness monitor (best-effort) →
//	build buyerStdIn → build ServiceRequest → RunBuyerCLISession (negotiate →
//	fund escrow → connectionSetup → dial → frames → serviceStop → invoice →
//	invoiceAck → COMPLETED).
//
// frameCallback receives each NormalizedTrack as it arrives off the libp2p
// stream, along with the seller's peer.ID (so the caller can tag
// per-seller in a multistream consolidated output sink). May be nil to
// skip per-frame delivery (useful for negotiation-only tests).
//
// ctx cancellation aborts cleanly without closing caller-owned resources.
//
// FR anchors: spec 008 FR-P21 (escrow lifecycle); spec 003 FR-R02
// (registry discovery); spec 005 FR-H05/H30 (liveness monitor).
func RunFullCommerceFlow(
	ctx context.Context,
	opts FullCommerceFlowOpts,
	frameCallback func(NormalizedTrack, peer.ID) error,
) (BuyerSessionResult, error) {
	if opts.Logger == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: Logger required")
	}
	if opts.BuyerHost == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: BuyerHost required")
	}
	if opts.Adapter == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: Adapter required")
	}
	if opts.Escrow == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: Escrow required (commerce-mode=full)")
	}
	if opts.Contract == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: Contract required")
	}
	if opts.Key == nil || opts.EcdsaPriv == nil {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: Key + EcdsaPriv required")
	}
	if opts.EscrowBinding == "evm-escrow" {
		if err := ValidatePricingForEVM(opts.PricingAmount); err != nil {
			return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: %w", err)
		}
	}

	discovery, err := DiscoverSeller(ctx, opts.SellerEVM, opts.RegistryAddress, opts.ChainID, opts.Contract)
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: discover seller: %w", err)
	}
	opts.Logger.Printf("[registry] discovered seller EVM=%s tokenId=%v peerID=%s",
		discovery.SellerEVM.Hex(), discovery.TokenID, discovery.PeerID)

	var dialAddrInfo peer.AddrInfo
	switch {
	case opts.SellerMaOverride != "" && opts.AllowMaOverride:
		parsed, perr := peer.AddrInfoFromString(opts.SellerMaOverride)
		if perr != nil {
			return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: parse --seller-multiaddr: %w", perr)
		}
		if parsed.ID.String() != discovery.PeerID {
			return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: --seller-multiaddr peerID=%s != registered=%s; refusing to dial",
				parsed.ID, discovery.PeerID)
		}
		opts.Logger.Printf("[buyer] override: --seller-multiaddr supplied; bypassing registry-derived multiaddrs (PeerID match ✓)")
		dialAddrInfo = *parsed
	default:
		regInfo, raErr := discovery.ResolveDialAddrs()
		if raErr != nil {
			return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: %w", raErr)
		}
		opts.Logger.Printf("[buyer] registry-derived multiaddrs (%d): %v", len(regInfo.Addrs), regInfo.Addrs)
		dialAddrInfo = regInfo
	}

	sellerStdInLoc := discovery.TopicConfigFor("stdIn")
	if sellerStdInLoc == "" {
		return BuyerSessionResult{}, errors.New("adsb.RunFullCommerceFlow: seller AgentURI has no stdIn topic config (--commerce-mode=full sellers embed topicId per channel)")
	}
	sellerStdIn, err := topic.NewTopicRef(opts.Adapter.SupportedTransport(), sellerStdInLoc)
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: build sellerStdIn topic ref: %w", err)
	}

	// Liveness monitor — best-effort; failure does not abort the session.
	if stdOutLoc := discovery.TopicConfigFor("stdOut"); stdOutLoc != "" {
		if sellerStdOut, refErr := topic.NewTopicRef(opts.Adapter.SupportedTransport(), stdOutLoc); refErr == nil {
			if livenessSub, subErr := opts.Adapter.Subscribe(ctx, sellerStdOut, topic.SubscribeOpts{}); subErr == nil {
				livenessEvents := make(chan AdsbLivenessEvent, 16)
				monitor := &AdsbLivenessMonitor{
					SellerEVM:              discovery.SellerEVM.Hex(),
					Deliveries:             livenessSub,
					PollInterval:           opts.LivenessPollInterval,
					Logger:                 opts.Logger,
					Events:                 livenessEvents,
					ExpectedAgentURISha256: discovery.AgentURISha256,
				}
				go func() {
					if rerr := monitor.Run(ctx); rerr != nil {
						opts.Logger.Printf("[liveness:adsb] monitor exited with error: %v", rerr)
					}
				}()
				go func() {
					for range livenessEvents {
					}
				}()
				opts.Logger.Printf("[liveness:adsb] monitor started sellerEVM=%s stdOut=%s expectedAgentURISha256=%s",
					discovery.SellerEVM.Hex(), stdOutLoc, discovery.AgentURISha256)
			} else {
				opts.Logger.Printf("[liveness:adsb] subscribe to stdOut failed: %v (continuing without liveness monitoring)", subErr)
			}
		} else {
			opts.Logger.Printf("[liveness:adsb] could not build sellerStdOut ref: %v (continuing without liveness monitoring)", refErr)
		}
	} else {
		opts.Logger.Printf("[liveness:adsb] seller AgentURI has no stdOut topic config; liveness monitoring disabled")
	}

	buyerStdIn, err := opts.Adapter.CreateTopic(topic.CreateTopicOpts{
		Memo: "adsb-buyer-stdin-" + opts.Key.PublicKey().EVMAddress().Hex(),
	})
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: create buyerStdIn topic: %w", err)
	}

	requestID := fmt.Sprintf("adsb-buyer-%s-%d",
		opts.Key.PublicKey().EVMAddress().Hex(), time.Now().UnixNano())

	servReq, err := BuildServiceRequest(ServiceRequestOptions{
		RequestID:      requestID,
		BuyerStdIn:     buyerStdIn.Locator(),
		ProposedAmount: opts.PricingAmount,
	})
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: build serviceRequest: %w", err)
	}

	// Wrap the caller's frameCallback to thread the seller peerID without
	// expanding BuyerCLIOptions.OnFrameReceived's signature.
	sellerPeerID, perr := peer.Decode(discovery.PeerID)
	if perr != nil {
		return BuyerSessionResult{}, fmt.Errorf("adsb.RunFullCommerceFlow: decode seller peerID: %w", perr)
	}
	var onFrame func(NormalizedTrack) error
	if frameCallback != nil {
		onFrame = func(track NormalizedTrack) error {
			return frameCallback(track, sellerPeerID)
		}
	}

	final, err := RunBuyerCLISession(ctx, BuyerCLIOptions{
		Key:                  opts.Key,
		EcdsaPriv:            opts.EcdsaPriv,
		Adapter:              opts.Adapter,
		SellerStdIn:          sellerStdIn,
		BuyerStdIn:           buyerStdIn,
		RequestID:            requestID,
		ExpectedSellerPeerID: discovery.PeerID,
		Mode:                 CommerceModeFull,
		Escrow:               opts.Escrow,
		EscrowBinding:        opts.EscrowBinding,
		SellerEVM:            opts.SellerEVM.Hex(),
		Logger:               opts.Logger,
		BuyerHost:            opts.BuyerHost,
		FrameLimit:           opts.FrameLimit,
		OnFrameReceived:      onFrame,
		ServiceRequest:       &servReq,
		DialAddrOverride:     dialAddrInfo,
	})
	if err != nil {
		return final, fmt.Errorf("adsb.RunFullCommerceFlow: orchestrator: %w", err)
	}
	opts.Logger.Printf("[buyer-session] complete action=%s requestID=%s", final.FinalAction, final.RequestID)
	return final, nil
}
