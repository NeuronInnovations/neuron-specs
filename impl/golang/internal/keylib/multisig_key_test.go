package keylib

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T034+T036: Construction and Protocol Tracking Tests ---

func TestMultisigKey_NewMultisigKey(t *testing.T) {
	// Helper: generate n valid NeuronPrivateKeys for testing.
	generateKeys := func(t *testing.T, n int) []NeuronPrivateKey {
		t.Helper()
		keys := make([]NeuronPrivateKey, n)
		for i := range n {
			ecKey, err := crypto.GenerateKey()
			require.NoError(t, err)
			nk, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
			require.NoError(t, err)
			keys[i] = nk
		}
		return keys
	}

	t.Run("valid 2-of-3 multisig", func(t *testing.T) {
		keys := generateKeys(t, 3)
		mk, err := NewMultisigKey(keys, 2)
		require.NoError(t, err)

		assert.Equal(t, "secp256k1-aggregated", mk.Protocol())
		assert.Equal(t, uint(2), mk.Threshold())
		assert.Equal(t, uint(3), mk.TotalKeys())
	})

	t.Run("valid 1-of-1 multisig", func(t *testing.T) {
		keys := generateKeys(t, 1)
		mk, err := NewMultisigKey(keys, 1)
		require.NoError(t, err)

		assert.Equal(t, "secp256k1-aggregated", mk.Protocol())
		assert.Equal(t, uint(1), mk.Threshold())
		assert.Equal(t, uint(1), mk.TotalKeys())
	})

	t.Run("valid n-of-n multisig", func(t *testing.T) {
		keys := generateKeys(t, 5)
		mk, err := NewMultisigKey(keys, 5)
		require.NoError(t, err)

		assert.Equal(t, uint(5), mk.Threshold())
		assert.Equal(t, uint(5), mk.TotalKeys())
	})

	t.Run("rejects empty keys slice", func(t *testing.T) {
		_, err := NewMultisigKey([]NeuronPrivateKey{}, 1)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "keys slice must not be empty")
	})

	t.Run("rejects nil keys slice", func(t *testing.T) {
		_, err := NewMultisigKey(nil, 1)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
	})

	t.Run("rejects threshold zero", func(t *testing.T) {
		keys := generateKeys(t, 3)
		_, err := NewMultisigKey(keys, 0)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "threshold must be greater than zero")
	})

	t.Run("rejects threshold exceeding key count", func(t *testing.T) {
		keys := generateKeys(t, 2)
		_, err := NewMultisigKey(keys, 3)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "threshold 3 exceeds number of keys 2")
	})

	t.Run("defensive copy prevents external mutation", func(t *testing.T) {
		keys := generateKeys(t, 3)
		originalHex := keys[0].Hex()

		mk, err := NewMultisigKey(keys, 2)
		require.NoError(t, err)

		// Mutate the original slice — should not affect the MultisigKey.
		keys[0].Zeroize()

		// Retrieve the stored keys via ToBlockchainKey and verify the first
		// key was not zeroized.
		stored, err := mk.ToBlockchainKey()
		require.NoError(t, err)
		storedKeys, ok := stored.([]NeuronPrivateKey)
		require.True(t, ok)
		assert.Equal(t, originalHex, storedKeys[0].Hex())
	})
}

func TestMultisigKey_MultisigKeyFromBlockchainKey(t *testing.T) {
	t.Run("valid hedera-threshold key", func(t *testing.T) {
		// Simulate an opaque Hedera threshold key (just a string for testing).
		hederaKey := "hedera-threshold-key-bytes"
		mk, err := MultisigKeyFromBlockchainKey(hederaKey, "hedera-threshold")
		require.NoError(t, err)

		assert.Equal(t, "hedera-threshold", mk.Protocol())
		assert.Equal(t, uint(1), mk.Threshold())
		assert.Equal(t, uint(1), mk.TotalKeys())
	})

	t.Run("valid frost key", func(t *testing.T) {
		frostKey := []byte{0x01, 0x02, 0x03}
		mk, err := MultisigKeyFromBlockchainKey(frostKey, "frost")
		require.NoError(t, err)

		assert.Equal(t, "frost", mk.Protocol())
	})

	t.Run("valid bls key", func(t *testing.T) {
		blsKey := struct{ data [48]byte }{}
		mk, err := MultisigKeyFromBlockchainKey(blsKey, "bls")
		require.NoError(t, err)

		assert.Equal(t, "bls", mk.Protocol())
	})

	t.Run("rejects nil key", func(t *testing.T) {
		_, err := MultisigKeyFromBlockchainKey(nil, "hedera-threshold")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "blockchain key must not be nil")
	})

	t.Run("rejects empty protocol", func(t *testing.T) {
		_, err := MultisigKeyFromBlockchainKey("some-key", "")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "protocol must not be empty")
	})
}

func TestMultisigKey_ProtocolTracking(t *testing.T) {
	generateKey := func(t *testing.T) NeuronPrivateKey {
		t.Helper()
		ecKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		nk, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
		require.NoError(t, err)
		return nk
	}

	t.Run("secp256k1-aggregated protocol from NewMultisigKey", func(t *testing.T) {
		mk, err := NewMultisigKey([]NeuronPrivateKey{generateKey(t)}, 1)
		require.NoError(t, err)
		assert.Equal(t, "secp256k1-aggregated", mk.Protocol())
	})

	t.Run("frost protocol from MultisigKeyFromBlockchainKey", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("frost-share", "frost")
		require.NoError(t, err)
		assert.Equal(t, "frost", mk.Protocol())
	})

	t.Run("bls protocol from MultisigKeyFromBlockchainKey", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("bls-key", "bls")
		require.NoError(t, err)
		assert.Equal(t, "bls", mk.Protocol())
	})

	t.Run("hedera-threshold protocol from MultisigKeyFromBlockchainKey", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("hedera-key", "hedera-threshold")
		require.NoError(t, err)
		assert.Equal(t, "hedera-threshold", mk.Protocol())
	})

	t.Run("custom protocol identifier preserved", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("key-data", "ed25519-musig2")
		require.NoError(t, err)
		assert.Equal(t, "ed25519-musig2", mk.Protocol())
	})
}

// --- T034+T036: EVMAddress and PeerID error behavior ---

func TestMultisigKey_EVMAddress(t *testing.T) {
	generateKey := func(t *testing.T) NeuronPrivateKey {
		t.Helper()
		ecKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		nk, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
		require.NoError(t, err)
		return nk
	}

	t.Run("hedera-threshold returns ErrUnsupportedKeyType", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("hedera-key", "hedera-threshold")
		require.NoError(t, err)

		_, err = mk.EVMAddress()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "hedera-threshold")
	})

	t.Run("frost returns ErrUnsupportedKeyType", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("frost-key", "frost")
		require.NoError(t, err)

		_, err = mk.EVMAddress()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
	})

	t.Run("bls returns ErrUnsupportedKeyType", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("bls-key", "bls")
		require.NoError(t, err)

		_, err = mk.EVMAddress()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
	})

	t.Run("secp256k1-aggregated returns GAP-005 error", func(t *testing.T) {
		keys := []NeuronPrivateKey{generateKey(t), generateKey(t)}
		mk, err := NewMultisigKey(keys, 2)
		require.NoError(t, err)

		_, err = mk.EVMAddress()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "GAP-005")
		assert.Contains(t, keyErr.Error(), "aggregation algorithm not yet specified")
	})
}

func TestMultisigKey_PeerID(t *testing.T) {
	generateKey := func(t *testing.T) NeuronPrivateKey {
		t.Helper()
		ecKey, err := crypto.GenerateKey()
		require.NoError(t, err)
		nk, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
		require.NoError(t, err)
		return nk
	}

	t.Run("hedera-threshold returns ErrUnsupportedKeyType", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("hedera-key", "hedera-threshold")
		require.NoError(t, err)

		_, err = mk.PeerID()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "hedera-threshold")
	})

	t.Run("frost returns ErrUnsupportedKeyType", func(t *testing.T) {
		mk, err := MultisigKeyFromBlockchainKey("frost-key", "frost")
		require.NoError(t, err)

		_, err = mk.PeerID()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
	})

	t.Run("secp256k1-aggregated returns GAP-005 error", func(t *testing.T) {
		keys := []NeuronPrivateKey{generateKey(t), generateKey(t)}
		mk, err := NewMultisigKey(keys, 2)
		require.NoError(t, err)

		_, err = mk.PeerID()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "GAP-005")
		assert.Contains(t, keyErr.Error(), "aggregation algorithm not yet specified")
	})
}

// --- T035+T037: ToBlockchainKey and Round-Trip Tests ---

func TestMultisigKey_ToBlockchainKey(t *testing.T) {
	generateKeys := func(t *testing.T, n int) []NeuronPrivateKey {
		t.Helper()
		keys := make([]NeuronPrivateKey, n)
		for i := range n {
			ecKey, err := crypto.GenerateKey()
			require.NoError(t, err)
			nk, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
			require.NoError(t, err)
			keys[i] = nk
		}
		return keys
	}

	t.Run("secp256k1-aggregated returns stored NeuronPrivateKeys", func(t *testing.T) {
		keys := generateKeys(t, 3)
		mk, err := NewMultisigKey(keys, 2)
		require.NoError(t, err)

		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKeys, ok := result.([]NeuronPrivateKey)
		require.True(t, ok, "expected []NeuronPrivateKey, got %T", result)
		require.Len(t, storedKeys, 3)

		// Verify each key matches the original.
		for i := range 3 {
			assert.Equal(t, keys[i].Hex(), storedKeys[i].Hex())
		}
	})

	t.Run("blockchain key returns underlying key for non-aggregated", func(t *testing.T) {
		hederaKey := "hedera-threshold-key-data"
		mk, err := MultisigKeyFromBlockchainKey(hederaKey, "hedera-threshold")
		require.NoError(t, err)

		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKey, ok := result.(string)
		require.True(t, ok, "expected string, got %T", result)
		assert.Equal(t, hederaKey, storedKey)
	})

	t.Run("blockchain key returns byte slice for frost", func(t *testing.T) {
		frostKey := []byte{0xDE, 0xAD, 0xBE, 0xEF}
		mk, err := MultisigKeyFromBlockchainKey(frostKey, "frost")
		require.NoError(t, err)

		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKey, ok := result.([]byte)
		require.True(t, ok, "expected []byte, got %T", result)
		assert.Equal(t, frostKey, storedKey)
	})

	t.Run("blockchain key returns struct for bls", func(t *testing.T) {
		type blsKey struct {
			PubKey [48]byte
			Share  uint
		}
		original := blsKey{PubKey: [48]byte{0x01}, Share: 42}
		mk, err := MultisigKeyFromBlockchainKey(original, "bls")
		require.NoError(t, err)

		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKey, ok := result.(blsKey)
		require.True(t, ok, "expected blsKey, got %T", result)
		assert.Equal(t, original, storedKey)
	})
}

func TestMultisigKey_RoundTrip(t *testing.T) {
	t.Run("blockchain key round-trip preserves key and protocol", func(t *testing.T) {
		originalKey := map[string]interface{}{
			"type":      "hedera-threshold",
			"threshold": 2,
			"keys":      []string{"key1", "key2", "key3"},
		}
		originalProtocol := "hedera-threshold"

		mk, err := MultisigKeyFromBlockchainKey(originalKey, originalProtocol)
		require.NoError(t, err)

		// Verify protocol is preserved.
		assert.Equal(t, originalProtocol, mk.Protocol())

		// Extract the key back.
		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKey, ok := result.(map[string]interface{})
		require.True(t, ok, "expected map[string]interface{}, got %T", result)
		assert.Equal(t, originalKey["type"], storedKey["type"])
		assert.Equal(t, originalKey["threshold"], storedKey["threshold"])
	})

	t.Run("secp256k1-aggregated round-trip preserves keys", func(t *testing.T) {
		// Generate keys and record their hex values.
		var keys []NeuronPrivateKey
		var hexValues []string
		for range 3 {
			ecKey, err := crypto.GenerateKey()
			require.NoError(t, err)
			nk, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
			require.NoError(t, err)
			keys = append(keys, nk)
			hexValues = append(hexValues, nk.Hex())
		}

		mk, err := NewMultisigKey(keys, 2)
		require.NoError(t, err)

		// Verify protocol.
		assert.Equal(t, "secp256k1-aggregated", mk.Protocol())

		// Extract keys back.
		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKeys, ok := result.([]NeuronPrivateKey)
		require.True(t, ok)
		require.Len(t, storedKeys, 3)

		for i, sk := range storedKeys {
			assert.Equal(t, hexValues[i], sk.Hex(), "key %d hex mismatch", i)
		}
	})

	t.Run("frost round-trip preserves key and protocol", func(t *testing.T) {
		frostKey := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
		mk, err := MultisigKeyFromBlockchainKey(frostKey, "frost")
		require.NoError(t, err)

		assert.Equal(t, "frost", mk.Protocol())

		result, err := mk.ToBlockchainKey()
		require.NoError(t, err)

		storedKey, ok := result.([]byte)
		require.True(t, ok)
		assert.Equal(t, frostKey, storedKey)
	})
}
