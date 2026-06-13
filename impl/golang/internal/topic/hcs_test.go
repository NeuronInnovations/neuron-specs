package topic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Phase 10 (Item 7): HCS End-to-End Lifecycle Integration Test ---

func TestHCS_EndToEnd_Lifecycle(t *testing.T) {
	// This test exercises the full HCS lifecycle:
	// Create topic -> publish signed message (WaitForConsensus) -> subscribe from seq 0
	// -> receive delivery -> validate message integrity -> verify consensus timestamp
	// is backend-authoritative -> verify per-sender ordering.

	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Prepare three signed messages from the same sender with increasing sequence numbers.
	msg1, err := NewTopicMessage(&key, 1700000000000000001, 1, []byte("message-one"))
	require.NoError(t, err)
	msg2, err := NewTopicMessage(&key, 1700000000000000002, 2, []byte("message-two"))
	require.NoError(t, err)
	msg3, err := NewTopicMessage(&key, 1700000000000000003, 3, []byte("message-three"))
	require.NoError(t, err)

	msg1JSON, err := json.Marshal(msg1)
	require.NoError(t, err)
	msg2JSON, err := json.Marshal(msg2)
	require.NoError(t, err)
	msg3JSON, err := json.Marshal(msg3)
	require.NoError(t, err)

	// Backend-authoritative consensus timestamps (different from sender timestamps).
	backendTS1 := uint64(1700000001000000000)
	backendTS2 := uint64(1700000002000000000)
	backendTS3 := uint64(1700000003000000000)

	publishSeq := uint64(0)
	client := &mockHCSClient{
		createTopicFn: func(memo string) (string, error) {
			assert.Equal(t, "integration-test-topic", memo)
			return "0.0.77777", nil
		},
		submitMessageWaitFn: func(topicId string, message []byte) (string, uint64, uint64, error) {
			assert.Equal(t, "0.0.77777", topicId)
			publishSeq++
			var ts uint64
			switch publishSeq {
			case 1:
				ts = backendTS1
			case 2:
				ts = backendTS2
			case 3:
				ts = backendTS3
			}
			return "tx-" + string(rune('0'+publishSeq)), ts, publishSeq, nil
		},
		subscribeTopicFn: func(topicId string, startSequence *uint64) (<-chan HCSMessage, error) {
			assert.Equal(t, "0.0.77777", topicId)
			require.NotNil(t, startSequence)
			assert.Equal(t, uint64(0), *startSequence)

			ch := make(chan HCSMessage, 3)
			ch <- HCSMessage{Contents: msg1JSON, ConsensusTimestamp: backendTS1, SequenceNumber: 1}
			ch <- HCSMessage{Contents: msg2JSON, ConsensusTimestamp: backendTS2, SequenceNumber: 2}
			ch <- HCSMessage{Contents: msg3JSON, ConsensusTimestamp: backendTS3, SequenceNumber: 3}
			close(ch)
			return ch, nil
		},
	}

	adapter := NewHCSAdapter(client)

	// Step 1: Create topic.
	ref, err := adapter.CreateTopic(CreateTopicOpts{Memo: "integration-test-topic"})
	require.NoError(t, err)
	assert.Equal(t, BackendHCS, ref.transport)
	assert.Equal(t, "0.0.77777", ref.locator)

	// Step 2: Publish all three messages with WaitForConsensus.
	for _, msg := range []TopicMessage{msg1, msg2, msg3} {
		result, err := adapter.Publish(ref, msg, PublishOpts{ConfirmationMode: WaitForConsensus})
		require.NoError(t, err)
		assert.True(t, result.Confirmed)
		require.NotNil(t, result.ConsensusTimestamp)
		require.NotNil(t, result.SequenceNumber)
	}

	// Step 3: Subscribe from sequence 0.
	startSeq := uint64(0)
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{FromSequence: &startSeq})
	require.NoError(t, err)

	// Step 4: Collect all deliveries.
	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}
	require.Len(t, deliveries, 3)

	// Step 5: Validate message integrity for each delivery.
	for i, delivery := range deliveries {
		err := ValidateTopicMessage(delivery.Message)
		assert.NoError(t, err, "message %d should be valid", i+1)
	}

	// Step 6: Verify consensus timestamps are backend-authoritative.
	assert.Equal(t, backendTS1, deliveries[0].ConsensusTimestamp)
	assert.Equal(t, backendTS2, deliveries[1].ConsensusTimestamp)
	assert.Equal(t, backendTS3, deliveries[2].ConsensusTimestamp)

	// Consensus timestamps must differ from sender timestamps.
	assert.NotEqual(t, deliveries[0].Message.timestamp, deliveries[0].ConsensusTimestamp)

	// Step 7: Verify per-sender ordering (monotonically increasing sequence numbers).
	for i := 1; i < len(deliveries); i++ {
		assert.Greater(t, deliveries[i].BackendSequence, deliveries[i-1].BackendSequence,
			"backend sequence should be monotonically increasing")
		assert.Greater(t, deliveries[i].Message.sequenceNumber, deliveries[i-1].Message.sequenceNumber,
			"sender sequence should be monotonically increasing")
	}
}

func TestHCS_SubscribeFromLatest(t *testing.T) {
	// When FromSequence is nil, subscription starts from latest.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("latest"))
	require.NoError(t, err)
	msgJSON, err := json.Marshal(msg)
	require.NoError(t, err)

	client := &mockHCSClient{
		subscribeTopicFn: func(_ string, startSequence *uint64) (<-chan HCSMessage, error) {
			assert.Nil(t, startSequence, "nil FromSequence means subscribe from latest")
			ch := make(chan HCSMessage, 1)
			ch <- HCSMessage{
				Contents:           msgJSON,
				ConsensusTimestamp: 200,
				SequenceNumber:     50,
			}
			close(ch)
			return ch, nil
		},
	}
	adapter := NewHCSAdapter(client)

	ref := TopicRef{transport: BackendHCS, locator: "0.0.12345"}
	deliveryCh, err := adapter.Subscribe(context.Background(), ref, SubscribeOpts{FromSequence: nil})
	require.NoError(t, err)

	var deliveries []MessageDelivery
	for d := range deliveryCh {
		deliveries = append(deliveries, d)
	}
	require.Len(t, deliveries, 1)
	assert.Equal(t, uint64(50), deliveries[0].BackendSequence)
}
