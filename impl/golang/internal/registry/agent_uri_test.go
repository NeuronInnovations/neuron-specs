package registry

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentURI_EmptyServices(t *testing.T) {
	uri := AgentURI{}

	data, err := json.Marshal(uri)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "services")
	services, ok := raw["services"].([]any)
	require.True(t, ok)
	assert.Empty(t, services)
}

func TestAgentURI_JSONStructure(t *testing.T) {
	uri := AgentURI{
		topicServices: []NeuronTopicService{
			{Type: ServiceTypeNeuronTopic, Name: "stdIn", Version: "1.0.0", Channel: "stdIn", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{"topicId": "0.0.1"}},
		},
		p2pServices: []NeuronP2PExchangeService{
			{Type: ServiceTypeNeuronP2PExchange, Name: "p2p", Version: "1.0.0", PeerID: "12D3KooW", Protocol: "/neuron/multiaddr-exchange/1.0.0", TopicRef: "stdIn"},
		},
	}

	data, err := json.Marshal(uri)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "services")
	services, ok := raw["services"].([]any)
	require.True(t, ok)
	assert.Len(t, services, 2)

	// First service should be neuron-topic.
	first := services[0].(map[string]any)
	assert.Equal(t, "neuron-topic", first["type"])

	// Second service should be neuron-p2p-exchange.
	second := services[1].(map[string]any)
	assert.Equal(t, "neuron-p2p-exchange", second["type"])
}

func TestAgentURI_RoundTrip(t *testing.T) {
	original := AgentURI{
		topicServices: []NeuronTopicService{
			{Type: ServiceTypeNeuronTopic, Name: "stdIn", Version: "1.0.0", Channel: "stdIn", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{"topicId": "0.0.1"}},
			{Type: ServiceTypeNeuronTopic, Name: "stdOut", Version: "1.0.0", Channel: "stdOut", Transport: "kafka", Anchor: "ethereum-mainnet", Config: map[string]any{"bootstrapServers": "kafka:9092"}},
			{Type: ServiceTypeNeuronTopic, Name: "stdErr", Version: "1.0.0", Channel: "stdErr", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{"topicId": "0.0.2"}},
		},
		p2pServices: []NeuronP2PExchangeService{
			{Type: ServiceTypeNeuronP2PExchange, Name: "p2p", Version: "1.0.0", PeerID: "12D3KooWGzBnW7wm6o5dRb", Protocol: "/neuron/multiaddr-exchange/1.0.0", TopicRef: "stdIn"},
		},
		didServices: []DIDService{
			{Type: ServiceTypeDID, Name: "DID", Endpoint: "did:key:zQ3shunBKsXmLGEBU3JcUY5JjFCkEXvMAFYVPYMBaDqMTAnWz", Version: "v1"},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded AgentURI
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.topicServices, 3)
	assert.Len(t, decoded.p2pServices, 1)
	assert.Len(t, decoded.didServices, 1)

	// Verify topic services preserved.
	assert.Equal(t, "stdIn", decoded.topicServices[0].Name)
	assert.Equal(t, "stdOut", decoded.topicServices[1].Name)
	assert.Equal(t, "stdErr", decoded.topicServices[2].Name)
	assert.Equal(t, "hcs", decoded.topicServices[0].Transport)
	assert.Equal(t, "kafka", decoded.topicServices[1].Transport)

	// Verify P2P service preserved.
	assert.Equal(t, "12D3KooWGzBnW7wm6o5dRb", decoded.p2pServices[0].PeerID)
	assert.Equal(t, "stdIn", decoded.p2pServices[0].TopicRef)

	// Verify DID service preserved.
	assert.Contains(t, decoded.didServices[0].Endpoint, "did:key:zQ3s")
}

func TestAgentURI_ServiceOrdering(t *testing.T) {
	uri := AgentURI{
		topicServices: []NeuronTopicService{
			{Type: ServiceTypeNeuronTopic, Name: "stdIn", Version: "1.0.0", Channel: "stdIn", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{}},
			{Type: ServiceTypeNeuronTopic, Name: "stdOut", Version: "1.0.0", Channel: "stdOut", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{}},
			{Type: ServiceTypeNeuronTopic, Name: "stdErr", Version: "1.0.0", Channel: "stdErr", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{}},
		},
		p2pServices: []NeuronP2PExchangeService{
			{Type: ServiceTypeNeuronP2PExchange, Name: "p2p", Version: "1.0.0", PeerID: "peer1", Protocol: "/proto", TopicRef: "stdIn"},
		},
		didServices: []DIDService{
			{Type: ServiceTypeDID, Name: "DID", Endpoint: "did:key:zQ3sTest", Version: "v1"},
		},
	}

	data, err := json.Marshal(uri)
	require.NoError(t, err)

	var raw jsonAgentURI
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Order: 3 topic, 1 p2p, 1 DID = 5 total.
	require.Len(t, raw.Services, 5)

	// Verify ordering: topics first, then p2p, then DID.
	var types []string
	for _, svc := range raw.Services {
		var disc serviceTypeDiscriminator
		require.NoError(t, json.Unmarshal(svc, &disc))
		types = append(types, disc.Type)
	}
	assert.Equal(t, []string{
		"neuron-topic", "neuron-topic", "neuron-topic",
		"neuron-p2p-exchange",
		"DID",
	}, types)
}

func TestAgentURI_ToJSON(t *testing.T) {
	uri := AgentURI{
		topicServices: []NeuronTopicService{
			{Type: ServiceTypeNeuronTopic, Name: "stdIn", Version: "1.0.0", Channel: "stdIn", Transport: "hcs", Anchor: "hedera-mainnet", Config: map[string]any{}},
		},
	}

	jsonStr, err := uri.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonStr, `"services"`)
	assert.Contains(t, jsonStr, `"neuron-topic"`)
}

func TestAgentURIFromJSON(t *testing.T) {
	input := `{"services":[{"type":"neuron-topic","name":"stdIn","version":"1.0.0","channel":"stdIn","transport":"hcs","anchor":"hedera-mainnet","config":{"topicId":"0.0.1"}}]}`

	uri, err := AgentURIFromJSON(input)
	require.NoError(t, err)
	require.Len(t, uri.topicServices, 1)
	assert.Equal(t, "stdIn", uri.topicServices[0].Name)
}

func TestAgentURI_UnknownServiceTypeIgnored(t *testing.T) {
	input := `{"services":[{"type":"unknown-service","name":"test"},{"type":"neuron-topic","name":"stdIn","version":"1.0.0","channel":"stdIn","transport":"hcs","anchor":"hedera","config":{}}]}`

	uri, err := AgentURIFromJSON(input)
	require.NoError(t, err)
	assert.Len(t, uri.topicServices, 1)
	assert.Empty(t, uri.p2pServices)
	assert.Empty(t, uri.didServices)
}
