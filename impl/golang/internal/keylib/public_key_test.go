package keylib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deriveTestPublicKey derives the compressed and uncompressed public key bytes
// from the Hardhat test private key, using go-ethereum as the reference implementation.
func deriveTestPublicKey(t *testing.T) (compressed []byte, uncompressed []byte, ecdsaPub *ecdsa.PublicKey) {
	t.Helper()

	raw, err := hex.DecodeString(testPrivateKeyHex)
	require.NoError(t, err)

	ecdsaKey, err := crypto.ToECDSA(raw)
	require.NoError(t, err)

	ecdsaPub = &ecdsaKey.PublicKey
	compressed = crypto.CompressPubkey(ecdsaPub)
	uncompressed = crypto.FromECDSAPub(ecdsaPub)
	return compressed, uncompressed, ecdsaPub
}

// --- Construction from Hex ---

func TestNeuronPublicKeyFromHex_Compressed(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)
	compressedHex := hex.EncodeToString(compressed)

	t.Run("valid compressed hex without prefix", func(t *testing.T) {
		key, err := NeuronPublicKeyFromHex(compressedHex)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
	})

	t.Run("valid compressed hex with 0x prefix", func(t *testing.T) {
		key, err := NeuronPublicKeyFromHex("0x" + compressedHex)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
	})

	t.Run("valid compressed hex with 0X uppercase prefix", func(t *testing.T) {
		key, err := NeuronPublicKeyFromHex("0X" + compressedHex)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
	})

	t.Run("uppercase hex accepted and normalized", func(t *testing.T) {
		upper := strings.ToUpper(compressedHex)
		key, err := NeuronPublicKeyFromHex(upper)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
	})
}

func TestNeuronPublicKeyFromHex_Uncompressed(t *testing.T) {
	compressed, uncompressed, _ := deriveTestPublicKey(t)
	uncompressedHex := hex.EncodeToString(uncompressed)

	t.Run("valid uncompressed hex without prefix", func(t *testing.T) {
		key, err := NeuronPublicKeyFromHex(uncompressedHex)
		require.NoError(t, err)
		// Must be stored as compressed internally.
		assert.Equal(t, compressed, key.Bytes())
		assert.Len(t, key.Bytes(), CompressedPublicKeyLength)
	})

	t.Run("valid uncompressed hex with 0x prefix", func(t *testing.T) {
		key, err := NeuronPublicKeyFromHex("0x" + uncompressedHex)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
	})

	t.Run("uncompressed input decompresses back to same uncompressed form", func(t *testing.T) {
		key, err := NeuronPublicKeyFromHex(uncompressedHex)
		require.NoError(t, err)
		assert.Equal(t, uncompressed, key.Uncompressed())
	})
}

func TestNeuronPublicKeyFromHex_InvalidHex(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)
	compressedHex := hex.EncodeToString(compressed)

	tests := []struct {
		name     string
		input    string
		wantPos  string
		wantChar string
	}{
		{
			name:     "invalid char 'g' near end",
			input:    compressedHex[:len(compressedHex)-2] + "g0",
			wantPos:  "position 64",
			wantChar: "'g'",
		},
		{
			name:     "invalid char 'z' at start",
			input:    "z" + compressedHex[1:],
			wantPos:  "position 0",
			wantChar: "'z'",
		},
		{
			name:     "invalid char with 0x prefix reports correct position",
			input:    "0x" + compressedHex[:10] + "!" + compressedHex[11:],
			wantPos:  "position 12",
			wantChar: "'!'",
		},
		{
			name:     "space character",
			input:    compressedHex[:4] + " " + compressedHex[5:],
			wantPos:  "position 4",
			wantChar: "' '",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NeuronPublicKeyFromHex(tt.input)
			require.Error(t, err)

			var keyErr *KeyError
			require.True(t, errors.As(err, &keyErr))
			assert.Equal(t, ErrInvalidHex, keyErr.Kind())
			assert.Contains(t, keyErr.Error(), tt.wantPos)
			assert.Contains(t, keyErr.Error(), tt.wantChar)
		})
	}
}

func TestNeuronPublicKeyFromHex_InvalidLength(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		_, err := NeuronPublicKeyFromHex("0x02abcdef")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "expected 66 or 130 hex characters")
	})

	t.Run("too long for compressed but too short for uncompressed", func(t *testing.T) {
		// 67 hex chars: 1 too many for compressed, far too few for uncompressed.
		input := "02" + strings.Repeat("ab", 33)
		_, err := NeuronPublicKeyFromHex(input)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := NeuronPublicKeyFromHex("")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("only 0x prefix", func(t *testing.T) {
		_, err := NeuronPublicKeyFromHex("0x")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})
}

func TestNeuronPublicKeyFromHex_InvalidCurvePoint(t *testing.T) {
	t.Run("valid length but invalid compressed point", func(t *testing.T) {
		// 33 bytes of zeros: not a valid secp256k1 point.
		invalidCompressed := "02" + strings.Repeat("00", 32)
		_, err := NeuronPublicKeyFromHex(invalidCompressed)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "not a valid secp256k1 curve point")
	})

	t.Run("valid length but invalid uncompressed point", func(t *testing.T) {
		// 65 bytes with 0x04 prefix but all-zero X and Y: not a valid point.
		invalidUncompressed := "04" + strings.Repeat("00", 64)
		_, err := NeuronPublicKeyFromHex(invalidUncompressed)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "not a valid secp256k1 curve point")
	})
}

// --- Construction from Bytes ---

func TestNeuronPublicKeyFromBytes_Compressed(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)

	t.Run("valid 33-byte compressed input", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
		assert.Len(t, key.Bytes(), CompressedPublicKeyLength)
	})

	t.Run("first byte is 0x02 or 0x03", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)
		b := key.Bytes()
		assert.True(t, b[0] == 0x02 || b[0] == 0x03,
			"compressed key must start with 0x02 or 0x03, got 0x%02x", b[0])
	})
}

func TestNeuronPublicKeyFromBytes_Uncompressed(t *testing.T) {
	compressed, uncompressed, _ := deriveTestPublicKey(t)

	t.Run("valid 65-byte uncompressed input is compressed internally", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(uncompressed)
		require.NoError(t, err)
		// Stored as compressed.
		assert.Equal(t, compressed, key.Bytes())
		assert.Len(t, key.Bytes(), CompressedPublicKeyLength)
	})

	t.Run("round-trip: uncompressed input produces correct Uncompressed output", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(uncompressed)
		require.NoError(t, err)
		assert.Equal(t, uncompressed, key.Uncompressed())
	})
}

func TestNeuronPublicKeyFromBytes_InvalidLength(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"nil slice", nil},
		{"empty slice", []byte{}},
		{"1 byte", []byte{0x02}},
		{"32 bytes (private key length)", make([]byte, 32)},
		{"34 bytes (one too many for compressed)", make([]byte, 34)},
		{"64 bytes (one too few for uncompressed)", make([]byte, 64)},
		{"66 bytes (one too many for uncompressed)", make([]byte, 66)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NeuronPublicKeyFromBytes(tt.input)
			require.Error(t, err)

			var keyErr *KeyError
			require.True(t, errors.As(err, &keyErr))
			assert.Equal(t, ErrInvalidLength, keyErr.Kind())
			assert.Contains(t, keyErr.Error(), "expected 33 or 65 bytes")
		})
	}
}

func TestNeuronPublicKeyFromBytes_InvalidCurvePoint(t *testing.T) {
	t.Run("invalid compressed point", func(t *testing.T) {
		invalid := make([]byte, CompressedPublicKeyLength)
		invalid[0] = 0x02 // Valid prefix but X coordinate of zero is not on curve.
		_, err := NeuronPublicKeyFromBytes(invalid)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
	})

	t.Run("invalid uncompressed point", func(t *testing.T) {
		invalid := make([]byte, UncompressedPublicKeyLen)
		invalid[0] = 0x04 // Valid prefix but all-zero X||Y is not on curve.
		_, err := NeuronPublicKeyFromBytes(invalid)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
	})
}

// --- Construction from Blockchain Key ---

func TestNeuronPublicKeyFromBlockchainKey(t *testing.T) {
	compressed, _, ecdsaPub := deriveTestPublicKey(t)

	t.Run("valid secp256k1 key", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBlockchainKey(ecdsaPub)
		require.NoError(t, err)
		assert.Equal(t, compressed, key.Bytes())
	})

	t.Run("round-trips with ToBlockchainKey", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBlockchainKey(ecdsaPub)
		require.NoError(t, err)

		gotECDSA, err := key.ToBlockchainKey()
		require.NoError(t, err)

		// Compare uncompressed representations (X and Y coordinates).
		assert.Equal(t, crypto.FromECDSAPub(ecdsaPub), crypto.FromECDSAPub(gotECDSA))
	})

	t.Run("random secp256k1 key", func(t *testing.T) {
		ecdsaKey, err := crypto.GenerateKey()
		require.NoError(t, err)

		key, err := NeuronPublicKeyFromBlockchainKey(&ecdsaKey.PublicKey)
		require.NoError(t, err)

		expectedCompressed := crypto.CompressPubkey(&ecdsaKey.PublicKey)
		assert.Equal(t, expectedCompressed, key.Bytes())
	})

	t.Run("P-256 curve rejected as unsupported", func(t *testing.T) {
		p256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		_, err = NeuronPublicKeyFromBlockchainKey(&p256Key.PublicKey)
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

		_, err = NeuronPublicKeyFromBlockchainKey(&p384Key.PublicKey)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrUnsupportedKeyType, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "P-384")
	})

	t.Run("nil key rejected", func(t *testing.T) {
		_, err := NeuronPublicKeyFromBlockchainKey(nil)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "nil")
	})
}

// --- Compressed / Uncompressed Output ---

func TestNeuronPublicKey_Compressed(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)

	key, err := NeuronPublicKeyFromBytes(compressed)
	require.NoError(t, err)

	t.Run("returns 33 bytes", func(t *testing.T) {
		assert.Len(t, key.Compressed(), CompressedPublicKeyLength)
	})

	t.Run("starts with 0x02 or 0x03", func(t *testing.T) {
		b := key.Compressed()
		assert.True(t, b[0] == 0x02 || b[0] == 0x03,
			"expected 0x02 or 0x03 prefix, got 0x%02x", b[0])
	})

	t.Run("matches known compressed bytes", func(t *testing.T) {
		assert.Equal(t, compressed, key.Compressed())
	})
}

func TestNeuronPublicKey_Uncompressed(t *testing.T) {
	_, uncompressed, _ := deriveTestPublicKey(t)

	// Construct from compressed input.
	compressed, _, _ := deriveTestPublicKey(t)
	key, err := NeuronPublicKeyFromBytes(compressed)
	require.NoError(t, err)

	t.Run("returns 65 bytes", func(t *testing.T) {
		assert.Len(t, key.Uncompressed(), UncompressedPublicKeyLen)
	})

	t.Run("starts with 0x04", func(t *testing.T) {
		b := key.Uncompressed()
		assert.Equal(t, byte(0x04), b[0], "uncompressed key must start with 0x04")
	})

	t.Run("matches known uncompressed bytes", func(t *testing.T) {
		assert.Equal(t, uncompressed, key.Uncompressed())
	})

	t.Run("compressed then uncompressed is identity", func(t *testing.T) {
		// Start from uncompressed, construct, then get uncompressed back.
		key2, err := NeuronPublicKeyFromBytes(uncompressed)
		require.NoError(t, err)
		assert.Equal(t, uncompressed, key2.Uncompressed())
	})
}

// --- Bytes and Hex ---

func TestNeuronPublicKey_Bytes(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)
	key, err := NeuronPublicKeyFromBytes(compressed)
	require.NoError(t, err)

	t.Run("Bytes is alias for Compressed", func(t *testing.T) {
		assert.Equal(t, key.Compressed(), key.Bytes())
	})

	t.Run("returns 33 bytes", func(t *testing.T) {
		assert.Len(t, key.Bytes(), CompressedPublicKeyLength)
	})
}

func TestNeuronPublicKey_Hex(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)
	key, err := NeuronPublicKeyFromBytes(compressed)
	require.NoError(t, err)

	t.Run("returns 0x-prefixed lowercase hex", func(t *testing.T) {
		h := key.Hex()
		assert.Equal(t, "0x", h[:2])
		assert.Len(t, h, 2+CompressedPublicKeyLength*2) // "0x" + 66 hex chars
	})

	t.Run("hex is always lowercase", func(t *testing.T) {
		h := key.Hex()
		assert.Equal(t, strings.ToLower(h), h)
	})

	t.Run("hex round-trip", func(t *testing.T) {
		h := key.Hex()
		key2, err := NeuronPublicKeyFromHex(h)
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), key2.Bytes())
	})
}

// --- ToBlockchainKey ---

func TestNeuronPublicKey_ToBlockchainKey(t *testing.T) {
	compressed, _, ecdsaPub := deriveTestPublicKey(t)

	key, err := NeuronPublicKeyFromBytes(compressed)
	require.NoError(t, err)

	t.Run("returns valid secp256k1 public key", func(t *testing.T) {
		gotECDSA, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, crypto.S256().Params().Name, gotECDSA.Curve.Params().Name)
	})

	t.Run("matches original ECDSA key coordinates", func(t *testing.T) {
		gotECDSA, err := key.ToBlockchainKey()
		require.NoError(t, err)
		assert.Equal(t, ecdsaPub.X.Bytes(), gotECDSA.X.Bytes())
		assert.Equal(t, ecdsaPub.Y.Bytes(), gotECDSA.Y.Bytes())
	})

	t.Run("round-trips with FromBlockchainKey", func(t *testing.T) {
		gotECDSA, err := key.ToBlockchainKey()
		require.NoError(t, err)

		key2, err := NeuronPublicKeyFromBlockchainKey(gotECDSA)
		require.NoError(t, err)
		assert.Equal(t, key.Bytes(), key2.Bytes())
	})
}

// --- Immutability ---

func TestNeuronPublicKey_Immutability(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)

	t.Run("modifying Bytes return does not affect internal state", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		original := key.Bytes()
		for i := range original {
			original[i] = 0xff
		}

		// Internal state must be unchanged.
		assert.Equal(t, compressed, key.Bytes())
		assert.NotEqual(t, original, key.Bytes())
	})

	t.Run("modifying Compressed return does not affect internal state", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		c := key.Compressed()
		for i := range c {
			c[i] = 0xff
		}

		assert.Equal(t, compressed, key.Compressed())
	})

	t.Run("modifying Uncompressed return does not affect internal state", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		original := key.Uncompressed()
		u := key.Uncompressed()
		for i := range u {
			u[i] = 0xff
		}

		assert.Equal(t, original, key.Uncompressed())
	})

	t.Run("modifying input bytes does not affect constructed key", func(t *testing.T) {
		input := make([]byte, CompressedPublicKeyLength)
		copy(input, compressed)

		key, err := NeuronPublicKeyFromBytes(input)
		require.NoError(t, err)

		// Mutate the original input slice.
		for i := range input {
			input[i] = 0x00
		}

		// The key must still hold the original value.
		assert.Equal(t, compressed, key.Bytes())
	})

	t.Run("consecutive Bytes calls return independent copies", func(t *testing.T) {
		key, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		b1 := key.Bytes()
		b2 := key.Bytes()

		// Equal content but independent backing arrays.
		assert.Equal(t, b1, b2)
		b1[0] = 0x00
		assert.NotEqual(t, b1, b2, "Bytes() must return independent copies")
	})
}

// --- Determinism ---

func TestNeuronPublicKey_Deterministic(t *testing.T) {
	compressed, uncompressed, _ := deriveTestPublicKey(t)

	t.Run("same compressed bytes always produce same key", func(t *testing.T) {
		key1, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		key2, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		assert.Equal(t, key1.Bytes(), key2.Bytes())
		assert.Equal(t, key1.Hex(), key2.Hex())
		assert.Equal(t, key1.Uncompressed(), key2.Uncompressed())
	})

	t.Run("same uncompressed bytes always produce same key", func(t *testing.T) {
		key1, err := NeuronPublicKeyFromBytes(uncompressed)
		require.NoError(t, err)

		key2, err := NeuronPublicKeyFromBytes(uncompressed)
		require.NoError(t, err)

		assert.Equal(t, key1.Bytes(), key2.Bytes())
		assert.Equal(t, key1.Hex(), key2.Hex())
	})

	t.Run("compressed and uncompressed inputs for same key produce same result", func(t *testing.T) {
		keyFromCompressed, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		keyFromUncompressed, err := NeuronPublicKeyFromBytes(uncompressed)
		require.NoError(t, err)

		assert.Equal(t, keyFromCompressed.Bytes(), keyFromUncompressed.Bytes())
		assert.Equal(t, keyFromCompressed.Hex(), keyFromUncompressed.Hex())
		assert.Equal(t, keyFromCompressed.Uncompressed(), keyFromUncompressed.Uncompressed())
	})

	t.Run("hex and bytes construction produce same result", func(t *testing.T) {
		compressedHex := hex.EncodeToString(compressed)

		keyFromHex, err := NeuronPublicKeyFromHex(compressedHex)
		require.NoError(t, err)

		keyFromBytes, err := NeuronPublicKeyFromBytes(compressed)
		require.NoError(t, err)

		assert.Equal(t, keyFromHex.Bytes(), keyFromBytes.Bytes())
	})
}

// --- Integration with NeuronPrivateKey ---

func TestNeuronPublicKey_IntegrationWithPrivateKey(t *testing.T) {
	t.Run("private key PublicKey matches direct construction", func(t *testing.T) {
		privKey, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		pubFromPriv := privKey.PublicKey()

		// Also construct directly from blockchain key.
		ecdsaPriv, err := privKey.ToBlockchainKey()
		require.NoError(t, err)

		pubFromBlockchain, err := NeuronPublicKeyFromBlockchainKey(&ecdsaPriv.PublicKey)
		require.NoError(t, err)

		assert.Equal(t, pubFromPriv.Bytes(), pubFromBlockchain.Bytes())
	})

	t.Run("PublicKey Compressed output matches go-ethereum", func(t *testing.T) {
		compressed, _, _ := deriveTestPublicKey(t)

		privKey, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		assert.Equal(t, compressed, privKey.PublicKey().Compressed())
	})

	t.Run("PublicKey Uncompressed output matches go-ethereum", func(t *testing.T) {
		_, uncompressed, _ := deriveTestPublicKey(t)

		privKey, err := NeuronPrivateKeyFromHex(testPrivateKeyHex)
		require.NoError(t, err)

		assert.Equal(t, uncompressed, privKey.PublicKey().Uncompressed())
	})
}

// --- Concurrent Safety ---

func TestNeuronPublicKey_ConcurrentAccess(t *testing.T) {
	compressed, _, _ := deriveTestPublicKey(t)
	key, err := NeuronPublicKeyFromBytes(compressed)
	require.NoError(t, err)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // 3 operations per goroutine

	compressedResults := make([][]byte, goroutines)
	uncompressedResults := make([][]byte, goroutines)
	hexResults := make([]string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			compressedResults[idx] = key.Compressed()
		}(i)
		go func(idx int) {
			defer wg.Done()
			uncompressedResults[idx] = key.Uncompressed()
		}(i)
		go func(idx int) {
			defer wg.Done()
			hexResults[idx] = key.Hex()
		}(i)
	}
	wg.Wait()

	for i := 1; i < goroutines; i++ {
		assert.Equal(t, compressedResults[0], compressedResults[i],
			"concurrent Compressed() call %d returned different result", i)
		assert.Equal(t, uncompressedResults[0], uncompressedResults[i],
			"concurrent Uncompressed() call %d returned different result", i)
		assert.Equal(t, hexResults[0], hexResults[i],
			"concurrent Hex() call %d returned different result", i)
	}
}

// --- Error Type Tests ---

func TestNeuronPublicKey_ErrorsImplementKeyError(t *testing.T) {
	t.Run("errors.Is matches by kind for invalid hex", func(t *testing.T) {
		_, err := NeuronPublicKeyFromHex("not-valid-hex!")
		require.Error(t, err)
		assert.True(t, errors.Is(err, NewKeyError(ErrInvalidHex, "", "")))
	})

	t.Run("errors.As extracts KeyError for invalid length", func(t *testing.T) {
		_, err := NeuronPublicKeyFromBytes([]byte{0x01})
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Equal(t, "NeuronPublicKeyFromBytes", keyErr.Operation())
	})

	t.Run("errors.As extracts KeyError for invalid curve point", func(t *testing.T) {
		invalid := make([]byte, CompressedPublicKeyLength)
		invalid[0] = 0x02
		_, err := NeuronPublicKeyFromBytes(invalid)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidKey, keyErr.Kind())
		assert.Equal(t, "NeuronPublicKeyFromBytes", keyErr.Operation())
	})

	t.Run("FromBlockchainKey operation name", func(t *testing.T) {
		_, err := NeuronPublicKeyFromBlockchainKey(nil)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, "NeuronPublicKeyFromBlockchainKey", keyErr.Operation())
	})

	t.Run("FromHex operation name", func(t *testing.T) {
		_, err := NeuronPublicKeyFromHex("")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, "NeuronPublicKeyFromHex", keyErr.Operation())
	})
}
