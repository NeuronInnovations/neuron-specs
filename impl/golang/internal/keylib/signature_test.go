package keylib

import (
	"crypto/subtle"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMessage is a deterministic message used across signature tests.
var testMessage = []byte("neuron-sdk: deterministic signature test")

// --- T012: Signature Type Tests ---

func TestSignatureFromBytes(t *testing.T) {
	t.Run("valid 65-byte input", func(t *testing.T) {
		// Sign a message to get a real 65-byte signature.
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		// Re-construct from raw bytes.
		raw := sig.Bytes()
		sig2, err := SignatureFromBytes(raw)
		require.NoError(t, err)
		assert.Equal(t, raw, sig2.Bytes())
	})

	t.Run("rejects short input", func(t *testing.T) {
		_, err := SignatureFromBytes(make([]byte, 64))
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("rejects long input", func(t *testing.T) {
		_, err := SignatureFromBytes(make([]byte, 66))
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("rejects empty input", func(t *testing.T) {
		_, err := SignatureFromBytes(nil)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})
}

func TestSignatureBytes(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	sig, err := key.Sign(testMessage)
	require.NoError(t, err)

	b := sig.Bytes()
	assert.Len(t, b, SignatureLength, "Bytes() must return exactly 65 bytes")

	// Verify it is a copy — mutating the returned slice must not affect the signature.
	original := make([]byte, SignatureLength)
	copy(original, b)
	b[0] ^= 0xFF
	assert.Equal(t, original, sig.Bytes(), "Bytes() must return a defensive copy")
}

func TestSignatureAccessors(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	sig, err := key.Sign(testMessage)
	require.NoError(t, err)

	raw := sig.Bytes()

	t.Run("R returns correct 32 bytes", func(t *testing.T) {
		r := sig.R()
		assert.Len(t, r, 32)
		assert.Equal(t, raw[0:32], r)
	})

	t.Run("S returns correct 32 bytes", func(t *testing.T) {
		s := sig.S()
		assert.Len(t, s, 32)
		assert.Equal(t, raw[32:64], s)
	})

	t.Run("V returns correct byte", func(t *testing.T) {
		v := sig.V()
		assert.Equal(t, raw[64], v)
	})

	t.Run("R returns defensive copy", func(t *testing.T) {
		r := sig.R()
		original := make([]byte, 32)
		copy(original, r)
		r[0] ^= 0xFF
		assert.Equal(t, original, sig.R(), "R() must return a defensive copy")
	})

	t.Run("S returns defensive copy", func(t *testing.T) {
		s := sig.S()
		original := make([]byte, 32)
		copy(original, s)
		s[0] ^= 0xFF
		assert.Equal(t, original, sig.S(), "S() must return a defensive copy")
	})
}

func TestStandardV(t *testing.T) {
	tests := []struct {
		name     string
		inputV   byte
		expected byte
	}{
		{"V=0 stays 0", 0, 0},
		{"V=1 stays 1", 1, 1},
		{"V=27 becomes 0", 27, 0},
		{"V=28 becomes 1", 28, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a synthetic 65-byte signature with the desired V.
			raw := make([]byte, SignatureLength)
			raw[64] = tt.inputV

			sig, err := SignatureFromBytes(raw)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, sig.StandardV())
		})
	}
}

func TestEthereumV(t *testing.T) {
	tests := []struct {
		name     string
		inputV   byte
		expected byte
	}{
		{"V=0 becomes 27", 0, 27},
		{"V=1 becomes 28", 1, 28},
		{"V=27 stays 27", 27, 27},
		{"V=28 stays 28", 28, 28},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := make([]byte, SignatureLength)
			raw[64] = tt.inputV

			sig, err := SignatureFromBytes(raw)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, sig.EthereumV())
		})
	}
}

func TestVerify(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	pubkey := key.PublicKey()

	sig, err := key.Sign(testMessage)
	require.NoError(t, err)

	t.Run("valid signature verifies", func(t *testing.T) {
		assert.True(t, sig.Verify(testMessage, pubkey))
	})

	t.Run("tampered message fails", func(t *testing.T) {
		tampered := append([]byte{}, testMessage...)
		tampered[0] ^= 0xFF
		assert.False(t, sig.Verify(tampered, pubkey))
	})

	t.Run("wrong key fails", func(t *testing.T) {
		// Generate a different key.
		otherKey, err := NeuronPrivateKeyFromHex(
			"59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
		)
		require.NoError(t, err)

		assert.False(t, sig.Verify(testMessage, otherKey.PublicKey()))
	})

	t.Run("empty message can be signed and verified", func(t *testing.T) {
		emptyMsg := []byte{}
		emptySig, err := key.Sign(emptyMsg)
		require.NoError(t, err)
		assert.True(t, emptySig.Verify(emptyMsg, pubkey))
	})
}

func TestRecoverPublicKey(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	expectedPubkey := key.PublicKey()

	sig, err := key.Sign(testMessage)
	require.NoError(t, err)

	t.Run("recovers correct public key", func(t *testing.T) {
		recovered, err := sig.RecoverPublicKey(testMessage)
		require.NoError(t, err)
		assert.Equal(t, expectedPubkey.Bytes(), recovered.Bytes())
	})

	t.Run("wrong message recovers different key", func(t *testing.T) {
		wrongMsg := []byte("completely different message")
		recovered, err := sig.RecoverPublicKey(wrongMsg)
		require.NoError(t, err)

		// Recovered key should differ from the actual signer.
		assert.NotEqual(t, expectedPubkey.Bytes(), recovered.Bytes())
	})
}

// --- T013: Sign Method Tests ---

func TestSign(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	t.Run("produces valid 65-byte signature", func(t *testing.T) {
		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		b := sig.Bytes()
		assert.Len(t, b, SignatureLength, "signature must be 65 bytes")
	})

	t.Run("V is 0 or 1", func(t *testing.T) {
		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		v := sig.V()
		assert.True(t, v == 0 || v == 1, "V from Sign() must be 0 or 1, got %d", v)
	})

	t.Run("uses Keccak256 hashing", func(t *testing.T) {
		// Verify by manually checking: sign the message, then ecrecover with
		// Keccak256 hash. If ecrecover succeeds and matches, Keccak256 was used.
		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		hash := crypto.Keccak256(testMessage)
		recovered, err := crypto.Ecrecover(hash, sig.Bytes())
		require.NoError(t, err)

		// Decompress the expected public key.
		ecPub, err := crypto.DecompressPubkey(key.PublicKey().Bytes())
		require.NoError(t, err)
		expected := crypto.FromECDSAPub(ecPub)

		assert.Equal(t, 1, subtle.ConstantTimeCompare(recovered, expected),
			"ecrecover with Keccak256 hash must match the signer's public key")
	})

	t.Run("signature verifies against derived public key", func(t *testing.T) {
		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		assert.True(t, sig.Verify(testMessage, key.PublicKey()))
	})

	t.Run("recovered public key matches derived public key", func(t *testing.T) {
		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		recovered, err := sig.RecoverPublicKey(testMessage)
		require.NoError(t, err)
		assert.Equal(t, key.PublicKey().Bytes(), recovered.Bytes())
	})

	t.Run("deterministic — same message produces same signature", func(t *testing.T) {
		sig1, err := key.Sign(testMessage)
		require.NoError(t, err)

		sig2, err := key.Sign(testMessage)
		require.NoError(t, err)

		assert.Equal(t, sig1.Bytes(), sig2.Bytes(),
			"RFC 6979 deterministic nonce must produce identical signatures")
	})

	t.Run("different messages produce different signatures", func(t *testing.T) {
		sig1, err := key.Sign([]byte("message A"))
		require.NoError(t, err)

		sig2, err := key.Sign([]byte("message B"))
		require.NoError(t, err)

		assert.NotEqual(t, sig1.Bytes(), sig2.Bytes())
	})

	t.Run("fails on zeroized key", func(t *testing.T) {
		zKey, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		zKey.Zeroize()
		require.True(t, zKey.IsZero())

		_, err = zKey.Sign(testMessage)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
		assert.Equal(t, "Sign", keyErr.Operation())
	})
}

// TestSignAndVerifyRoundTrip exercises the full sign-verify-recover cycle
// with the well-known Hardhat account #0 test vector.
func TestSignAndVerifyRoundTrip(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
	require.NoError(t, err)

	messages := [][]byte{
		[]byte("hello neuron"),
		[]byte(""),
		[]byte{0x00, 0x01, 0x02, 0xFF},
		[]byte("a]!@#$%^&*() unicode: \u00e9\u00e8\u00ea"),
	}

	for _, msg := range messages {
		sig, err := key.Sign(msg)
		require.NoError(t, err)

		// Verify succeeds.
		assert.True(t, sig.Verify(msg, key.PublicKey()),
			"verify must succeed for message %q", msg)

		// RecoverPublicKey matches.
		recovered, err := sig.RecoverPublicKey(msg)
		require.NoError(t, err)
		assert.Equal(t, key.PublicKey().Bytes(), recovered.Bytes(),
			"recovered key must match for message %q", msg)
	}
}

// --- FR-A10a: V-Value Canonical Normalization Tests ---

func TestVValueCanonicalNormalization(t *testing.T) {
	// FR-A10a: Canonical storage MUST use V ∈ {0, 1}.
	// Input V values 27/28 MUST be normalized to 0/1 at construction.

	t.Run("V=27 normalized to 0 on construction", func(t *testing.T) {
		raw := make([]byte, SignatureLength)
		raw[64] = 27

		sig, err := SignatureFromBytes(raw)
		require.NoError(t, err)
		assert.Equal(t, byte(0), sig.V(), "V=27 must be stored as 0 per FR-A10a")
	})

	t.Run("V=28 normalized to 1 on construction", func(t *testing.T) {
		raw := make([]byte, SignatureLength)
		raw[64] = 28

		sig, err := SignatureFromBytes(raw)
		require.NoError(t, err)
		assert.Equal(t, byte(1), sig.V(), "V=28 must be stored as 1 per FR-A10a")
	})

	t.Run("Bytes() emits canonical V after normalization", func(t *testing.T) {
		raw := make([]byte, SignatureLength)
		raw[64] = 27

		sig, err := SignatureFromBytes(raw)
		require.NoError(t, err)

		out := sig.Bytes()
		assert.Equal(t, byte(0), out[64],
			"Bytes() must emit canonical V=0, not input V=27")
	})

	t.Run("V=0 and V=1 remain unchanged", func(t *testing.T) {
		for _, v := range []byte{0, 1} {
			raw := make([]byte, SignatureLength)
			raw[64] = v

			sig, err := SignatureFromBytes(raw)
			require.NoError(t, err)
			assert.Equal(t, v, sig.V())
		}
	})

	t.Run("invalid V values rejected", func(t *testing.T) {
		for _, v := range []byte{2, 26, 29, 255} {
			raw := make([]byte, SignatureLength)
			raw[64] = v

			_, err := SignatureFromBytes(raw)
			require.Error(t, err, "V=%d must be rejected", v)

			var keyErr *KeyError
			require.True(t, errors.As(err, &keyErr))
			assert.Equal(t, ErrInvalidFormat, keyErr.Kind())
		}
	})

	t.Run("EthereumV returns 27/28 regardless of input format", func(t *testing.T) {
		// Input V=0 → EthereumV=27
		raw0 := make([]byte, SignatureLength)
		raw0[64] = 0
		sig0, err := SignatureFromBytes(raw0)
		require.NoError(t, err)
		assert.Equal(t, byte(27), sig0.EthereumV())

		// Input V=27 → stored as 0 → EthereumV=27
		raw27 := make([]byte, SignatureLength)
		raw27[64] = 27
		sig27, err := SignatureFromBytes(raw27)
		require.NoError(t, err)
		assert.Equal(t, byte(27), sig27.EthereumV())
	})

	t.Run("signature with V=27 verifies correctly", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		sig, err := key.Sign(testMessage)
		require.NoError(t, err)

		// Construct a new signature with V in Ethereum legacy format (27/28).
		raw := sig.Bytes()
		raw[64] = sig.EthereumV() // 27 or 28
		legacySig, err := SignatureFromBytes(raw)
		require.NoError(t, err)

		// Must still verify correctly after normalization.
		assert.True(t, legacySig.Verify(testMessage, key.PublicKey()),
			"signature with Ethereum legacy V must verify after FR-A10a normalization")
	})
}
