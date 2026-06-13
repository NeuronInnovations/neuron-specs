package adsb

import (
	"errors"
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// Descriptor naming / version constants per docs/normalized-track-contract.md
// + Spec 016 FR-A01 / FR-A02 / FR-A18 + Spec 008 FR-P58 + Spec 013 FR-F-02.
// Mirrors internal/dapp/remoteid with the ADS-B substitutions.
const (
	// CommerceServiceName is the canonical neuron-commerce service name for
	// the ADS-B BaseStation DApp. Aligns with Spec 016 FR-A01 (`name="adsb"`).
	CommerceServiceName = "adsb"

	// CommerceServiceVersion is stamped on the neuron-commerce entry.
	CommerceServiceVersion = "1.0.0"

	// P2PServiceName is the neuron-p2p-exchange entry for the basestation
	// stream. Referenced by commerce.delivery.serviceRef.
	P2PServiceName = "adsb-p2p"

	// P2PServiceVersion mirrors the protocol-ID minor version.
	P2PServiceVersion = "1.0.0"

	// TopicNameStdIn / StdOut / StdErr are the three canonical topic
	// service names — the prefix is `adsb-` rather than remoteid's
	// `remoteid-` so HashScan-style explorers can group them.
	TopicNameStdIn  = "adsb-stdin"
	TopicNameStdOut = "adsb-stdout"
	TopicNameStdErr = "adsb-stderr"

	// TopicVersion stamps every NeuronTopicService.
	TopicVersion = "1.0.0"

	// PricingUnit is the canonical pricing.unit per Spec 016 FR-A01.
	PricingUnit = "frame"

	// ProfileR1 is the registry-backed profile identifier (013 FR-F-02).
	ProfileR1 = "r1-eip8004-registry/1"

	// ProfileFixture is the fixture-direct profile (013 FR-F-02 Profile F).
	ProfileFixture = "f-fixture-direct/1"

	// Commerce-mode values per Spec 008 FR-P58.
	CommerceModeFull             = "full"
	CommerceModeRegistrationOnly = "registration-only"
	CommerceModeDataOnly         = "data-only"

	// Feed-source values per Spec 016 FR-A18 (live/replay/synthetic/placeholder).
	// BaseStation TCP sources use FeedSourceLive with feedSourceConfig.upstream
	// = "basestation:<host:port>" per the BaseStation fast-fusion audit
	// §5.5 / decision Q-5.
	FeedSourceLive        = "live"
	FeedSourceReplay      = "replay"
	FeedSourceSynthetic   = "synthetic"
	FeedSourcePlaceholder = "placeholder"

	// DefaultSettlementBinding is the binding value embedded in the
	// neuron-commerce service descriptor by default. CLI overrides via
	// DescriptorOptions.EscrowBinding.
	DefaultSettlementBinding = "evm-escrow"
)

// DescriptorOptions configures the ADS-B service descriptor. ChildKey is
// mandatory; everything else carries a default.
type DescriptorOptions struct {
	ChildKey *keylib.NeuronPrivateKey

	ChainID         uint64
	EscrowBinding   string
	EscrowContract  string
	PricingAmount   string
	PricingCurrency string
	PricingInterval string

	FeedSource   string
	CommerceMode string
	ProfileID    string

	Anchor         string
	TopicTransport string
	TopicConfig    map[string]map[string]any

	Multiaddrs []string
	Catalog    CatalogOptions
}

// ServiceDescriptor bundles every artefact the ADS-B BaseStation seller
// emits during registration / discovery.
type ServiceDescriptor struct {
	AgentURI     registry.AgentURI
	Streams      []payment.StreamCatalogEntry
	FeedSource   string
	CommerceMode string
	ProfileID    string
}

// BuildServiceDescriptor mirrors internal/dapp/remoteid.BuildServiceDescriptor
// with ADS-B-specific constants. Satisfies V-REG-01..V-REG-13 against
// internal/registry.ValidateRegistrationCompleteness.
func BuildServiceDescriptor(opts DescriptorOptions) (ServiceDescriptor, error) {
	if opts.ChildKey == nil {
		return ServiceDescriptor{}, errors.New("adsb.BuildServiceDescriptor: ChildKey is required")
	}

	opts = applyDescriptorDefaults(opts)

	pub := opts.ChildKey.PublicKey()
	peerID, err := pub.PeerID()
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: derive peerID: %w", err)
	}

	stdIn, err := registry.NewNeuronTopicService(
		TopicNameStdIn, TopicVersion, "stdIn",
		opts.TopicTransport, opts.Anchor,
		topicConfigFor(opts.TopicConfig, "stdIn"),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: stdIn topic: %w", err)
	}
	stdOut, err := registry.NewNeuronTopicService(
		TopicNameStdOut, TopicVersion, "stdOut",
		opts.TopicTransport, opts.Anchor,
		topicConfigFor(opts.TopicConfig, "stdOut"),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: stdOut topic: %w", err)
	}
	stdErr, err := registry.NewNeuronTopicService(
		TopicNameStdErr, TopicVersion, "stdErr",
		opts.TopicTransport, opts.Anchor,
		topicConfigFor(opts.TopicConfig, "stdErr"),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: stdErr topic: %w", err)
	}

	p2pOpts := []registry.P2PServiceOption{}
	if len(opts.Multiaddrs) > 0 {
		p2pOpts = append(p2pOpts, registry.WithMultiaddrs(opts.Multiaddrs))
	}
	p2pSvc, err := registry.NewNeuronP2PExchangeService(
		P2PServiceName, P2PServiceVersion,
		peerID.String(), ProtocolBaseStation, TopicNameStdOut,
		p2pOpts...,
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: p2p service: %w", err)
	}

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
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: commerce service: %w", err)
	}

	didSvc, err := registry.NewDIDService(pub)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: DID service: %w", err)
	}

	agentURI, err := registry.NewAgentURI(
		[]registry.NeuronTopicService{stdIn, stdOut, stdErr},
		[]registry.NeuronP2PExchangeService{p2pSvc},
		&didSvc,
		registry.WithCommerceServices([]payment.NeuronCommerceService{commerceSvc}),
	)
	if err != nil {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: agentURI: %w", err)
	}
	if valid, vErrs := registry.ValidateRegistrationCompleteness(agentURI, pub); !valid {
		return ServiceDescriptor{}, fmt.Errorf("adsb.BuildServiceDescriptor: agentURI invalid: %v", vErrs)
	}

	return ServiceDescriptor{
		AgentURI:     agentURI,
		Streams:      BuildAdsbBasestationStreamCatalog(opts.Catalog),
		FeedSource:   opts.FeedSource,
		CommerceMode: opts.CommerceMode,
		ProfileID:    opts.ProfileID,
	}, nil
}

func topicConfigFor(table map[string]map[string]any, channel string) map[string]any {
	if table == nil {
		return map[string]any{}
	}
	if cfg, ok := table[channel]; ok && cfg != nil {
		out := make(map[string]any, len(cfg))
		for k, v := range cfg {
			out[k] = v
		}
		return out
	}
	return map[string]any{}
}

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
