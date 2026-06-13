package keylib

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T030: Encryption Tests ---

func TestEncrypt(t *testing.T) {
	t.Run("returns EncryptedPrivateKey with version 1 and correct field sizes", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "test-password")
		require.NoError(t, err)

		assert.Equal(t, uint8(1), encrypted.version)
		assert.Len(t, encrypted.salt, EncryptionSaltLength, "salt must be 16 bytes")
		assert.Len(t, encrypted.nonce, EncryptionNonceLength, "nonce must be 12 bytes")
		assert.Len(t, encrypted.ciphertext, EncryptionCiphertextLength, "ciphertext must be 48 bytes")

		// Version 1 should not store custom params.
		assert.Equal(t, uint32(0), encrypted.time, "v1 must not store Time")
		assert.Equal(t, uint32(0), encrypted.memory, "v1 must not store Memory")
		assert.Equal(t, uint8(0), encrypted.threads, "v1 must not store Threads")
	})

	t.Run("two encryptions of the same key produce different ciphertext", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		enc1, err := Encrypt(key, "password")
		require.NoError(t, err)

		enc2, err := Encrypt(key, "password")
		require.NoError(t, err)

		// Salt and nonce are random, so ciphertext must differ.
		assert.NotEqual(t, enc1.salt, enc2.salt, "salts must differ")
		assert.NotEqual(t, enc1.nonce, enc2.nonce, "nonces must differ")
		assert.NotEqual(t, enc1.ciphertext, enc2.ciphertext, "ciphertexts must differ")
	})

	t.Run("encrypting zeroized key fails", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		key.Zeroize()

		_, err = Encrypt(key, "password")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
	})
}

func TestDecrypt(t *testing.T) {
	t.Run("decryption with correct password returns byte-identical key", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "correct-password")
		require.NoError(t, err)

		decrypted, err := Decrypt(encrypted, "correct-password")
		require.NoError(t, err)

		assert.Equal(t, key.Bytes(), decrypted.Bytes(),
			"decrypted key must be byte-identical to original")
		assert.Equal(t, key.Hex(), decrypted.Hex())
	})

	t.Run("decrypted key can derive identical public key and address", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "password")
		require.NoError(t, err)

		decrypted, err := Decrypt(encrypted, "password")
		require.NoError(t, err)

		assert.Equal(t, key.PublicKey().Bytes(), decrypted.PublicKey().Bytes())
		assert.Equal(t, key.PublicKey().EVMAddress().Hex(), decrypted.PublicKey().EVMAddress().Hex())
	})
}

func TestDecrypt_WrongPassword(t *testing.T) {
	t.Run("wrong password returns ErrEncryption", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "correct-password")
		require.NoError(t, err)

		_, err = Decrypt(encrypted, "wrong-password")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrEncryption, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "decryption failed")

		// Error message must NOT contain any key material (SEC-003).
		assert.NotContains(t, keyErr.Error(), testPrivateKeyHex)
	})

	t.Run("empty password returns ErrEncryption when key was encrypted with non-empty", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "some-password")
		require.NoError(t, err)

		_, err = Decrypt(encrypted, "")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrEncryption, keyErr.Kind())
	})

	t.Run("unsupported version returns ErrEncryption", func(t *testing.T) {
		encrypted := EncryptedPrivateKey{
			version:    99,
			salt:       make([]byte, EncryptionSaltLength),
			nonce:      make([]byte, EncryptionNonceLength),
			ciphertext: make([]byte, EncryptionCiphertextLength),
		}

		_, err := Decrypt(encrypted, "password")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrEncryption, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "unsupported encryption version")
	})
}

func TestEncryptedPrivateKey_JSON(t *testing.T) {
	t.Run("marshal and unmarshal round-trip for v1", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "json-test-password")
		require.NoError(t, err)

		// Marshal to JSON.
		data, err := json.Marshal(encrypted)
		require.NoError(t, err)

		// Unmarshal back.
		var restored EncryptedPrivateKey
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, encrypted.version, restored.version)
		assert.Equal(t, encrypted.salt, restored.salt)
		assert.Equal(t, encrypted.nonce, restored.nonce)
		assert.Equal(t, encrypted.ciphertext, restored.ciphertext)

		// Decrypt the restored envelope.
		decrypted, err := Decrypt(restored, "json-test-password")
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), decrypted.Bytes())
	})

	t.Run("v1 JSON omits time, memory, threads", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "password")
		require.NoError(t, err)

		data, err := json.Marshal(encrypted)
		require.NoError(t, err)

		// Parse as generic map to check field presence.
		var m map[string]interface{}
		err = json.Unmarshal(data, &m)
		require.NoError(t, err)

		_, hasTime := m["time"]
		_, hasMemory := m["memory"]
		_, hasThreads := m["threads"]
		assert.False(t, hasTime, "v1 JSON must not include 'time'")
		assert.False(t, hasMemory, "v1 JSON must not include 'memory'")
		assert.False(t, hasThreads, "v1 JSON must not include 'threads'")
	})
}

func TestEncryptV2(t *testing.T) {
	t.Run("version 2 with custom Argon2 params", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		customTime := uint32(3)
		customMemory := uint32(32 * 1024)
		customThreads := uint8(2)

		encrypted, err := Encrypt(key, "v2-password",
			WithArgon2Params(customTime, customMemory, customThreads))
		require.NoError(t, err)

		assert.Equal(t, uint8(2), encrypted.version)
		assert.Equal(t, customTime, encrypted.time)
		assert.Equal(t, customMemory, encrypted.memory)
		assert.Equal(t, customThreads, encrypted.threads)
		assert.Len(t, encrypted.salt, EncryptionSaltLength)
		assert.Len(t, encrypted.nonce, EncryptionNonceLength)
		assert.Len(t, encrypted.ciphertext, EncryptionCiphertextLength)
	})

	t.Run("v2 can be decrypted back", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "v2-password",
			WithArgon2Params(2, 32*1024, 2))
		require.NoError(t, err)

		decrypted, err := Decrypt(encrypted, "v2-password")
		require.NoError(t, err)

		assert.Equal(t, key.Bytes(), decrypted.Bytes(),
			"v2 decrypted key must match original")
	})

	t.Run("v2 wrong password returns ErrEncryption", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "v2-correct",
			WithArgon2Params(1, 32*1024, 2))
		require.NoError(t, err)

		_, err = Decrypt(encrypted, "v2-wrong")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrEncryption, keyErr.Kind())
	})

	t.Run("v2 JSON includes time, memory, threads", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "v2-password",
			WithArgon2Params(3, 32*1024, 2))
		require.NoError(t, err)

		data, err := json.Marshal(encrypted)
		require.NoError(t, err)

		// Parse as generic map to check field presence.
		var m map[string]interface{}
		err = json.Unmarshal(data, &m)
		require.NoError(t, err)

		timeVal, hasTime := m["time"]
		memoryVal, hasMemory := m["memory"]
		threadsVal, hasThreads := m["threads"]

		assert.True(t, hasTime, "v2 JSON must include 'time'")
		assert.True(t, hasMemory, "v2 JSON must include 'memory'")
		assert.True(t, hasThreads, "v2 JSON must include 'threads'")

		assert.Equal(t, float64(3), timeVal)
		assert.Equal(t, float64(32*1024), memoryVal)
		assert.Equal(t, float64(2), threadsVal)
	})

	t.Run("v2 JSON round-trip preserves params and decryption works", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "v2-roundtrip",
			WithArgon2Params(2, 48*1024, 3))
		require.NoError(t, err)

		data, err := json.Marshal(encrypted)
		require.NoError(t, err)

		var restored EncryptedPrivateKey
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err)

		assert.Equal(t, uint8(2), restored.version)
		assert.Equal(t, uint32(2), restored.time)
		assert.Equal(t, uint32(48*1024), restored.memory)
		assert.Equal(t, uint8(3), restored.threads)

		decrypted, err := Decrypt(restored, "v2-roundtrip")
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), decrypted.Bytes())
	})
}
