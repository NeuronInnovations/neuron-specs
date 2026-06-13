package registry

import (
	"encoding/json"
	"fmt"

	"github.com/neuron-sdk/neuron-go-sdk/internal/payment"
)

// AgentURI represents the EIP-8004 agent URI containing the services array.
// A complete AgentURI MUST contain exactly 3 neuron-topic services (stdIn,
// stdOut, stdErr), at least 1 neuron-p2p-exchange, optionally 1 DID
// service (FR-R08), and 0–N neuron-commerce services (FR-P05).
type AgentURI struct {
	topicServices    []NeuronTopicService
	p2pServices      []NeuronP2PExchangeService
	commerceServices []payment.NeuronCommerceService
	didServices      []DIDService
}

// TopicServices returns a copy of the neuron-topic services.
func (a *AgentURI) TopicServices() []NeuronTopicService {
	out := make([]NeuronTopicService, len(a.topicServices))
	copy(out, a.topicServices)
	return out
}

// P2PServices returns a copy of the neuron-p2p-exchange services.
func (a *AgentURI) P2PServices() []NeuronP2PExchangeService {
	out := make([]NeuronP2PExchangeService, len(a.p2pServices))
	copy(out, a.p2pServices)
	return out
}

// DIDServices returns a copy of the DID services.
func (a *AgentURI) DIDServices() []DIDService {
	out := make([]DIDService, len(a.didServices))
	copy(out, a.didServices)
	return out
}

// CommerceServices returns a copy of the neuron-commerce services. FR-P05.
func (a *AgentURI) CommerceServices() []payment.NeuronCommerceService {
	out := make([]payment.NeuronCommerceService, len(a.commerceServices))
	copy(out, a.commerceServices)
	return out
}

// jsonAgentURI is the intermediate JSON representation using raw messages
// for heterogeneous service types.
type jsonAgentURI struct {
	Services []json.RawMessage `json:"services"`
}

// serviceTypeDiscriminator extracts the "type" field from a JSON service object.
type serviceTypeDiscriminator struct {
	Type string `json:"type"`
}

// MarshalJSON serializes the AgentURI into the canonical JSON format with
// services ordered: neuron-topic → neuron-p2p-exchange → neuron-commerce → DID.
// Per 003 data-model.md service ordering amendment.
func (a AgentURI) MarshalJSON() ([]byte, error) {
	var services []json.RawMessage

	for _, svc := range a.topicServices {
		data, err := json.Marshal(svc)
		if err != nil {
			return nil, fmt.Errorf("marshal neuron-topic service: %w", err)
		}
		services = append(services, data)
	}

	for _, svc := range a.p2pServices {
		data, err := json.Marshal(svc)
		if err != nil {
			return nil, fmt.Errorf("marshal neuron-p2p-exchange service: %w", err)
		}
		services = append(services, data)
	}

	for _, svc := range a.commerceServices {
		data, err := json.Marshal(svc)
		if err != nil {
			return nil, fmt.Errorf("marshal neuron-commerce service: %w", err)
		}
		services = append(services, data)
	}

	for _, svc := range a.didServices {
		data, err := json.Marshal(svc)
		if err != nil {
			return nil, fmt.Errorf("marshal DID service: %w", err)
		}
		services = append(services, data)
	}

	if services == nil {
		services = []json.RawMessage{}
	}

	return json.Marshal(jsonAgentURI{Services: services})
}

// UnmarshalJSON deserializes the AgentURI from JSON, using the "type" field
// as a discriminator to route each service to the correct typed slice.
func (a *AgentURI) UnmarshalJSON(data []byte) error {
	var raw jsonAgentURI
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal agentURI: %w", err)
	}

	a.topicServices = nil
	a.p2pServices = nil
	a.commerceServices = nil
	a.didServices = nil

	for i, svcData := range raw.Services {
		var disc serviceTypeDiscriminator
		if err := json.Unmarshal(svcData, &disc); err != nil {
			return fmt.Errorf("unmarshal service[%d] type discriminator: %w", i, err)
		}

		switch disc.Type {
		case ServiceTypeNeuronTopic:
			var svc NeuronTopicService
			if err := json.Unmarshal(svcData, &svc); err != nil {
				return fmt.Errorf("unmarshal neuron-topic service[%d]: %w", i, err)
			}
			a.topicServices = append(a.topicServices, svc)

		case ServiceTypeNeuronP2PExchange:
			var svc NeuronP2PExchangeService
			if err := json.Unmarshal(svcData, &svc); err != nil {
				return fmt.Errorf("unmarshal neuron-p2p-exchange service[%d]: %w", i, err)
			}
			a.p2pServices = append(a.p2pServices, svc)

		case ServiceTypeNeuronCommerce:
			var svc payment.NeuronCommerceService
			if err := json.Unmarshal(svcData, &svc); err != nil {
				return fmt.Errorf("unmarshal neuron-commerce service[%d]: %w", i, err)
			}
			a.commerceServices = append(a.commerceServices, svc)

		case ServiceTypeDID:
			var svc DIDService
			if err := json.Unmarshal(svcData, &svc); err != nil {
				return fmt.Errorf("unmarshal DID service[%d]: %w", i, err)
			}
			a.didServices = append(a.didServices, svc)

		default:
			// Unknown service types are silently ignored for forward compatibility (FR-P12a).
		}
	}

	return nil
}

// ToJSON serializes the AgentURI to a JSON string.
func (a AgentURI) ToJSON() (string, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// AgentURIFromJSON parses an AgentURI from a JSON string.
func AgentURIFromJSON(jsonStr string) (AgentURI, error) {
	var uri AgentURI
	if err := json.Unmarshal([]byte(jsonStr), &uri); err != nil {
		return AgentURI{}, err
	}
	return uri, nil
}

// AgentURIOption is a functional option for NewAgentURI.
type AgentURIOption func(*AgentURI)

// WithCommerceServices adds neuron-commerce services to the AgentURI. FR-P05.
func WithCommerceServices(services []payment.NeuronCommerceService) AgentURIOption {
	return func(a *AgentURI) {
		a.commerceServices = services
	}
}

// NewAgentURI constructs a validated AgentURI from the required service
// components. It enforces that exactly 3 neuron-topic services and at least
// 1 neuron-p2p-exchange service are provided. DID and commerce services are optional.
func NewAgentURI(topicServices []NeuronTopicService, p2pServices []NeuronP2PExchangeService, didService *DIDService, opts ...AgentURIOption) (AgentURI, error) {
	if len(topicServices) != 3 {
		return AgentURI{}, IncompleteRegistration{
			Detail: fmt.Sprintf("expected 3 neuron-topic services, got %d", len(topicServices)),
		}
	}
	if len(p2pServices) < 1 {
		return AgentURI{}, IncompleteRegistration{
			Detail: "at least 1 neuron-p2p-exchange service is required",
		}
	}

	uri := AgentURI{
		topicServices: topicServices,
		p2pServices:   p2pServices,
	}

	if didService != nil {
		uri.didServices = []DIDService{*didService}
	}

	for _, opt := range opts {
		opt(&uri)
	}

	return uri, nil
}
