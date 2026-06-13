package topic

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTopicMessage_Success(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	timestamp := uint64(1700000000000000000)
	seqNum := uint64(42)
	payload := []byte("hello neuron")

	msg, err := NewTopicMessage(&key, timestamp, seqNum, payload)
	require.NoError(t, err)

	// SenderAddress should be EIP-55 checksummed hex of the key's EVM address.
	expectedAddr := key.PublicKey().EVMAddress().Hex()
	assert.Equal(t, expectedAddr, msg.senderAddress)

	// Signature should be 65 bytes (R||S||V).
	assert.Len(t, msg.signature, 65)

	// Timestamp and sequence number should match.
	assert.Equal(t, timestamp, msg.timestamp)
	assert.Equal(t, seqNum, msg.sequenceNumber)

	// Payload should match.
	assert.Equal(t, payload, msg.payload)
}

func TestNewTopicMessage_NilPayload(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, nil)
	require.NoError(t, err)

	assert.Nil(t, msg.payload)
	assert.Len(t, msg.signature, 65)
}

func TestNewTopicMessage_EmptyPayload(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte{})
	require.NoError(t, err)

	assert.Empty(t, msg.payload)
	assert.Len(t, msg.signature, 65)
}

func TestNewTopicMessage_NilKey(t *testing.T) {
	_, err := NewTopicMessage(nil, 100, 1, []byte("test"))
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidSignature, topicErr.Kind)
}

func TestNewTopicMessage_SignatureVerification(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	timestamp := uint64(1700000000000000000)
	seqNum := uint64(7)
	payload := []byte("verify me")

	msg, err := NewTopicMessage(&key, timestamp, seqNum, payload)
	require.NoError(t, err)

	// Rebuild signing input and verify using keylib.
	signingInput := TopicMessageSigningInput(timestamp, seqNum, payload)

	sig, err := keylib.SignatureFromBytes(msg.signature)
	require.NoError(t, err)

	pub := key.PublicKey()
	assert.True(t, sig.Verify(signingInput, pub), "signature should verify against signer's public key")
}

func TestTopicMessageSigningInput(t *testing.T) {
	timestamp := uint64(0x0102030405060708)
	seqNum := uint64(0x090A0B0C0D0E0F10)
	payload := []byte{0xAA, 0xBB}

	input := TopicMessageSigningInput(timestamp, seqNum, payload)

	// Total length should be 8 + 8 + 2 = 18.
	assert.Len(t, input, 18)

	// Verify big-endian encoding of timestamp.
	gotTS := binary.BigEndian.Uint64(input[0:8])
	assert.Equal(t, timestamp, gotTS)

	// Verify big-endian encoding of sequence number.
	gotSeq := binary.BigEndian.Uint64(input[8:16])
	assert.Equal(t, seqNum, gotSeq)

	// Verify payload bytes.
	assert.Equal(t, payload, input[16:])
}

func TestTopicMessage_SignedJSONRoundTrip(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	original, err := NewTopicMessage(&key, 1700000000000000000, 1, []byte("round trip test"))
	require.NoError(t, err)

	// Serialize to JSON.
	jsonBytes, err := original.ToJSON()
	require.NoError(t, err)

	// Deserialize from JSON.
	restored, err := TopicMessageFromJSON(jsonBytes)
	require.NoError(t, err)

	assert.Equal(t, original.senderAddress, restored.senderAddress)
	assert.Equal(t, original.signature, restored.signature)
	assert.Equal(t, original.timestamp, restored.timestamp)
	assert.Equal(t, original.sequenceNumber, restored.sequenceNumber)
	assert.Equal(t, original.payload, restored.payload)

	// Re-serialize and verify byte-for-byte identity.
	jsonBytes2, err := restored.ToJSON()
	require.NoError(t, err)
	assert.Equal(t, jsonBytes, jsonBytes2, "re-serialized JSON should be byte-for-byte identical")
}

func TestTopicMessage_CanonicalFieldOrder(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := NewTopicMessage(&key, 100, 1, []byte("test"))
	require.NoError(t, err)

	jsonBytes, err := msg.ToJSON()
	require.NoError(t, err)

	// Parse into ordered key list using json.Decoder.
	var raw map[string]json.RawMessage
	err = json.Unmarshal(jsonBytes, &raw)
	require.NoError(t, err)

	// Verify all expected fields are present.
	expectedFields := []string{"senderAddress", "signature", "timestamp", "sequenceNumber", "payload"}
	for _, field := range expectedFields {
		_, ok := raw[field]
		assert.True(t, ok, "expected field %q in JSON", field)
	}
}

func TestTopicMessageFromJSON_InvalidJSON(t *testing.T) {
	_, err := TopicMessageFromJSON([]byte("not json"))
	require.Error(t, err)

	topicErr, ok := err.(TopicError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidConfig, topicErr.Kind)
}
