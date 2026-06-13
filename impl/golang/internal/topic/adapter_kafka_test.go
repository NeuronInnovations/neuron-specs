package topic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockKafkaClient implements KafkaClient for testing.
type mockKafkaClient struct {
	createTopicFn     func(name string, partitions int, replicationFactor int) error
	publishFn         func(topicName string, message []byte) (string, error)
	publishAndWaitFn  func(topicName string, message []byte) (string, uint64, error)
	subscribeFn       func(topicName string, fromOffset *uint64) (<-chan KafkaMessage, error)
	getTopicMetadataFn func(topicName string) (TopicMetadata, error)
}

func (m *mockKafkaClient) CreateTopic(name string, partitions int, replicationFactor int) error {
	if m.createTopicFn != nil {
		return m.createTopicFn(name, partitions, replicationFactor)
	}
	return nil
}

func (m *mockKafkaClient) Publish(topicName string, message []byte) (string, error) {
	if m.publishFn != nil {
		return m.publishFn(topicName, message)
	}
	return "kafka-tx-001", nil
}

func (m *mockKafkaClient) PublishAndWait(topicName string, message []byte) (string, uint64, error) {
	if m.publishAndWaitFn != nil {
		return m.publishAndWaitFn(topicName, message)
	}
	return "kafka-tx-002", 42, nil
}

func (m *mockKafkaClient) Subscribe(topicName string, fromOffset *uint64) (<-chan KafkaMessage, error) {
	if m.subscribeFn != nil {
		return m.subscribeFn(topicName, fromOffset)
	}
	ch := make(chan KafkaMessage)
	close(ch)
	return ch, nil
}

func (m *mockKafkaClient) GetTopicMetadata(topicName string) (TopicMetadata, error) {
	if m.getTopicMetadataFn != nil {
		return m.getTopicMetadataFn(topicName)
	}
	return TopicMetadata{
		TopicRef:       TopicRef{transport: BackendKafka, locator: topicName},
		SequenceNumber: 100,
		Memo:           "test kafka topic",
	}, nil
}

func validAnchoringConfig() AnchoringConfig {
	return AnchoringConfig{
		Method:        "periodic",
		AnchorTopicId: "0.0.54321",
		AnchorNetwork: "mainnet",
		Interval:      "10s",
	}
}

func TestNewKafkaAdapter_ValidConfig(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)
	require.NotNil(t, adapter)
}

func TestNewKafkaAdapter_MissingMethod(t *testing.T) {
	cfg := validAnchoringConfig()
	cfg.Method = ""
	_, err := NewKafkaAdapter(&mockKafkaClient{}, cfg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "method")
}

func TestNewKafkaAdapter_MissingAnchorTopicId(t *testing.T) {
	cfg := validAnchoringConfig()
	cfg.AnchorTopicId = ""
	_, err := NewKafkaAdapter(&mockKafkaClient{}, cfg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "anchorTopicId")
}

func TestNewKafkaAdapter_MissingAnchorNetwork(t *testing.T) {
	cfg := validAnchoringConfig()
	cfg.AnchorNetwork = ""
	_, err := NewKafkaAdapter(&mockKafkaClient{}, cfg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "anchorNetwork")
}

func TestNewKafkaAdapter_MissingInterval(t *testing.T) {
	cfg := validAnchoringConfig()
	cfg.Interval = ""
	_, err := NewKafkaAdapter(&mockKafkaClient{}, cfg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "interval")
}

func TestKafkaAdapter_CreateTopic(t *testing.T) {
	client := &mockKafkaClient{
		createTopicFn: func(name string, partitions int, replicationFactor int) error {
			assert.Equal(t, "my-kafka-topic", name)
			assert.Equal(t, 1, partitions)
			assert.Equal(t, 1, replicationFactor)
			return nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "my-kafka-topic"})
	require.NoError(t, err)
	assert.Equal(t, BackendKafka, ref.transport)
	assert.Equal(t, "my-kafka-topic", ref.locator)
}

func TestKafkaAdapter_CreateTopic_WithConfig(t *testing.T) {
	client := &mockKafkaClient{
		createTopicFn: func(name string, partitions int, replicationFactor int) error {
			assert.Equal(t, "config-topic", name)
			assert.Equal(t, 3, partitions)
			assert.Equal(t, 2, replicationFactor)
			return nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref, err := adapter.CreateTopic(CreateTopicOpts{
		Config: map[string]interface{}{
			"topicName":         "config-topic",
			"partitions":        3,
			"replicationFactor": 2,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "config-topic", ref.locator)
}

func TestKafkaAdapter_CreateTopic_EmptyName(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	_, err = adapter.CreateTopic(CreateTopicOpts{})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
}

func TestKafkaAdapter_CreateTopic_ClientError(t *testing.T) {
	client := &mockKafkaClient{
		createTopicFn: func(_ string, _ int, _ int) error {
			return errors.New("broker unreachable")
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	_, err = adapter.CreateTopic(CreateTopicOpts{Memo: "fail"})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestKafkaAdapter_Publish_FireAndForget(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hello kafka"))
	require.NoError(t, err)

	client := &mockKafkaClient{
		publishFn: func(topicName string, message []byte) (string, error) {
			assert.Equal(t, "test-topic", topicName)
			var decoded TopicMessage
			require.NoError(t, json.Unmarshal(message, &decoded))
			return "kafka-fire-tx", nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: FireAndForget})
	require.NoError(t, err)

	assert.Equal(t, "kafka-fire-tx", result.TransactionRef)
	assert.False(t, result.Confirmed)
	assert.Nil(t, result.SequenceNumber)
}

func TestKafkaAdapter_Publish_WaitForConsensus(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hello kafka"))
	require.NoError(t, err)

	client := &mockKafkaClient{
		publishAndWaitFn: func(topicName string, _ []byte) (string, uint64, error) {
			assert.Equal(t, "test-topic", topicName)
			return "kafka-wait-tx", 99, nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)

	assert.Equal(t, "kafka-wait-tx", result.TransactionRef)
	assert.True(t, result.Confirmed)
	require.NotNil(t, result.SequenceNumber)
	assert.Equal(t, uint64(99), *result.SequenceNumber)
}

func TestKafkaAdapter_Publish_MessageTooLarge(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Create a payload that will exceed KafkaMaxMessageSize when serialized.
	bigPayload := make([]byte, KafkaMaxMessageSize+1)
	for i := range bigPayload {
		bigPayload[i] = 0xAB
	}

	msg, err := NewTopicMessage(&key, 100, 1, bigPayload)
	require.NoError(t, err)

	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	_, err = adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: FireAndForget})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
}

func TestKafkaAdapter_Publish_InvalidRef(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("test"))
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: ""}
	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.Error(t, err)
}

func TestKafkaAdapter_Publish_ClientError(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("test"))
	require.NoError(t, err)

	client := &mockKafkaClient{
		publishFn: func(_ string, _ []byte) (string, error) {
			return "", errors.New("broker down")
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestKafkaAdapter_Subscribe_Success(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("kafka sub"))
	require.NoError(t, err)

	msgJSON, err := json.Marshal(msg)
	require.NoError(t, err)

	client := &mockKafkaClient{
		subscribeFn: func(topicName string, _ *uint64) (<-chan KafkaMessage, error) {
			assert.Equal(t, "test-topic", topicName)
			ch := make(chan KafkaMessage, 1)
			ch <- KafkaMessage{
				Value:     msgJSON,
				Offset:    7,
				Timestamp: 1700000000000000000,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	require.Len(t, deliveries, 1)
	assert.Equal(t, msg.senderAddress, deliveries[0].Message.senderAddress)
	assert.Equal(t, uint64(1700000000000000000), deliveries[0].ConsensusTimestamp)
	assert.Equal(t, uint64(7), deliveries[0].BackendSequence)
}

func TestKafkaAdapter_Subscribe_MalformedMessage(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	validMsg, err := NewTopicMessage(&key, 200, 2, []byte("valid"))
	require.NoError(t, err)
	validJSON, err := json.Marshal(validMsg)
	require.NoError(t, err)

	client := &mockKafkaClient{
		subscribeFn: func(_ string, _ *uint64) (<-chan KafkaMessage, error) {
			ch := make(chan KafkaMessage, 2)
			ch <- KafkaMessage{
				Value:     []byte("bad json"),
				Offset:    1,
				Timestamp: 100,
			}
			ch <- KafkaMessage{
				Value:     validJSON,
				Offset:    2,
				Timestamp: 200,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}

	require.Len(t, deliveries, 1)
	assert.Equal(t, uint64(2), deliveries[0].BackendSequence)
}

func TestKafkaAdapter_Subscribe_Error(t *testing.T) {
	client := &mockKafkaClient{
		subscribeFn: func(_ string, _ *uint64) (<-chan KafkaMessage, error) {
			return nil, errors.New("subscribe failed")
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	_, err = adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestKafkaAdapter_Subscribe_InvalidRef(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: ""}
	_, err = adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)
}

func TestKafkaAdapter_Resolve(t *testing.T) {
	client := &mockKafkaClient{
		getTopicMetadataFn: func(topicName string) (TopicMetadata, error) {
			return TopicMetadata{
				TopicRef:       TopicRef{transport: BackendKafka, locator: topicName},
				SequenceNumber: 250,
				Memo:           "resolved kafka",
			}, nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	meta, err := adapter.Resolve(ref)
	require.NoError(t, err)
	assert.Equal(t, uint64(250), meta.SequenceNumber)
	assert.Equal(t, "resolved kafka", meta.Memo)
}

func TestKafkaAdapter_Resolve_InvalidRef(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: ""}
	_, err = adapter.Resolve(ref)
	require.Error(t, err)
}

func TestKafkaAdapter_MaxMessageSize(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)
	assert.Equal(t, uint64(KafkaMaxMessageSize), adapter.MaxMessageSize())
}

func TestKafkaAdapter_EstimatePublishCost(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	cost, err := adapter.EstimatePublishCost(512)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), cost.Amount)
	assert.Equal(t, "none", cost.Unit)
}

func TestKafkaAdapter_SupportedTransport(t *testing.T) {
	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)
	assert.Equal(t, BackendKafka, adapter.SupportedTransport())
}

func TestKafkaAdapter_NilClient(t *testing.T) {
	adapter, err := NewKafkaAdapter(nil, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}

	_, err = adapter.CreateTopic(CreateTopicOpts{Memo: "test"})
	require.Error(t, err)
	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)

	key, _ := keylib.NewNeuronPrivateKey()
	msg, _ := NewTopicMessage(&key, 100, 1, []byte("test"))
	_, err = adapter.Publish(ref, msg, PublishOpts{})
	require.Error(t, err)
	topicErr, ok = err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)

	_, err = adapter.Subscribe(context.Background(), ref, SubscribeOpts{})
	require.Error(t, err)
	topicErr, ok = err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)

	_, err = adapter.Resolve(ref)
	require.Error(t, err)
	topicErr, ok = err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrBackendUnavailable, topicErr.Kind)
}

func TestKafkaAdapter_ImplementsInterface(t *testing.T) {
	var _ TopicAdapter = (*KafkaAdapter)(nil)
}

func TestKafkaAdapter_Publish_DefaultMode(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("hi"))
	require.NoError(t, err)

	submitted := false
	client := &mockKafkaClient{
		publishFn: func(_ string, _ []byte) (string, error) {
			submitted = true
			return "kafka-default-tx", nil
		},
	}
	adapter, err := NewKafkaAdapter(client, validAnchoringConfig())
	require.NoError(t, err)

	ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
	result, err := adapter.Publish(ref, msg, PublishOpts{})
	require.NoError(t, err)

	assert.True(t, submitted, "Publish should be called for default mode")
	assert.Equal(t, "kafka-default-tx", result.TransactionRef)
	assert.False(t, result.Confirmed)
}
