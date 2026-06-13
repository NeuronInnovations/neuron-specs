package topic

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHCSClient implements HCSClient for testing.
type mockHCSClient struct {
	createTopicFn        func(memo string) (string, error)
	submitMessageFn      func(topicId string, message []byte) (string, error)
	submitMessageWaitFn  func(topicId string, message []byte) (string, uint64, uint64, error)
	subscribeTopicFn     func(topicId string, startSequence *uint64) (<-chan HCSMessage, error)
	getTopicInfoFn       func(topicId string) (TopicMetadata, error)
}

func (m *mockHCSClient) CreateTopic(memo string) (string, error) {
	if m.createTopicFn != nil {
		return m.createTopicFn(memo)
	}
	return "0.0.12345", nil
}

func (m *mockHCSClient) SubmitMessage(topicId string, message []byte) (string, error) {
	if m.submitMessageFn != nil {
		return m.submitMessageFn(topicId, message)
	}
	return "tx-001", nil
}

func (m *mockHCSClient) SubmitMessageAndWait(topicId string, message []byte) (string, uint64, uint64, error) {
	if m.submitMessageWaitFn != nil {
		return m.submitMessageWaitFn(topicId, message)
	}
	return "tx-002", 1700000000000000000, 1, nil
}

func (m *mockHCSClient) SubscribeTopic(topicId string, startSequence *uint64) (<-chan HCSMessage, error) {
	if m.subscribeTopicFn != nil {
		return m.subscribeTopicFn(topicId, startSequence)
	}
	ch := make(chan HCSMessage)
	close(ch)
	return ch, nil
}

func (m *mockHCSClient) GetTopicInfo(topicId string) (TopicMetadata, error) {
	if m.getTopicInfoFn != nil {
		return m.getTopicInfoFn(topicId)
	}
	return TopicMetadata{
		TopicRef:       TopicRef{transport: BackendHCS, locator: topicId},
		SequenceNumber: 42,
		Memo:           "test topic",
	}, nil
}

func TestHCSAdapter_CreateTopic(t *testing.T) {
	client := &mockHCSClient{
		createTopicFn: func(memo string) (string, error) {
			assert.Equal(t, "my topic", memo)
			return "0.0.99999", nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "my topic"})
	require.NoError(t, err)
	assert.Equal(t, BackendHCS, ref.transport)
	assert.Equal(t, "0.0.99999", ref.locator)
}

func TestHCSAdapter_CreateTopic_ClientError(t *testing.T) {
	client := &mockHCSClient{
		createTopicFn: func(_ string) (string, error) {
			return "", errors.New("network error")
		},
	}
	adapter := NewHCSAdapter(client)

	_, err := adapter.CreateTopic(CreateTopicOpts{Memo: "fail"})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestHCSAdapter_Publish_FireAndForget(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hello"))
	require.NoError(t, err)

	client := &mockHCSClient{
		submitMessageFn: func(topicId string, message []byte) (string, error) {
			assert.Equal(t, "0.0.12345", topicId)
			// Verify the message is valid JSON.
			var decoded TopicMessage
			require.NoError(t, json.Unmarshal(message, &decoded))
			return "tx-fire", nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: FireAndForget})
	require.NoError(t, err)

	assert.Equal(t, "tx-fire", result.TransactionRef)
	assert.False(t, result.Confirmed)
	assert.Nil(t, result.ConsensusTimestamp)
	assert.Nil(t, result.SequenceNumber)
}

func TestHCSAdapter_Publish_WaitForConsensus(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hello"))
	require.NoError(t, err)

	client := &mockHCSClient{
		submitMessageWaitFn: func(topicId string, _ []byte) (string, uint64, uint64, error) {
			assert.Equal(t, "0.0.12345", topicId)
			return "tx-wait", 1700000000000000000, 7, nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)

	assert.Equal(t, "tx-wait", result.TransactionRef)
	assert.True(t, result.Confirmed)
	require.NotNil(t, result.ConsensusTimestamp)
	assert.Equal(t, uint64(1700000000000000000), *result.ConsensusTimestamp)
	require.NotNil(t, result.SequenceNumber)
	assert.Equal(t, uint64(7), *result.SequenceNumber)
}

func TestHCSAdapter_Publish_MessageTooLarge(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Create a payload that will exceed 1024 bytes when serialized to JSON.
	bigPayload := make([]byte, 2000)
	for i := range bigPayload {
		bigPayload[i] = 0xAB
	}

	msg, err := NewTopicMessage(&key, 100, 1, bigPayload)
	require.NoError(t, err)

	client := &mockHCSClient{}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	_, err = adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: FireAndForget})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
}

func TestHCSAdapter_MaxMessageSize(t *testing.T) {
	adapter := NewHCSAdapter(&mockHCSClient{})
	assert.Equal(t, uint64(1024), adapter.MaxMessageSize())
}

func TestHCSAdapter_SupportedTransport(t *testing.T) {
	adapter := NewHCSAdapter(&mockHCSClient{})
	assert.Equal(t, BackendHCS, adapter.SupportedTransport())
}

func TestHCSAdapter_EstimatePublishCost(t *testing.T) {
	adapter := NewHCSAdapter(&mockHCSClient{})

	cost, err := adapter.EstimatePublishCost(512)
	require.NoError(t, err)
	assert.Equal(t, "tinybar", cost.Unit)
	assert.Greater(t, cost.Amount, uint64(0))
}

func TestHCSAdapter_EstimatePublishCost_TooLarge(t *testing.T) {
	adapter := NewHCSAdapter(&mockHCSClient{})

	_, err := adapter.EstimatePublishCost(2000)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
}

func TestHCSAdapter_Subscribe(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("sub test"))
	require.NoError(t, err)

	msgJSON, err := json.Marshal(msg)
	require.NoError(t, err)

	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, 1)
			ch <- HCSMessage{
				Contents:           msgJSON,
				ConsensusTimestamp: 1700000000000000001,
				SequenceNumber:     1,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	require.Len(t, deliveries, 1)
	assert.Equal(t, msg.senderAddress, deliveries[0].Message.senderAddress)
	assert.Equal(t, uint64(1700000000000000001), deliveries[0].ConsensusTimestamp)
	assert.Equal(t, uint64(1), deliveries[0].BackendSequence)
}

func TestHCSAdapter_Subscribe_MalformedMessage(t *testing.T) {
	// Malformed messages should be silently skipped.
	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, 2)
			ch <- HCSMessage{
				Contents:           []byte("not valid json"),
				ConsensusTimestamp: 100,
				SequenceNumber:     1,
			}
			// Add a valid message after the malformed one.
			key, _ := keylib.NewNeuronPrivateKey()
			msg, _ := NewTopicMessage(&key, 200, 2, []byte("valid"))
			msgJSON, _ := json.Marshal(msg)
			ch <- HCSMessage{
				Contents:           msgJSON,
				ConsensusTimestamp: 200,
				SequenceNumber:     2,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	// Only the valid message should come through.
	require.Len(t, deliveries, 1)
	assert.Equal(t, uint64(2), deliveries[0].BackendSequence)
}

func TestHCSAdapter_Subscribe_Error(t *testing.T) {
	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			return nil, errors.New("subscribe failed")
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	_, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestHCSAdapter_Resolve(t *testing.T) {
	client := &mockHCSClient{
		getTopicInfoFn: func(topicId string) (TopicMetadata, error) {
			return TopicMetadata{
				TopicRef:       TopicRef{transport: BackendHCS, locator: topicId},
				SequenceNumber: 100,
				Memo:           "resolved",
			}, nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	meta, err := adapter.Resolve(ref)
	require.NoError(t, err)
	assert.Equal(t, uint64(100), meta.SequenceNumber)
	assert.Equal(t, "resolved", meta.Memo)
}

func TestHCSAdapter_NilClient(t *testing.T) {
	adapter := NewHCSAdapter(nil)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}

	_, err := adapter.CreateTopic(CreateTopicOpts{})
	require.Error(t, err)

	key, _ := keylib.NewNeuronPrivateKey()
	msg, _ := NewTopicMessage(&key, 100, 1, []byte("test"))
	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.Error(t, err)

	_, err = adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)

	_, err = adapter.Resolve(ref)
	require.Error(t, err)
}

func TestHCSAdapter_Publish_InvalidRef(t *testing.T) {
	client := &mockHCSClient{}
	adapter := NewHCSAdapter(client)

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("test"))
	require.NoError(t, err)

	// Empty locator should fail validation.
	ref := TopicRef{transport: BackendHCS, locator: ""}
	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.Error(t, err)
}

func TestHCSAdapter_Publish_DefaultMode(t *testing.T) {
	// When ConfirmationMode is not set (empty string), it should default to fire-and-forget.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hi"))
	require.NoError(t, err)

	submitted := false
	client := &mockHCSClient{
		submitMessageFn: func(_ string, _ []byte) (string, error) {
			submitted = true
			return "tx-default", nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	result, err := adapter.Publish(ref, msg, PublishOpts{})
	require.NoError(t, err)

	assert.True(t, submitted, "SubmitMessage should be called for default mode")
	assert.Equal(t, "tx-default", result.TransactionRef)
	assert.False(t, result.Confirmed)
}

func TestHCSAdapter_Subscribe_ErrorHandler_InvalidJSON(t *testing.T) {
	// Verify that ErrorHandler is called when an invalid (non-JSON) message is received.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	validMsg, err := NewTopicMessage(&key, 200, 2, []byte("valid"))
	require.NoError(t, err)
	validJSON, err := json.Marshal(validMsg)
	require.NoError(t, err)

	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, 2)
			ch <- HCSMessage{
				Contents:           []byte("not valid json"),
				ConsensusTimestamp: 100,
				SequenceNumber:     1,
			}
			ch <- HCSMessage{
				Contents:           validJSON,
				ConsensusTimestamp: 200,
				SequenceNumber:     2,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewHCSAdapter(client)

	var mu sync.Mutex
	var handlerErrors []error
	errorHandler := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		handlerErrors = append(handlerErrors, err)
	}

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{
		ErrorHandler: errorHandler,
	})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	// Only the valid message should come through.
	require.Len(t, deliveries, 1)
	assert.Equal(t, uint64(2), deliveries[0].BackendSequence)

	// ErrorHandler should have been called once for the malformed JSON message.
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, handlerErrors, 1)
	topicErr, ok := handlerErrors[0].(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "failed to deserialize message")
}

func TestHCSAdapter_Subscribe_ErrorHandler_InvalidSignature(t *testing.T) {
	// Verify that ErrorHandler is called when a message with invalid signature is received.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	validMsg, err := NewTopicMessage(&key, 200, 2, []byte("valid"))
	require.NoError(t, err)
	validJSON, err := json.Marshal(validMsg)
	require.NoError(t, err)

	// Create a message with a tampered signature (valid JSON but invalid signature).
	tamperedMsg, err := NewTopicMessage(&key, 100, 1, []byte("tampered"))
	require.NoError(t, err)

	// Tamper with the signature by serializing, modifying sig bytes, and re-serializing.
	tamperedJSON, err := json.Marshal(tamperedMsg)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(tamperedJSON, &raw))
	// Replace signature with an invalid one (wrong bytes, but correct base64 length).
	raw["signature"] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	tamperedJSON, err = json.Marshal(raw)
	require.NoError(t, err)

	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, 2)
			ch <- HCSMessage{
				Contents:           tamperedJSON,
				ConsensusTimestamp: 100,
				SequenceNumber:     1,
			}
			ch <- HCSMessage{
				Contents:           validJSON,
				ConsensusTimestamp: 200,
				SequenceNumber:     2,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewHCSAdapter(client)

	var mu sync.Mutex
	var handlerErrors []error
	errorHandler := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		handlerErrors = append(handlerErrors, err)
	}

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{
		ErrorHandler: errorHandler,
	})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	// Only the valid message should come through.
	require.Len(t, deliveries, 1)
	assert.Equal(t, uint64(2), deliveries[0].BackendSequence)

	// ErrorHandler should have been called once for the invalid-signature message.
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, handlerErrors, 1)
	// The error from ValidateTopicMessage should be a TopicError with either
	// InvalidSignature (recovery failure) or SenderMismatch (wrong recovered address).
	topicErr, ok := handlerErrors[0].(TopicError)
	require.True(t, ok)
	assert.True(t, topicErr.Kind == ErrInvalidSignature || topicErr.Kind == ErrSenderMismatch,
		"expected InvalidSignature or SenderMismatch, got %s", topicErr.Kind)
}
