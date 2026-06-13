package validation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTopicAdapter captures published messages for verification.
type mockTopicAdapter struct {
	maxMsgSize  uint64
	transport   topic.BackendKind
	publishFn   func(topic.TopicRef, topic.TopicMessage, topic.PublishOpts) (topic.PublishResult, error)
	capturedMsg *topic.TopicMessage
}

func (m *mockTopicAdapter) CreateTopic(_ topic.CreateTopicOpts) (topic.TopicRef, error) {
	return topic.TopicRef{}, nil
}

func (m *mockTopicAdapter) Publish(ref topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) {
	m.capturedMsg = &msg
	if m.publishFn != nil {
		return m.publishFn(ref, msg, opts)
	}
	ts := uint64(1234567890)
	return topic.PublishResult{
		Confirmed:          true,
		TransactionRef:     "mock-tx",
		ConsensusTimestamp: &ts,
	}, nil
}

func (m *mockTopicAdapter) Subscribe(_ context.Context, _ topic.TopicRef, _ topic.SubscribeOpts) (<-chan topic.MessageDelivery, error) {
	return nil, nil
}

func (m *mockTopicAdapter) Resolve(_ topic.TopicRef) (topic.TopicMetadata, error) {
	return topic.TopicMetadata{}, nil
}

func (m *mockTopicAdapter) MaxMessageSize() uint64 {
	if m.maxMsgSize > 0 {
		return m.maxMsgSize
	}
	return 4096
}

func (m *mockTopicAdapter) EstimatePublishCost(_ uint64) (topic.CostEstimate, error) {
	return topic.CostEstimate{}, nil
}

func (m *mockTopicAdapter) SupportedTransport() topic.BackendKind {
	return m.transport
}

func TestPublishEvidence_Success(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	doc := []byte(`test evidence`)
	hash := ComputeEvidenceHash(doc)
	hashStr := FormatEvidenceHash(hash)

	env, err := NewEvidenceEnvelope("1", "2", "008-payment",
		VerdictCompliant, hashStr, "ipfs://QmTest")
	require.NoError(t, err)

	adapter := &mockTopicAdapter{transport: topic.BackendHCS}
	stdOut := topic.TopicRef{}

	result, err := PublishEvidence(env, &key, stdOut, adapter, 1000, 1)
	require.NoError(t, err)
	assert.True(t, result.Confirmed)

	// Verify the captured message contains the envelope JSON.
	require.NotNil(t, adapter.capturedMsg)
	payload := adapter.capturedMsg.Payload()
	var parsed EvidenceEnvelope
	require.NoError(t, json.Unmarshal(payload, &parsed))
	assert.Equal(t, "1", parsed.ValidatorAgentId())
	assert.Equal(t, VerdictCompliant, parsed.Verdict())
}
