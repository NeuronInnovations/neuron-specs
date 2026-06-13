package keylib

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T010: PeerID derivation from NeuronPublicKey.

func TestNeuronPublicKey_PeerID(t *testing.T) {
	t.Run("derives valid PeerID from known test vector", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		peerID, err := pub.PeerID()
		require.NoError(t, err)

		peerStr := peerID.String()
		assert.NotEmpty(t, peerStr)

		// libp2p PeerIDs for secp256k1 keys use the identity multihash
		// when the key is small enough, resulting in a "16Uiu2HA" prefix,
		// or use SHA-256 multihash resulting in a "12D3KooW" prefix.
		// Either way, the string should be non-trivial.
		assert.True(t, len(peerStr) > 10,
			"PeerID string should be a substantial base58btc multihash, got %q", peerStr)
	})

	t.Run("String returns base58btc encoded multihash", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		peerID, err := pub.PeerID()
		require.NoError(t, err)

		peerStr := peerID.String()

		// Base58btc alphabet: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
		// Verify string only contains valid base58btc characters.
		const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
		for _, c := range peerStr {
			assert.True(t, strings.ContainsRune(base58Alphabet, c),
				"PeerID string contains invalid base58btc character: %c", c)
		}
	})

	t.Run("deterministic: same key always produces same PeerID", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()

		peerID1, err := pub.PeerID()
		require.NoError(t, err)

		peerID2, err := pub.PeerID()
		require.NoError(t, err)

		assert.Equal(t, peerID1.String(), peerID2.String(),
			"same public key must always produce the same PeerID")
	})

	t.Run("deterministic across separate key constructions", func(t *testing.T) {
		key1, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		key2, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		peerID1, err := key1.PublicKey().PeerID()
		require.NoError(t, err)

		peerID2, err := key2.PublicKey().PeerID()
		require.NoError(t, err)

		assert.Equal(t, peerID1.String(), peerID2.String(),
			"independently constructed keys with same material must produce the same PeerID")
	})

	t.Run("two different keys produce different PeerIDs", func(t *testing.T) {
		key1, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		// Second test vector: Hardhat account #1.
		const testPrivateKeyHex2 = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
		key2, err := NeuronPrivateKeyFromHex(testPrivateKeyHex2)
		require.NoError(t, err)

		peerID1, err := key1.PublicKey().PeerID()
		require.NoError(t, err)

		peerID2, err := key2.PublicKey().PeerID()
		require.NoError(t, err)

		assert.NotEqual(t, peerID1.String(), peerID2.String(),
			"different keys must produce different PeerIDs")
	})
}
