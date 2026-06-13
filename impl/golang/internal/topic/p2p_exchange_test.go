package topic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNeuronP2PExchangeService_Construction(t *testing.T) {
	svc := NeuronP2PExchangeServiceDef{
		Type:     "neuron-p2p-exchange",
		Name:     "p2p-link",
		Version:  "1.0.0",
		PeerID:   "12D3KooWAbcdefg",
		Protocol: "/neuron/exchange/1.0",
		TopicRef: "hcs://0.0.12345",
	}

	assert.Equal(t, "neuron-p2p-exchange", svc.Type)
	assert.Equal(t, "p2p-link", svc.Name)
	assert.Equal(t, "1.0.0", svc.Version)
	assert.Equal(t, "12D3KooWAbcdefg", svc.PeerID)
	assert.Equal(t, "/neuron/exchange/1.0", svc.Protocol)
	assert.Equal(t, "hcs://0.0.12345", svc.TopicRef)
}

func TestValidateCrossReferences_Valid(t *testing.T) {
	topics := []NeuronTopicServiceDef{
		{
			Transport: "hcs",
			Config: map[string]interface{}{
				"network": "mainnet",
				"topicId": "0.0.12345",
			},
		},
		{
			Transport: "hcs",
			Config: map[string]interface{}{
				"network": "mainnet",
				"topicId": "0.0.67890",
			},
		},
	}

	p2p := []NeuronP2PExchangeServiceDef{
		{
			Name:     "link-1",
			TopicRef: "hcs://0.0.12345",
		},
		{
			Name:     "link-2",
			TopicRef: "hcs://0.0.67890",
		},
	}

	err := ValidateCrossReferences(topics, p2p)
	assert.NoError(t, err)
}

func TestValidateCrossReferences_BrokenRef(t *testing.T) {
	topics := []NeuronTopicServiceDef{
		{
			Transport: "hcs",
			Config: map[string]interface{}{
				"network": "mainnet",
				"topicId": "0.0.12345",
			},
		},
	}

	p2p := []NeuronP2PExchangeServiceDef{
		{
			Name:     "broken-link",
			TopicRef: "hcs://0.0.99999", // Does not match any topic service.
		},
	}

	err := ValidateCrossReferences(topics, p2p)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBrokenTopicRef, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "broken-link")
	assert.Contains(t, topicErr.Message, "0.0.99999")
}

func TestValidateCrossReferences_EmptyTopicRef(t *testing.T) {
	topics := []NeuronTopicServiceDef{
		{
			Transport: "hcs",
			Config: map[string]interface{}{
				"network": "mainnet",
				"topicId": "0.0.12345",
			},
		},
	}

	p2p := []NeuronP2PExchangeServiceDef{
		{
			Name:     "empty-ref",
			TopicRef: "",
		},
	}

	err := ValidateCrossReferences(topics, p2p)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBrokenTopicRef, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "empty")
}

func TestValidateCrossReferences_InvalidURI(t *testing.T) {
	topics := []NeuronTopicServiceDef{
		{
			Transport: "hcs",
			Config: map[string]interface{}{
				"network": "mainnet",
				"topicId": "0.0.12345",
			},
		},
	}

	p2p := []NeuronP2PExchangeServiceDef{
		{
			Name:     "bad-uri",
			TopicRef: "no-scheme-here",
		},
	}

	err := ValidateCrossReferences(topics, p2p)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBrokenTopicRef, topicErr.Kind)
}

func TestValidateCrossReferences_EmptyP2P(t *testing.T) {
	topics := []NeuronTopicServiceDef{
		{
			Transport: "hcs",
			Config: map[string]interface{}{
				"network": "mainnet",
				"topicId": "0.0.12345",
			},
		},
	}

	// No P2P services to validate -- should pass.
	err := ValidateCrossReferences(topics, nil)
	assert.NoError(t, err)
}

func TestValidateCrossReferences_EmptyTopics(t *testing.T) {
	p2p := []NeuronP2PExchangeServiceDef{
		{
			Name:     "orphan",
			TopicRef: "hcs://0.0.12345",
		},
	}

	// No topics to match against -- all refs are broken.
	err := ValidateCrossReferences(nil, p2p)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBrokenTopicRef, topicErr.Kind)
}
