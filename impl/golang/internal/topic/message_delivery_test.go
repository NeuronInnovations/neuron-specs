package topic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageDelivery_FieldAccessibility(t *testing.T) {
	msg := TopicMessage{
		senderAddress:  "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD",
		signature:      []byte{0x01, 0x02, 0x03},
		timestamp:      1234567890000000000,
		sequenceNumber: 1,
		payload:        []byte("hello"),
	}

	delivery := MessageDelivery{
		Message:            msg,
		ConsensusTimestamp: 1234567890000000001,
		BackendSequence:    42,
	}

	assert.Equal(t, msg.senderAddress, delivery.Message.senderAddress)
	assert.Equal(t, msg.signature, delivery.Message.signature)
	assert.Equal(t, msg.timestamp, delivery.Message.timestamp)
	assert.Equal(t, msg.sequenceNumber, delivery.Message.sequenceNumber)
	assert.Equal(t, msg.payload, delivery.Message.payload)
	assert.Equal(t, uint64(1234567890000000001), delivery.ConsensusTimestamp)
	assert.Equal(t, uint64(42), delivery.BackendSequence)
}

func TestMessageDelivery_ZeroValue(t *testing.T) {
	var delivery MessageDelivery

	assert.Equal(t, "", delivery.Message.senderAddress)
	assert.Nil(t, delivery.Message.signature)
	assert.Equal(t, uint64(0), delivery.Message.timestamp)
	assert.Equal(t, uint64(0), delivery.Message.sequenceNumber)
	assert.Nil(t, delivery.Message.payload)
	assert.Equal(t, uint64(0), delivery.ConsensusTimestamp)
	assert.Equal(t, uint64(0), delivery.BackendSequence)
}

func TestMessageDelivery_JSONRoundTrip(t *testing.T) {
	original := MessageDelivery{
		Message: TopicMessage{
			senderAddress:  "0xAbCdEf0123456789AbCdEf0123456789AbCdEf01",
			signature:      []byte{0xAA, 0xBB, 0xCC},
			timestamp:      9999999999,
			sequenceNumber: 7,
			payload:        []byte(`{"action":"ping"}`),
		},
		ConsensusTimestamp: 10000000000,
		BackendSequence:    100,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var deserialized MessageDelivery
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, original.Message.senderAddress, deserialized.Message.senderAddress)
	assert.Equal(t, original.Message.timestamp, deserialized.Message.timestamp)
	assert.Equal(t, original.Message.sequenceNumber, deserialized.Message.sequenceNumber)
	assert.Equal(t, original.ConsensusTimestamp, deserialized.ConsensusTimestamp)
	assert.Equal(t, original.BackendSequence, deserialized.BackendSequence)
}

func TestTopicMessage_ZeroValue(t *testing.T) {
	var msg TopicMessage

	assert.Equal(t, "", msg.senderAddress)
	assert.Nil(t, msg.signature)
	assert.Equal(t, uint64(0), msg.timestamp)
	assert.Equal(t, uint64(0), msg.sequenceNumber)
	assert.Nil(t, msg.payload)
}

func TestTopicMessage_JSONRoundTrip(t *testing.T) {
	original := TopicMessage{
		senderAddress:  "0x1234567890abcdef1234567890abcdef12345678",
		signature:      []byte{0x01, 0x02, 0x03, 0x04, 0x05},
		timestamp:      1700000000000000000,
		sequenceNumber: 42,
		payload:        []byte("test payload"),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var deserialized TopicMessage
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, original.senderAddress, deserialized.senderAddress)
	assert.Equal(t, original.signature, deserialized.signature)
	assert.Equal(t, original.timestamp, deserialized.timestamp)
	assert.Equal(t, original.sequenceNumber, deserialized.sequenceNumber)
	assert.Equal(t, original.payload, deserialized.payload)
}
