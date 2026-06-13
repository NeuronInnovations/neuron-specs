package keylib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hardhat account #0 private key — a well-known deterministic test vector.
const testPrivateKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// --- Construction Tests ---

func TestNeuronPrivateKeyFromHex(t *testing.T) {
	t.Run("valid hex with 0x prefix", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex("0x" + testPrivateKeyHex)
		require.NoError(t, err)
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
	})

	t.Run("valid hex without prefix", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
	})

	t.Run("valid hex with 0X uppercase prefix", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex("0X" + testPrivateKeyHex)
		require.NoError(t, err)
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
	})

	t.Run("known test vector round-trips", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		// Bytes should be exactly 32 bytes.
		b := key.Bytes()
		assert.Len(t, b, PrivateKeyLength)

		// Hex round-trip.
		key2, err := NeuronPrivateKeyFromHex(key.Hex())
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), key2.Bytes())
	})

	t.Run("uppercase hex accepted", func(t *testing.T) {
		upper := "AC0974BEC39A17E36BA4A6B4D238FF944BACB478CBED5EFCAE784D7BF4F2FF80"
		key, err := NeuronPrivateKeyFromHex(upper)
		require.NoError(t, err)
		// Output is always lowercase.
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
	})
}

func TestNeuronPrivateKeyFromHex_InvalidHex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPos  string // expected substring in error message about position
		wantChar string // expected invalid character in error message
	}{
		{
			name:     "invalid char 'g' near end",
			input:    "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2fg80",
			wantPos:  "position 61",
			wantChar: "'g'",
		},
		{
			name:     "invalid char 'z' at start",
			input:    "zc0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			wantPos:  "position 0",
			wantChar: "'z'",
		},
		{
			name:     "invalid char with 0x prefix reports correct position",
			input:    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2fX80",
			wantPos:  "position 63",
			wantChar: "'X'",
		},
		{
			name:     "space character",
			input:    "ac09 74bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			wantPos:  "position 4",
			wantChar: "' '",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NeuronPrivateKeyFromHex(tt.input)
			require.Error(t, err)

			var keyErr *KeyError
			require.True(t, errors.As(err, &keyErr))
			assert.Equal(t, ErrInvalidHex, keyErr.Kind())
			assert.Contains(t, keyErr.Error(), tt.wantPos)
			assert.Contains(t, keyErr.Error(), tt.wantChar)
		})
	}
}

func TestNeuronPrivateKeyFromHex_InvalidLength(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromHex("0xac0974bec39a")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "expected 64 hex characters")
	})

	t.Run("too long", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromHex("0x" + testPrivateKeyHex + "ff")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "got 66")
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromHex("")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("only 0x prefix", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromHex("0x")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})
}

func TestNeuronPrivateKeyFromHex_ZeroValue(t *testing.T) {
	t.Run("all zeros rejected", func(t *testing.T) {
		zeroHex := "0000000000000000000000000000000000000000000000000000000000000000"
		_, err := NeuronPrivateKeyFromHex(zeroHex)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "all zeros")
	})

	t.Run("all zeros with prefix rejected", func(t *testing.T) {
		zeroHex := "0x0000000000000000000000000000000000000000000000000000000000000000"
		_, err := NeuronPrivateKeyFromHex(zeroHex)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
	})
}

func TestNeuronPrivateKeyFromBytes(t *testing.T) {
	t.Run("valid 32 bytes", func(t *testing.T) {
		raw, err := hex.DecodeString(testPrivateKeyHex)
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromBytes(raw)
		require.NoError(t, err)
		assert.Equal(t, raw, key.Bytes())
	})

	t.Run("wrong length too short", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromBytes([]byte{0x01, 0x02, 0x03})
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "expected 32 bytes, got 3")
	})

	t.Run("wrong length too long", func(t *testing.T) {
		raw := make([]byte, 33)
		raw[0] = 0x01
		_, err := NeuronPrivateKeyFromBytes(raw)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "got 33")
	})

	t.Run("nil slice rejected", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromBytes(nil)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("zero bytes rejected", func(t *testing.T) {
		zeroes := make([]byte, 32)
		_, err := NeuronPrivateKeyFromBytes(zeroes)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
	})
}

func TestNeuronPrivateKeyFromBlockchainKey(t *testing.T) {
	t.Run("valid secp256k1 key", func(t *testing.T) {
		ecdsaKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
		require.NoError(t, err)

		// Round-trip: convert back and compare raw bytes.
		gotECDSA, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, crypto.FromECDSA(ecdsaKey), crypto.FromECDSA(gotECDSA))
	})

	t.Run("known test vector via blockchain key", func(t *testing.T) {
		raw, err := hex.DecodeString(testPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := crypto.ToECDSA(raw)
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
		require.NoError(t, err)
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
	})

	t.Run("P-256 curve rejected as unsupported", func(t *testing.T) {
		// Generate a P-256 key to simulate non-secp256k1.
		p256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		_, err = NeuronPrivateKeyFromBlockchainKey(p256Key)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "expected secp256k1 curve")
		assert.Contains(t, keyErr.Error(), "P-256")
	})

	t.Run("P-384 curve rejected as unsupported", func(t *testing.T) {
		p384Key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		require.NoError(t, err)

		_, err = NeuronPrivateKeyFromBlockchainKey(p384Key)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "P-384")
	})

	t.Run("nil key rejected", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromBlockchainKey(nil)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "nil")
	})
}

// --- Immutability Tests ---

func TestNeuronPrivateKey_Immutability(t *testing.T) {
	t.Run("modifying Bytes return value does not affect internal state", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		original := key.Bytes()
		// Mutate the returned slice.
		for i := range original {
			original[i] = 0xff
		}

		// Internal state must be unchanged.
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
		assert.NotEqual(t, original, key.Bytes())
	})

	t.Run("modifying input bytes does not affect constructed key", func(t *testing.T) {
		raw, err := hex.DecodeString(testPrivateKeyHex)
		require.NoError(t, err)

		key, err := NeuronPrivateKeyFromBytes(raw)
		require.NoError(t, err)

		// Mutate the original input slice.
		for i := range raw {
			raw[i] = 0x00
		}

		// The key must still hold the original value.
		assert.Equal(t, "0x"+testPrivateKeyHex, key.Hex())
	})

	t.Run("consecutive Bytes calls return independent copies", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		b1 := key.Bytes()
		b2 := key.Bytes()

		// Equal content but different backing arrays.
		assert.Equal(t, b1, b2)
		b1[0] = 0x00
		assert.NotEqual(t, b1, b2, "Bytes() must return independent copies")
	})
}

// --- PublicKey Derivation Tests ---

func TestNeuronPrivateKey_PublicKey(t *testing.T) {
	t.Run("derives valid compressed public key", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()
		pubBytes := pub.Bytes()

		// Compressed public key is 33 bytes.
		assert.Len(t, pubBytes, CompressedPublicKeyLength)

		// First byte must be 0x02 or 0x03 (compressed point prefix).
		assert.True(t, pubBytes[0] == 0x02 || pubBytes[0] == 0x03,
			"compressed public key must start with 0x02 or 0x03, got 0x%02x", pubBytes[0])
	})

	t.Run("PublicKey is deterministic", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub1 := key.PublicKey()
		pub2 := key.PublicKey()
		assert.Equal(t, pub1.Bytes(), pub2.Bytes())
	})

	t.Run("PublicKey matches go-ethereum derivation", func(t *testing.T) {
		raw, err := hex.DecodeString(testPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := crypto.ToECDSA(raw)
		require.NoError(t, err)
		expectedPub := crypto.CompressPubkey(&ecdsaKey.PublicKey)

		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		assert.Equal(t, expectedPub, key.PublicKey().Bytes())
	})

	t.Run("concurrent PublicKey calls are safe", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		const goroutines = 100
		var wg sync.WaitGroup
		wg.Add(goroutines)

		results := make([][]byte, goroutines)
		for i := 0; i < goroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				results[idx] = key.PublicKey().Bytes()
			}(i)
		}
		wg.Wait()

		// All results must be identical.
		for i := 1; i < goroutines; i++ {
			assert.Equal(t, results[0], results[i],
				"concurrent PublicKey() call %d returned different result", i)
		}
	})
}

// --- Hex / Bytes Derivation Tests ---

func TestNeuronPrivateKey_Hex(t *testing.T) {
	t.Run("returns 0x-prefixed lowercase hex", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		h := key.Hex()
		assert.True(t, len(h) == 66, "hex string should be 66 chars (0x + 64)")
		assert.Equal(t, "0x", h[:2])
		assert.Equal(t, testPrivateKeyHex, h[2:])
	})

	t.Run("hex is always lowercase", func(t *testing.T) {
		upper := "AC0974BEC39A17E36BA4A6B4D238FF944BACB478CBED5EFCAE784D7BF4F2FF80"
		key, err := NeuronPrivateKeyFromHex(upper)
		require.NoError(t, err)

		h := key.Hex()
		assert.Equal(t, "0x"+testPrivateKeyHex, h)
	})
}

func TestNeuronPrivateKey_Bytes(t *testing.T) {
	t.Run("returns copy of 32 bytes", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		b := key.Bytes()
		assert.Len(t, b, PrivateKeyLength)

		// Verify against known hex decode.
		expected, err := hex.DecodeString(testPrivateKeyHex)
		require.NoError(t, err)
		assert.Equal(t, expected, b)
	})
}

// --- Zeroize Tests ---

func TestNeuronPrivateKey_Zeroize(t *testing.T) {
	t.Run("zeroes memory", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		// Before zeroize.
		assert.False(t, key.IsZero())

		key.Zeroize()

		// After zeroize.
		assert.True(t, key.IsZero())

		// Bytes should all be zero.
		b := key.Bytes()
		allZero := true
		for _, v := range b {
			if v != 0 {
				allZero = false
				break
			}
		}
		assert.True(t, allZero, "all bytes must be zero after Zeroize()")
	})

	t.Run("Hex returns zeroed hex after zeroize", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		key.Zeroize()

		h := key.Hex()
		assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", h)
	})

	t.Run("ToBlockchainKey fails after zeroize", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		key.Zeroize()

		_, err = key.ToBlockchainKey()
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "zeroized")
	})

	t.Run("IsZero is false for valid key", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)
		assert.False(t, key.IsZero())
	})
}

// --- ToBlockchainKey Tests ---

func TestNeuronPrivateKey_ToBlockchainKey(t *testing.T) {
	t.Run("round-trips with FromBlockchainKey", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)

		key2, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), key2.Bytes())
	})

	t.Run("returned key has secp256k1 curve", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, crypto.S256().Params().Name, ecdsaKey.Curve.Params().Name)
	})
}

// --- Key Generation Tests ---

func TestNewNeuronPrivateKey(t *testing.T) {
	t.Run("generated key is non-zero and on secp256k1 curve", func(t *testing.T) {
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		// Must not be zeroized.
		assert.False(t, key.IsZero())

		// Must be exactly 32 bytes.
		assert.Len(t, key.Bytes(), PrivateKeyLength)

		// Must not be all zeros.
		allZero := true
		for _, b := range key.Bytes() {
			if b != 0 {
				allZero = false
				break
			}
		}
		assert.False(t, allZero, "generated key must not be all zeros")

		// Must be valid on secp256k1 (ToBlockchainKey succeeds).
		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, crypto.S256().Params().Name, ecdsaKey.Curve.Params().Name)
	})

	t.Run("two generated keys differ", func(t *testing.T) {
		key1, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		key2, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		assert.NotEqual(t, key1.Bytes(), key2.Bytes(),
			"two independently generated keys must differ")
	})

	t.Run("can derive pubkey, EVMAddress, PeerID, DIDKey", func(t *testing.T) {
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		// PublicKey derivation.
		pub := key.PublicKey()
		pubBytes := pub.Bytes()
		assert.Len(t, pubBytes, CompressedPublicKeyLength)
		assert.True(t, pubBytes[0] == 0x02 || pubBytes[0] == 0x03,
			"compressed public key prefix must be 0x02 or 0x03")

		// EVMAddress derivation.
		evmAddr := pub.EVMAddress()
		assert.Len(t, evmAddr.Bytes(), EVMAddressLength)

		// PeerID derivation.
		peerID, err := pub.PeerID()
		require.NoError(t, err)
		assert.NotEmpty(t, peerID.String())

		// DIDKey derivation.
		didKey := pub.DIDKey()
		assert.True(t, len(didKey) > len("did:key:z"),
			"DID:key must be longer than the bare prefix")
		assert.Contains(t, didKey, "did:key:z")
	})

	t.Run("can sign and verify", func(t *testing.T) {
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		message := []byte("test message for generated key")
		sig, err := key.Sign(message)
		require.NoError(t, err)

		// Verify the signature against the key's public key.
		pub := key.PublicKey()
		assert.True(t, sig.Verify(message, pub), "signature must verify against signer's public key")

		// Verify with wrong message fails.
		assert.False(t, sig.Verify([]byte("wrong message"), pub),
			"signature must not verify against wrong message")
	})
}

// --- Error Type Tests ---

func TestNeuronPrivateKey_ErrorsImplementKeyError(t *testing.T) {
	t.Run("errors.Is matches by kind", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromHex("not-valid-hex!")
		require.Error(t, err)

		assert.True(t, errors.Is(err, NewKeyError(ErrInvalidHex, "", "")))
	})

	t.Run("errors.As extracts KeyError", func(t *testing.T) {
		_, err := NeuronPrivateKeyFromBytes([]byte{0x01})
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Equal(t, "NeuronPrivateKeyFromBytes", keyErr.Operation())
	})
}

// --- T023: Bidirectional Blockchain Key Conversion Round-Trip Tests ---

func TestNeuronPrivateKey_ToBlockchainKey_RoundTrip(t *testing.T) {
	t.Run("private key round-trips through blockchain key", func(t *testing.T) {
		// NeuronPrivateKey -> ToBlockchainKey() -> NeuronPrivateKeyFromBlockchainKey() -> byte-equal
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)

		roundTripped, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
		require.NoError(t, err)

		assert.Equal(t, key.Bytes(), roundTripped.Bytes(),
			"private key bytes must be identical after round-trip through blockchain key")
	})

	t.Run("public key round-trips through blockchain key", func(t *testing.T) {
		// NeuronPublicKey -> ToBlockchainKey() -> NeuronPublicKeyFromBlockchainKey() -> identical compressed bytes
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pub := key.PublicKey()

		ecdsaPub, err := pub.ToBlockchainKey()
		require.NoError(t, err)

		roundTripped, err := NeuronPublicKeyFromBlockchainKey(ecdsaPub)
		require.NoError(t, err)

		assert.Equal(t, pub.Bytes(), roundTripped.Bytes(),
			"public key compressed bytes must be identical after round-trip through blockchain key")
	})

	t.Run("all derived values match after private key round-trip", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		// Derive all values from the original key.
		origPub := key.PublicKey()
		origAddr := origPub.EVMAddress()
		origPID, err := origPub.PeerID()
		require.NoError(t, err)
		origDID := origPub.DIDKey()

		// Round-trip through blockchain key.
		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)
		roundTripped, err := NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
		require.NoError(t, err)

		// Derive all values from the round-tripped key.
		rtPub := roundTripped.PublicKey()
		rtAddr := rtPub.EVMAddress()
		rtPID, err := rtPub.PeerID()
		require.NoError(t, err)
		rtDID := rtPub.DIDKey()

		// All derived values must match.
		assert.Equal(t, origPub.Bytes(), rtPub.Bytes(), "public key mismatch after round-trip")
		assert.Equal(t, origAddr.Bytes(), rtAddr.Bytes(), "EVMAddress mismatch after round-trip")
		assert.Equal(t, origPID.String(), rtPID.String(), "PeerID mismatch after round-trip")
		assert.Equal(t, origDID, rtDID, "DID:key mismatch after round-trip")
	})

	t.Run("all derived values match after public key round-trip", func(t *testing.T) {
		key, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		origPub := key.PublicKey()
		origAddr := origPub.EVMAddress()
		origPID, err := origPub.PeerID()
		require.NoError(t, err)
		origDID := origPub.DIDKey()

		// Round-trip public key through blockchain key.
		ecdsaPub, err := origPub.ToBlockchainKey()
		require.NoError(t, err)
		rtPub, err := NeuronPublicKeyFromBlockchainKey(ecdsaPub)
		require.NoError(t, err)

		rtAddr := rtPub.EVMAddress()
		rtPID, err := rtPub.PeerID()
		require.NoError(t, err)
		rtDID := rtPub.DIDKey()

		assert.Equal(t, origPub.Bytes(), rtPub.Bytes(), "public key mismatch after round-trip")
		assert.Equal(t, origAddr.Bytes(), rtAddr.Bytes(), "EVMAddress mismatch after round-trip")
		assert.Equal(t, origPID.String(), rtPID.String(), "PeerID mismatch after round-trip")
		assert.Equal(t, origDID, rtDID, "DID:key mismatch after round-trip")
	})

	t.Run("round-trip works with randomly generated keys", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			ecKey, err := crypto.GenerateKey()
			require.NoError(t, err)

			key, err := NeuronPrivateKeyFromBlockchainKey(ecKey)
			require.NoError(t, err)

			// Private key round-trip.
			bcKey, err := key.ToBlockchainKey()
			require.NoError(t, err)
			rtKey, err := NeuronPrivateKeyFromBlockchainKey(bcKey)
			require.NoError(t, err)
			assert.Equal(t, key.Bytes(), rtKey.Bytes(), "key %d: private key round-trip failed", i)

			// Public key round-trip.
			pub := key.PublicKey()
			bcPub, err := pub.ToBlockchainKey()
			require.NoError(t, err)
			rtPub, err := NeuronPublicKeyFromBlockchainKey(bcPub)
			require.NoError(t, err)
			assert.Equal(t, pub.Bytes(), rtPub.Bytes(), "key %d: public key round-trip failed", i)
		}
	})
}

// --- T028+T031: NewNeuronPrivateKey (Key Generation) Extended Tests ---

func TestNewNeuronPrivateKey_Extended(t *testing.T) {
	t.Run("generated key is non-zero and on secp256k1 curve", func(t *testing.T) {
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		assert.False(t, key.IsZero(), "generated key must not be zeroized")
		assert.Len(t, key.Bytes(), PrivateKeyLength, "private key must be 32 bytes")

		// Verify the key is on the secp256k1 curve by converting to blockchain key.
		ecdsaKey, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, crypto.S256().Params().Name, ecdsaKey.Curve.Params().Name,
			"generated key must be on secp256k1 curve")
	})

	t.Run("two generated keys are different", func(t *testing.T) {
		key1, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		key2, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		assert.NotEqual(t, key1.Bytes(), key2.Bytes(),
			"two independently generated keys must have different bytes")
		assert.NotEqual(t, key1.Hex(), key2.Hex(),
			"two independently generated keys must have different hex representations")
	})

	t.Run("generated key can derive public key, EVMAddress, PeerID, DIDKey", func(t *testing.T) {
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		pub := key.PublicKey()
		assert.Len(t, pub.Bytes(), CompressedPublicKeyLength,
			"public key must be 33 bytes compressed")

		addr := pub.EVMAddress()
		assert.Len(t, addr.Bytes(), EVMAddressLength,
			"EVM address must be 20 bytes")

		pid, err := pub.PeerID()
		require.NoError(t, err)
		assert.NotEmpty(t, pid.String(), "PeerID must be non-empty")

		did := pub.DIDKey()
		assert.Contains(t, did, "did:key:z", "DID:key must start with did:key:z")
	})

	t.Run("generated key can sign and verify", func(t *testing.T) {
		key, err := NewNeuronPrivateKey()
		require.NoError(t, err)

		message := []byte("test message for key generation")
		sig, err := key.Sign(message)
		require.NoError(t, err)

		assert.Len(t, sig.Bytes(), SignatureLength, "signature must be 65 bytes")
		assert.True(t, sig.Verify(message, key.PublicKey()),
			"signature must verify against the signer's public key")

		// Verify that the recovered public key matches.
		recovered, err := sig.RecoverPublicKey(message)
		require.NoError(t, err)
		assert.Equal(t, key.PublicKey().Bytes(), recovered.Bytes(),
			"recovered public key must match the original")
	})

	t.Run("multiple generated keys produce unique public keys and addresses", func(t *testing.T) {
		const count = 10
		pubKeys := make(map[string]struct{}, count)
		addresses := make(map[string]struct{}, count)

		for i := 0; i < count; i++ {
			key, err := NewNeuronPrivateKey()
			require.NoError(t, err)

			pubHex := key.PublicKey().Hex()
			addrHex := key.PublicKey().EVMAddress().Hex()

			_, pubDup := pubKeys[pubHex]
			assert.False(t, pubDup, "public key collision at iteration %d", i)
			pubKeys[pubHex] = struct{}{}

			_, addrDup := addresses[addrHex]
			assert.False(t, addrDup, "address collision at iteration %d", i)
			addresses[addrHex] = struct{}{}
		}
	})
}

