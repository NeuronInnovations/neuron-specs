package topic

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicSigning_TopicMessage(t *testing.T) {
	// RFC 6979 guarantees deterministic nonces, so signing the same message
	// with the same key MUST produce identical signatures.
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	timestamp := uint64(1700000000000000000)
	seqNum := uint64(99)
	payload := []byte("deterministic signing test")

	msg1, err := NewTopicMessage(&key, timestamp, seqNum, payload)
	require.NoError(t, err)

	msg2, err := NewTopicMessage(&key, timestamp, seqNum, payload)
	require.NoError(t, err)

	assert.Equal(t, msg1.signature, msg2.signature,
		"RFC 6979 signing must be deterministic: same key + same message = same signature")
	assert.Equal(t, msg1.senderAddress, msg2.senderAddress)
}

func TestDifferentPayloads_DifferentSignatures(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	timestamp := uint64(1700000000000000000)
	seqNum := uint64(1)

	msg1, err := NewTopicMessage(&key, timestamp, seqNum, []byte("payload A"))
	require.NoError(t, err)

	msg2, err := NewTopicMessage(&key, timestamp, seqNum, []byte("payload B"))
	require.NoError(t, err)

	assert.NotEqual(t, msg1.signature, msg2.signature,
		"different payloads must produce different signatures")
}

func TestDifferentTimestamps_DifferentSignatures(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	payload := []byte("same payload")

	msg1, err := NewTopicMessage(&key, 100, 1, payload)
	require.NoError(t, err)

	msg2, err := NewTopicMessage(&key, 200, 1, payload)
	require.NoError(t, err)

	assert.NotEqual(t, msg1.signature, msg2.signature,
		"different timestamps must produce different signatures")
}

func TestDifferentSequenceNumbers_DifferentSignatures(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	payload := []byte("same payload")

	msg1, err := NewTopicMessage(&key, 100, 1, payload)
	require.NoError(t, err)

	msg2, err := NewTopicMessage(&key, 100, 2, payload)
	require.NoError(t, err)

	assert.NotEqual(t, msg1.signature, msg2.signature,
		"different sequence numbers must produce different signatures")
}

func TestDifferentKeys_DifferentSignatures(t *testing.T) {
	key1, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	key2, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	payload := []byte("same payload")

	msg1, err := NewTopicMessage(&key1, 100, 1, payload)
	require.NoError(t, err)

	msg2, err := NewTopicMessage(&key2, 100, 1, payload)
	require.NoError(t, err)

	assert.NotEqual(t, msg1.signature, msg2.signature,
		"different keys must produce different signatures")
	assert.NotEqual(t, msg1.senderAddress, msg2.senderAddress,
		"different keys must produce different sender addresses")
}
