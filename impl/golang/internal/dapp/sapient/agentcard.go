package sapient

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
	"github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// Agent Card naming / version constants for the SAPIENT Remote ID seller.
// The card IS the seller's EIP-8004 agentURI registration file (015 FR-S20a);
// these mirror the remote-id DApp conventions (internal/dapp/remoteid) so the
// SAPIENT seller registers through the same registry machinery.
const (
	// CommerceServiceName is the neuron-commerce service name advertised by a
	// SAPIENT seller — the "rid" service. It is an advertisement/discovery
	// descriptor only (settlement binding "none", price 0); no payment flow is
	// engaged (deferred). FR-S20a / 017.
	CommerceServiceName = "rid"

	// CommerceServiceVersion is stamped on the neuron-commerce entry.
	CommerceServiceVersion = "1.0.0"

	// P2PServiceName is the neuron-p2p-exchange name carrying the assembled
	// SAPIENT DetectionReport stream. The commerce service's delivery.serviceRef
	// points here (V-REG-13); the p2p topicRef points at stdOut (V-REG-05).
	P2PServiceName = "sapient-detection"

	// P2PServiceVersion mirrors the /sapient/detection protocol-ID major.minor.
	P2PServiceVersion = "2.0.0"

	// TopicNameStdIn / StdOut / StdErr are the three canonical topic service
	// names. They model the SAPIENT 004 auditable lane (FR-S92): Task in on
	// stdIn; TaskAck / StatusReport / Registration out on stdOut; errors on
	// stdErr. The p2p topicRef points at stdOut.
	TopicNameStdIn  = "sapient-stdin"
	TopicNameStdOut = "sapient-stdout"
	TopicNameStdErr = "sapient-stderr"

	// TopicVersion is stamped on every NeuronTopicService.
	TopicVersion = "1.0.0"

	// ExtensionID is the capability-extension identifier under which the
	// SAPIENT/RID-specific metadata is namespaced inside the stdOut topic
	// service Config map. It is also used as the commerce service termsRef.
	ExtensionID = "neuron.rid/1"

	// SapientWire names the on-the-wire format the seller speaks on
	// /sapient/detection/2.0.0 (the authoritative wire is protobuf; protojson
	// is inspection-only).
	SapientWire = "BSI Flex 335 v2.0 protobuf"

	// DefaultAnchor is the NeuronTopicService.anchor (required non-empty by
	// V-REG-03). Mirrors remote-id's r1 anchor; identifies the demo profile.
	DefaultAnchor = "sapient-rid-r1"

	// DefaultTopicTransport is the NeuronTopicService.transport (required
	// non-empty by V-REG-03). The local demo carries the auditable lane over a
	// file; a Profile-E deployment would use "hcs".
	DefaultTopicTransport = "auditlane-file"
)

// DefaultSensorModels is the DroneScout sensor family the reference
// neuron-rid-bridge sources. Advertised as a capability in the neuron.rid/1
// extension. Override via SellerCardOptions.SensorModels.
var DefaultSensorModels = []string{"DroneScout DS240", "DroneScout DS-400"}

// detectionReportSchema is the vendored BSI Flex 335 v2.0 DetectionReport proto
// — the authoritative wire schema for /sapient/detection/2.0.0. Its SHA-256 is
// published in the neuron.rid/1 extension so a verifier can confirm the seller's
// asserted schema matches the bytes in the repo.
//
//go:embed sapientpb/proto/sapient_msg/bsi_flex_335_v2_0/detection_report.proto
var detectionReportSchema []byte

// SchemaSha256 returns the hex-encoded SHA-256 of the vendored DetectionReport
// proto schema. Deterministic; the value lands in every seller's Agent Card.
func SchemaSha256() string {
	sum := sha256.Sum256(detectionReportSchema)
	return hex.EncodeToString(sum[:])
}

// SellerCardOptions configures BuildSellerCard. ChildKey is mandatory; the rest
// carry safe defaults so a CLI can pass {ChildKey: k} and get a valid card.
type SellerCardOptions struct {
	// ChildKey is the seller's secp256k1 key. Used to derive PeerID, EVMAddress,
	// node_id, and the DID service entry. REQUIRED.
	ChildKey *keylib.NeuronPrivateKey

	// Multiaddrs is the seller's optional libp2p listen multiaddr list, embedded
	// in the NeuronP2PExchangeService for registry-backed discovery. In the
	// reverse-connect topology the seller has no public listen address, so this
	// is usually empty; included for completeness / parity with remote-id.
	Multiaddrs []string

	// SensorModels overrides the profile's sensor models in the capability
	// extension (DefaultSensorModels on the default RID profile).
	SensorModels []string

	// Profile selects the seller's modality posture (service name, extension
	// id, anchor, capabilities). nil means RIDProfile() — byte-identical to the
	// pre-profile card (load-bearing for the AgentURI SHA-256 and idempotent
	// RegisterOrUpdate on deployed sellers).
	Profile *SellerProfile

	// ChainID is the EVM chain ID where the Identity Registry lives. 0 is valid
	// (memory-contract / local evidence mode). Embedded in the settlement
	// config when Commerce is set.
	ChainID uint64

	// Commerce, when non-nil, switches the rid commerce entry from the
	// advertisement-only posture (settlement "none", price 0) to a REAL
	// settlement advertisement (escrow binding + contract config + non-zero
	// pricing) and embeds resolvable topic locators per channel so a buyer
	// can drive the full 008 lifecycle from the card alone. nil keeps the
	// card byte-identical to the original advertisement-only shape.
	Commerce *CommerceCardOptions

	// TopicTransport / TopicConfig disclose the audit-lane (004) backend on the
	// NON-commerce card. Empty TopicTransport keeps DefaultTopicTransport
	// ("auditlane-file") with no locators (file mode — no overclaim); "hcs"
	// advertises the real HCS topic transport and TopicConfig (keyed
	// "stdIn"/"stdOut"/"stdErr" → {"topicId": "0.0.X"}) carries the topic IDs.
	// The Commerce variant carries its own transport/locators and takes
	// precedence when both are set.
	TopicTransport string
	TopicConfig    map[string]map[string]any
}

// CommerceCardOptions parameterises the full-payment variant of the card.
type CommerceCardOptions struct {
	// SettlementBinding is the SettlementDescriptor.Binding. Defaults to
	// "evm-escrow"; in-process tests pass "memory".
	SettlementBinding string

	// EscrowContract / TokenContract are embedded in the settlement config
	// ("contract" / "token") so the buyer learns the escrow coordinates from
	// the card. Optional for memory-binding tests.
	EscrowContract string
	TokenContract  string

	// Pricing per FR-P03. Amount defaults to "1" (the EVM escrow path
	// rejects 0 — anti-scope rule); Currency defaults to "NTT" (the demo
	// TestToken); Interval defaults to "0" (one-shot session).
	PricingAmount   string
	PricingCurrency string
	PricingInterval string

	// TopicConfig embeds resolvable locators per standard channel, keyed by
	// "stdIn"/"stdOut"/"stdErr" (e.g. {"topicId": "0.0.123"}). Merged into
	// the channel's Config map (stdOut keeps the neuron.rid/1 extension).
	TopicConfig map[string]map[string]any

	// TopicTransport overrides DefaultTopicTransport ("auditlane-file") —
	// commerce runs pass "memory" or "hcs" to disclose the real lane.
	TopicTransport string
}

// SellerCard bundles the artefacts a SAPIENT seller emits at registration.
// AgentURI is the on-chain card (003/007); Streams is the 008 connectionSetup
// stream catalog captured for evidence (not part of the on-chain card); NodeID,
// PeerID, and SchemaSha256 are surfaced for the evidence record / explorer.
type SellerCard struct {
	AgentURI     registry.AgentURI
	Streams      []payment.StreamCatalogEntry
	NodeID       string
	PeerID       string
	SchemaSha256 string
}

// BuildSellerCard constructs the SAPIENT seller's Agent Card from the given
// options. The returned AgentURI satisfies every rule in
// registry.ValidateRegistrationCompleteness (V-REG-01..V-REG-13): 3 standard
// channels, p2p topicRef → stdOut, commerce delivery.serviceRef → p2p name,
// peerID derived from ChildKey, DID derived from ChildKey.
//
// FR anchors:
//   - 015 FR-S20a: the agent-card envelope IS the 003 agentURI registration file.
//   - 015 FR-S11: p2p service advertises protocol /sapient/detection/2.0.0.
//   - 015 FR-S94: node_id is identity-bound (NodeIDFromIdentity(EVM)) and
//     surfaced in the neuron.rid/1 extension alongside the PeerID binding.
func BuildSellerCard(opts SellerCardOptions) (SellerCard, error) {
	if opts.ChildKey == nil {
		return SellerCard{}, errors.New("sapient.BuildSellerCard: ChildKey is required")
	}
	pub := opts.ChildKey.PublicKey()
	peerID, err := pub.PeerID()
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: derive peerID: %w", err)
	}
	evmHex := pub.EVMAddress().Hex()
	nodeID := NodeIDFromIdentity(evmHex)

	prof := RIDProfile()
	if opts.Profile != nil {
		prof = *opts.Profile
	}

	models := opts.SensorModels
	if len(models) == 0 {
		models = prof.SensorModels
	}

	// The capability extension (neuron.rid/1 on the default profile).
	// Namespaced under the extension id inside the stdOut topic Config (the
	// only free-form field in the core AgentURI types). encoding/json sorts map
	// keys, so the card JSON is deterministic (load-bearing for the AgentURI
	// SHA-256 and the idempotent RegisterOrUpdate string compare).
	extension := map[string]any{
		"nodeId":       nodeID,
		"wire":         SapientWire,
		"schema":       SchemaURL,
		"schemaSha256": SchemaSha256(),
		"sensorModels": append([]string(nil), models...),
	}
	// The capabilities key is profile-gated: nil on the RID profile keeps
	// pre-profile cards byte-identical.
	if len(prof.Capabilities) > 0 {
		extension["capabilities"] = append([]string(nil), prof.Capabilities...)
	}

	// V-REG-03 / V-REG-11: each standard channel exactly once, with non-empty
	// name/version/channel/transport/anchor and a non-nil config. The commerce
	// variant merges resolvable topic locators into the channel configs and
	// discloses the real lane transport; the default stays byte-identical.
	transport := DefaultTopicTransport
	stdInCfg := map[string]any{}
	stdOutCfg := map[string]any{prof.ExtensionID: extension}
	stdErrCfg := map[string]any{}
	applyTopicCfg := func(t string, cfg map[string]map[string]any) {
		if t != "" {
			transport = t
		}
		for ch, c := range cfg {
			switch ch {
			case "stdIn":
				maps.Copy(stdInCfg, c)
			case "stdOut":
				maps.Copy(stdOutCfg, c)
			case "stdErr":
				maps.Copy(stdErrCfg, c)
			}
		}
	}
	// Non-commerce audit-lane disclosure (file vs hcs) first; the Commerce
	// variant then overrides when present (both empty keeps DefaultTopicTransport).
	applyTopicCfg(opts.TopicTransport, opts.TopicConfig)
	if opts.Commerce != nil {
		applyTopicCfg(opts.Commerce.TopicTransport, opts.Commerce.TopicConfig)
	}
	stdIn, err := registry.NewNeuronTopicService(
		TopicNameStdIn, TopicVersion, "stdIn",
		transport, prof.Anchor, stdInCfg,
	)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: stdIn topic: %w", err)
	}
	stdOut, err := registry.NewNeuronTopicService(
		TopicNameStdOut, TopicVersion, "stdOut",
		transport, prof.Anchor, stdOutCfg,
	)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: stdOut topic: %w", err)
	}
	stdErr, err := registry.NewNeuronTopicService(
		TopicNameStdErr, TopicVersion, "stdErr",
		transport, prof.Anchor, stdErrCfg,
	)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: stdErr topic: %w", err)
	}

	// V-REG-04 / V-REG-05 / V-REG-12: p2p service carries the detection protocol,
	// topicRef → stdOut, peerID == ChildKey.PeerID.
	p2pOpts := []registry.P2PServiceOption{}
	if len(opts.Multiaddrs) > 0 {
		p2pOpts = append(p2pOpts, registry.WithMultiaddrs(opts.Multiaddrs))
	}
	p2pSvc, err := registry.NewNeuronP2PExchangeService(
		P2PServiceName, P2PServiceVersion,
		peerID.String(), ProtocolDetection, TopicNameStdOut,
		p2pOpts...,
	)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: p2p service: %w", err)
	}

	// V-REG-13: commerce delivery.serviceRef → p2p name. Default posture is
	// advertisement-only (settlement binding "none", price 0 — no payment flow
	// engaged). The Commerce variant advertises the REAL settlement: escrow
	// binding + contract/token/chainId config + non-zero pricing, so a buyer
	// can drive the full 008 lifecycle from the resolved card.
	settlement := payment.SettlementDescriptor{Binding: "none"}
	pricing := payment.PricingDescriptor{
		Amount:   "0",
		Currency: "USDC",
		Unit:     "detection",
		Interval: "0",
	}
	if opts.Commerce != nil {
		binding := opts.Commerce.SettlementBinding
		if binding == "" {
			binding = "evm-escrow"
		}
		settlementCfg := map[string]any{}
		if opts.ChainID != 0 {
			settlementCfg["chainId"] = opts.ChainID
		}
		if opts.Commerce.EscrowContract != "" {
			settlementCfg["contract"] = opts.Commerce.EscrowContract
		}
		if opts.Commerce.TokenContract != "" {
			settlementCfg["token"] = opts.Commerce.TokenContract
		}
		settlement = payment.SettlementDescriptor{Binding: binding, Config: settlementCfg}

		amount := opts.Commerce.PricingAmount
		if amount == "" {
			amount = "1"
		}
		currency := opts.Commerce.PricingCurrency
		if currency == "" {
			currency = "NTT"
		}
		interval := opts.Commerce.PricingInterval
		if interval == "" {
			interval = "0"
		}
		pricing = payment.PricingDescriptor{
			Amount:   amount,
			Currency: currency,
			Unit:     "detection",
			Interval: interval,
		}
	}
	commerceSvc, err := payment.NewNeuronCommerceService(
		prof.CommerceServiceName, CommerceServiceVersion,
		payment.DeliveryDescriptor{
			Mode:       payment.DeliveryModeP2P,
			ServiceRef: P2PServiceName,
		},
		settlement,
		pricing,
		payment.WithTermsRef(prof.ExtensionID),
	)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: commerce service: %w", err)
	}

	// V-REG-06: DID endpoint == ChildKey.DIDKey().
	didSvc, err := registry.NewDIDService(pub)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: DID service: %w", err)
	}

	agentURI, err := registry.NewAgentURI(
		[]registry.NeuronTopicService{stdIn, stdOut, stdErr},
		[]registry.NeuronP2PExchangeService{p2pSvc},
		&didSvc,
		registry.WithCommerceServices([]payment.NeuronCommerceService{commerceSvc}),
	)
	if err != nil {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: agentURI: %w", err)
	}

	// Belt-and-braces: re-run the registry validator so a wiring bug surfaces
	// here, not at register time.
	if valid, vErrs := registry.ValidateRegistrationCompleteness(agentURI, pub); !valid {
		return SellerCard{}, fmt.Errorf("sapient.BuildSellerCard: agentURI invalid: %v", vErrs)
	}

	return SellerCard{
		AgentURI:     agentURI,
		Streams:      BuildSapientStreamCatalog(DefaultCatalogOptions()),
		NodeID:       nodeID,
		PeerID:       peerID.String(),
		SchemaSha256: SchemaSha256(),
	}, nil
}
