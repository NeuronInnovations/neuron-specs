package registry

import "github.com/neuron-sdk/neuron-go-sdk/internal/keylib"

// ServiceTypeNeuronTopic is the discriminator value for NeuronTopicService.
const ServiceTypeNeuronTopic = "neuron-topic"

// ServiceTypeNeuronP2PExchange is the discriminator value for NeuronP2PExchangeService.
const ServiceTypeNeuronP2PExchange = "neuron-p2p-exchange"

// ServiceTypeNeuronCommerce is the discriminator value for NeuronCommerceService.
// FR-P01: neuron-commerce service type in agentURI services[].
const ServiceTypeNeuronCommerce = "neuron-commerce"

// ServiceTypeDID is the discriminator value for DIDService.
const ServiceTypeDID = "DID"

// standardChannels holds the canonical channel names internally.
var standardChannels = []string{"stdIn", "stdOut", "stdErr"}

// StandardChannels returns a copy of the three required channel roles.
func StandardChannels() []string {
	out := make([]string, len(standardChannels))
	copy(out, standardChannels)
	return out
}

// NeuronTopicService represents a Neuron topic channel entry in the agentURI
// services array. Each Child MUST have three topic services: stdIn, stdOut,
// stdErr (per 004 FR-T14).
type NeuronTopicService struct {
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	Channel   string         `json:"channel"`
	Transport string         `json:"transport"`
	Anchor    string         `json:"anchor"`
	Config    map[string]any `json:"config"`
	Endpoint  string         `json:"endpoint,omitempty"`
}

// TopicServiceOption is a functional option for configuring a NeuronTopicService.
type TopicServiceOption func(*NeuronTopicService)

// WithTopicEndpoint sets the optional endpoint field on a NeuronTopicService.
func WithTopicEndpoint(endpoint string) TopicServiceOption {
	return func(s *NeuronTopicService) {
		s.Endpoint = endpoint
	}
}

// NewNeuronTopicService constructs a validated NeuronTopicService with all
// required fields. Returns InvalidServiceSchema if any MUST field is empty.
func NewNeuronTopicService(name, version, channel, transport, anchor string, config map[string]any, opts ...TopicServiceOption) (NeuronTopicService, error) {
	svc := NeuronTopicService{
		Type:      ServiceTypeNeuronTopic,
		Name:      name,
		Version:   version,
		Channel:   channel,
		Transport: transport,
		Anchor:    anchor,
		Config:    config,
	}

	// Validate all MUST fields.
	type field struct {
		name  string
		value string
	}
	fields := []field{
		{"name", name},
		{"version", version},
		{"channel", channel},
		{"transport", transport},
		{"anchor", anchor},
	}
	for _, f := range fields {
		if f.value == "" {
			return NeuronTopicService{}, InvalidServiceSchema{
				ServiceType: ServiceTypeNeuronTopic,
				Field:       f.name,
				Detail:      "must not be empty",
			}
		}
	}
	if config == nil {
		return NeuronTopicService{}, InvalidServiceSchema{
			ServiceType: ServiceTypeNeuronTopic,
			Field:       "config",
			Detail:      "must not be empty",
		}
	}

	for _, opt := range opts {
		opt(&svc)
	}

	return svc, nil
}

// NeuronP2PExchangeService represents a peer-to-peer exchange endpoint
// in the agentURI services array. It references a topic by name via
// TopicRef, enabling bidirectional communication (per 004 FR-T17).
//
// Multiaddrs (added in the Stage 3B amendment to the Remote ID DApp) is
// the producer's optional list of current libp2p listen multiaddrs.
// When non-empty, registry-backed buyers can discover the producer's
// transport without out-of-band coordination (just `peer.AddrInfo` =
// {PeerID, Multiaddrs}). Consumers MUST treat the field as advisory —
// the addresses may have changed since registration. Empty is valid;
// older descriptors omit the field entirely.
type NeuronP2PExchangeService struct {
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	PeerID     string   `json:"peerID"`
	Protocol   string   `json:"protocol"`
	TopicRef   string   `json:"topicRef"`
	Multiaddrs []string `json:"multiaddrs,omitempty"`
}

// P2PServiceOption is a functional option for configuring a NeuronP2PExchangeService.
type P2PServiceOption func(*NeuronP2PExchangeService)

// WithMultiaddrs sets the optional Multiaddrs slice on a
// NeuronP2PExchangeService. Use when registering a Remote ID seller
// that wants registry-only discovery (Stage 3B): the buyer reads
// `Multiaddrs` straight from the AgentURI and dials without
// out-of-band coordination. Empty / nil slices are no-ops.
func WithMultiaddrs(addrs []string) P2PServiceOption {
	return func(s *NeuronP2PExchangeService) {
		if len(addrs) == 0 {
			return
		}
		s.Multiaddrs = append([]string(nil), addrs...)
	}
}

// NewNeuronP2PExchangeService constructs a validated NeuronP2PExchangeService
// with all required fields. Returns InvalidServiceSchema if any MUST field is empty.
func NewNeuronP2PExchangeService(name, version, peerID, protocol, topicRef string, opts ...P2PServiceOption) (NeuronP2PExchangeService, error) {
	svc := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     name,
		Version:  version,
		PeerID:   peerID,
		Protocol: protocol,
		TopicRef: topicRef,
	}

	type field struct {
		name  string
		value string
	}
	fields := []field{
		{"name", name},
		{"version", version},
		{"peerID", peerID},
		{"protocol", protocol},
		{"topicRef", topicRef},
	}
	for _, f := range fields {
		if f.value == "" {
			return NeuronP2PExchangeService{}, InvalidServiceSchema{
				ServiceType: ServiceTypeNeuronP2PExchange,
				Field:       f.name,
				Detail:      "must not be empty",
			}
		}
	}

	for _, opt := range opts {
		opt(&svc)
	}

	return svc, nil
}

// DIDService represents a generic DID service entry in the agentURI services
// array (per FR-R13/FR-R14). This covers non-Neuron-specific services such
// as external API endpoints.
type DIDService struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version"`
}

// NewDIDService constructs a DIDService from a child's public key. It derives
// the did:key identifier using the child's compressed secp256k1 public key.
func NewDIDService(childPublicKey keylib.NeuronPublicKey) (DIDService, error) {
	didKey := childPublicKey.DIDKey()

	return DIDService{
		Type:     ServiceTypeDID,
		Name:     "DID",
		Endpoint: didKey,
		Version:  "v1",
	}, nil
}
