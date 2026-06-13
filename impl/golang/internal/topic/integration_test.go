package topic

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase 10 (Item 8): Full Lifecycle Integration Test ---

func TestIntegration_FullLifecycle_HCS(t *testing.T) {
	// Create topic (HCS) -> publish signed message -> subscribe -> receive MessageDelivery
	// -> validate -> serialize as neuron-topic service in agentURI -> parse agentURI
	// -> extract TopicRef -> verify matches original.

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("full lifecycle test"))
	require.NoError(t, err)

	msgJSON, err := json.Marshal(msg)
	require.NoError(t, err)

	backendTS := uint64(1700000001000000000)

	client := &mockHCSClient{
		createTopicFn: func(memo string) (string, error) {
			return "0.0.88888", nil
		},
		submitMessageWaitFn: func(_ string, _ []byte) (string, uint64, uint64, error) {
			return "tx-lifecycle", backendTS, 1, nil
		},
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, 1)
			ch <- HCSMessage{
				Contents:           msgJSON,
				ConsensusTimestamp: backendTS,
				SequenceNumber:     1,
			}
			close(ch)
			return ch, nil
		},
	}

	adapter := NewHCSAdapter(client)

	// Step 1: Create topic.
	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "lifecycle-topic"})
	require.NoError(t, err)
	assert.Equal(t, "0.0.88888", ref.locator)

	// Step 2: Publish signed message with WaitForConsensus.
	result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)
	assert.True(t, result.Confirmed)

	// Step 3: Subscribe and receive delivery.
	startSeq := uint64(0)
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{FromSequence: &startSeq})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}
	require.Len(t, deliveries, 1)

	// Step 4: Validate message integrity.
	err = ValidateTopicMessage(deliveries[0].Message)
	require.NoError(t, err)

	// Step 5: Serialize as neuron-topic service in agentURI.
	svc := NeuronTopicServiceDef{
		Type:      "neuron-topic",
		Name:      "lifecycle-stdin",
		Version:   "1.0.0",
		Channel:   "stdIn",
		Transport: "hcs",
		Anchor:    "native",
		Config: HCSConfig{
			Network: "mainnet",
			TopicId: ref.locator,
		},
	}

	agentURIDoc := map[string]interface{}{
		"services": []interface{}{svc},
	}
	agentURIJSON, err := json.Marshal(agentURIDoc)
	require.NoError(t, err)

	// Step 6: Parse agentURI.
	topicServices, _, err := ParseAgentURIServices(agentURIJSON)
	require.NoError(t, err)
	require.Len(t, topicServices, 1)

	// Step 7: Extract TopicRef.
	extractedRef, err := ExtractTopicRef(topicServices[0])
	require.NoError(t, err)

	// Step 8: Verify matches original.
	assert.Equal(t, ref.transport, extractedRef.transport)
	assert.Equal(t, ref.locator, extractedRef.locator)
	assert.Equal(t, ref.URI(), extractedRef.URI())
}

// --- Phase 10 (Item 9): Cross-Chain Compatibility Test ---

func TestIntegration_CrossChain_HCS_And_Kafka(t *testing.T) {
	// Same TopicMessage published to HCS and Kafka (two different mock adapters).
	// Verify both return valid PublishResult.
	// Verify subscribers on both receive identical TopicMessage content.
	// Verify consensus timestamps use backend-authoritative sources.

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("cross-chain payload"))
	require.NoError(t, err)

	msgJSON, err := json.Marshal(msg)
	require.NoError(t, err)

	hcsBackendTS := uint64(1700000001000000000)
	kafkaBackendTS := uint64(1700000002000000000)

	// HCS mock.
	hcsClient := &mockHCSClient{
		submitMessageWaitFn: func(_ string, _ []byte) (string, uint64, uint64, error) {
			return "hcs-tx-1", hcsBackendTS, 1, nil
		},
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, 1)
			ch <- HCSMessage{
				Contents:           msgJSON,
				ConsensusTimestamp: hcsBackendTS,
				SequenceNumber:     1,
			}
			close(ch)
			return ch, nil
		},
	}
	hcsAdapter := NewHCSAdapter(hcsClient)

	// Kafka mock.
	kafkaClient := &mockKafkaClient{
		publishAndWaitFn: func(_ string, _ []byte) (string, uint64, error) {
			return "kafka-tx-1", 42, nil
		},
		subscribeFn: func(_ string, _ *uint64) (<-chan KafkaMessage, error) {
			ch := make(chan KafkaMessage, 1)
			ch <- KafkaMessage{
				Value:     msgJSON,
				Offset:    42,
				Timestamp: kafkaBackendTS,
			}
			close(ch)
			return ch, nil
		},
	}
	kafkaAdapter, err := NewKafkaAdapter(kafkaClient, validAnchoringConfig())
	require.NoError(t, err)

	hcsRef := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	kafkaRef := TopicRef{transport: BackendKafka, locator: "cross-chain-topic"}

	// Publish to both.
	hcsResult, err := hcsAdapter.Publish(hcsRef, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)
	assert.True(t, hcsResult.Confirmed)
	require.NotNil(t, hcsResult.ConsensusTimestamp)

	kafkaResult, err := kafkaAdapter.Publish(kafkaRef, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
	require.NoError(t, err)
	assert.True(t, kafkaResult.Confirmed)
	require.NotNil(t, kafkaResult.SequenceNumber)

	// Subscribe from both.
	startSeq := uint64(0)
	hcsDeliveryCh, err := hcsAdapter.Subscribe(context.Background(), hcsRef, SubscribeOpts{FromSequence: &startSeq})
	require.NoError(t, err)

	kafkaDeliveryCh, err := kafkaAdapter.Subscribe(context.Background(), kafkaRef, SubscribeOpts{FromSequence: &startSeq})
	require.NoError(t, err)

	// Collect deliveries.
	var hcsDeliveries, kafkaDeliveries []MessageDelivery
	for d := range hcsDeliveryCh {
		hcsDeliveries = append(hcsDeliveries, d)
	}
	for d := range kafkaDeliveryCh {
		kafkaDeliveries = append(kafkaDeliveries, d)
	}

	require.Len(t, hcsDeliveries, 1)
	require.Len(t, kafkaDeliveries, 1)

	// Verify both received identical TopicMessage content.
	assert.Equal(t, hcsDeliveries[0].Message.senderAddress, kafkaDeliveries[0].Message.senderAddress)
	assert.Equal(t, hcsDeliveries[0].Message.payload, kafkaDeliveries[0].Message.payload)
	assert.Equal(t, hcsDeliveries[0].Message.timestamp, kafkaDeliveries[0].Message.timestamp)
	assert.Equal(t, hcsDeliveries[0].Message.sequenceNumber, kafkaDeliveries[0].Message.sequenceNumber)
	assert.Equal(t, hcsDeliveries[0].Message.signature, kafkaDeliveries[0].Message.signature)

	// Verify consensus timestamps are backend-authoritative (different per backend).
	assert.Equal(t, hcsBackendTS, hcsDeliveries[0].ConsensusTimestamp)
	assert.Equal(t, kafkaBackendTS, kafkaDeliveries[0].ConsensusTimestamp)
	assert.NotEqual(t, hcsDeliveries[0].ConsensusTimestamp, kafkaDeliveries[0].ConsensusTimestamp)

	// Validate messages on both backends.
	assert.NoError(t, ValidateTopicMessage(hcsDeliveries[0].Message))
	assert.NoError(t, ValidateTopicMessage(kafkaDeliveries[0].Message))
}

// --- Phase 10 (Item 10): Transactional Invariant Test ---

func TestIntegration_TransactionalInvariants(t *testing.T) {
	// Verify:
	// 1. Sequenced: messages have monotonically increasing sequence numbers per sender
	// 2. Immutable: published messages not modifiable
	// 3. Verifiable: signatures verifiable against sender
	// 4. Signed: every message has valid signature

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	const numMessages = 5
	var messages []TopicMessage
	var hcsMessages []HCSMessage

	for i := uint64(1); i <= numMessages; i++ {
		msg, err := NewTopicMessage(&key, 1700000000000000000+i, i, []byte(fmt.Sprintf("payload-%d", i)))
		require.NoError(t, err)
		messages = append(messages, msg)

		msgJSON, err := json.Marshal(msg)
		require.NoError(t, err)
		hcsMessages = append(hcsMessages, HCSMessage{
			Contents:           msgJSON,
			ConsensusTimestamp: 1700000000000000000 + i*1000000000,
			SequenceNumber:     i,
		})
	}

	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, _ *uint64) (<-chan HCSMessage, error) {
			ch := make(chan HCSMessage, numMessages)
			for _, m := range hcsMessages {
				ch <- m
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	startSeq := uint64(0)
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{FromSequence: &startSeq})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}
	require.Len(t, deliveries, numMessages)

	t.Run("Sequenced", func(t *testing.T) {
		// Messages have monotonically increasing sequence numbers per sender.
		for i := 1; i < len(deliveries); i++ {
			assert.Greater(t, deliveries[i].Message.sequenceNumber, deliveries[i-1].Message.sequenceNumber,
				"sender sequence numbers must be monotonically increasing")
			assert.Greater(t, deliveries[i].BackendSequence, deliveries[i-1].BackendSequence,
				"backend sequence numbers must be monotonically increasing")
		}
	})

	t.Run("Immutable", func(t *testing.T) {
		// Published messages should not be modifiable through the delivery.
		// Verify the original messages match what was received.
		for i, delivery := range deliveries {
			assert.Equal(t, messages[i].senderAddress, delivery.Message.senderAddress)
			assert.Equal(t, messages[i].payload, delivery.Message.payload)
			assert.Equal(t, messages[i].timestamp, delivery.Message.timestamp)
			assert.Equal(t, messages[i].sequenceNumber, delivery.Message.sequenceNumber)
			assert.Equal(t, messages[i].signature, delivery.Message.signature)
		}
	})

	t.Run("Verifiable", func(t *testing.T) {
		// Signatures must be verifiable against the sender.
		for i, delivery := range deliveries {
			err := ValidateTopicMessage(delivery.Message)
			assert.NoError(t, err, "message %d should have verifiable signature", i+1)
		}
	})

	t.Run("Signed", func(t *testing.T) {
		// Every message has a valid signature (non-empty, 65 bytes).
		for i, delivery := range deliveries {
			assert.NotEmpty(t, delivery.Message.signature, "message %d must have signature", i+1)
			assert.Len(t, delivery.Message.signature, 65, "message %d signature must be 65 bytes", i+1)
		}
	})
}

// --- Phase 10 (Item 12): MaxMessageSize Enforcement Test ---

func TestIntegration_MaxMessageSize_HCS(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	adapter := NewHCSAdapter(&mockHCSClient{})

	t.Run("MaxMessageSize returns documented value", func(t *testing.T) {
		assert.Equal(t, uint64(HCSMaxMessageSize), adapter.MaxMessageSize())
		assert.Equal(t, uint64(1024), adapter.MaxMessageSize())
	})

	t.Run("Publish at limit succeeds", func(t *testing.T) {
		// We need a message that when serialized to JSON is exactly at or under the limit.
		// With empty payload, the JSON overhead is small enough to fit.
		smallMsg, err := NewTopicMessage(&key, 100, 1, nil)
		require.NoError(t, err)

		// Verify the serialized message fits within the limit.
		msgBytes, err := json.Marshal(smallMsg)
		require.NoError(t, err)
		require.LessOrEqual(t, uint64(len(msgBytes)), uint64(HCSMaxMessageSize),
			"test message must fit within HCS limit")

		ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
		_, err = adapter.Publish(ref, smallMsg, PublishOpts{})
		assert.NoError(t, err)
	})

	t.Run("Publish over limit fails with MessageTooLarge", func(t *testing.T) {
		bigPayload := make([]byte, 2000)
		for i := range bigPayload {
			bigPayload[i] = 0xAB
		}
		bigMsg, err := NewTopicMessage(&key, 100, 1, bigPayload)
		require.NoError(t, err)

		ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
		_, err = adapter.Publish(ref, bigMsg, PublishOpts{})
		require.Error(t, err)

		topicErr, ok := err.(TopicError)
		require.True(t, ok)
		assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
	})
}

func TestIntegration_MaxMessageSize_Kafka(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	adapter, err := NewKafkaAdapter(&mockKafkaClient{}, validAnchoringConfig())
	require.NoError(t, err)

	t.Run("MaxMessageSize returns documented value", func(t *testing.T) {
		assert.Equal(t, uint64(KafkaMaxMessageSize), adapter.MaxMessageSize())
		assert.Equal(t, uint64(1_048_576), adapter.MaxMessageSize())
	})

	t.Run("Publish at limit succeeds", func(t *testing.T) {
		// Create a message that fits within the Kafka limit.
		smallMsg, err := NewTopicMessage(&key, 100, 1, []byte("small"))
		require.NoError(t, err)

		msgBytes, err := json.Marshal(smallMsg)
		require.NoError(t, err)
		require.LessOrEqual(t, uint64(len(msgBytes)), uint64(KafkaMaxMessageSize))

		ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
		_, err = adapter.Publish(ref, smallMsg, PublishOpts{})
		assert.NoError(t, err)
	})

	t.Run("Publish over limit fails with MessageTooLarge", func(t *testing.T) {
		bigPayload := make([]byte, KafkaMaxMessageSize+1)
		for i := range bigPayload {
			bigPayload[i] = 0xAB
		}
		bigMsg, err := NewTopicMessage(&key, 100, 1, bigPayload)
		require.NoError(t, err)

		ref := TopicRef{transport: BackendKafka, locator: "test-topic"}
		_, err = adapter.Publish(ref, bigMsg, PublishOpts{})
		require.Error(t, err)

		topicErr, ok := err.(TopicError)
		require.True(t, ok)
		assert.Equal(t, ErrMessageTooLarge, topicErr.Kind)
	})
}
