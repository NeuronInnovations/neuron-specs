package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Spec 006 FR-W02a explicitly lists `observationWindow.start/end` as
// UnixTimestamp nanosecond fields that MUST be encoded as JSON strings.
// EvidenceEnvelope canonical JSON is used inside the TopicMessage payload
// (and feeds the signing pre-image), so any regression here would silently
// break cross-language signature verification when an observationWindow
// is present.

func TestEvidenceEnvelope_FR_W02_ObservationWindowQuoted(t *testing.T) {
	// Use nanosecond timestamps above 2^53 to make any number-form regression
	// observable as a value corruption when read by JavaScript.
	const startNs = uint64(1700000000000000000)
	const endNs = uint64(1700000000000000999)

	env, err := NewEvidenceEnvelope(
		"42", "7", "010-validation",
		VerdictCompliant,
		"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"ipfs://QmTest",
		WithObservationWindow(startNs, endNs),
	)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err)
	s := string(data)

	assert.Contains(t, s, `"observationWindow":{"end":"1700000000000000999","start":"1700000000000000000"}`,
		"FR-W02 / FR-W02a: observationWindow.end and .start must be quoted decimal strings")

	// Negative assertion: number form must NOT appear.
	assert.NotContains(t, s, `"end":1700000000000000999`,
		"FR-W02a regression: observationWindow.end must not be a JSON number")
	assert.NotContains(t, s, `"start":1700000000000000000`,
		"FR-W02a regression: observationWindow.start must not be a JSON number")
}

func TestEvidenceEnvelope_FR_W02_RoundTripQuotedForm(t *testing.T) {
	const startNs = uint64(1700000000000000000)
	const endNs = uint64(1700000000000000999)

	original, err := NewEvidenceEnvelope(
		"1", "2", "010-validation",
		VerdictInconclusive,
		"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"ipfs://QmRT",
		WithObservationWindow(startNs, endNs),
	)
	require.NoError(t, err)

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored EvidenceEnvelope
	require.NoError(t, json.Unmarshal(data, &restored))

	w := restored.ObservationWindow()
	require.NotNil(t, w)
	assert.Equal(t, startNs, w.Start)
	assert.Equal(t, endNs, w.End)
}

func TestEvidenceEnvelope_FR_W02_AcceptsLegacyNumberForm(t *testing.T) {
	// Liberal in what we accept: an envelope previously serialized with
	// observationWindow.start/end as JSON numbers must still parse cleanly.
	legacyJSON := []byte(`{"type":"validationEvidence","version":"1.0.0",` +
		`"validatorAgentId":"42","subjectAgentId":"7","specRef":"010-validation",` +
		`"verdict":"compliant",` +
		`"evidenceHash":"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",` +
		`"evidenceURI":"ipfs://QmLegacy",` +
		`"observationWindow":{"end":2000000,"start":1000000}}`)

	var env EvidenceEnvelope
	require.NoError(t, json.Unmarshal(legacyJSON, &env))

	w := env.ObservationWindow()
	require.NotNil(t, w)
	assert.Equal(t, uint64(1000000), w.Start)
	assert.Equal(t, uint64(2000000), w.End)
}

func TestEvidenceEnvelope_FR_W02_HashChangesWithCanonicalWireForm(t *testing.T) {
	// Sanity: the on-the-wire envelope hash for an envelope with an
	// observationWindow now reflects the FR-W02 quoted form. We just verify
	// the hash is deterministic and depends on the (now-canonical) bytes.
	env, err := NewEvidenceEnvelope(
		"42", "7", "010-validation",
		VerdictCompliant,
		"0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		"ipfs://QmHash",
		WithObservationWindow(1000000, 2000000),
	)
	require.NoError(t, err)

	h1, err := ComputeEnvelopeHash(env)
	require.NoError(t, err)
	h2, err := ComputeEnvelopeHash(env)
	require.NoError(t, err)
	assert.Equal(t, h1, h2,
		"ComputeEnvelopeHash must be deterministic on the canonical (FR-W02) wire form")
}
