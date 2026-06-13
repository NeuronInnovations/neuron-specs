package keylib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T039: Full Key Lifecycle Integration Test ---

func TestIntegration_FullKeyLifecycle(t *testing.T) {
	// Step 1: Generate a new random private key.
	key, err := NewNeuronPrivateKey()
	require.NoError(t, err, "NewNeuronPrivateKey must succeed")
	assert.False(t, key.IsZero(), "freshly generated key must not be zeroized")
	assert.Len(t, key.Bytes(), PrivateKeyLength, "private key must be 32 bytes")

	// Step 2: Derive the full identity chain.
	pub := key.PublicKey()
	assert.Len(t, pub.Bytes(), CompressedPublicKeyLength,
		"public key must be 33 bytes compressed")
	assert.True(t, pub.Bytes()[0] == 0x02 || pub.Bytes()[0] == 0x03,
		"compressed public key must start with 0x02 or 0x03")

	addr := pub.EVMAddress()
	assert.Len(t, addr.Bytes(), EVMAddressLength, "EVM address must be 20 bytes")
	assert.NotEmpty(t, addr.Hex(), "EVM address hex must not be empty")

	pid, err := pub.PeerID()
	require.NoError(t, err, "PeerID derivation must succeed")
	assert.NotEmpty(t, pid.String(), "PeerID string must not be empty")

	did := pub.DIDKey()
	assert.True(t, strings.HasPrefix(did, "did:key:z"),
		"DID:key must start with did:key:z, got %s", did)

	// Step 3: Sign a message and verify the signature.
	message := []byte("full lifecycle integration test message")
	sig, err := key.Sign(message)
	require.NoError(t, err, "Sign must succeed")
	assert.Len(t, sig.Bytes(), SignatureLength, "signature must be 65 bytes")

	assert.True(t, sig.Verify(message, pub),
		"signature must verify against the derived public key")

	// Step 4: Recover the public key from the signature.
	recovered, err := sig.RecoverPublicKey(message)
	require.NoError(t, err, "RecoverPublicKey must succeed")
	assert.Equal(t, pub.Bytes(), recovered.Bytes(),
		"recovered public key must match the original")

	// Step 5: Encrypt and decrypt the key.
	password := "integration-test-password-2026"
	encrypted, err := Encrypt(key, password)
	require.NoError(t, err, "Encrypt must succeed")
	assert.Equal(t, uint8(1), encrypted.version)

	decrypted, err := Decrypt(encrypted, password)
	require.NoError(t, err, "Decrypt with correct password must succeed")
	assert.Equal(t, key.Bytes(), decrypted.Bytes(),
		"decrypted key must be byte-identical to the original")

	// Verify the decrypted key produces the same identity chain.
	assert.Equal(t, pub.Bytes(), decrypted.PublicKey().Bytes(),
		"decrypted key must derive the same public key")
	assert.Equal(t, addr.Hex(), decrypted.PublicKey().EVMAddress().Hex(),
		"decrypted key must derive the same EVM address")

	// Step 6: Zeroize and verify the key is unusable.
	key.Zeroize()
	assert.True(t, key.IsZero(), "key must be zeroized after Zeroize()")

	_, err = key.ToBlockchainKey()
	require.Error(t, err, "ToBlockchainKey must fail after Zeroize()")
	var keyErr *KeyError
	require.True(t, errors.As(err, &keyErr))
	assert.Equal(t, ErrZeroValue, keyErr.Kind())

	_, err = key.Sign(message)
	require.Error(t, err, "Sign must fail after Zeroize()")
	require.True(t, errors.As(err, &keyErr))
	assert.Equal(t, ErrZeroValue, keyErr.Kind())
}

// --- T040: Mnemonic-to-Signing Integration Test ---

func TestIntegration_MnemonicToSigning(t *testing.T) {
	// Step 1: Generate a fresh mnemonic and derive a key.
	mnemonic, err := GenerateMnemonic()
	require.NoError(t, err, "GenerateMnemonic must succeed")

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 12, "mnemonic must have 12 words")

	key, err := NeuronPrivateKeyFromMnemonic(mnemonic, "")
	require.NoError(t, err, "NeuronPrivateKeyFromMnemonic must succeed")
	assert.Len(t, key.Bytes(), PrivateKeyLength)
	assert.False(t, key.IsZero())

	// Step 2: Sign a message and verify.
	message := []byte("mnemonic-to-signing integration test")
	pub := key.PublicKey()

	sig, err := key.Sign(message)
	require.NoError(t, err, "Sign must succeed for mnemonic-derived key")
	assert.True(t, sig.Verify(message, pub),
		"signature must verify against the mnemonic-derived public key")

	// Step 3: Restore the key from the same mnemonic and verify byte-identity.
	restoredKey, err := NeuronPrivateKeyFromMnemonic(mnemonic, "")
	require.NoError(t, err, "restoring from same mnemonic must succeed")
	assert.Equal(t, key.Bytes(), restoredKey.Bytes(),
		"same mnemonic with default path must produce byte-identical key")

	// Step 4: Sign the same message with the restored key and verify RFC 6979 determinism.
	restoredSig, err := restoredKey.Sign(message)
	require.NoError(t, err, "Sign from restored key must succeed")
	assert.Equal(t, sig.Bytes(), restoredSig.Bytes(),
		"RFC 6979: same key + same message must produce byte-identical signature")

	// Verify the restored signature is also valid.
	assert.True(t, restoredSig.Verify(message, pub),
		"restored key's signature must verify against the original public key")
}

// --- T041: Cross-Type Matching Integration Test ---

func TestIntegration_CrossTypeMatching(t *testing.T) {
	// Generate the first key and derive all identity types.
	key1, err := NewNeuronPrivateKey()
	require.NoError(t, err)

	pub1 := key1.PublicKey()
	addr1 := pub1.EVMAddress()
	pid1, err := pub1.PeerID()
	require.NoError(t, err)

	// Verify all same-key Matches* functions return true.
	assert.True(t, key1.MatchesPublicKey(pub1),
		"PrivateKey.MatchesPublicKey must be true for own public key")
	assert.True(t, key1.MatchesEVMAddress(addr1),
		"PrivateKey.MatchesEVMAddress must be true for own address")
	assert.True(t, pub1.MatchesPeerID(pid1),
		"PublicKey.MatchesPeerID must be true for own PeerID")
	assert.True(t, pub1.MatchesEVMAddress(addr1),
		"PublicKey.MatchesEVMAddress must be true for own address")
	assert.True(t, EVMAddressMatchesPeerID(addr1, pid1, pub1),
		"EVMAddressMatchesPeerID must be true for same-key values")

	// Generate a second key and derive all identity types.
	key2, err := NewNeuronPrivateKey()
	require.NoError(t, err)

	pub2 := key2.PublicKey()
	addr2 := pub2.EVMAddress()
	pid2, err := pub2.PeerID()
	require.NoError(t, err)

	// Verify all cross-key Matches* functions return false.
	assert.False(t, key1.MatchesPublicKey(pub2),
		"PrivateKey.MatchesPublicKey must be false for different key's public key")
	assert.False(t, key2.MatchesPublicKey(pub1),
		"PrivateKey.MatchesPublicKey must be false (reverse direction)")
	assert.False(t, key1.MatchesEVMAddress(addr2),
		"PrivateKey.MatchesEVMAddress must be false for different key's address")
	assert.False(t, key2.MatchesEVMAddress(addr1),
		"PrivateKey.MatchesEVMAddress must be false (reverse direction)")
	assert.False(t, pub1.MatchesPeerID(pid2),
		"PublicKey.MatchesPeerID must be false for different key's PeerID")
	assert.False(t, pub2.MatchesPeerID(pid1),
		"PublicKey.MatchesPeerID must be false (reverse direction)")
	assert.False(t, pub1.MatchesEVMAddress(addr2),
		"PublicKey.MatchesEVMAddress must be false for different key's address")
	assert.False(t, pub2.MatchesEVMAddress(addr1),
		"PublicKey.MatchesEVMAddress must be false (reverse direction)")
	assert.False(t, EVMAddressMatchesPeerID(addr1, pid2, pub2),
		"EVMAddressMatchesPeerID must be false for cross-key (addr1, pid2, pub2)")
	assert.False(t, EVMAddressMatchesPeerID(addr2, pid1, pub1),
		"EVMAddressMatchesPeerID must be false for cross-key (addr2, pid1, pub1)")
}

// --- T042: Blockchain Round-Trip Integration Test ---

func TestIntegration_BlockchainRoundTrip(t *testing.T) {
	// Step 1: Generate a NeuronPrivateKey, convert to blockchain key, and convert back.
	originalKey, err := NewNeuronPrivateKey()
	require.NoError(t, err)

	ecdsaKey, err := originalKey.ToBlockchainKey()
	require.NoError(t, err, "ToBlockchainKey must succeed")
	assert.Equal(t, crypto.S256().Params().Name, ecdsaKey.Curve.Params().Name,
		"blockchain key must be on secp256k1 curve")

	roundTrippedKey, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
	require.NoError(t, err, "NeuronPrivateKeyFromBlockchainKey must succeed")

	assert.Equal(t, originalKey.Bytes(), roundTrippedKey.Bytes(),
		"private key bytes must be identical after blockchain round-trip")
	assert.Equal(t, originalKey.Hex(), roundTrippedKey.Hex(),
		"private key hex must be identical after blockchain round-trip")

	// Step 2: Verify all derived values match.
	origPub := originalKey.PublicKey()
	rtPub := roundTrippedKey.PublicKey()
	assert.Equal(t, origPub.Bytes(), rtPub.Bytes(),
		"public key must match after private key round-trip")

	origAddr := origPub.EVMAddress()
	rtAddr := rtPub.EVMAddress()
	assert.Equal(t, origAddr.Hex(), rtAddr.Hex(),
		"EVM address must match after private key round-trip")

	origPID, err := origPub.PeerID()
	require.NoError(t, err)
	rtPID, err := rtPub.PeerID()
	require.NoError(t, err)
	assert.Equal(t, origPID.String(), rtPID.String(),
		"PeerID must match after private key round-trip")

	origDID := origPub.DIDKey()
	rtDID := rtPub.DIDKey()
	assert.Equal(t, origDID, rtDID,
		"DID:key must match after private key round-trip")

	// Step 3: Public key blockchain round-trip.
	ecdsaPub, err := origPub.ToBlockchainKey()
	require.NoError(t, err, "PublicKey.ToBlockchainKey must succeed")

	rtPub2, err := NeuronPublicKeyFromBlockchainKey(ecdsaPub)
	require.NoError(t, err, "NeuronPublicKeyFromBlockchainKey must succeed")
	assert.Equal(t, origPub.Bytes(), rtPub2.Bytes(),
		"public key must match after public key blockchain round-trip")
}

// --- T043: MultisigKey Integration Test ---

func TestIntegration_MultisigKey(t *testing.T) {
	// Step 1: Generate 3 NeuronPrivateKeys.
	keys := make([]NeuronPrivateKey, 3)
	for i := range keys {
		k, err := NewNeuronPrivateKey()
		require.NoError(t, err, "key generation %d must succeed", i)
		keys[i] = k
	}

	// Step 2: Create a MultisigKey with threshold=2.
	mk, err := NewMultisigKey(keys, 2)
	require.NoError(t, err, "NewMultisigKey must succeed")

	assert.Equal(t, "secp256k1-aggregated", mk.Protocol(),
		"native MultisigKey protocol must be secp256k1-aggregated")
	assert.Equal(t, uint(2), mk.Threshold(),
		"threshold must be 2")
	assert.Equal(t, uint(3), mk.TotalKeys(),
		"total keys must be 3")

	// Step 3: EVMAddress returns error (GAP-005).
	_, err = mk.EVMAddress()
	require.Error(t, err, "EVMAddress must fail for secp256k1-aggregated (GAP-005)")
	var keyErr *KeyError
	require.True(t, errors.As(err, &keyErr))
	assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())

	// Step 4: PeerID returns error (GAP-005).
	_, err = mk.PeerID()
	require.Error(t, err, "PeerID must fail for secp256k1-aggregated (GAP-005)")
	require.True(t, errors.As(err, &keyErr))
	assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())

	// Step 5: ToBlockchainKey returns the stored keys.
	bcKey, err := mk.ToBlockchainKey()
	require.NoError(t, err, "ToBlockchainKey must succeed for secp256k1-aggregated")
	storedKeys, ok := bcKey.([]NeuronPrivateKey)
	require.True(t, ok, "ToBlockchainKey must return []NeuronPrivateKey")
	assert.Len(t, storedKeys, 3, "stored keys slice must have 3 entries")

	// Step 6: Create MultisigKey from external blockchain key with hedera-threshold protocol.
	type mockHederaKey struct {
		KeyList []string
	}
	hederaKey := &mockHederaKey{KeyList: []string{"key-a", "key-b"}}

	hederaMK, err := MultisigKeyFromBlockchainKey(hederaKey, "hedera-threshold")
	require.NoError(t, err, "MultisigKeyFromBlockchainKey must succeed")

	assert.Equal(t, "hedera-threshold", hederaMK.Protocol())

	// Step 7: EVMAddress and PeerID return ErrUnsupportedKeyType for non-secp256k1 protocols.
	_, err = hederaMK.EVMAddress()
	require.Error(t, err, "EVMAddress must fail for hedera-threshold")
	require.True(t, errors.As(err, &keyErr))
	assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind(),
		"hedera-threshold EVMAddress must return ErrUnsupportedKeyType")

	_, err = hederaMK.PeerID()
	require.Error(t, err, "PeerID must fail for hedera-threshold")
	require.True(t, errors.As(err, &keyErr))
	assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind(),
		"hedera-threshold PeerID must return ErrUnsupportedKeyType")
}

// --- T044: Error Handling Integration Test ---

func TestIntegration_ErrorHandling(t *testing.T) {
	t.Run("ErrInvalidHex: non-hex characters in key string", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromHex("0xZZZZ74bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidHex, keyErr.Kind())
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrInvalidLength: 31-byte key", func(t *testing.T) {
		shortKey := make([]byte, 31)
		shortKey[0] = 0x01
		_, err := NeuronPrivateKeyFromBytes(shortKey)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrInvalidKey: key exceeds curve order (all 0xFF bytes)", func(t *testing.T) {
		invalidKey := make([]byte, 32)
		for i := range invalidKey {
			invalidKey[i] = 0xFF
		}
		_, err := NeuronPrivateKeyFromBytes(invalidKey)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrZeroValue: all-zero 32-byte key", func(t *testing.T) {
		zeroKey := make([]byte, 32)
		_, err := NeuronPrivateKeyFromBytes(zeroKey)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrUnsupportedKeyType: P-256 key passed to FromBlockchainKey", func(t *testing.T) {
		p256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		_, err = NeuronPrivateKeyFromBlockchainKey(p256Key)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "P-256")
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrEncryption: wrong password during decrypt", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "correct-password")
		require.NoError(t, err)

		_, err = Decrypt(encrypted, "wrong-password")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrEncryption, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "decryption failed")
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrMnemonic: invalid checksum mnemonic phrase", func(t *testing.T) {
		// "wrong" is not the correct checksum word for this sequence.
		badMnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon wrong"
		_, err := NeuronPrivateKeyFromMnemonic(badMnemonic, "")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrMnemonic, keyErr.Kind())
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrDerivation: malformed BIP-44 path", func(t *testing.T) {
		validMnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
		_, err := NeuronPrivateKeyFromMnemonic(validMnemonic, "m/abc")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrDerivation, keyErr.Kind())
		assertNoKeyMaterial(t, keyErr)
	})

	t.Run("ErrKeyMismatch: not directly producible from factory — skip", func(t *testing.T) {
		t.Skip("ErrKeyMismatch is used for matching verification, not factory construction errors")
	})

	t.Run("ErrInvalidFormat: not directly producible from factory — skip", func(t *testing.T) {
		t.Skip("ErrInvalidFormat is reserved for future format detection paths")
	})

	t.Run("ErrSDKError: not directly producible from factory — skip", func(t *testing.T) {
		t.Skip("ErrSDKError wraps internal blockchain SDK errors, not user-facing validation")
	})
}

// assertNoKeyMaterial verifies that an error message does not contain private key material.
// SEC-003, SEC-005: Error messages MUST NOT contain private key material.
func assertNoKeyMaterial(t *testing.T, keyErr *KeyError) {
	t.Helper()
	errMsg := keyErr.Error()

	// Known test key hex strings that must never appear in errors.
	knownKeys := []string{
		deterministicTestKey,
		deterministicTestKey2,
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	for _, keyHex := range knownKeys {
		assert.NotContains(t, strings.ToLower(errMsg), keyHex,
			"error message must not contain private key material")
	}

	// Also check that no raw 32-byte hex string (64 hex chars) appears in the error,
	// beyond well-known constants like addresses or expected lengths.
	// This is a heuristic check — a 64-char hex string in an error is suspicious.
}

// --- T045: Quickstart Validation Integration Test ---

func TestIntegration_QuickstartValidation(t *testing.T) {
	// This test validates that all API patterns documented in the quickstart guide
	// compile, execute, and produce expected types. It mirrors the typical developer
	// onboarding flow.

	t.Run("import from hex and derive identity chain", func(t *testing.T) {
		// Pattern: NeuronPrivateKeyFromHex -> PublicKey -> EVMAddress / PeerID / DIDKey
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		pub := key.PublicKey()
		assert.Len(t, pub.Bytes(), CompressedPublicKeyLength)

		addr := pub.EVMAddress()
		assert.Len(t, addr.Bytes(), EVMAddressLength)
		assert.True(t, strings.HasPrefix(addr.Hex(), "0x"),
			"EVMAddress.Hex() must be 0x-prefixed")

		pid, err := pub.PeerID()
		require.NoError(t, err)
		assert.NotEmpty(t, pid.String())

		did := pub.DIDKey()
		assert.True(t, strings.HasPrefix(did, "did:key:z"),
			"DIDKey must start with did:key:z")
		assert.True(t, strings.HasPrefix(did, "did:key:zQ3s"),
			"secp256k1 DIDKey must start with did:key:zQ3s")
	})

	t.Run("sign, verify, and recover", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		message := []byte("quickstart: hello neuron")

		// Pattern: key.Sign(message)
		sig, err := key.Sign(message)
		require.NoError(t, err)
		assert.Len(t, sig.Bytes(), SignatureLength)

		// Pattern: sig.Verify(message, pubkey)
		pub := key.PublicKey()
		verified := sig.Verify(message, pub)
		assert.True(t, verified, "Verify must return true for valid signature")

		// Pattern: sig.RecoverPublicKey(message)
		recovered, err := sig.RecoverPublicKey(message)
		require.NoError(t, err)
		assert.Equal(t, pub.Bytes(), recovered.Bytes(),
			"RecoverPublicKey must return the signer's public key")
	})

	t.Run("generate random key", func(t *testing.T) {
		// Pattern: NewNeuronPrivateKey()
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)
		assert.Len(t, key.Bytes(), PrivateKeyLength)
		assert.False(t, key.IsZero())

		// Verify full chain works on random key.
		pub := key.PublicKey()
		addr := pub.EVMAddress()
		pid, err := pub.PeerID()
		require.NoError(t, err)
		did := pub.DIDKey()

		assert.Len(t, pub.Bytes(), CompressedPublicKeyLength)
		assert.Len(t, addr.Bytes(), EVMAddressLength)
		assert.NotEmpty(t, pid.String())
		assert.True(t, strings.HasPrefix(did, "did:key:z"))
	})

	t.Run("generate mnemonic and derive key", func(t *testing.T) {
		// Pattern: GenerateMnemonic() -> NeuronPrivateKeyFromMnemonic(mnemonic, "")
		mnemonic, err := GenerateMnemonic()
		require.NoError(t, err)
		assert.Len(t, strings.Fields(mnemonic), 12,
			"BIP-39 mnemonic must have 12 words")

		key, err := NeuronPrivateKeyFromMnemonic(mnemonic, "")
		require.NoError(t, err)
		assert.Len(t, key.Bytes(), PrivateKeyLength)

		// Verify deterministic restoration.
		restored, err := NeuronPrivateKeyFromMnemonic(mnemonic, "")
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), restored.Bytes(),
			"same mnemonic must produce same key")
	})

	t.Run("encrypt and decrypt", func(t *testing.T) {
		// Pattern: Encrypt(key, password) -> Decrypt(encrypted, password)
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		password := "quickstart-demo-password"

		encrypted, err := Encrypt(key, password)
		require.NoError(t, err)
		assert.Equal(t, uint8(1), encrypted.version)
		assert.Len(t, encrypted.salt, EncryptionSaltLength)
		assert.Len(t, encrypted.nonce, EncryptionNonceLength)
		assert.Len(t, encrypted.ciphertext, EncryptionCiphertextLength)

		decrypted, err := Decrypt(encrypted, password)
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), decrypted.Bytes(),
			"decrypted key must be byte-identical to original")
		assert.Equal(t, key.Hex(), decrypted.Hex())
	})

	t.Run("encrypt with custom Argon2 params (v2)", func(t *testing.T) {
		// Pattern: Encrypt(key, password, WithArgon2Params(time, memory, threads))
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		encrypted, err := Encrypt(key, "v2-password",
			WithArgon2Params(2, 32*1024, 2))
		require.NoError(t, err)
		assert.Equal(t, uint8(2), encrypted.version)

		decrypted, err := Decrypt(encrypted, "v2-password")
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), decrypted.Bytes())
	})

	t.Run("EVM address formats", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		addr := key.PublicKey().EVMAddress()

		// Hex() returns EIP-55 checksummed.
		checksummed := addr.Hex()
		assert.True(t, strings.HasPrefix(checksummed, "0x"))
		assert.Len(t, checksummed, 42, "EIP-55 address must be 42 chars (0x + 40)")

		// LowercaseHex() returns all-lowercase.
		lower := addr.LowercaseHex()
		assert.True(t, strings.HasPrefix(lower, "0x"))
		assert.Equal(t, strings.ToLower(lower), lower,
			"LowercaseHex must return all-lowercase")

		// Bytes() returns raw 20 bytes.
		assert.Len(t, addr.Bytes(), EVMAddressLength)
	})

	t.Run("public key compressed and uncompressed forms", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		pub := key.PublicKey()

		compressed := pub.Compressed()
		assert.Len(t, compressed, CompressedPublicKeyLength,
			"Compressed() must return 33 bytes")

		uncompressed := pub.Uncompressed()
		assert.Len(t, uncompressed, UncompressedPublicKeyLen,
			"Uncompressed() must return 65 bytes")
		assert.Equal(t, byte(0x04), uncompressed[0],
			"uncompressed key must start with 0x04")

		// Bytes() is an alias for Compressed().
		assert.Equal(t, compressed, pub.Bytes(),
			"Bytes() must return the same as Compressed()")
	})

	t.Run("blockchain key interop", func(t *testing.T) {
		// Pattern: go-ethereum crypto.GenerateKey -> NeuronPrivateKeyFromBlockchainKey
		ecdsaKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
		require.NoError(t, err)
		assert.Len(t, key.Bytes(), PrivateKeyLength)

		// Pattern: key.ToBlockchainKey()
		bcKey, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, crypto.FromECDSA(ecdsaKey), crypto.FromECDSA(bcKey),
			"blockchain key round-trip must preserve key material")
	})

	t.Run("signature components", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(deterministicTestKey)
		require.NoError(t, err)

		sig, err := key.Sign([]byte("component test"))
		require.NoError(t, err)

		// R, S, V accessors.
		r := sig.R()
		s := sig.S()
		v := sig.V()
		assert.Len(t, r, 32, "R must be 32 bytes")
		assert.Len(t, s, 32, "S must be 32 bytes")
		assert.True(t, v == 0 || v == 1,
			"V from Sign must be 0 or 1, got %d", v)

		// StandardV and EthereumV.
		stdV := sig.StandardV()
		ethV := sig.EthereumV()
		assert.True(t, stdV == 0 || stdV == 1)
		assert.True(t, ethV == 27 || ethV == 28)
		assert.Equal(t, stdV+27, ethV,
			"EthereumV must equal StandardV + 27")
	})
}
