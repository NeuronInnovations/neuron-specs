package registry

import (
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NeuronTopicService Tests (T006) ---

func TestNeuronTopicService_Construction(t *testing.T) {
	svc := NeuronTopicService{
		Type:      ServiceTypeNeuronTopic,
		Name:      "stdIn",
		Version:   "1.0.0",
		Channel:   "stdIn",
		Transport: "hcs",
		Anchor:    "hedera-mainnet",
		Config:    map[string]any{"network": "mainnet", "topicId": "0.0.12345"},
	}

	assert.Equal(t, "neuron-topic", svc.Type)
	assert.Equal(t, "stdIn", svc.Name)
	assert.Equal(t, "1.0.0", svc.Version)
	assert.Equal(t, "stdIn", svc.Channel)
	assert.Equal(t, "hcs", svc.Transport)
	assert.Equal(t, "hedera-mainnet", svc.Anchor)
	assert.NotNil(t, svc.Config)
	assert.Equal(t, "mainnet", svc.Config["network"])
	assert.Equal(t, "0.0.12345", svc.Config["topicId"])
	assert.Empty(t, svc.Endpoint)
}

func TestNeuronTopicService_WithOptionalEndpoint(t *testing.T) {
	svc := NeuronTopicService{
		Type:      ServiceTypeNeuronTopic,
		Name:      "stdOut",
		Version:   "1.0.0",
		Channel:   "stdOut",
		Transport: "kafka",
		Anchor:    "ethereum-mainnet",
		Config:    map[string]any{"bootstrapServers": []string{"kafka:9092"}},
		Endpoint:  "neuron-topic://stdOut",
	}

	assert.Equal(t, "neuron-topic://stdOut", svc.Endpoint)
}

func TestNeuronTopicService_ZeroValueIsDistinguishable(t *testing.T) {
	var svc NeuronTopicService
	assert.Empty(t, svc.Type)
	assert.Empty(t, svc.Name)
	assert.Empty(t, svc.Version)
	assert.Empty(t, svc.Channel)
	assert.Empty(t, svc.Transport)
	assert.Empty(t, svc.Anchor)
	assert.Nil(t, svc.Config)
	assert.Empty(t, svc.Endpoint)
}

func TestNeuronTopicService_JSONTags(t *testing.T) {
	svc := NeuronTopicService{
		Type:      ServiceTypeNeuronTopic,
		Name:      "stdErr",
		Version:   "1.0.0",
		Channel:   "stdErr",
		Transport: "hcs",
		Anchor:    "hedera-mainnet",
		Config:    map[string]any{"topicId": "0.0.99999"},
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Verify JSON field names match expected 004 FR-T14 names.
	assert.Contains(t, raw, "type")
	assert.Contains(t, raw, "name")
	assert.Contains(t, raw, "version")
	assert.Contains(t, raw, "channel")
	assert.Contains(t, raw, "transport")
	assert.Contains(t, raw, "anchor")
	assert.Contains(t, raw, "config")
	assert.NotContains(t, raw, "endpoint") // omitempty
}

func TestNeuronTopicService_EndpointOmittedWhenEmpty(t *testing.T) {
	svc := NeuronTopicService{
		Type:      ServiceTypeNeuronTopic,
		Name:      "stdIn",
		Version:   "1.0.0",
		Channel:   "stdIn",
		Transport: "hcs",
		Anchor:    "hedera-mainnet",
		Config:    map[string]any{},
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)
	assert.NotContains(t, raw, "endpoint")
}

func TestNeuronTopicService_EndpointIncludedWhenSet(t *testing.T) {
	svc := NeuronTopicService{
		Type:      ServiceTypeNeuronTopic,
		Name:      "stdIn",
		Version:   "1.0.0",
		Channel:   "stdIn",
		Transport: "hcs",
		Anchor:    "hedera-mainnet",
		Config:    map[string]any{},
		Endpoint:  "neuron-topic://stdIn",
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)
	assert.Equal(t, "neuron-topic://stdIn", raw["endpoint"])
}

// --- NeuronP2PExchangeService Tests (T006) ---

func TestNeuronP2PExchangeService_Construction(t *testing.T) {
	svc := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     "p2p",
		Version:  "1.0.0",
		PeerID:   "12D3KooWGzBnW7wm6o5dRbAFhKuXjKq9HfvNqQZaGuGRkVpipKX",
		Protocol: "/neuron/multiaddr-exchange/1.0.0",
		TopicRef: "stdIn",
	}

	assert.Equal(t, "neuron-p2p-exchange", svc.Type)
	assert.Equal(t, "p2p", svc.Name)
	assert.Equal(t, "1.0.0", svc.Version)
	assert.NotEmpty(t, svc.PeerID)
	assert.Equal(t, "/neuron/multiaddr-exchange/1.0.0", svc.Protocol)
	assert.Equal(t, "stdIn", svc.TopicRef)
}

func TestNeuronP2PExchangeService_ZeroValueIsDistinguishable(t *testing.T) {
	var svc NeuronP2PExchangeService
	assert.Empty(t, svc.Type)
	assert.Empty(t, svc.Name)
	assert.Empty(t, svc.Version)
	assert.Empty(t, svc.PeerID)
	assert.Empty(t, svc.Protocol)
	assert.Empty(t, svc.TopicRef)
}

func TestNeuronP2PExchangeService_JSONTags(t *testing.T) {
	svc := NeuronP2PExchangeService{
		Type:     ServiceTypeNeuronP2PExchange,
		Name:     "p2p",
		Version:  "1.0.0",
		PeerID:   "12D3KooWTestPeerID",
		Protocol: "/neuron/multiaddr-exchange/1.0.0",
		TopicRef: "stdIn",
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "type")
	assert.Contains(t, raw, "name")
	assert.Contains(t, raw, "version")
	assert.Contains(t, raw, "peerID")
	assert.Contains(t, raw, "protocol")
	assert.Contains(t, raw, "topicRef")
	// Stage 3B: Multiaddrs is omitempty — the field is absent when nil.
	assert.NotContains(t, raw, "multiaddrs", "Multiaddrs omitempty: nil slice MUST NOT appear on the wire")
}

// TestNeuronP2PExchangeService_MultiaddrsRoundTrip exercises the
// Stage 3B `multiaddrs` field. The seller's registered AgentURI
// carries listen multiaddrs so registry-backed buyers can dial
// without out-of-band coordination.
func TestNeuronP2PExchangeService_MultiaddrsRoundTrip(t *testing.T) {
	svc, err := NewNeuronP2PExchangeService(
		"remoteid-p2p", "1.0.0",
		"12D3KooWTestPeer", "/ds240/raw/1.0.0", "remoteid-stdout",
		WithMultiaddrs([]string{
			"/ip4/127.0.0.1/udp/41523/quic-v1",
			"/ip4/10.0.0.5/udp/41523/quic-v1",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/ip4/127.0.0.1/udp/41523/quic-v1",
		"/ip4/10.0.0.5/udp/41523/quic-v1",
	}, svc.Multiaddrs)

	data, err := json.Marshal(svc)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"multiaddrs":["/ip4/127.0.0.1/udp/41523/quic-v1","/ip4/10.0.0.5/udp/41523/quic-v1"]`)

	var back NeuronP2PExchangeService
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, svc.Multiaddrs, back.Multiaddrs)
}

// TestWithMultiaddrs_EmptyIsNoOp asserts that passing nil or zero-length
// slices leaves Multiaddrs untouched (no zero-length array in the JSON).
func TestWithMultiaddrs_EmptyIsNoOp(t *testing.T) {
	svc, err := NewNeuronP2PExchangeService(
		"p2p", "1.0.0", "12D3KooWX", "/foo/1.0.0", "stdIn",
		WithMultiaddrs(nil),
	)
	require.NoError(t, err)
	assert.Nil(t, svc.Multiaddrs)

	svc, err = NewNeuronP2PExchangeService(
		"p2p", "1.0.0", "12D3KooWX", "/foo/1.0.0", "stdIn",
		WithMultiaddrs([]string{}),
	)
	require.NoError(t, err)
	assert.Nil(t, svc.Multiaddrs)
}

// --- DIDService Tests (T006) ---

func TestDIDService_Construction(t *testing.T) {
	svc := DIDService{
		Type:     ServiceTypeDID,
		Name:     "DID",
		Endpoint: "did:key:zQ3shunBKsXmLGEBU3JcUY5JjFCkEXvMAFYVPYMBaDqMTAnWz",
		Version:  "v1",
	}

	assert.Equal(t, "DID", svc.Type)
	assert.Equal(t, "DID", svc.Name)
	assert.Contains(t, svc.Endpoint, "did:key:zQ3s")
	assert.Equal(t, "v1", svc.Version)
}

func TestDIDService_ZeroValueIsDistinguishable(t *testing.T) {
	var svc DIDService
	assert.Empty(t, svc.Type)
	assert.Empty(t, svc.Name)
	assert.Empty(t, svc.Endpoint)
	assert.Empty(t, svc.Version)
}

func TestDIDService_JSONTags(t *testing.T) {
	svc := DIDService{
		Type:     ServiceTypeDID,
		Name:     "DID",
		Endpoint: "did:key:zQ3sTest",
		Version:  "v1",
	}

	data, err := json.Marshal(svc)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Contains(t, raw, "type")
	assert.Contains(t, raw, "name")
	assert.Contains(t, raw, "endpoint")
	assert.Contains(t, raw, "version")
}

// --- Service Type Constants ---

func TestServiceTypeConstants(t *testing.T) {
	assert.Equal(t, "neuron-topic", ServiceTypeNeuronTopic)
	assert.Equal(t, "neuron-p2p-exchange", ServiceTypeNeuronP2PExchange)
	assert.Equal(t, "DID", ServiceTypeDID)
}

// --- Standard Channels ---

func TestStandardChannels(t *testing.T) {
	assert.Equal(t, []string{"stdIn", "stdOut", "stdErr"}, StandardChannels())
}

// --- Service Builder Tests (T026, T037) ---

func TestNewNeuronTopicService_AllChannels(t *testing.T) {
	for _, ch := range StandardChannels() {
		t.Run(ch, func(t *testing.T) {
			svc, err := NewNeuronTopicService(ch, "1.0.0", ch, "hcs", "hedera-mainnet",
				map[string]any{"topicId": "0.0.1"})
			require.NoError(t, err)

			assert.Equal(t, ServiceTypeNeuronTopic, svc.Type)
			assert.Equal(t, ch, svc.Name)
			assert.Equal(t, "1.0.0", svc.Version)
			assert.Equal(t, ch, svc.Channel)
			assert.Equal(t, "hcs", svc.Transport)
			assert.Equal(t, "hedera-mainnet", svc.Anchor)
			assert.NotNil(t, svc.Config)
			assert.Empty(t, svc.Endpoint)
		})
	}
}

func TestNewNeuronTopicService_MissingField(t *testing.T) {
	tests := []struct {
		name    string
		svcName string
		version string
		channel string
		transport string
		anchor  string
		config  map[string]any
		wantField string
	}{
		{
			name: "missing name", svcName: "", version: "1.0.0", channel: "stdIn",
			transport: "hcs", anchor: "hedera", config: map[string]any{}, wantField: "name",
		},
		{
			name: "missing version", svcName: "stdIn", version: "", channel: "stdIn",
			transport: "hcs", anchor: "hedera", config: map[string]any{}, wantField: "version",
		},
		{
			name: "missing channel", svcName: "stdIn", version: "1.0.0", channel: "",
			transport: "hcs", anchor: "hedera", config: map[string]any{}, wantField: "channel",
		},
		{
			name: "missing transport", svcName: "stdIn", version: "1.0.0", channel: "stdIn",
			transport: "", anchor: "hedera", config: map[string]any{}, wantField: "transport",
		},
		{
			name: "missing anchor", svcName: "stdIn", version: "1.0.0", channel: "stdIn",
			transport: "hcs", anchor: "", config: map[string]any{}, wantField: "anchor",
		},
		{
			name: "nil config", svcName: "stdIn", version: "1.0.0", channel: "stdIn",
			transport: "hcs", anchor: "hedera", config: nil, wantField: "config",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNeuronTopicService(tc.svcName, tc.version, tc.channel, tc.transport, tc.anchor, tc.config)
			require.Error(t, err)

			var schemaErr InvalidServiceSchema
			require.ErrorAs(t, err, &schemaErr)
			assert.Equal(t, ServiceTypeNeuronTopic, schemaErr.ServiceType)
			assert.Equal(t, tc.wantField, schemaErr.Field)
		})
	}
}

func TestNewNeuronTopicService_OptionalEndpoint(t *testing.T) {
	svc, err := NewNeuronTopicService("stdIn", "1.0.0", "stdIn", "hcs", "hedera-mainnet",
		map[string]any{"topicId": "0.0.1"}, WithTopicEndpoint("neuron-topic://stdIn"))
	require.NoError(t, err)

	assert.Equal(t, "neuron-topic://stdIn", svc.Endpoint)
}

func TestNewNeuronP2PExchangeService_Success(t *testing.T) {
	svc, err := NewNeuronP2PExchangeService("p2p", "1.0.0", "12D3KooWTest",
		"/neuron/multiaddr-exchange/1.0.0", "stdIn")
	require.NoError(t, err)

	assert.Equal(t, ServiceTypeNeuronP2PExchange, svc.Type)
	assert.Equal(t, "p2p", svc.Name)
	assert.Equal(t, "1.0.0", svc.Version)
	assert.Equal(t, "12D3KooWTest", svc.PeerID)
	assert.Equal(t, "/neuron/multiaddr-exchange/1.0.0", svc.Protocol)
	assert.Equal(t, "stdIn", svc.TopicRef)
}

func TestNewNeuronP2PExchangeService_MissingFields(t *testing.T) {
	tests := []struct {
		name      string
		svcName   string
		version   string
		peerID    string
		protocol  string
		topicRef  string
		wantField string
	}{
		{name: "missing name", svcName: "", version: "1.0.0", peerID: "peer", protocol: "/proto", topicRef: "stdIn", wantField: "name"},
		{name: "missing version", svcName: "p2p", version: "", peerID: "peer", protocol: "/proto", topicRef: "stdIn", wantField: "version"},
		{name: "missing peerID", svcName: "p2p", version: "1.0.0", peerID: "", protocol: "/proto", topicRef: "stdIn", wantField: "peerID"},
		{name: "missing protocol", svcName: "p2p", version: "1.0.0", peerID: "peer", protocol: "", topicRef: "stdIn", wantField: "protocol"},
		{name: "missing topicRef", svcName: "p2p", version: "1.0.0", peerID: "peer", protocol: "/proto", topicRef: "", wantField: "topicRef"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNeuronP2PExchangeService(tc.svcName, tc.version, tc.peerID, tc.protocol, tc.topicRef)
			require.Error(t, err)

			var schemaErr InvalidServiceSchema
			require.ErrorAs(t, err, &schemaErr)
			assert.Equal(t, ServiceTypeNeuronP2PExchange, schemaErr.ServiceType)
			assert.Equal(t, tc.wantField, schemaErr.Field)
		})
	}
}

func TestNewDIDService_Valid(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	svc, err := NewDIDService(pub)
	require.NoError(t, err)

	assert.Equal(t, ServiceTypeDID, svc.Type)
	assert.Equal(t, "DID", svc.Name)
	assert.Equal(t, "v1", svc.Version)
	assert.Contains(t, svc.Endpoint, "did:key:zQ3s")
}

func TestNewDIDService_DIDMatchesPublicKey(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pub := childKey.PublicKey()
	expectedDID := pub.DIDKey()

	svc, err := NewDIDService(pub)
	require.NoError(t, err)

	assert.Equal(t, expectedDID, svc.Endpoint)
}
