package remoteid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_FR_T_HCSBackend_RejectsMissingEnv(t *testing.T) {
	t.Parallel()
	t.Run("missing-both", TestMissingHCSEnvVars_MissingBoth)
	t.Run("missing-key-only", TestMissingHCSEnvVars_MissingKeyOnly)
	t.Run("missing-account-only", TestMissingHCSEnvVars_MissingAccountOnly)
	t.Run("complete", TestMissingHCSEnvVars_Complete)
}

func TestMissingHCSEnvVars_MissingBoth(t *testing.T) {
	t.Parallel()
	missing := MissingHCSEnvVars(func(_ string) string { return "" })
	require.Len(t, missing, 2)
	assert.Contains(t, missing, HCSEnvOperatorAccountID)
	assert.Contains(t, missing, HCSEnvOperatorPrivateKey)
}

func TestMissingHCSEnvVars_MissingKeyOnly(t *testing.T) {
	t.Parallel()
	missing := MissingHCSEnvVars(func(k string) string {
		if k == HCSEnvOperatorAccountID {
			return "0.0.1234"
		}
		return ""
	})
	require.Len(t, missing, 1)
	assert.Equal(t, HCSEnvOperatorPrivateKey, missing[0])
}

func TestMissingHCSEnvVars_MissingAccountOnly(t *testing.T) {
	t.Parallel()
	missing := MissingHCSEnvVars(func(k string) string {
		if k == HCSEnvOperatorPrivateKey {
			return "deadbeef"
		}
		return ""
	})
	require.Len(t, missing, 1)
	assert.Equal(t, HCSEnvOperatorAccountID, missing[0])
}

func TestMissingHCSEnvVars_Complete(t *testing.T) {
	t.Parallel()
	missing := MissingHCSEnvVars(func(k string) string {
		switch k {
		case HCSEnvOperatorAccountID:
			return "0.0.1234"
		case HCSEnvOperatorPrivateKey:
			return "deadbeef"
		}
		return ""
	})
	assert.Empty(t, missing)
}
