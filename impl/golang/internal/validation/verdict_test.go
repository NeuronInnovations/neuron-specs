package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidVerdict(t *testing.T) {
	// FR-V04: Accept the three allowed values.
	assert.True(t, IsValidVerdict(VerdictCompliant))
	assert.True(t, IsValidVerdict(VerdictNonCompliant))
	assert.True(t, IsValidVerdict(VerdictInconclusive))

	// Reject invalid values.
	assert.False(t, IsValidVerdict("pass"))
	assert.False(t, IsValidVerdict("fail"))
	assert.False(t, IsValidVerdict(""))
	assert.False(t, IsValidVerdict("Compliant"))
}

func TestVerdictToCode(t *testing.T) {
	// FR-V08: Verdict to registry code mapping.
	code, err := VerdictToCode(VerdictCompliant)
	require.NoError(t, err)
	assert.Equal(t, CodePass, code)

	code, err = VerdictToCode(VerdictNonCompliant)
	require.NoError(t, err)
	assert.Equal(t, CodeFail, code)

	code, err = VerdictToCode(VerdictInconclusive)
	require.NoError(t, err)
	assert.Equal(t, CodeInconclusive, code)
}

func TestVerdictToCode_Invalid(t *testing.T) {
	_, err := VerdictToCode("bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown verdict")
}

func TestCodeToVerdict(t *testing.T) {
	// FR-V08: Registry code to verdict mapping.
	v, err := CodeToVerdict(CodePass)
	require.NoError(t, err)
	assert.Equal(t, VerdictCompliant, v)

	v, err = CodeToVerdict(CodeFail)
	require.NoError(t, err)
	assert.Equal(t, VerdictNonCompliant, v)

	v, err = CodeToVerdict(CodeInconclusive)
	require.NoError(t, err)
	assert.Equal(t, VerdictInconclusive, v)
}

func TestCodeToVerdict_Pending(t *testing.T) {
	// CodePending (0) has no verdict mapping.
	_, err := CodeToVerdict(CodePending)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PENDING")
}

func TestCodeToVerdict_Unknown(t *testing.T) {
	_, err := CodeToVerdict(RegistryCode(99))
	require.Error(t, err)
}

func TestVerdictRoundTrip(t *testing.T) {
	// Verify that verdict→code→verdict round-trips correctly.
	for _, v := range []Verdict{VerdictCompliant, VerdictNonCompliant, VerdictInconclusive} {
		code, err := VerdictToCode(v)
		require.NoError(t, err)
		back, err := CodeToVerdict(code)
		require.NoError(t, err)
		assert.Equal(t, v, back)
	}
}
