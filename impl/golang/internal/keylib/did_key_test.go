package keylib

import (
	"strings"
	"testing"

	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T011: DID:key derivation from NeuronPublicKey.

func TestNeuronPublicKey_DIDKey(t *testing.T) {
	t.Run("returns did:key:zQ3s prefix for secp256k1", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		didKey := pub.DIDKey()

		assert.True(t, strings.HasPrefix(didKey, "did:key:zQ3s"),
			"secp256k1 DID:key must start with 'did:key:zQ3s', got %q", didKey)
	})

	t.Run("has correct did:key:z prefix", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		didKey := pub.DIDKey()

		assert.True(t, strings.HasPrefix(didKey, DIDKeyPrefix),
			"DID:key must start with %q, got %q", DIDKeyPrefix, didKey)
	})

	t.Run("multicodec varint prepended correctly", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		didKey := pub.DIDKey()

		// Extract the base58btc-encoded portion (after "did:key:z").
		encoded := strings.TrimPrefix(didKey, DIDKeyPrefix)
		require.NotEmpty(t, encoded)

		// Decode base58btc to get the raw bytes.
		decoded, err := base58.Decode(encoded)
		require.NoError(t, err)

		// The first two bytes must be the multicodec varint for secp256k1-pub.
		require.True(t, len(decoded) >= 2, "decoded bytes too short")
		assert.Equal(t, MulticodecSecp256k1Pub(), decoded[:2],
			"first two bytes must be multicodec secp256k1-pub varint [0xe7, 0x01]")

		// The remaining 33 bytes must be the compressed public key.
		assert.Equal(t, CompressedPublicKeyLength+len(MulticodecSecp256k1Pub()), len(decoded),
			"decoded length must be multicodec prefix (2) + compressed key (33) = 35 bytes")
		assert.Equal(t, pub.Bytes(), decoded[2:],
			"bytes after multicodec prefix must equal compressed public key")
	})

	t.Run("deterministic: same key always produces same DID:key", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()

		did1 := pub.DIDKey()
		did2 := pub.DIDKey()

		assert.Equal(t, did1, did2,
			"same public key must always produce the same DID:key")
	})

	t.Run("deterministic across separate key constructions", func(t *testing.T) {
		key1, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		key2, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		did1 := key1.PublicKey().DIDKey()
		did2 := key2.PublicKey().DIDKey()

		assert.Equal(t, did1, did2,
			"independently constructed keys with same material must produce the same DID:key")
	})

	t.Run("two different keys produce different DID:key values", func(t *testing.T) {
		key1, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		// Second test vector: Hardhat account #1.
		const testPrivateKeyHex2 = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
		key2, err := NeuronPrivateKeyFromHex(testPrivateKeyHex2)
		require.NoError(t, err)

		did1 := key1.PublicKey().DIDKey()
		did2 := key2.PublicKey().DIDKey()

		assert.NotEqual(t, did1, did2,
			"different keys must produce different DID:key values")

		// Both should still have the correct prefix.
		assert.True(t, strings.HasPrefix(did1, "did:key:zQ3s"))
		assert.True(t, strings.HasPrefix(did2, "did:key:zQ3s"))
	})

	t.Run("encoded portion uses valid base58btc characters", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		didKey := pub.DIDKey()

		encoded := strings.TrimPrefix(didKey, DIDKeyPrefix)

		const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
		for _, c := range encoded {
			assert.True(t, strings.ContainsRune(base58Alphabet, c),
				"DID:key encoded portion contains invalid base58btc character: %c", c)
		}
	})
}
