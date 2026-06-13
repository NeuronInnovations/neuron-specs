package keylib

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hardhat account #0 test vector.
const (
	testEVMPrivateKeyHex = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	testEVMAddressHex    = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" // EIP-55 checksummed
)

// --- Derivation from NeuronPublicKey (T009) ---

func TestNeuronPublicKey_EVMAddress(t *testing.T) {
	t.Run("derives correct address from known test vector", func(t *testing.T) {
		// Construct NeuronPrivateKey from the Hardhat #0 private key.
		privKey, err := NeuronPrivateKeyFromHex(testEVMPrivateKeyHex)
		require.NoError(t, err)

		pubKey := privKey.PublicKey()
		addr := pubKey.EVMAddress()

		// Verify against the known EIP-55 checksummed address.
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("derivation matches go-ethereum reference implementation", func(t *testing.T) {
		// Use go-ethereum to derive the address independently.
		raw, err := hex.DecodeString(testEVMPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := crypto.ToECDSA(raw)
		require.NoError(t, err)

		// go-ethereum's canonical address derivation.
		expectedAddr := crypto.PubkeyToAddress(ecdsaKey.PublicKey)

		// Our derivation via NeuronPublicKey.
		privKey, err := NeuronPrivateKeyFromHex(testEVMPrivateKeyHex)
		require.NoError(t, err)

		pubKey := privKey.PublicKey()
		addr := pubKey.EVMAddress()

		assert.Equal(t, expectedAddr.Hex(), addr.Hex())
		assert.Equal(t, expectedAddr.Bytes(), addr.Bytes())
	})

	t.Run("derivation uses Keccak256 of uncompressed pubkey without 04 prefix", func(t *testing.T) {
		// Manual verification of the derivation algorithm:
		// 1. Decompress compressed pubkey to 65 bytes (04 || X || Y)
		// 2. Keccak256(X || Y) -- skip the 04 prefix
		// 3. Take last 20 bytes
		raw, err := hex.DecodeString(testEVMPrivateKeyHex)
		require.NoError(t, err)

		ecdsaKey, err := crypto.ToECDSA(raw)
		require.NoError(t, err)

		uncompressed := crypto.FromECDSAPub(&ecdsaKey.PublicKey)
		require.Len(t, uncompressed, 65, "uncompressed pubkey must be 65 bytes")
		require.Equal(t, byte(0x04), uncompressed[0], "uncompressed prefix must be 0x04")

		hash := crypto.Keccak256(uncompressed[1:])
		expectedBytes := hash[len(hash)-EVMAddressLength:]

		// Our derivation.
		privKey, err := NeuronPrivateKeyFromHex(testEVMPrivateKeyHex)
		require.NoError(t, err)
		addr := privKey.PublicKey().EVMAddress()

		assert.Equal(t, expectedBytes, addr.Bytes())
	})

	t.Run("derivation is deterministic", func(t *testing.T) {
		privKey, err := NeuronPrivateKeyFromHex(testEVMPrivateKeyHex)
		require.NoError(t, err)

		pubKey := privKey.PublicKey()

		addr1 := pubKey.EVMAddress()
		addr2 := pubKey.EVMAddress()
		assert.Equal(t, addr1.Hex(), addr2.Hex())
		assert.Equal(t, addr1.Bytes(), addr2.Bytes())
	})
}

// --- Hex() EIP-55 Checksum Tests ---

func TestEVMAddress_Hex(t *testing.T) {
	t.Run("returns EIP-55 checksummed address", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		h := addr.Hex()
		assert.Equal(t, testEVMAddressHex, h)

		// Verify it matches go-ethereum's EIP-55 output.
		goAddr := common.HexToAddress(testEVMAddressHex)
		assert.Equal(t, goAddr.Hex(), h)
	})

	t.Run("EIP-55 output from lowercase input", func(t *testing.T) {
		lowered := strings.ToLower(testEVMAddressHex)
		addr, err := EVMAddressFromHex(lowered)
		require.NoError(t, err)

		// Hex() must still produce the canonical EIP-55 checksum.
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("starts with 0x prefix", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		h := addr.Hex()
		assert.True(t, strings.HasPrefix(h, "0x"))
		assert.Len(t, h, 42, "0x + 40 hex chars = 42 total")
	})
}

// --- LowercaseHex() Tests ---

func TestEVMAddress_LowercaseHex(t *testing.T) {
	t.Run("returns all lowercase with 0x prefix", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		lh := addr.LowercaseHex()
		assert.True(t, strings.HasPrefix(lh, "0x"))
		assert.Equal(t, strings.ToLower(lh), lh, "must be entirely lowercase")
		assert.Len(t, lh, 42)
	})

	t.Run("lowercase matches Hex lowered", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		assert.Equal(t, strings.ToLower(addr.Hex()), addr.LowercaseHex())
	})
}

// --- EVMAddressFromHex Tests ---

func TestEVMAddressFromHex(t *testing.T) {
	t.Run("accepts EIP-55 checksummed address", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("accepts lowercase address", func(t *testing.T) {
		lowered := strings.ToLower(testEVMAddressHex)
		addr, err := EVMAddressFromHex(lowered)
		require.NoError(t, err)
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("accepts uppercase address", func(t *testing.T) {
		// Uppercase everything after 0x.
		upper := "0x" + strings.ToUpper(testEVMAddressHex[2:])
		addr, err := EVMAddressFromHex(upper)
		require.NoError(t, err)
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("accepts without 0x prefix", func(t *testing.T) {
		noPfx := testEVMAddressHex[2:] // strip "0x"
		addr, err := EVMAddressFromHex(noPfx)
		require.NoError(t, err)
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("accepts 0X uppercase prefix", func(t *testing.T) {
		upper := "0X" + testEVMAddressHex[2:]
		addr, err := EVMAddressFromHex(upper)
		require.NoError(t, err)
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("round-trips through Hex()", func(t *testing.T) {
		addr1, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		addr2, err := EVMAddressFromHex(addr1.Hex())
		require.NoError(t, err)

		assert.Equal(t, addr1.Bytes(), addr2.Bytes())
		assert.Equal(t, addr1.Hex(), addr2.Hex())
	})

	t.Run("round-trips through LowercaseHex()", func(t *testing.T) {
		addr1, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		addr2, err := EVMAddressFromHex(addr1.LowercaseHex())
		require.NoError(t, err)

		assert.Equal(t, addr1.Bytes(), addr2.Bytes())
	})
}

// --- EVMAddressFromHex Error Tests ---

func TestEVMAddressFromHex_InvalidHex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPos  string
		wantChar string
	}{
		{
			name:     "invalid char 'g' in address",
			input:    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb9226g",
			wantPos:  "position 41",
			wantChar: "'g'",
		},
		{
			name:     "invalid char 'z' at start without prefix",
			input:    "z39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
			wantPos:  "position 0",
			wantChar: "'z'",
		},
		{
			name:     "space character",
			input:    "0xf39F d6e51aad88F6F4ce6aB8827279cffFb92266",
			wantPos:  "position 6",
			wantChar: "' '",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EVMAddressFromHex(tt.input)
			require.Error(t, err)

			var keyErr *KeyError
			require.True(t, errors.As(err, &keyErr))
			assert.Equal(t, ErrInvalidHex, keyErr.Kind())
			assert.Contains(t, keyErr.Error(), tt.wantPos)
			assert.Contains(t, keyErr.Error(), tt.wantChar)
		})
	}
}

func TestEVMAddressFromHex_InvalidLength(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		_, err := EVMAddressFromHex("0xf39Fd6e51aad88")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "expected 40 hex characters")
	})

	t.Run("too long", func(t *testing.T) {
		_, err := EVMAddressFromHex("0x" + testEVMAddressHex[2:] + "ff")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "got 42")
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := EVMAddressFromHex("")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("only 0x prefix", func(t *testing.T) {
		_, err := EVMAddressFromHex("0x")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})
}

func TestEVMAddressFromHex_ZeroAddress(t *testing.T) {
	t.Run("all zeros rejected", func(t *testing.T) {
		_, err := EVMAddressFromHex("0x0000000000000000000000000000000000000000")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "all zeros")
	})

	t.Run("all zeros without prefix rejected", func(t *testing.T) {
		_, err := EVMAddressFromHex("0000000000000000000000000000000000000000")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
	})
}

// --- EVMAddressFromBytes Tests ---

func TestEVMAddressFromBytes(t *testing.T) {
	t.Run("valid 20 bytes", func(t *testing.T) {
		// Decode the known address to raw bytes.
		addrBytes := common.HexToAddress(testEVMAddressHex).Bytes()
		require.Len(t, addrBytes, EVMAddressLength)

		addr, err := EVMAddressFromBytes(addrBytes)
		require.NoError(t, err)
		assert.Equal(t, testEVMAddressHex, addr.Hex())
	})

	t.Run("round-trips with Bytes()", func(t *testing.T) {
		addr1, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		addr2, err := EVMAddressFromBytes(addr1.Bytes())
		require.NoError(t, err)

		assert.Equal(t, addr1.Hex(), addr2.Hex())
		assert.Equal(t, addr1.Bytes(), addr2.Bytes())
	})

	t.Run("wrong length too short", func(t *testing.T) {
		_, err := EVMAddressFromBytes([]byte{0x01, 0x02, 0x03})
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "expected 20 bytes, got 3")
	})

	t.Run("wrong length too long", func(t *testing.T) {
		raw := make([]byte, 21)
		raw[0] = 0x01
		_, err := EVMAddressFromBytes(raw)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Contains(t, keyErr.Error(), "got 21")
	})

	t.Run("nil slice rejected", func(t *testing.T) {
		_, err := EVMAddressFromBytes(nil)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
	})

	t.Run("zero bytes rejected", func(t *testing.T) {
		zeroes := make([]byte, EVMAddressLength)
		_, err := EVMAddressFromBytes(zeroes)
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrZeroValue, keyErr.Kind())
	})
}

// --- Bytes() Tests ---

func TestEVMAddress_Bytes(t *testing.T) {
	t.Run("returns copy of 20 bytes", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		b := addr.Bytes()
		assert.Len(t, b, EVMAddressLength)

		// Verify against go-ethereum's parsing.
		expected := common.HexToAddress(testEVMAddressHex).Bytes()
		assert.Equal(t, expected, b)
	})

	t.Run("modifying Bytes return value does not affect internal state", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		original := addr.Bytes()
		// Mutate the returned slice.
		for i := range original {
			original[i] = 0xff
		}

		// Internal state must be unchanged.
		assert.Equal(t, testEVMAddressHex, addr.Hex())
		assert.NotEqual(t, original, addr.Bytes())
	})

	t.Run("consecutive Bytes calls return independent copies", func(t *testing.T) {
		addr, err := EVMAddressFromHex(testEVMAddressHex)
		require.NoError(t, err)

		b1 := addr.Bytes()
		b2 := addr.Bytes()

		// Equal content but different backing arrays.
		assert.Equal(t, b1, b2)
		b1[0] = 0x00
		assert.NotEqual(t, b1, b2, "Bytes() must return independent copies")
	})
}

// --- Error Type Tests ---

func TestEVMAddress_ErrorsImplementKeyError(t *testing.T) {
	t.Run("errors.Is matches by kind for hex errors", func(t *testing.T) {
		_, err := EVMAddressFromHex("0xnot-valid-hex-at-all!!!!!!!!!!!!!!!!!!!!")
		require.Error(t, err)

		assert.True(t, errors.Is(err, NewKeyError(ErrInvalidHex, "", "")))
	})

	t.Run("errors.As extracts KeyError from FromBytes", func(t *testing.T) {
		_, err := EVMAddressFromBytes([]byte{0x01})
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Equal(t, "EVMAddressFromBytes", keyErr.Operation())
	})

	t.Run("errors.As extracts KeyError from FromHex", func(t *testing.T) {
		_, err := EVMAddressFromHex("0xabcd")
		require.Error(t, err)

		var keyErr *KeyError
		require.True(t, errors.As(err, &keyErr))
		assert.Equal(t, ErrInvalidLength, keyErr.Kind())
		assert.Equal(t, "EVMAddressFromHex", keyErr.Operation())
	})
}
