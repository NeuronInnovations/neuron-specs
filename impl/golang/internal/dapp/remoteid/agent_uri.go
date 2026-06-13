package remoteid

import (
	"errors"
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// Descriptor naming / version constants. These are DApp-level conventions
// for the Remote ID seller's EIP-8004 AgentURI. They mirror values in
// `specs/017-remote-id-dapp/spec.md` FR-R01 / FR-R02 and the layered-DApp
// envelope rules in `docs/dapp-frame-format-precedent.md`.
const (
	// CommerceServiceName is the canonical neuron-commerce service name
	// for the Remote ID DApp. FR-R01.
	CommerceServiceName = "remote-id"

	// CommerceServiceVersion is the version stamped on the neuron-commerce
	// entry; bump per spec 017 amendments.
	CommerceServiceVersion = "1.0.0"

	// P2PServiceName is the canonical neuron-p2p-exchange name for the
	// Remote ID raw stream. Referenced by the commerce service's
	// delivery.serviceRef (V-REG-13).
	P2PServiceName = "remoteid-p2p"

	// P2PServiceVersion mirrors the protocol-ID minor.
	P2PServiceVersion = "1.0.0"

	// TopicNameStdIn / StdOut / StdErr are the three canonical topic
	// service names. The p2p service's topicRef points at stdOut so
	// observers tailing stdOut can correlate the data-plane stream.
	TopicNameStdIn  = "remoteid-stdin"
	TopicNameStdOut = "remoteid-stdout"
	TopicNameStdErr = "remoteid-stderr"

	// TopicVersion is the version stamped on every NeuronTopicService.
	TopicVersion = "1.0.0"

	// PricingUnit is the canonical pricing.unit value for Remote ID. FR-R01.
	PricingUnit = "frame"

	// ProfileR1 is the registry-backed registration-only profile identifier.
	// Sibling of `f-fixture-direct/1` (013 FR-F-02).
	ProfileR1 = "r1-eip8004-registry/1"

	// ProfileFixture is the disclosure value for fixture-direct runs (013 FR-F-02).
	ProfileFixture = "f-fixture-direct/1"

	// Commerce-mode values per 008 FR-P58.
	CommerceModeFull             = "full"
	CommerceModeRegistrationOnly = "registration-only"
	CommerceModeDataOnly         = "data-only"

	// Feed-source values per 017 FR-R14.
	FeedSourceLive        = "live"
	FeedSourceReplay      = "replay"
	FeedSourceSynthetic   = "synthetic"
	FeedSourcePlaceholder = "placeholder"

	// Default settlement binding for the R1 demo. Real escrow contract
	// address is configured via DescriptorOptions.EscrowContract.
	DefaultSettlementBinding = "evm-escrow"
)

// DescriptorOptions configures the Remote ID service descriptor.
// ChildKey is mandatory; the rest carry safe defaults so a CLI can
// pass a sparsely-populated struct.
type DescriptorOptions struct {
	// ChildKey is the seller's secp256k1 key. Used to derive PeerID,
	// EVMAddress, and the DID service entry. REQUIRED.
	ChildKey *keylib.NeuronPrivateKey

	// ChainID is the EVM chain ID where the Identity Registry lives.
	// Embedded in SettlementDescriptor.Config when EscrowBinding is
	// "evm-escrow". 0 is allowed (memory-contract / unit tests).
	ChainID uint64

	// EscrowBinding is the SettlementDescriptor.Binding value. Defaults
	// to "evm-escrow" if empty.
	EscrowBinding string

	// EscrowContract is the on-chain escrow address (0x...) embedded as
	// SettlementDescriptor.Config["contract"]. Optional.
	EscrowContract string

	// Pricing fields per FR-P03. Defaults: amount "0", currency "USDC",
	// interval "0". Unit is always "frame" per FR-R01.
	PricingAmount   string
	PricingCurrency string
	PricingInterval string

	// FeedSource per FR-R14. Defaults to "live". Captured on the
	// returned descriptor so a heartbeat publisher (FR-R15) can
	// emit it later.
	FeedSource string

	// CommerceMode per 008 FR-P58. Defaults to "registration-only" for
	// R1 demos (no escrow + invoice + settlement engagement).
	CommerceMode string

	// ProfileID disclosed in evidence / future heartbeat. Defaults to
	// ProfileR1 ("r1-eip8004-registry/1").
	ProfileID string

	// Anchor is the NeuronTopicService.anchor field. Required to be
	// non-empty per V-REG-03; defaults to "r1-eip8004-registry" if empty.
	Anchor string

	// TopicTransport is the NeuronTopicService.transport field. Required
	// non-empty per V-REG-03; defaults to "memory" if empty. Tests and
	// the R1 demo use "memory"; a Profile E / Profile D production
	// deployment would use "hcs" or similar.
	TopicTransport string

	// TopicConfig overrides the `config` map for one or more standard
	// channels. Keyed by channel name (`stdIn` / `stdOut` / `stdErr`).
	// When a key is present, the corresponding NeuronTopicService.Config
	// is set to its value (the buyer reads back e.g. config["topicId"]
	// to subscribe to the seller's stdIn / stdOut). When absent or
	// nil-valued, the default empty map is used (Stage 1 behaviour).
	//
	// Stage 2 of the demo populates `topicId` per channel so the buyer
	// can resolve seller topic refs out of the registered AgentURI.
	TopicConfig map[string]map[string]any

	// Multiaddrs (Stage 3B) is the seller's filtered libp2p listen
	// multiaddr list, embedded in the NeuronP2PExchangeService for
	// registry-backed discovery. The seller typically passes
	// `delivery.FilterMultiaddrs(host.Addrs())` then `.String()`-converts
	// each multiaddr. When empty the descriptor remains Stage 1/2
	// compatible (buyer must fall back to --seller-multiaddr); Stage 3B
	// runs require the slice to be non-empty so the buyer can dial
	// without out-of-band coordination.
	Multiaddrs []string

	// Catalog selects which streams[] entries the descriptor exposes.
	// Defaults to DefaultCatalogOptions (raw-only).
	Catalog CatalogOptions
}

// ServiceDescriptor bundles every artefact a Remote ID seller emits
// during registration / discovery. AgentURI goes on-chain (003 / 007).
// Streams is the seller's stream catalog for inclusion in 008
// connectionSetup (FR-P33a / FR-R02) — for R1 this is not yet sent on
// the wire but is captured in evidence artefacts. FeedSource / CommerceMode /
// ProfileID are heartbeat capability values (005 FR-H05) for the seller's
// future heartbeat publisher.
type ServiceDescriptor struct {
	AgentURI     registry.AgentURI
	Streams      []payment.StreamCatalogEntry
	FeedSource   string
	CommerceMode string
	ProfileID    string
}

// BuildServiceDescriptor constructs a Remote ID service descriptor from
// the given options. The returned descriptor's AgentURI satisfies every
// rule in `internal/registry` ValidateRegistrationCompleteness
// (V-REG-01..V-REG-13): 3 standard channels, p2p topicRef cross-reference,
// commerce delivery.serviceRef cross-reference, peerID derived from key.
//
// FR anchors:
//   - FR-R01: neuron-commerce entry with name="remote-id", pricing.unit="frame"
//   - FR-R02: streams[] catalog includes /ds240/raw/1.0.0
//   - FR-R14/R15: feedSource captured on the descriptor
//   - FR-P58 (008): commerceMode captured on the descriptor
func BuildServiceDescriptor(opts DescriptorOptions) (ServiceDescriptor, error) {
	if opts.ChildKey == nil {
		return ServiceDescriptor{}, errors.New("remoteid.BuildServiceDescriptor: ChildKey is required")
	}

	opts = applyDescriptorDefaults(opts)

	pub := opts.ChildKey.PublicKey()
	peerID, err := pub.PeerID()
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: derive peerID: %w", err)
	}

	// V-REG-03: each topic service needs name/version/channel/transport/anchor/config.
	// V-REG-11: each standard channel (stdIn/stdOut/stdErr) exactly once.
	stdIn, err := registry.NewNeuronTopicService(
		TopicNameStdIn, TopicVersion, "stdIn",
		opts.TopicTransport, opts.Anchor,
		topicConfigFor(opts.TopicConfig, "stdIn"),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: stdIn topic: %w", err)
	}
	stdOut, err := registry.NewNeuronTopicService(
		TopicNameStdOut, TopicVersion, "stdOut",
		opts.TopicTransport, opts.Anchor,
		topicConfigFor(opts.TopicConfig, "stdOut"),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: stdOut topic: %w", err)
	}
	stdErr, err := registry.NewNeuronTopicService(
		TopicNameStdErr, TopicVersion, "stdErr",
		opts.TopicTransport, opts.Anchor,
		topicConfigFor(opts.TopicConfig, "stdErr"),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: stdErr topic: %w", err)
	}

	// V-REG-04: p2p service needs name/version/peerID/protocol/topicRef.
	// V-REG-05: topicRef must match a topic service name → uses TopicNameStdOut.
	// V-REG-12: peerID MUST equal childPublicKey.PeerID().String().
	// Stage 3B: optionally embed listen multiaddrs so the buyer can
	// resolve transport via DiscoverResult.ResolveDialAddrs().
	p2pOpts := []registry.P2PServiceOption{}
	if len(opts.Multiaddrs) > 0 {
		p2pOpts = append(p2pOpts, registry.WithMultiaddrs(opts.Multiaddrs))
	}
	p2pSvc, err := registry.NewNeuronP2PExchangeService(
		P2PServiceName, P2PServiceVersion,
		peerID.String(), ProtocolRaw, TopicNameStdOut,
		p2pOpts...,
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: p2p service: %w", err)
	}

	// V-REG-13: commerce delivery.serviceRef MUST match a p2p service name → P2PServiceName.
	settlementConfig := map[string]any{}
	if opts.ChainID != 0 {
		settlementConfig["chainId"] = opts.ChainID
	}
	if opts.EscrowContract != "" {
		settlementConfig["contract"] = opts.EscrowContract
	}
	commerceSvc, err := payment.NewNeuronCommerceService(
		CommerceServiceName, CommerceServiceVersion,
		payment.DeliveryDescriptor{
			Mode:       payment.DeliveryModeP2P,
			ServiceRef: P2PServiceName,
		},
		payment.SettlementDescriptor{
			Binding: opts.EscrowBinding,
			Config:  settlementConfig,
		},
		payment.PricingDescriptor{
			Amount:   opts.PricingAmount,
			Currency: opts.PricingCurrency,
			Unit:     PricingUnit,
			Interval: opts.PricingInterval,
		},
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: commerce service: %w", err)
	}

	// V-REG-06: DID endpoint must equal childPublicKey.DIDKey().
	didSvc, err := registry.NewDIDService(pub)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: DID service: %w", err)
	}

	agentURI, err := registry.NewAgentURI(
		[]registry.NeuronTopicService{stdIn, stdOut, stdErr},
		[]registry.NeuronP2PExchangeService{p2pSvc},
		&didSvc,
		registry.WithCommerceServices([]payment.NeuronCommerceService{commerceSvc}),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: agentURI: %w", err)
	}

	// Belt-and-braces: re-run the package validator before handing the
	// descriptor back. A wiring bug here would surface much later at
	// register-time, which makes the failure harder to diagnose.
	if valid, vErrs := registry.ValidateRegistrationCompleteness(agentURI, pub); !valid {
		return ServiceDescriptor{}, fmt.Errorf("remoteid.BuildServiceDescriptor: agentURI invalid: %v", vErrs)
	}

	return ServiceDescriptor{
		AgentURI:     agentURI,
		Streams:      BuildRemoteIDStreamCatalog(opts.Catalog),
		FeedSource:   opts.FeedSource,
		CommerceMode: opts.CommerceMode,
		ProfileID:    opts.ProfileID,
	}, nil
}

// topicConfigFor returns the caller-supplied Config map for the given
// channel name when present, or an empty map otherwise. NeuronTopicService
// validation rejects a nil Config (V-REG-03) so we must always return a
// non-nil map.
func topicConfigFor(table map[string]map[string]any, channel string) map[string]any {
	if table == nil {
		return map[string]any{}
	}
	if cfg, ok := table[channel]; ok && cfg != nil {
		// Defensive copy so the caller can't mutate the embedded map.
		out := make(map[string]any, len(cfg))
		for k, v := range cfg {
			out[k] = v
		}
		return out
	}
	return map[string]any{}
}

// applyDescriptorDefaults fills in defaults for empty option fields so a
// caller can pass just {ChildKey: k} and still get a valid descriptor.
func applyDescriptorDefaults(opts DescriptorOptions) DescriptorOptions {
	if opts.EscrowBinding == "" {
		opts.EscrowBinding = DefaultSettlementBinding
	}
	if opts.PricingAmount == "" {
		opts.PricingAmount = "0"
	}
	if opts.PricingCurrency == "" {
		opts.PricingCurrency = "USDC"
	}
	if opts.PricingInterval == "" {
		opts.PricingInterval = "0"
	}
	if opts.FeedSource == "" {
		opts.FeedSource = FeedSourceLive
	}
	if opts.CommerceMode == "" {
		// Stage 2 default: full 008 lifecycle is engaged in the demo CLI.
		// Stage 1's R1 default was "registration-only" — callers who want
		// the R1 short-circuit pass it explicitly.
		opts.CommerceMode = CommerceModeFull
	}
	if opts.ProfileID == "" {
		opts.ProfileID = ProfileR1
	}
	if opts.Anchor == "" {
		opts.Anchor = "r1-eip8004-registry"
	}
	if opts.TopicTransport == "" {
		opts.TopicTransport = "memory"
	}
	return opts
}
