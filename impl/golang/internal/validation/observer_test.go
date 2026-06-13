package validation

import (
	"encoding/json"
	"testing"

	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeEvidenceTopicMessage(t *testing.T, key *keylib.NeuronPrivateKey, env *EvidenceEnvelope) topic.TopicMessage {
	t.Helper()
	payload, err := json.Marshal(env)
	require.NoError(t, err)
	msg, err := topic.NewTopicMessage(key, 1000, 1, payload)
	require.NoError(t, err)
	return msg
}

func TestValidateInboundEvidence_Valid(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	env, err := NewEvidenceEnvelope("1", "2", "005-health",
		VerdictCompliant,
		"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"ipfs://QmTest")
	require.NoError(t, err)

	msg := makeEvidenceTopicMessage(t, &key, env)

	parsed, err := ValidateInboundEvidence(msg)
	require.NoError(t, err)
	assert.Equal(t, "1", parsed.ValidatorAgentId())
	assert.Equal(t, VerdictCompliant, parsed.Verdict())
}

func TestValidateInboundEvidence_WrongType(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Create a message with heartbeat type instead of evidence type.
	payload := []byte(`{"type":"heartbeat","version":"1.0.0"}`)
	msg, err := topic.NewTopicMessage(&key, 1000, 1, payload)
	require.NoError(t, err)

	_, err = ValidateInboundEvidence(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validationEvidence")
}

func TestValidateInboundEvidence_InvalidJSON(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	msg, err := topic.NewTopicMessage(&key, 1000, 1, []byte(`{invalid json`))
	require.NoError(t, err)

	_, err = ValidateInboundEvidence(msg)
	require.Error(t, err)
}

func TestValidateInboundEvidence_MissingFields(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	// Envelope with type but missing required fields.
	payload := []byte(`{"type":"validationEvidence","version":"1.0.0","validatorAgentId":"1","subjectAgentId":"2","specRef":"005-health","verdict":"compliant","evidenceHash":"","evidenceURI":"ipfs://test"}`)
	msg, err := topic.NewTopicMessage(&key, 1000, 1, payload)
	require.NoError(t, err)

	_, err = ValidateInboundEvidence(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "evidenceHash")
}

func TestValidateInboundEvidence_InvalidVersion(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	payload := []byte(`{"type":"validationEvidence","version":"2.0.0","validatorAgentId":"1","subjectAgentId":"2","specRef":"005-health","verdict":"compliant","evidenceHash":"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","evidenceURI":"ipfs://test"}`)
	msg, err := topic.NewTopicMessage(&key, 1000, 1, payload)
	require.NoError(t, err)

	_, err = ValidateInboundEvidence(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestValidateInboundEvidence_InvalidVerdict(t *testing.T) {
	key, err := keylib.NewNeuronPrivateKey()
	require.NoError(t, err)

	payload := []byte(`{"type":"validationEvidence","version":"1.0.0","validatorAgentId":"1","subjectAgentId":"2","specRef":"005-health","verdict":"pass","evidenceHash":"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","evidenceURI":"ipfs://test"}`)
	msg, err := topic.NewTopicMessage(&key, 1000, 1, payload)
	require.NoError(t, err)

	_, err = ValidateInboundEvidence(msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verdict")
}
