package remoteid

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
// Spec 008 Remote ID buyer-side commerce session. Symmetric with
// adsb.FullCommerceFlowOpts; see that type's doc comment for rationale.
type FullCommerceFlowOpts struct {
	Logger               *log.Logger
	Key                  *keylib.NeuronPrivateKey
	EcdsaPriv            *ecdsa.PrivateKey
	BuyerHost            host.Host
	Adapter              topic.TopicAdapter
	Escrow               payment.EscrowAdapter
	EscrowBinding        string
	Contract             registry.RegistryContract
	RegistryAddress      keylib.EVMAddress
	ChainID              uint64
	SellerEVM            keylib.EVMAddress
	PricingAmount        string
	FrameLimit           uint64
	SellerMaOverride     string
	AllowMaOverride      bool
	LivenessPollInterval time.Duration
}

// RunFullCommerceFlow drives the Remote ID buyer's Spec 008 commerce flow
// end-to-end. Symmetric with adsb.RunFullCommerceFlow; see that function
// for the full state-machine documentation.
//
// frameCallback receives each RemoteIdFrame as it arrives off the libp2p
// stream, along with the seller's peer.ID. May be nil.
//
// FR anchors: spec 017 FR-R02 / FR-R05 (Remote ID frame format); spec 008
// FR-P21 (escrow lifecycle); spec 005 FR-H30 (liveness).
func RunFullCommerceFlow(
	ctx context.Context,
	opts FullCommerceFlowOpts,
	frameCallback func(RemoteIdFrame, peer.ID) error,
) (BuyerSessionResult, error) {
	if opts.Logger == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: Logger required")
	}
	if opts.BuyerHost == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: BuyerHost required")
	}
	if opts.Adapter == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: Adapter required")
	}
	if opts.Escrow == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: Escrow required (commerce-mode=full)")
	}
	if opts.Contract == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: Contract required")
	}
	if opts.Key == nil || opts.EcdsaPriv == nil {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: Key + EcdsaPriv required")
	}
	if opts.EscrowBinding == "evm-escrow" {
		if err := ValidatePricingForEVM(opts.PricingAmount); err != nil {
			return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: %w", err)
		}
	}

	discovery, err := DiscoverSeller(ctx, opts.SellerEVM, opts.RegistryAddress, opts.ChainID, opts.Contract)
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: discover seller: %w", err)
	}
	opts.Logger.Printf("[registry] discovered seller EVM=%s tokenId=%v peerID=%s",
		discovery.SellerEVM.Hex(), discovery.TokenID, discovery.PeerID)

	var dialAddrInfo peer.AddrInfo
	switch {
	case opts.SellerMaOverride != "" && opts.AllowMaOverride:
		parsed, perr := peer.AddrInfoFromString(opts.SellerMaOverride)
		if perr != nil {
			return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: parse --seller-multiaddr: %w", perr)
		}
		if parsed.ID.String() != discovery.PeerID {
			return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: --seller-multiaddr peerID=%s != registered=%s; refusing to dial",
				parsed.ID, discovery.PeerID)
		}
		opts.Logger.Printf("[buyer] override: --seller-multiaddr supplied; bypassing registry-derived multiaddrs (PeerID match ✓)")
		dialAddrInfo = *parsed
	default:
		regInfo, raErr := discovery.ResolveDialAddrs()
		if raErr != nil {
			return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: %w", raErr)
		}
		opts.Logger.Printf("[buyer] registry-derived multiaddrs (%d): %v", len(regInfo.Addrs), regInfo.Addrs)
		dialAddrInfo = regInfo
	}

	sellerStdInLoc := discovery.TopicConfigFor("stdIn")
	if sellerStdInLoc == "" {
		return BuyerSessionResult{}, errors.New("remoteid.RunFullCommerceFlow: seller AgentURI has no stdIn topic config (--commerce-mode=full sellers embed topicId per channel)")
	}
	sellerStdIn, err := topic.NewTopicRef(opts.Adapter.SupportedTransport(), sellerStdInLoc)
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: build sellerStdIn topic ref: %w", err)
	}

	// Liveness monitor — best-effort.
	if stdOutLoc := discovery.TopicConfigFor("stdOut"); stdOutLoc != "" {
		if sellerStdOut, refErr := topic.NewTopicRef(opts.Adapter.SupportedTransport(), stdOutLoc); refErr == nil {
			if livenessSub, subErr := opts.Adapter.Subscribe(ctx, sellerStdOut, topic.SubscribeOpts{}); subErr == nil {
				livenessEvents := make(chan RemoteIdLivenessEvent, 16)
				monitor := &RemoteIdLivenessMonitor{
					SellerEVM:              discovery.SellerEVM.Hex(),
					Deliveries:             livenessSub,
					PollInterval:           opts.LivenessPollInterval,
					Logger:                 opts.Logger,
					Events:                 livenessEvents,
					ExpectedAgentURISha256: discovery.AgentURISha256,
				}
				go func() {
					if rerr := monitor.Run(ctx); rerr != nil {
						opts.Logger.Printf("[liveness:remote-id] monitor exited with error: %v", rerr)
					}
				}()
				go func() {
					for range livenessEvents {
					}
				}()
				opts.Logger.Printf("[liveness:remote-id] monitor started sellerEVM=%s stdOut=%s expectedAgentURISha256=%s",
					discovery.SellerEVM.Hex(), stdOutLoc, discovery.AgentURISha256)
			} else {
				opts.Logger.Printf("[liveness:remote-id] subscribe to stdOut failed: %v (continuing without liveness monitoring)", subErr)
			}
		} else {
			opts.Logger.Printf("[liveness:remote-id] could not build sellerStdOut ref: %v (continuing without liveness monitoring)", refErr)
		}
	} else {
		opts.Logger.Printf("[liveness:remote-id] seller AgentURI has no stdOut topic config; liveness monitoring disabled")
	}

	buyerStdIn, err := opts.Adapter.CreateTopic(topic.CreateTopicOpts{
		Memo: "remoteid-buyer-stdin-" + opts.Key.PublicKey().EVMAddress().Hex(),
	})
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: create buyerStdIn topic: %w", err)
	}

	requestID := fmt.Sprintf("remoteid-buyer-%s-%d",
		opts.Key.PublicKey().EVMAddress().Hex(), time.Now().UnixNano())

	servReq, err := BuildServiceRequest(ServiceRequestOptions{
		RequestID:      requestID,
		BuyerStdIn:     buyerStdIn.Locator(),
		ProposedAmount: opts.PricingAmount,
	})
	if err != nil {
		return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: build serviceRequest: %w", err)
	}

	sellerPeerID, perr := peer.Decode(discovery.PeerID)
	if perr != nil {
		return BuyerSessionResult{}, fmt.Errorf("remoteid.RunFullCommerceFlow: decode seller peerID: %w", perr)
	}
	var onFrame func(RemoteIdFrame) error
	if frameCallback != nil {
		onFrame = func(frame RemoteIdFrame) error {
			return frameCallback(frame, sellerPeerID)
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
		return final, fmt.Errorf("remoteid.RunFullCommerceFlow: orchestrator: %w", err)
	}
	opts.Logger.Printf("[buyer-session] complete action=%s requestID=%s", final.FinalAction, final.RequestID)
	return final, nil
}
