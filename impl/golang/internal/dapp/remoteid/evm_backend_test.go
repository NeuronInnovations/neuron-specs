package remoteid

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_FR_P16_EVMBackend_RejectsMissingEnv(t *testing.T) {
	t.Parallel()
	t.Run("missing-all", TestMissingEVMEnvVars_MissingAll)
	t.Run("missing-escrow-only", TestMissingEVMEnvVars_MissingEscrowOnly)
	t.Run("missing-token-only", TestMissingEVMEnvVars_MissingTokenOnly)
	t.Run("missing-signer-fallback", TestMissingEVMEnvVars_SignerFallback)
	t.Run("complete-with-fallback-key", TestMissingEVMEnvVars_CompleteWithFallback)
	t.Run("complete-with-primary-key", TestMissingEVMEnvVars_CompleteWithPrimary)
}

func TestMissingEVMEnvVars_MissingAll(t *testing.T) {
	t.Parallel()
	missing := MissingEVMEnvVars(func(_ string) string { return "" })
	// escrow, token, signer-fallback combined string.
	require.Len(t, missing, 3)
	assert.Contains(t, missing, EVMEnvEscrowContract)
	assert.Contains(t, missing, EVMEnvTokenContract)
	assert.Contains(t, missing[2], EVMEnvSignerKey)
	assert.Contains(t, missing[2], EVMEnvSignerKeyFallback)
}

func TestMissingEVMEnvVars_MissingEscrowOnly(t *testing.T) {
	t.Parallel()
	missing := MissingEVMEnvVars(func(k string) string {
		switch k {
		case EVMEnvTokenContract:
			return "0xCAFE0000000000000000000000000000000000ce"
		case EVMEnvSignerKey:
			return "deadbeef"
		}
		return ""
	})
	require.Len(t, missing, 1)
	assert.Equal(t, EVMEnvEscrowContract, missing[0])
}

func TestMissingEVMEnvVars_MissingTokenOnly(t *testing.T) {
	t.Parallel()
	missing := MissingEVMEnvVars(func(k string) string {
		switch k {
		case EVMEnvEscrowContract:
			return "0xCAFE0000000000000000000000000000000000ce"
		case EVMEnvSignerKey:
			return "deadbeef"
		}
		return ""
	})
	require.Len(t, missing, 1)
	assert.Equal(t, EVMEnvTokenContract, missing[0])
}

func TestMissingEVMEnvVars_SignerFallback(t *testing.T) {
	t.Parallel()
	// HEDERA_OPERATOR_KEY satisfies the signer requirement.
	missing := MissingEVMEnvVars(func(k string) string {
		switch k {
		case EVMEnvEscrowContract:
			return "0xCAFE0000000000000000000000000000000000ce"
		case EVMEnvTokenContract:
			return "0xCAFE0000000000000000000000000000000000ff"
		case EVMEnvSignerKeyFallback:
			return "abc123"
		}
		return ""
	})
	assert.Empty(t, missing)
}

func TestMissingEVMEnvVars_CompleteWithFallback(t *testing.T) {
	t.Parallel()
	missing := MissingEVMEnvVars(func(k string) string {
		switch k {
		case EVMEnvEscrowContract, EVMEnvTokenContract:
			return "0xCAFE0000000000000000000000000000000000ce"
		case EVMEnvSignerKeyFallback:
			return "abc"
		}
		return ""
	})
	assert.Empty(t, missing)
}

func TestMissingEVMEnvVars_CompleteWithPrimary(t *testing.T) {
	t.Parallel()
	missing := MissingEVMEnvVars(func(k string) string {
		switch k {
		case EVMEnvEscrowContract, EVMEnvTokenContract:
			return "0xCAFE0000000000000000000000000000000000ce"
		case EVMEnvSignerKey:
			return "abc"
		}
		return ""
	})
	assert.Empty(t, missing)
}

func Test_FR_P25a_EVMBackend_ValidatePricing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		amount  string
		wantErr error
	}{
		{"zero", "0", ErrEVMZeroPricing},
		{"empty", "", ErrEVMZeroPricing},
		{"negative", "-1", ErrEVMZeroPricing},
		{"non-decimal", "abc", nil}, // non-nil error, but not ErrEVMZeroPricing
		{"positive-one", "1", nil},
		{"hbar-tinybar", "100000000", nil},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePricingForEVM(c.amount)
			if c.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, c.wantErr), "want %v, got %v", c.wantErr, err)
			} else if c.amount == "abc" {
				require.Error(t, err)
				assert.False(t, errors.Is(err, ErrEVMZeroPricing))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test_FR_P16_EVMBackend_NewRejectsMissingEnv constructs through the
// public factory with an env-lookup that returns nothing; asserts a
// clear "missing env" error (no fallback to memory).
func Test_FR_P16_EVMBackend_NewRejectsMissingEnv(t *testing.T) {
	t.Parallel()
	_, err := NewEVMBackend(t.Context(), EVMBackendOptions{
		LookupEnv: func(_ string) string { return "" },
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing env")
	assert.Contains(t, err.Error(), EVMEnvEscrowContract)
	assert.Contains(t, err.Error(), "refusing to fall back to memory")
}

func Test_FR_T_HCSBackend_NewRejectsMissingEnv(t *testing.T) {
	t.Parallel()
	_, err := NewHCSBackend(t.Context(), HCSBackendOptions{
		Role:      HCSRoleBuyer,
		LookupEnv: func(_ string) string { return "" },
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing env")
	assert.Contains(t, err.Error(), HCSEnvOperatorAccountID)
	assert.Contains(t, err.Error(), "refusing to fall back to memory")
}
