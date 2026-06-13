package registry

import (
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Deterministic Signing Tests (T047) ---

func TestDeterministicSigning_Register(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)
	payload, err := agentURI.ToJSON()
	require.NoError(t, err)

	// Sign the same payload twice.
	sig1, err := childKey.Sign([]byte(payload))
	require.NoError(t, err)

	sig2, err := childKey.Sign([]byte(payload))
	require.NoError(t, err)

	// Signatures MUST be identical (RFC 6979 determinism).
	assert.Equal(t, sig1.Bytes(), sig2.Bytes(),
		"signatures for the same key and payload must be identical (RFC 6979)")

	// Verify both signatures are valid.
	assert.True(t, sig1.Verify([]byte(payload), childKey.PublicKey()))
	assert.True(t, sig2.Verify([]byte(payload), childKey.PublicKey()))
}

func TestDeterministicSigning_Update(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	agentURI := buildValidAgentURI(t, &childKey)
	payload, err := agentURI.ToJSON()
	require.NoError(t, err)

	// Simulate an update: slightly modified payload.
	updatePayload := payload + "_updated"

	sig1, err := childKey.Sign([]byte(updatePayload))
	require.NoError(t, err)

	sig2, err := childKey.Sign([]byte(updatePayload))
	require.NoError(t, err)

	// Deterministic: same key + same payload = same signature.
	assert.Equal(t, sig1.Bytes(), sig2.Bytes(),
		"update signatures for the same key and payload must be identical (RFC 6979)")

	// Verify.
	assert.True(t, sig1.Verify([]byte(updatePayload), childKey.PublicKey()))
}

func TestDeterministicSigning_DifferentPayloadsDifferentSignatures(t *testing.T) {
	childKey, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	sig1, err := childKey.Sign([]byte("payload-A"))
	require.NoError(t, err)

	sig2, err := childKey.Sign([]byte("payload-B"))
	require.NoError(t, err)

	// Different payloads MUST produce different signatures.
	assert.NotEqual(t, sig1.Bytes(), sig2.Bytes(),
		"different payloads should produce different signatures")
}
