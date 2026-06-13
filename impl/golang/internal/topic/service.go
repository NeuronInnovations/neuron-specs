package topic

import (
	"encoding/json"
	"fmt"
)

// HCSConfig represents the transport configuration for an HCS-backed topic service.
type HCSConfig struct {
	Network string `json:"network"`
	TopicId string `json:"topicId"`
}

// ERCLogConfig represents the transport configuration for an ERC event log topic service.
type ERCLogConfig struct {
	ChainId         uint64 `json:"chainId"`
	ContractAddress string `json:"contractAddress"`
	EventSignature  string `json:"eventSignature"`
}

// FR-T16: Anchoring for non-ledger transports
// AnchoringConfig defines how a non-ledger transport anchors its messages to a ledger.
type AnchoringConfig struct {
	Method        string `json:"method"`
	AnchorTopicId string `json:"anchorTopicId"`
	AnchorNetwork string `json:"anchorNetwork"`
	Interval      string `json:"interval"`
}

// KafkaConfig represents the transport configuration for a Kafka-backed topic service.
type KafkaConfig struct {
	BootstrapServers []string        `json:"bootstrapServers"`
	TopicName        string          `json:"topicName"`
	SASLMechanism    string          `json:"saslMechanism,omitempty"`
	Anchoring        AnchoringConfig `json:"anchoring"`
}

// FR-T14: EIP-8004 service representation (type: neuron-topic)
// NeuronTopicServiceDef is the Spec 004 representation of a neuron-topic service entry
// as found in an agentURI JSON document. It captures the typed transport configuration
// and channel binding for a single topic service.
type NeuronTopicServiceDef struct {
	Type      string      `json:"type"`
	Name      string      `json:"name"`
	Endpoint  string      `json:"endpoint,omitempty"`
	Version   string      `json:"version"`
	Channel   string      `json:"channel"`
	Transport string      `json:"transport"`
	Anchor    string      `json:"anchor"`
	Config    interface{} `json:"config"`
}

// NeuronP2PExchangeServiceDef is defined in p2p_exchange.go to avoid circular references.
// This forward declaration comment is for documentation only.

// agentURIDocument is a minimal representation of an agentURI JSON document.
// It is used only for parsing purposes; the full schema is defined in the registry package.
type agentURIDocument struct {
	Services []json.RawMessage `json:"services"`
}

// serviceTypeField is used to peek at the "type" field of a service entry.
type serviceTypeField struct {
	Type string `json:"type"`
}

// FR-T15: Transport config schemas (HCS, ERC-log, Kafka)
// ValidateTransportConfig validates a transport configuration map against the
// schema for the given transport type. Returns ErrInvalidConfig if required
// fields are missing.
func ValidateTransportConfig(transport string, config map[string]interface{}) error {
	switch transport {
	case "hcs":
		return validateHCSConfig(config)
	case "erc-log":
		return validateERCLogConfig(config)
	case "kafka":
		return validateKafkaConfig(config)
	default:
		return NewTopicError(ErrUnsupportedTransport,
			fmt.Sprintf("unknown transport for config validation: %s", transport))
	}
}

func validateHCSConfig(config map[string]interface{}) error {
	if _, ok := config["network"]; !ok {
		return NewTopicError(ErrInvalidConfig, "HCS config missing required field: network")
	}
	if _, ok := config["topicId"]; !ok {
		return NewTopicError(ErrInvalidConfig, "HCS config missing required field: topicId")
	}
	return nil
}

func validateERCLogConfig(config map[string]interface{}) error {
	if _, ok := config["chainId"]; !ok {
		return NewTopicError(ErrInvalidConfig, "ERC-log config missing required field: chainId")
	}
	if _, ok := config["contractAddress"]; !ok {
		return NewTopicError(ErrInvalidConfig, "ERC-log config missing required field: contractAddress")
	}
	if _, ok := config["eventSignature"]; !ok {
		return NewTopicError(ErrInvalidConfig, "ERC-log config missing required field: eventSignature")
	}
	return nil
}

func validateKafkaConfig(config map[string]interface{}) error {
	if _, ok := config["bootstrapServers"]; !ok {
		return NewTopicError(ErrInvalidConfig, "Kafka config missing required field: bootstrapServers")
	}
	if _, ok := config["topicName"]; !ok {
		return NewTopicError(ErrInvalidConfig, "Kafka config missing required field: topicName")
	}
	anchoring, ok := config["anchoring"]
	if !ok {
		return NewTopicError(ErrInvalidConfig, "Kafka config missing required field: anchoring")
	}
	// Validate the anchoring sub-object if it is a map.
	if anchorMap, ok := anchoring.(map[string]interface{}); ok {
		for _, field := range []string{"method", "anchorTopicId", "anchorNetwork", "interval"} {
			if _, ok := anchorMap[field]; !ok {
				return NewTopicError(ErrInvalidConfig,
					fmt.Sprintf("Kafka anchoring config missing required field: %s", field))
			}
		}
	}
	return nil
}

// SerializeTopicService serializes a NeuronTopicServiceDef to JSON.
func SerializeTopicService(svc NeuronTopicServiceDef) ([]byte, error) {
	return json.Marshal(svc)
}

// FR-T09: Topic discovery via Peer Registry
// ParseAgentURIServices parses an agentURI JSON document and extracts
// neuron-topic and neuron-p2p-exchange service entries.
// Returns separate slices for topic services and P2P exchange services.
func ParseAgentURIServices(jsonBytes []byte) ([]NeuronTopicServiceDef, []NeuronP2PExchangeServiceDef, error) {
	var doc agentURIDocument
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return nil, nil, WrapTopicError(ErrInvalidConfig, "failed to parse agentURI JSON", err)
	}

	var topicServices []NeuronTopicServiceDef
	var p2pServices []NeuronP2PExchangeServiceDef

	for _, raw := range doc.Services {
		var typeField serviceTypeField
		if err := json.Unmarshal(raw, &typeField); err != nil {
			continue // Skip entries that cannot be parsed.
		}

		switch typeField.Type {
		case "neuron-topic":
			var svc NeuronTopicServiceDef
			if err := json.Unmarshal(raw, &svc); err != nil {
				continue
			}
			topicServices = append(topicServices, svc)
		case "neuron-p2p-exchange":
			var svc NeuronP2PExchangeServiceDef
			if err := json.Unmarshal(raw, &svc); err != nil {
				continue
			}
			p2pServices = append(p2pServices, svc)
		}
	}

	return topicServices, p2pServices, nil
}

// ExtractTopicRef extracts a TopicRef from a NeuronTopicServiceDef.
// It uses the transport field to determine the BackendKind, then builds
// a locator from the typed config.
func ExtractTopicRef(svc NeuronTopicServiceDef) (TopicRef, error) {
	transport, err := ParseBackendKind(svc.Transport)
	if err != nil {
		return TopicRef{}, err
	}

	// Re-marshal the config to a map for uniform field extraction.
	configBytes, err := json.Marshal(svc.Config)
	if err != nil {
		return TopicRef{}, WrapTopicError(ErrInvalidConfig, "failed to marshal service config", err)
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(configBytes, &configMap); err != nil {
		return TopicRef{}, WrapTopicError(ErrInvalidConfig, "service config is not a JSON object", err)
	}

	var locator string
	switch transport {
	case BackendHCS:
		topicId, ok := configMap["topicId"]
		if !ok {
			return TopicRef{}, NewTopicError(ErrInvalidConfig, "HCS config missing topicId")
		}
		locator = fmt.Sprintf("%v", topicId)
	case BackendERCLog:
		addr, ok := configMap["contractAddress"]
		if !ok {
			return TopicRef{}, NewTopicError(ErrInvalidConfig, "ERC-log config missing contractAddress")
		}
		locator = fmt.Sprintf("%v", addr)
	case BackendKafka:
		name, ok := configMap["topicName"]
		if !ok {
			return TopicRef{}, NewTopicError(ErrInvalidConfig, "Kafka config missing topicName")
		}
		locator = fmt.Sprintf("%v", name)
	default:
		return TopicRef{}, NewTopicError(ErrUnsupportedTransport,
			fmt.Sprintf("cannot extract locator for transport: %s", transport))
	}

	return NewTopicRef(transport, locator)
}

// DiscoverPeerChannels parses an agentURI JSON document and returns a map of
// standard channel roles to their TopicRef. It extracts all neuron-topic services
// and maps the ones with standard channel names (stdIn, stdOut, stdErr).
func DiscoverPeerChannels(agentURIJson []byte) (map[ChannelRole]TopicRef, error) {
	topicServices, _, err := ParseAgentURIServices(agentURIJson)
	if err != nil {
		return nil, err
	}

	result := make(map[ChannelRole]TopicRef)
	for _, svc := range topicServices {
		role, err := ChannelRoleFromString(svc.Channel)
		if err != nil {
			// Skip non-standard channels or invalid names.
			continue
		}

		ref, err := ExtractTopicRef(svc)
		if err != nil {
			continue
		}

		// Only include standard channel roles in the discovery result.
		if !IsCustomChannel(role) {
			result[role] = ref
		}
	}

	return result, nil
}

// FindStdIn finds the first neuron-topic service with channel "stdIn".
func FindStdIn(services []NeuronTopicServiceDef) (*NeuronTopicServiceDef, error) {
	return findServiceByChannel(services, string(ChannelStdIn))
}

// FindStdOut finds the first neuron-topic service with channel "stdOut".
func FindStdOut(services []NeuronTopicServiceDef) (*NeuronTopicServiceDef, error) {
	return findServiceByChannel(services, string(ChannelStdOut))
}

// FindStdErr finds the first neuron-topic service with channel "stdErr".
func FindStdErr(services []NeuronTopicServiceDef) (*NeuronTopicServiceDef, error) {
	return findServiceByChannel(services, string(ChannelStdErr))
}

// findServiceByChannel returns the first service with the matching channel name.
// Returns nil and ErrTopicNotFound if no matching service is found.
func findServiceByChannel(services []NeuronTopicServiceDef, channel string) (*NeuronTopicServiceDef, error) {
	for i := range services {
		if services[i].Channel == channel {
			return &services[i], nil
		}
	}
	return nil, NewTopicError(ErrTopicNotFound,
		fmt.Sprintf("no neuron-topic service found with channel: %s", channel))
}
