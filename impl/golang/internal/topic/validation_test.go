package topic

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTopicMessage_Valid(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("valid message"))
	require.NoError(t, err)

	err = ValidateTopicMessage(msg)
	assert.NoError(t, err)
}

func TestValidateTopicMessage_TamperedPayload(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("original"))
	require.NoError(t, err)

	// Tamper with the payload after signing.
	msg.payload = []byte("tampered")

	err = ValidateTopicMessage(msg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrSenderMismatch, topicErr.Kind)
}

func TestValidateTopicMessage_AlteredSender(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("test"))
	require.NoError(t, err)

	// Replace the sender address with a different address.
	otherKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	msg.senderAddress = otherKey.PublicKey().EVMAddress().Hex()

	err = ValidateTopicMessage(msg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrSenderMismatch, topicErr.Kind)
}

func TestValidateTopicMessage_EmptySignature(t *testing.T) {
	msg := TopicMessage{
		senderAddress:  "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
		signature:      nil,
		timestamp:      100,
		sequenceNumber: 1,
		payload:        []byte("test"),
	}

	err := ValidateTopicMessage(msg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidSignature, topicErr.Kind)
	assert.Contains(t, topicErr.Message, "empty")
}

func TestValidateTopicMessage_InvalidSignatureBytes(t *testing.T) {
	msg := TopicMessage{
		senderAddress:  "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD00",
		signature:      []byte{0x01, 0x02, 0x03}, // Not 65 bytes
		timestamp:      100,
		sequenceNumber: 1,
		payload:        []byte("test"),
	}

	err := ValidateTopicMessage(msg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidSignature, topicErr.Kind)
}

func TestValidateTopicMessage_TamperedTimestamp(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("test"))
	require.NoError(t, err)

	// Tamper with the timestamp.
	msg.timestamp = 9999999999999999999

	err = ValidateTopicMessage(msg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrSenderMismatch, topicErr.Kind)
}

func TestValidateTopicMessage_TamperedSequenceNumber(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("test"))
	require.NoError(t, err)

	// Tamper with the sequence number.
	msg.sequenceNumber = 999

	err = ValidateTopicMessage(msg)
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrSenderMismatch, topicErr.Kind)
}

func TestValidateTopicMessage_NilPayload(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, nil)
	require.NoError(t, err)

	// Validation should succeed for nil payload messages.
	err = ValidateTopicMessage(msg)
	assert.NoError(t, err)
}
