package account

import (
	"strings"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateDID verifies that GenerateDID produces a valid did:key:zQ3s... identifier
// from a valid NeuronPublicKey.
func TestGenerateDID(t *testing.T) {
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pubKey := pk.PublicKey()
	did, err := GenerateDID(pubKey)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(did.identifier, "did:key:zQ3s"),
		"DID should start with did:key:zQ3s, got: %s", did.identifier)
	assert.NotEmpty(t, did.identifier)
}

// TestGenerateDID_Deterministic verifies that the same public key always produces
// the same DID.
func TestGenerateDID_Deterministic(t *testing.T) {
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pubKey := pk.PublicKey()
	did1, err := GenerateDID(pubKey)
	require.NoError(t, err)

	did2, err := GenerateDID(pubKey)
	require.NoError(t, err)

	assert.Equal(t, did1.identifier, did2.identifier,
		"same public key must produce the same DID")
}

// TestGenerateDID_DifferentKeys verifies that different public keys produce different DIDs.
func TestGenerateDID_DifferentKeys(t *testing.T) {
	pk1, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)
	pk2, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	did1, err := GenerateDID(pk1.PublicKey())
	require.NoError(t, err)
	did2, err := GenerateDID(pk2.PublicKey())
	require.NoError(t, err)

	assert.NotEqual(t, did1.identifier, did2.identifier,
		"different public keys must produce different DIDs")
}

// TestGenerateDID_InvalidKey verifies that GenerateDID returns an error for a
// zero-value public key.
func TestGenerateDID_InvalidKey(t *testing.T) {
	var zeroPubKey keylib.NeuronPublicKey
	_, err := GenerateDID(zeroPubKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zero-value public key")
}

// TestGenerateDID_MatchesDIDKey verifies that GenerateDID produces the same
// identifier as pubKey.DIDKey().
func TestGenerateDID_MatchesDIDKey(t *testing.T) {
	pk, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	pubKey := pk.PublicKey()
	did, err := GenerateDID(pubKey)
	require.NoError(t, err)

	assert.Equal(t, pubKey.DIDKey(), did.identifier,
		"GenerateDID must match pubKey.DIDKey()")
}
