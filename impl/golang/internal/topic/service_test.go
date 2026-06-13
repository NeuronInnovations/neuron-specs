package topic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Service Schema Round-Trip Tests ---

func TestNeuronTopicServiceDef_HCSConfig_RoundTrip(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Type:      "neuron-topic",
		Name:      "agent-stdin",
		Version:   "1.0.0",
		Channel:   "stdIn",
		Transport: "hcs",
		Anchor:    "native",
		Config: HCSConfig{
			Network: "mainnet",
			TopicId: "0.0.12345",
		},
	}

	data, err := SerializeTopicService(svc)
	require.NoError(t, err)

	var restored NeuronTopicServiceDef
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, svc.Type, restored.Type)
	assert.Equal(t, svc.Name, restored.Name)
	assert.Equal(t, svc.Channel, restored.Channel)
	assert.Equal(t, svc.Transport, restored.Transport)
}

func TestNeuronTopicServiceDef_ERCLogConfig_RoundTrip(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Type:      "neuron-topic",
		Name:      "erc-events",
		Version:   "1.0.0",
		Channel:   "stdOut",
		Transport: "erc-log",
		Anchor:    "native",
		Config: ERCLogConfig{
			ChainId:         1,
			ContractAddress: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
			EventSignature:  "Transfer(address,address,uint256)",
		},
	}

	data, err := SerializeTopicService(svc)
	require.NoError(t, err)

	var restored NeuronTopicServiceDef
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, svc.Type, restored.Type)
	assert.Equal(t, svc.Transport, restored.Transport)
}

func TestNeuronTopicServiceDef_KafkaConfig_RoundTrip(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Type:      "neuron-topic",
		Name:      "kafka-stdin",
		Version:   "1.0.0",
		Channel:   "stdIn",
		Transport: "kafka",
		Anchor:    "hcs",
		Config: KafkaConfig{
			BootstrapServers: []string{"kafka-1:9092", "kafka-2:9092"},
			TopicName:        "agent-commands",
			SASLMechanism:    "SCRAM-SHA-256",
			Anchoring: AnchoringConfig{
				Method:        "periodic",
				AnchorTopicId: "0.0.54321",
				AnchorNetwork: "mainnet",
				Interval:      "10s",
			},
		},
	}

	data, err := SerializeTopicService(svc)
	require.NoError(t, err)

	var restored NeuronTopicServiceDef
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, svc.Type, restored.Type)
	assert.Equal(t, svc.Transport, restored.Transport)
}

// --- ValidateTransportConfig Tests ---

func TestValidateTransportConfig_HCS_Valid(t *testing.T) {
	config := map[string]interface{}{
		"network": "mainnet",
		"topicId": "0.0.12345",
	}
	err := ValidateTransportConfig("hcs", config)
	assert.NoError(t, err)
}

func TestValidateTransportConfig_HCS_MissingNetwork(t *testing.T) {
	config := map[string]interface{}{
		"topicId": "0.0.12345",
	}
	err := ValidateTransportConfig("hcs", config)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "network")
}

func TestValidateTransportConfig_HCS_MissingTopicId(t *testing.T) {
	config := map[string]interface{}{
		"network": "mainnet",
	}
	err := ValidateTransportConfig("hcs", config)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "topicId")
}

func TestValidateTransportConfig_ERCLog_Valid(t *testing.T) {
	config := map[string]interface{}{
		"chainId":         float64(1),
		"contractAddress": "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
		"eventSignature":  "Transfer(address,address,uint256)",
	}
	err := ValidateTransportConfig("erc-log", config)
	assert.NoError(t, err)
}

func TestValidateTransportConfig_ERCLog_MissingChainId(t *testing.T) {
	config := map[string]interface{}{
		"contractAddress": "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
		"eventSignature":  "Transfer(address,address,uint256)",
	}
	err := ValidateTransportConfig("erc-log", config)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "chainId")
}

func TestValidateTransportConfig_Kafka_Valid(t *testing.T) {
	config := map[string]interface{}{
		"bootstrapServers": []string{"kafka:9092"},
		"topicName":        "test-topic",
		"anchoring": map[string]interface{}{
			"method":        "periodic",
			"anchorTopicId": "0.0.99999",
			"anchorNetwork": "mainnet",
			"interval":      "10s",
		},
	}
	err := ValidateTransportConfig("kafka", config)
	assert.NoError(t, err)
}

func TestValidateTransportConfig_Kafka_MissingAnchoring(t *testing.T) {
	config := map[string]interface{}{
		"bootstrapServers": []string{"kafka:9092"},
		"topicName":        "test-topic",
	}
	err := ValidateTransportConfig("kafka", config)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "anchoring")
}

func TestValidateTransportConfig_Kafka_MissingAnchoringFields(t *testing.T) {
	config := map[string]interface{}{
		"bootstrapServers": []string{"kafka:9092"},
		"topicName":        "test-topic",
		"anchoring": map[string]interface{}{
			"method": "periodic",
			// Missing anchorTopicId, anchorNetwork, interval
		},
	}
	err := ValidateTransportConfig("kafka", config)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
}

func TestValidateTransportConfig_UnknownTransport(t *testing.T) {
	config := map[string]interface{}{}
	err := ValidateTransportConfig("unknown", config)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrUnsupportedTransport, topicErr.Kind)
}

// --- ParseAgentURIServices Tests ---

func TestParseAgentURIServices_Complete(t *testing.T) {
	agentURI := `{
		"services": [
			{
				"type": "neuron-topic",
				"name": "agent-stdin",
				"version": "1.0.0",
				"channel": "stdIn",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.11111"}
			},
			{
				"type": "neuron-topic",
				"name": "agent-stdout",
				"version": "1.0.0",
				"channel": "stdOut",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.22222"}
			},
			{
				"type": "neuron-p2p-exchange",
				"name": "p2p-link",
				"version": "1.0.0",
				"peerID": "12D3KooWA...",
				"protocol": "/neuron/exchange/1.0",
				"topicRef": "hcs://0.0.11111"
			},
			{
				"type": "other-service",
				"name": "ignored"
			}
		]
	}`

	topics, p2p, err := ParseAgentURIServices([]byte(agentURI))
	require.NoError(t, err)

	assert.Len(t, topics, 2)
	assert.Len(t, p2p, 1)

	assert.Equal(t, "agent-stdin", topics[0].Name)
	assert.Equal(t, "stdIn", topics[0].Channel)
	assert.Equal(t, "agent-stdout", topics[1].Name)
	assert.Equal(t, "stdOut", topics[1].Channel)

	assert.Equal(t, "p2p-link", p2p[0].Name)
	assert.Equal(t, "hcs://0.0.11111", p2p[0].TopicRef)
}

func TestParseAgentURIServices_EmptyServices(t *testing.T) {
	agentURI := `{"services": []}`

	topics, p2p, err := ParseAgentURIServices([]byte(agentURI))
	require.NoError(t, err)

	assert.Empty(t, topics)
	assert.Empty(t, p2p)
}

func TestParseAgentURIServices_InvalidJSON(t *testing.T) {
	_, _, err := ParseAgentURIServices([]byte("not json"))
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
}

// --- ExtractTopicRef Tests ---

func TestExtractTopicRef_HCS(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Transport: "hcs",
		Config: map[string]interface{}{
			"network": "mainnet",
			"topicId": "0.0.12345",
		},
	}

	ref, err := ExtractTopicRef(svc)
	require.NoError(t, err)
	assert.Equal(t, BackendHCS, ref.transport)
	assert.Equal(t, "0.0.12345", ref.locator)
}

func TestExtractTopicRef_ERCLog(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Transport: "erc-log",
		Config: map[string]interface{}{
			"chainId":         float64(1),
			"contractAddress": "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
			"eventSignature":  "Transfer(address,address,uint256)",
		},
	}

	ref, err := ExtractTopicRef(svc)
	require.NoError(t, err)
	assert.Equal(t, BackendERCLog, ref.transport)
	assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00", ref.locator)
}

func TestExtractTopicRef_Kafka(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Transport: "kafka",
		Config: map[string]interface{}{
			"bootstrapServers": []string{"kafka:9092"},
			"topicName":        "agent-commands",
			"anchoring":        map[string]interface{}{},
		},
	}

	ref, err := ExtractTopicRef(svc)
	require.NoError(t, err)
	assert.Equal(t, BackendKafka, ref.transport)
	assert.Equal(t, "agent-commands", ref.locator)
}

func TestExtractTopicRef_InvalidTransport(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Transport: "",
		Config:    map[string]interface{}{},
	}

	_, err := ExtractTopicRef(svc)
	require.Error(t, err)
}

func TestExtractTopicRef_HCS_MissingTopicId(t *testing.T) {
	svc := NeuronTopicServiceDef{
		Transport: "hcs",
		Config: map[string]interface{}{
			"network": "mainnet",
		},
	}

	_, err := ExtractTopicRef(svc)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
}

// --- DiscoverPeerChannels Tests ---

func TestDiscoverPeerChannels(t *testing.T) {
	agentURI := `{
		"services": [
			{
				"type": "neuron-topic",
				"name": "stdin",
				"version": "1.0.0",
				"channel": "stdIn",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.11111"}
			},
			{
				"type": "neuron-topic",
				"name": "stdout",
				"version": "1.0.0",
				"channel": "stdOut",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.22222"}
			},
			{
				"type": "neuron-topic",
				"name": "stderr",
				"version": "1.0.0",
				"channel": "stdErr",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.33333"}
			}
		]
	}`

	channels, err := DiscoverPeerChannels([]byte(agentURI))
	require.NoError(t, err)

	assert.Len(t, channels, 3)

	assert.Equal(t, "0.0.11111", channels[ChannelStdIn].locator)
	assert.Equal(t, "0.0.22222", channels[ChannelStdOut].locator)
	assert.Equal(t, "0.0.33333", channels[ChannelStdErr].locator)
}

func TestDiscoverPeerChannels_Partial(t *testing.T) {
	// Only stdIn defined.
	agentURI := `{
		"services": [
			{
				"type": "neuron-topic",
				"name": "stdin",
				"version": "1.0.0",
				"channel": "stdIn",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.11111"}
			}
		]
	}`

	channels, err := DiscoverPeerChannels([]byte(agentURI))
	require.NoError(t, err)

	assert.Len(t, channels, 1)
	_, hasStdIn := channels[ChannelStdIn]
	assert.True(t, hasStdIn)
	_, hasStdOut := channels[ChannelStdOut]
	assert.False(t, hasStdOut)
}

// --- FindStd* Tests ---

func TestFindStdIn(t *testing.T) {
	services := []NeuronTopicServiceDef{
		{Channel: "stdOut", Name: "out"},
		{Channel: "stdIn", Name: "in"},
		{Channel: "stdErr", Name: "err"},
	}

	found, err := FindStdIn(services)
	require.NoError(t, err)
	assert.Equal(t, "in", found.Name)
}

func TestFindStdOut(t *testing.T) {
	services := []NeuronTopicServiceDef{
		{Channel: "stdIn", Name: "in"},
		{Channel: "stdOut", Name: "out"},
	}

	found, err := FindStdOut(services)
	require.NoError(t, err)
	assert.Equal(t, "out", found.Name)
}

func TestFindStdErr(t *testing.T) {
	services := []NeuronTopicServiceDef{
		{Channel: "stdErr", Name: "err"},
	}

	found, err := FindStdErr(services)
	require.NoError(t, err)
	assert.Equal(t, "err", found.Name)
}

func TestFindStdIn_NotFound(t *testing.T) {
	services := []NeuronTopicServiceDef{
		{Channel: "stdOut", Name: "out"},
	}

	_, err := FindStdIn(services)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrTopicNotFound, topicErr.Kind)
}

func TestFindStdIn_EmptySlice(t *testing.T) {
	_, err := FindStdIn(nil)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrTopicNotFound, topicErr.Kind)
}

// --- Phase 9 (US7): Custom Channels in agentURI ---

func TestParseAgentURIServices_MixedStandardAndCustomChannels(t *testing.T) {
	agentURI := `{
		"services": [
			{
				"type": "neuron-topic",
				"name": "agent-stdin",
				"version": "1.0.0",
				"channel": "stdIn",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.11111"}
			},
			{
				"type": "neuron-topic",
				"name": "agent-stdout",
				"version": "1.0.0",
				"channel": "stdOut",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.22222"}
			},
			{
				"type": "neuron-topic",
				"name": "metrics-channel",
				"version": "1.0.0",
				"channel": "custom:metrics",
				"transport": "kafka",
				"anchor": "hcs",
				"config": {
					"bootstrapServers": ["kafka:9092"],
					"topicName": "agent-metrics",
					"anchoring": {
						"method": "periodic",
						"anchorTopicId": "0.0.99999",
						"anchorNetwork": "mainnet",
						"interval": "10s"
					}
				}
			},
			{
				"type": "neuron-topic",
				"name": "heartbeat-channel",
				"version": "1.0.0",
				"channel": "custom:heartbeat",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.44444"}
			}
		]
	}`

	topics, _, err := ParseAgentURIServices([]byte(agentURI))
	require.NoError(t, err)
	assert.Len(t, topics, 4)

	// Verify all channels parse correctly.
	assert.Equal(t, "stdIn", topics[0].Channel)
	assert.Equal(t, "stdOut", topics[1].Channel)
	assert.Equal(t, "custom:metrics", topics[2].Channel)
	assert.Equal(t, "custom:heartbeat", topics[3].Channel)

	// Verify custom channels are valid ChannelRole values.
	for _, svc := range topics {
		_, err := ChannelRoleFromString(svc.Channel)
		assert.NoError(t, err, "channel %s should be valid", svc.Channel)
	}
}

func TestDiscoverPeerChannels_ExcludesCustomChannels(t *testing.T) {
	// DiscoverPeerChannels only returns standard channels (stdIn, stdOut, stdErr).
	// Custom channels are valid but not included in the standard discovery result.
	agentURI := `{
		"services": [
			{
				"type": "neuron-topic",
				"name": "stdin",
				"version": "1.0.0",
				"channel": "stdIn",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.11111"}
			},
			{
				"type": "neuron-topic",
				"name": "metrics",
				"version": "1.0.0",
				"channel": "custom:metrics",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.55555"}
			}
		]
	}`

	channels, err := DiscoverPeerChannels([]byte(agentURI))
	require.NoError(t, err)

	// Only stdIn should be in the result, not custom:metrics.
	assert.Len(t, channels, 1)
	_, hasStdIn := channels[ChannelStdIn]
	assert.True(t, hasStdIn)
}

func TestDiscoverPeerChannels_CustomChannelsAreValid(t *testing.T) {
	// Even though DiscoverPeerChannels excludes custom channels from its map,
	// the custom channels should still parse successfully from the agentURI.
	agentURI := `{
		"services": [
			{
				"type": "neuron-topic",
				"name": "metrics",
				"version": "1.0.0",
				"channel": "custom:metrics",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.55555"}
			},
			{
				"type": "neuron-topic",
				"name": "heartbeat",
				"version": "1.0.0",
				"channel": "custom:heartbeat",
				"transport": "hcs",
				"anchor": "native",
				"config": {"network": "mainnet", "topicId": "0.0.66666"}
			}
		]
	}`

	// ParseAgentURIServices should successfully parse custom channels.
	topics, _, err := ParseAgentURIServices([]byte(agentURI))
	require.NoError(t, err)
	assert.Len(t, topics, 2)

	// DiscoverPeerChannels returns empty map (no standard channels).
	channels, err := DiscoverPeerChannels([]byte(agentURI))
	require.NoError(t, err)
	assert.Len(t, channels, 0)
}
