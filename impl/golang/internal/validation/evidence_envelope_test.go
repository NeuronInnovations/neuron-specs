package validation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Construction tests (T004) ---

func TestNewEvidenceEnvelope_Valid(t *testing.T) {
	env, err := NewEvidenceEnvelope(
		"42", "7", "005-health",
		VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTestHash",
	)
	require.NoError(t, err)
	assert.Equal(t, PayloadTypeEvidence, env.Type())
	assert.Equal(t, CurrentVersion, env.Version())
	assert.Equal(t, "42", env.ValidatorAgentId())
	assert.Equal(t, "7", env.SubjectAgentId())
	assert.Equal(t, "005-health", env.SpecRef())
	assert.Equal(t, VerdictCompliant, env.Verdict())
	assert.Nil(t, env.CompositeRefs())
	assert.Nil(t, env.ObservationWindow())
}

func TestNewEvidenceEnvelope_WithOptions(t *testing.T) {
	env, err := NewEvidenceEnvelope(
		"1", "2", "008-payment",
		VerdictNonCompliant,
		"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"https://evidence.example.com/doc1",
		WithObservationWindow(1000, 2000),
		WithCompositeRefs([]string{
			"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}),
	)
	require.NoError(t, err)

	w := env.ObservationWindow()
	require.NotNil(t, w)
	assert.Equal(t, uint64(1000), w.Start)
	assert.Equal(t, uint64(2000), w.End)

	refs := env.CompositeRefs()
	require.Len(t, refs, 1)
}

func TestNewEvidenceEnvelope_InvalidVerdict(t *testing.T) {
	_, err := NewEvidenceEnvelope("1", "2", "005-health", "pass",
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verdict")
}

func TestNewEvidenceEnvelope_EmptyValidatorAgentId(t *testing.T) {
	_, err := NewEvidenceEnvelope("", "2", "005-health", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validatorAgentId")
}

func TestNewEvidenceEnvelope_InvalidAgentId(t *testing.T) {
	_, err := NewEvidenceEnvelope("abc", "2", "005-health", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UnsignedInt256")
}

func TestNewEvidenceEnvelope_LeadingZeroAgentId(t *testing.T) {
	_, err := NewEvidenceEnvelope("042", "2", "005-health", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest")
	require.Error(t, err)
}

func TestNewEvidenceEnvelope_ZeroAgentId(t *testing.T) {
	// "0" is a valid UnsignedInt256.
	env, err := NewEvidenceEnvelope("0", "0", "005-health", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest")
	require.NoError(t, err)
	assert.Equal(t, "0", env.ValidatorAgentId())
}

func TestNewEvidenceEnvelope_EmptySpecRef(t *testing.T) {
	_, err := NewEvidenceEnvelope("1", "2", "", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest")
	require.Error(t, err)
}

func TestNewEvidenceEnvelope_EmptyEvidenceHash(t *testing.T) {
	_, err := NewEvidenceEnvelope("1", "2", "005-health", VerdictCompliant,
		"", "ipfs://QmTest")
	require.Error(t, err)
}

func TestNewEvidenceEnvelope_InvalidEvidenceHash(t *testing.T) {
	// Uppercase hex is rejected.
	_, err := NewEvidenceEnvelope("1", "2", "005-health", VerdictCompliant,
		"0xABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789",
		"ipfs://QmTest")
	require.Error(t, err)
}

func TestNewEvidenceEnvelope_EmptyEvidenceURI(t *testing.T) {
	_, err := NewEvidenceEnvelope("1", "2", "005-health", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"")
	require.Error(t, err)
}

func TestNewEvidenceEnvelope_InvalidObservationWindow(t *testing.T) {
	_, err := NewEvidenceEnvelope("1", "2", "005-health", VerdictCompliant,
		"0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
		"ipfs://QmTest",
		WithObservationWindow(2000, 1000), // end < start
	)
	require.Error(t, err)
}

// --- Serialization tests (T007) ---

func TestEvidenceEnvelope_MarshalJSON_CanonicalOrder(t *testing.T) {
	// FR-V03: Verify exact field ordering.
	env, err := NewEvidenceEnvelope(
		"42", "7", "005-health",
		VerdictNonCompliant,
		"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"ipfs://QmTestEvidence",
	)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err)

	expected := `{"type":"validationEvidence","version":"1.0.0","validatorAgentId":"42","subjectAgentId":"7","specRef":"005-health","verdict":"non-compliant","evidenceHash":"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","evidenceURI":"ipfs://QmTestEvidence"}`
	assert.Equal(t, expected, string(data))
}

func TestEvidenceEnvelope_MarshalJSON_WithOptionalFields(t *testing.T) {
	// FR-V06: Optional fields in alphabetical order (compositeRefs before observationWindow).
	env, err := NewEvidenceEnvelope(
		"10", "20", "008-payment",
		VerdictCompliant,
		"0x1111111111111111111111111111111111111111111111111111111111111111",
		"https://evidence.example.com/doc",
		WithCompositeRefs([]string{
			"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}),
		WithObservationWindow(1000000, 2000000),
	)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err)

	// Verify compositeRefs comes before observationWindow.
	// observationWindow.end/start are quoted decimal strings per 006 FR-W02 / FR-W02a.
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"compositeRefs":["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]`)
	assert.Contains(t, jsonStr, `"observationWindow":{"end":"2000000","start":"1000000"}`)

	// Verify compositeRefs appears before observationWindow in the JSON string.
	crIdx := strings.Index(jsonStr, `"compositeRefs"`)
	owIdx := strings.Index(jsonStr, `"observationWindow"`)
	require.NotEqual(t, -1, crIdx, "compositeRefs must be present")
	require.NotEqual(t, -1, owIdx, "observationWindow must be present")
	assert.Less(t, crIdx, owIdx, "compositeRefs must appear before observationWindow")
}

func TestEvidenceEnvelope_MarshalJSON_OmitAbsentOptional(t *testing.T) {
	// 006 FR-W04: Absent optional fields are omitted entirely.
	env, err := NewEvidenceEnvelope(
		"1", "2", "005-health",
		VerdictInconclusive,
		"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"ipfs://QmTest",
	)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err)

	jsonStr := string(data)
	assert.NotContains(t, jsonStr, "compositeRefs")
	assert.NotContains(t, jsonStr, "observationWindow")
}

// --- Round-trip test ---

func TestEvidenceEnvelope_RoundTrip(t *testing.T) {
	env, err := NewEvidenceEnvelope(
		"42", "7", "005-health",
		VerdictCompliant,
		"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"ipfs://QmTestHash",
		WithObservationWindow(100, 200),
		WithCompositeRefs([]string{
			"0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		}),
	)
	require.NoError(t, err)

	data, err := json.Marshal(env)
	require.NoError(t, err)

	var parsed EvidenceEnvelope
	require.NoError(t, json.Unmarshal(data, &parsed))

	assert.Equal(t, env.Type(), parsed.Type())
	assert.Equal(t, env.Version(), parsed.Version())
	assert.Equal(t, env.ValidatorAgentId(), parsed.ValidatorAgentId())
	assert.Equal(t, env.SubjectAgentId(), parsed.SubjectAgentId())
	assert.Equal(t, env.SpecRef(), parsed.SpecRef())
	assert.Equal(t, env.Verdict(), parsed.Verdict())
	assert.Equal(t, env.EvidenceHash(), parsed.EvidenceHash())
	assert.Equal(t, env.EvidenceURI(), parsed.EvidenceURI())
	assert.Equal(t, env.CompositeRefs(), parsed.CompositeRefs())
	assert.Equal(t, env.ObservationWindow().Start, parsed.ObservationWindow().Start)
	assert.Equal(t, env.ObservationWindow().End, parsed.ObservationWindow().End)
}

// --- Hash tests (T008) ---

func TestComputeEnvelopeHash_Deterministic(t *testing.T) {
	// FR-V09: responseHash = keccak256(canonicalJSON(envelope)).
	env, err := NewEvidenceEnvelope(
		"42", "7", "005-health",
		VerdictCompliant,
		"0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		"ipfs://QmTestHash",
	)
	require.NoError(t, err)

	hash1, err := ComputeEnvelopeHash(env)
	require.NoError(t, err)

	hash2, err := ComputeEnvelopeHash(env)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "envelope hash must be deterministic")
}

func TestComputeEvidenceHash(t *testing.T) {
	// FR-V05: evidenceHash = keccak256(document bytes).
	doc := []byte(`{"finding":"agent was operational","timestamp":1234567890}`)
	hash := ComputeEvidenceHash(doc)

	expected := crypto.Keccak256(doc)
	assert.Equal(t, expected, hash[:])
}

func TestVerifyEvidenceHash(t *testing.T) {
	// SC-V07: Consumer verifies keccak256(content) == evidenceHash.
	doc := []byte(`test evidence document`)
	hash := ComputeEvidenceHash(doc)
	hashStr := FormatEvidenceHash(hash)

	env, err := NewEvidenceEnvelope(
		"1", "2", "005-health",
		VerdictCompliant,
		hashStr,
		"ipfs://QmTest",
	)
	require.NoError(t, err)

	assert.True(t, VerifyEvidenceHash(doc, env))
	assert.False(t, VerifyEvidenceHash([]byte("different document"), env))
}

func TestFormatEvidenceHash(t *testing.T) {
	// 006 FR-W06: 0x-prefixed lowercase hex, 66 chars.
	hash := [32]byte{0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b,
		0x9c, 0x0d, 0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d,
		0x7e, 0x8f, 0x9a, 0x0b, 0x1c, 0x2d, 0x3e, 0x4f,
		0x5a, 0x6b, 0x7c, 0x8d, 0x9e, 0x0f, 0x1a, 0x2b}
	formatted := FormatEvidenceHash(hash)
	assert.Equal(t, "0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b", formatted)
	assert.Len(t, formatted, 66)
}

// --- Validation helper tests ---

func TestIsValidHexBytes(t *testing.T) {
	assert.True(t, isValidHexBytes("0xabcd"))
	assert.True(t, isValidHexBytes("0x1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b"))
	assert.False(t, isValidHexBytes("0xABCD"))   // uppercase
	assert.False(t, isValidHexBytes("abcd"))      // missing 0x
	assert.False(t, isValidHexBytes("0x"))         // too short
	assert.False(t, isValidHexBytes("0xabc"))      // odd length
	assert.False(t, isValidHexBytes("0xghij"))     // invalid hex chars
}

func TestIsValidUnsignedInt256(t *testing.T) {
	assert.True(t, isValidUnsignedInt256("0"))
	assert.True(t, isValidUnsignedInt256("42"))
	assert.True(t, isValidUnsignedInt256("123456789"))
	assert.False(t, isValidUnsignedInt256(""))
	assert.False(t, isValidUnsignedInt256("042"))  // leading zero
	assert.False(t, isValidUnsignedInt256("-1"))
	assert.False(t, isValidUnsignedInt256("abc"))
}

func TestIsValidSpecRef(t *testing.T) {
	assert.True(t, IsValidSpecRef("005-health"))
	assert.True(t, IsValidSpecRef("008-payment"))
	assert.True(t, IsValidSpecRef("aviation"))
	assert.False(t, IsValidSpecRef(""))
	assert.False(t, IsValidSpecRef("has spaces"))
	assert.False(t, IsValidSpecRef("has_underscore"))
}
