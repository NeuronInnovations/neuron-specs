package keylib

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T038: Deterministic Signing Verification (Constitution X MANDATORY) ---
//
// Constitution Principle X requires RFC 6979 deterministic nonces for ECDSA signing.
// go-ethereum's crypto.Sign implements this: given the same private key and message,
// the nonce k is derived deterministically from the key and hash, producing an
// identical R, S, and V every time.
//
// These tests verify that the Neuron SDK signing pipeline preserves this property
// end-to-end through the NeuronPrivateKey.Sign() -> Signature abstraction.

// Hardhat account #0 — well-known deterministic test key.
const deterministicTestKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// Hardhat account #1 — second deterministic test key for cross-key comparison.
const deterministicTestKey2 = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"

func TestDeterministicSigning(t *testing.T) {
	key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
	require.NoError(t, err)

	// Three distinct messages for comprehensive determinism verification.
	messages := []struct {
		name string
		data []byte
	}{
		{"plaintext message", []byte("neuron-sdk: deterministic nonce verification")},
		{"binary payload", []byte{0x00, 0x01, 0x02, 0xDE, 0xAD, 0xBE, 0xEF}},
		{"empty message", []byte{}},
	}

	t.Run("same key and same message produce byte-identical signature", func(t *testing.T) {
		for _, msg := range messages {
			t.Run(msg.name, func(t *testing.T) {
				sig1, err := key.Sign(msg.data)
				require.NoError(t, err)

				sig2, err := key.Sign(msg.data)
				require.NoError(t, err)

				// Full 65-byte comparison (R || S || V).
				assert.Equal(t, sig1.Bytes(), sig2.Bytes(),
					"RFC 6979: identical key + message MUST produce identical R||S||V")

				// Component-level comparison for clarity.
				assert.Equal(t, sig1.R(), sig2.R(), "R components must be identical")
				assert.Equal(t, sig1.S(), sig2.S(), "S components must be identical")
				assert.Equal(t, sig1.V(), sig2.V(), "V components must be identical")
			})
		}
	})

	t.Run("same key with different messages produce different signatures", func(t *testing.T) {
		msgA := []byte("message alpha")
		msgB := []byte("message beta")

		sigA, err := key.Sign(msgA)
		require.NoError(t, err)

		sigB, err := key.Sign(msgB)
		require.NoError(t, err)

		assert.NotEqual(t, sigA.Bytes(), sigB.Bytes(),
			"different messages MUST produce different signatures")

		// At minimum, R or S must differ (V can coincidentally match).
		rDiffers := !bytes.Equal(sigA.R(), sigB.R())
		sDiffers := !bytes.Equal(sigA.S(), sigB.S())
		assert.True(t, rDiffers || sDiffers,
			"at least R or S must differ for different messages")
	})

	t.Run("different keys with same message produce different signatures", func(t *testing.T) {
		key2, err := NeuronPrivateKeyFromHex(deterministicTestKey2)
		require.NoError(t, err)

		msg := []byte("shared message for cross-key test")

		sig1, err := key.Sign(msg)
		require.NoError(t, err)

		sig2, err := key2.Sign(msg)
		require.NoError(t, err)

		assert.NotEqual(t, sig1.Bytes(), sig2.Bytes(),
			"different keys MUST produce different signatures for the same message")

		rDiffers := !bytes.Equal(sig1.R(), sig2.R())
		sDiffers := !bytes.Equal(sig1.S(), sig2.S())
		assert.True(t, rDiffers || sDiffers,
			"at least R or S must differ for different keys")
	})

	t.Run("RFC 6979 determinism holds across 10 repeated signings", func(t *testing.T) {
		msg := []byte("repeated signing stress test for RFC 6979 nonce determinism")

		baseline, err := key.Sign(msg)
		require.NoError(t, err)

		for i := 0; i < 10; i++ {
			sig, err := key.Sign(msg)
			require.NoError(t, err)
			assert.Equal(t, baseline.Bytes(), sig.Bytes(),
				"iteration %d: signature must be byte-identical to baseline", i)
		}
	})

	t.Run("deterministic signature is verifiable and recoverable", func(t *testing.T) {
		msg := []byte("verify + recover deterministic signature")

		sig, err := key.Sign(msg)
		require.NoError(t, err)

		// Verify.
		pub := key.PublicKey()
		assert.True(t, sig.Verify(msg, pub),
			"deterministic signature must verify against the signer's public key")

		// Recover.
		recovered, err := sig.RecoverPublicKey(msg)
		require.NoError(t, err)
		assert.Equal(t, pub.Bytes(), recovered.Bytes(),
			"recovered public key must match the signer's public key")
	})
}
