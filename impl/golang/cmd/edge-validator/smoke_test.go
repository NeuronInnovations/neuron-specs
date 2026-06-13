package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// loadValidatorKey + mustParseTopicRef + envOr are the env-bound helpers
// that determine whether `edge-validator` can construct its config.
// Test them directly so a misconfigured deploy fails fast in CI rather
// than at runtime on the validator host.

func TestLoadValidatorKey_PreferDedicatedKey(t *testing.T) {
	hexKey := "1111111111111111111111111111111111111111111111111111111111111111"
	t.Setenv("NEURON_EDGE_VALIDATOR_PRIVATE_KEY", hexKey)
	t.Setenv("NEURON_EDGE_PRIVATE_KEY", "2222222222222222222222222222222222222222222222222222222222222222")
	priv, err := loadValidatorKey()
	require.NoError(t, err)
	// Don't assert exact bytes — secrecy. Confirm derivable pubkey is
	// what the dedicated key would produce, distinct from PRIVATE_KEY.
	pub := priv.PublicKey()
	assert.NotEmpty(t, pub.EVMAddress().Hex())
	// Cross-check: not the EVM addr derived from PRIVATE_KEY.
	t.Setenv("NEURON_EDGE_VALIDATOR_PRIVATE_KEY", "")
	t.Setenv("NEURON_EDGE_PRIVATE_KEY", "2222222222222222222222222222222222222222222222222222222222222222")
	priv2, err := loadValidatorKey()
	require.NoError(t, err)
	assert.NotEqual(t, pub.EVMAddress().Hex(), priv2.PublicKey().EVMAddress().Hex())
}

func TestLoadValidatorKey_FallsBackToOperatorKey(t *testing.T) {
	t.Setenv("NEURON_EDGE_VALIDATOR_PRIVATE_KEY", "")
	t.Setenv("NEURON_EDGE_PRIVATE_KEY", "3333333333333333333333333333333333333333333333333333333333333333")
	priv, err := loadValidatorKey()
	require.NoError(t, err)
	assert.NotEmpty(t, priv.PublicKey().EVMAddress().Hex())
}

func TestLoadValidatorKey_RejectsMissing(t *testing.T) {
	t.Setenv("NEURON_EDGE_VALIDATOR_PRIVATE_KEY", "")
	t.Setenv("NEURON_EDGE_PRIVATE_KEY", "")
	_, err := loadValidatorKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestLoadValidatorKey_RejectsMalformedHex(t *testing.T) {
	t.Setenv("NEURON_EDGE_VALIDATOR_PRIVATE_KEY", "this is not hex")
	t.Setenv("NEURON_EDGE_PRIVATE_KEY", "")
	_, err := loadValidatorKey()
	require.Error(t, err)
}

func TestMustParseTopicRef_RejectsEmpty(t *testing.T) {
	if err := os.Unsetenv("NEURON_EDGE_VALIDATOR_STDOUT"); err != nil {
		t.Fatal(err)
	}
	_, err := mustParseTopicRef("NEURON_EDGE_VALIDATOR_STDOUT", topic.BackendHCS)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestMustParseTopicRef_BuildsRef(t *testing.T) {
	t.Setenv("NEURON_EDGE_VALIDATOR_STDOUT", "0.0.123456")
	ref, err := mustParseTopicRef("NEURON_EDGE_VALIDATOR_STDOUT", topic.BackendHCS)
	require.NoError(t, err)
	assert.Equal(t, "0.0.123456", ref.Locator())
	assert.Equal(t, topic.BackendHCS, ref.Transport())
}

func TestEnvOr(t *testing.T) {
	t.Setenv("NEURON_EDGE_VALIDATOR_AGENT_ID", "42")
	assert.Equal(t, "42", envOr("NEURON_EDGE_VALIDATOR_AGENT_ID", "1"))
	assert.Equal(t, "default", envOr("NEURON_EDGE_VALIDATOR_NONEXISTENT_KEY", "default"))
}
