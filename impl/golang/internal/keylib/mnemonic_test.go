package keylib

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T029: Mnemonic Generation Tests ---

func TestGenerateMnemonic(t *testing.T) {
	t.Run("produces valid BIP-39 phrase with 12 words", func(t *testing.T) {
		mnemonic, err := GenerateMnemonic()
		require.NoError(t, err)

		words := strings.Fields(mnemonic)
		assert.Len(t, words, 12, "BIP-39 mnemonic from 128-bit entropy must have 12 words")
	})

	t.Run("two generated mnemonics differ", func(t *testing.T) {
		m1, err := GenerateMnemonic()
		require.NoError(t, err)

		m2, err := GenerateMnemonic()
		require.NoError(t, err)

		assert.NotEqual(t, m1, m2, "two independently generated mnemonics must differ")
	})

	t.Run("generated mnemonic can derive a key", func(t *testing.T) {
		mnemonic, err := GenerateMnemonic()
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromMnemonic(mnemonic, "")
		require.NoError(t, err)
		assert.Len(t, key.Bytes(), PrivateKeyLength)
		assert.False(t, key.IsZero())
	})
}

// --- T029+T032: Mnemonic Derivation Tests ---

func TestNeuronPrivateKeyFromMnemonic(t *testing.T) {
	// Well-known BIP-39 test mnemonic (12 words).
	const testMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	t.Run("default path produces valid key", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "")
		require.NoError(t, err)

		assert.Len(t, key.Bytes(), PrivateKeyLength)
		assert.False(t, key.IsZero())

		// Key must be valid on secp256k1.
		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.NotNil(t, ecdsaKey)
	})

	t.Run("custom path produces different key than default", func(t *testing.T) {
		keyDefault, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "")
		require.NoError(t, err)

		keyCustom, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "m/44'/60'/0'/0/1")
		require.NoError(t, err)

		assert.NotEqual(t, keyDefault.Bytes(), keyCustom.Bytes(),
			"different derivation paths must produce different keys")
	})

	t.Run("same mnemonic and path is deterministic", func(t *testing.T) {
		key1, err := NeuronPrivateKeyFromMnemonic(testMnemonic, DefaultDerivationPath)
		require.NoError(t, err)

		key2, err := NeuronPrivateKeyFromMnemonic(testMnemonic, DefaultDerivationPath)
		require.NoError(t, err)

		assert.Equal(t, key1.Bytes(), key2.Bytes(),
			"same mnemonic + path must produce identical keys")
		assert.Equal(t, key1.Hex(), key2.Hex())
	})

	t.Run("empty path uses default derivation path", func(t *testing.T) {
		keyEmpty, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "")
		require.NoError(t, err)

		keyExplicit, err := NeuronPrivateKeyFromMnemonic(testMnemonic, DefaultDerivationPath)
		require.NoError(t, err)

		assert.Equal(t, keyEmpty.Bytes(), keyExplicit.Bytes(),
			"empty path must produce the same key as DefaultDerivationPath")
	})

	t.Run("derived key can derive full identity chain", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "")
		require.NoError(t, err)

		// PublicKey.
		pub := key.PublicKey()
		assert.Len(t, pub.Bytes(), CompressedPublicKeyLength)

		// EVMAddress.
		addr := pub.EVMAddress()
		assert.Len(t, addr.Bytes(), EVMAddressLength)

		// PeerID.
		pid, err := pub.PeerID()
		require.NoError(t, err)
		assert.NotEmpty(t, pid.String())

		// DIDKey.
		did := pub.DIDKey()
		assert.Contains(t, did, "did:key:z")
	})

	t.Run("invalid mnemonic rejected with ErrMnemonic", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromMnemonic("invalid mnemonic words that do not form a valid phrase", "")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrMnemonic, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "invalid BIP-39 mnemonic")
	})

	t.Run("empty mnemonic rejected with ErrMnemonic", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromMnemonic("", "")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrMnemonic, keyErr.Kind())
	})

	t.Run("invalid derivation path rejected with ErrDerivation", func(t *testing.T) {
		tests := []struct {
			name string
			path string
		}{
			{"path with invalid characters", "m/44'/abc/0'/0/0"},
			{"empty component", "m/44'//0'/0/0"},
			{"path is just m", "m"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NeuronPrivateKeyFromMnemonic(testMnemonic, tt.path)
				require.Error(t, err)

				var keyErr *KeyError
				require.True(t, errors.As(err, &keyErr))
				assert.Equal(t, ErrDerivation, keyErr.Kind(),
					"path %q should produce ErrDerivation, got %s", tt.path, keyErr.Kind())
			})
		}
	})

	t.Run("hardened path with h suffix works", func(t *testing.T) {
		// "h" suffix is an alternative to "'" for hardened derivation.
		keyPrime, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "m/44'/60'/0'/0/0")
		require.NoError(t, err)

		keyH, err := NeuronPrivateKeyFromMnemonic(testMnemonic, "m/44h/60h/0h/0/0")
		require.NoError(t, err)

		assert.Equal(t, keyPrime.Bytes(), keyH.Bytes(),
			"hardened notation ' and h must produce the same key")
	})

	t.Run("different mnemonics produce different keys", func(t *testing.T) {
		m1 := testMnemonic

		m2, err := GenerateMnemonic()
		require.NoError(t, err)

		key1, err := NeuronPrivateKeyFromMnemonic(m1, "")
		require.NoError(t, err)

		key2, err := NeuronPrivateKeyFromMnemonic(m2, "")
		require.NoError(t, err)

		assert.NotEqual(t, key1.Bytes(), key2.Bytes(),
			"different mnemonics must produce different keys")
	})
}
