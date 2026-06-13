package keylib

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T004: Failing tests for all 11 error kinds.
// FR-008, FR-008a, SEC-003, SEC-005, SC-002

func TestErrorKind_String(t *testing.T) {
	tests := []struct {
		kind ErrorKind
		want string
	}{
		{ErrInvalidFormat, "InvalidFormat"},
		{ErrInvalidLength, "InvalidLength"},
		{ErrInvalidHex, "InvalidHex"},
		{ErrInvalidKey, "InvalidKey"},
		{ErrZeroValue, "ZeroValue"},
		{ErrKeyMismatch, "KeyMismatch"},
		{ErrEncryption, "Encryption"},
		{ErrMnemonic, "Mnemonic"},
		{ErrDerivation, "Derivation"},
		{ErrUnsupportedKeyType, "UnsupportedKeyType"},
		{ErrSDKError, "SDKError"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestKeyError_Error(t *testing.T) {
	t.Run("descriptive message with operation context", func(t *testing.T) {
		err := NewKeyError(ErrInvalidHex, "NeuronPrivateKeyFromHex", "invalid hex character at position 5")
		msg := err.Error()
		assert.Contains(t, msg, "InvalidHex")
		assert.Contains(t, msg, "NeuronPrivateKeyFromHex")
		assert.Contains(t, msg, "position 5")
	})

	t.Run("message includes error kind", func(t *testing.T) {
		err := NewKeyError(ErrInvalidLength, "NeuronPublicKeyFromBytes", "expected 33 or 65 bytes, got 32")
		msg := err.Error()
		assert.Contains(t, msg, "InvalidLength")
	})

	t.Run("message includes operation name", func(t *testing.T) {
		err := NewKeyError(ErrZeroValue, "NeuronPrivateKeyFromBytes", "private key is all zeros")
		msg := err.Error()
		assert.Contains(t, msg, "NeuronPrivateKeyFromBytes")
	})
}

func TestKeyError_Kind(t *testing.T) {
	tests := []ErrorKind{
		ErrInvalidFormat,
		ErrInvalidLength,
		ErrInvalidHex,
		ErrInvalidKey,
		ErrZeroValue,
		ErrKeyMismatch,
		ErrEncryption,
		ErrMnemonic,
		ErrDerivation,
		ErrUnsupportedKeyType,
		ErrSDKError,
	}

	for _, kind := range tests {
		t.Run(kind.String(), func(t *testing.T) {
			err := NewKeyError(kind, "test", "test message")
			assert.Equal(t, kind, err.Kind())
		})
	}
}

func TestKeyError_Is(t *testing.T) {
	t.Run("same kind matches", func(t *testing.T) {
		err1 := NewKeyError(ErrInvalidHex, "op1", "msg1")
		err2 := NewKeyError(ErrInvalidHex, "op2", "msg2")
		assert.True(t, errors.Is(err1, err2))
	})

	t.Run("different kind does not match", func(t *testing.T) {
		err1 := NewKeyError(ErrInvalidHex, "op1", "msg1")
		err2 := NewKeyError(ErrInvalidLength, "op2", "msg2")
		assert.False(t, errors.Is(err1, err2))
	})
}

func TestKeyError_As(t *testing.T) {
	t.Run("can extract KeyError from wrapped error", func(t *testing.T) {
		original := NewKeyError(ErrInvalidKey, "TestOp", "key fails curve validation")
		wrapped := fmt.Errorf("outer: %w", original)

		var keyErr *KeyError
		require.True(t, errors.As(wrapped, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "TestOp")
	})
}

func TestSDKError_Unwrap(t *testing.T) {
	t.Run("wraps underlying error", func(t *testing.T) {
		underlying := errors.New("secp256k1: invalid point")
		err := NewSDKError("Sign", underlying)

		assert.Equal(t, ErrSDKError, err.Kind())
		assert.True(t, errors.Is(err, underlying), "Unwrap should expose underlying error")
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlying := errors.New("crypto/ecdsa: invalid key")
		err := NewSDKError("FromBlockchainKey", underlying)

		unwrapped := errors.Unwrap(err)
		assert.Equal(t, underlying, unwrapped)
	})

	t.Run("descriptive message includes operation", func(t *testing.T) {
		underlying := errors.New("bad key")
		err := NewSDKError("NeuronPrivateKeyFromBlockchainKey", underlying)
		msg := err.Error()
		assert.Contains(t, msg, "SDKError")
		assert.Contains(t, msg, "NeuronPrivateKeyFromBlockchainKey")
		assert.Contains(t, msg, "bad key")
	})
}

func TestKeyError_NoPrivateKeyMaterial(t *testing.T) {
	// SEC-003, SEC-005: Error messages MUST NOT contain private key material.
	secretHex := "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	secretBytes := []byte{0xac, 0x09, 0x74, 0xbe}

	t.Run("InvalidHex error does not contain secret key hex", func(t *testing.T) {
		err := NewKeyError(ErrInvalidHex, "NeuronPrivateKeyFromHex", "invalid hex character at position 5")
		msg := err.Error()
		assert.NotContains(t, msg, secretHex)
	})

	t.Run("error message from operation context does not leak bytes", func(t *testing.T) {
		err := NewKeyError(ErrInvalidLength, "NeuronPrivateKeyFromBytes", "expected 32 bytes, got 31")
		msg := err.Error()
		assert.NotContains(t, msg, fmt.Sprintf("%x", secretBytes))
	})
}

func TestAllErrorKinds_Exist(t *testing.T) {
	// Verify all 11 error kinds are defined and distinct.
	kinds := []ErrorKind{
		ErrInvalidFormat,
		ErrInvalidLength,
		ErrInvalidHex,
		ErrInvalidKey,
		ErrZeroValue,
		ErrKeyMismatch,
		ErrEncryption,
		ErrMnemonic,
		ErrDerivation,
		ErrUnsupportedKeyType,
		ErrSDKError,
	}

	assert.Len(t, kinds, 11, "must have exactly 11 error kinds")

	seen := make(map[ErrorKind]bool)
	for _, k := range kinds {
		assert.False(t, seen[k], "duplicate error kind: %s", k.String())
		seen[k] = true
	}
}
