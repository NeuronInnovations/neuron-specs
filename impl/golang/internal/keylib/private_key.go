package keylib

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// NeuronPrivateKey is the immutable, type-safe wrapper for a secp256k1 private key.
// Once constructed through a validated factory, the internal key material cannot be
// modified externally. The only mutation is Zeroize(), which securely erases the key.
//
// The compressed public key is derived eagerly at construction time and cached
// internally, making PublicKey() safe for concurrent use with no synchronization.
//
// FR-001: Private key MUST be exactly 32 bytes (secp256k1 scalar).
// FR-002: Hex import MUST accept with/without 0x prefix.
// SEC-003: Key material MUST NOT appear in error messages or logs.
type NeuronPrivateKey struct {
	data     [PrivateKeyLength]byte
	pubKey   NeuronPublicKey
	zeroized bool
}

// NeuronPrivateKeyFromHex constructs a NeuronPrivateKey from a hexadecimal string.
// It accepts input with or without the "0x" prefix.
//
// Validation order:
//  1. Strip optional 0x prefix
//  2. Validate hex characters (report exact position of first invalid char)
//  3. Validate length (must be exactly 64 hex characters = 32 bytes)
//  4. Validate non-zero (all-zero key is rejected)
//  5. Validate secp256k1 curve membership
func NeuronPrivateKeyFromHex(hexStr string) (NeuronPrivateKey, error) {
	const op = "NeuronPrivateKeyFromHex"

	// Strip optional 0x prefix, tracking offset for error positions.
	cleaned := hexStr
	prefixOffset := 0
	if strings.HasPrefix(cleaned, "0x") || strings.HasPrefix(cleaned, "0X") {
		cleaned = cleaned[2:]
		prefixOffset = 2
	}

	// Validate hex characters before checking length, reporting exact position.
	for i, c := range cleaned {
		if !isHexChar(c) {
			return NeuronPrivateKey{}, NewKeyError(
				ErrInvalidHex, op,
				fmt.Sprintf("invalid hex character '%c' at position %d", c, i+prefixOffset),
			)
		}
	}

	// Validate length: must be exactly 64 hex chars (32 bytes).
	if len(cleaned) != PrivateKeyLength*2 {
		return NeuronPrivateKey{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d hex characters, got %d", PrivateKeyLength*2, len(cleaned)),
		)
	}

	// Decode hex to bytes.
	raw, err := hex.DecodeString(cleaned)
	if err != nil {
		// Should not happen after our character validation, but handle defensively.
		return NeuronPrivateKey{}, NewSDKError(op, err)
	}

	return neuronPrivateKeyFromValidatedBytes(raw, op)
}

// NeuronPrivateKeyFromBytes constructs a NeuronPrivateKey from raw 32 bytes.
//
// Validation order:
//  1. Validate length (must be exactly 32 bytes)
//  2. Validate non-zero
//  3. Validate secp256k1 curve membership
func NeuronPrivateKeyFromBytes(b []byte) (NeuronPrivateKey, error) {
	const op = "NeuronPrivateKeyFromBytes"

	if len(b) != PrivateKeyLength {
		return NeuronPrivateKey{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d bytes, got %d", PrivateKeyLength, len(b)),
		)
	}

	return neuronPrivateKeyFromValidatedBytes(b, op)
}

// NeuronPrivateKeyFromBlockchainKey constructs a NeuronPrivateKey from a go-ethereum
// *ecdsa.PrivateKey. The key's curve MUST be secp256k1; any other curve (e.g. P-256,
// Ed25519 via a mistyped interface) is rejected with ErrUnsupportedKeyType.
func NeuronPrivateKeyFromBlockchainKey(key *ecdsa.PrivateKey) (NeuronPrivateKey, error) {
	const op = "NeuronPrivateKeyFromBlockchainKey"

	if key == nil {
		return NeuronPrivateKey{}, NewKeyError(
			ErrInvalidKey, op,
			"private key is nil",
		)
	}

	// Verify the curve is secp256k1 (S256).
	curveName := key.Curve.Params().Name
	s256Name := crypto.S256().Params().Name
	if curveName != s256Name {
		return NeuronPrivateKey{}, NewKeyError(
			ErrUnsupportedKeyType, op,
			fmt.Sprintf("expected secp256k1 curve, got %s", curveName),
		)
	}

	// Extract the 32-byte scalar from the ECDSA key.
	raw := crypto.FromECDSA(key)
	return neuronPrivateKeyFromValidatedBytes(raw, op)
}

// neuronPrivateKeyFromValidatedBytes is the internal constructor that performs
// zero-value and curve-membership checks on already length-validated bytes,
// then eagerly derives the compressed public key.
func neuronPrivateKeyFromValidatedBytes(raw []byte, op string) (NeuronPrivateKey, error) {
	// Check for all-zero key.
	if isZeroBytes(raw) {
		return NeuronPrivateKey{}, NewKeyError(
			ErrZeroValue, op,
			"private key is all zeros",
		)
	}

	// Validate secp256k1 curve membership via go-ethereum's ToECDSA,
	// which checks that the scalar is in [1, N-1].
	ecdsaKey, err := crypto.ToECDSA(raw)
	if err != nil {
		return NeuronPrivateKey{}, NewKeyError(
			ErrInvalidKey, op,
			"key fails secp256k1 curve validation",
		)
	}

	// Eagerly derive the compressed public key at construction time.
	// This makes PublicKey() inherently thread-safe with no synchronization needed.
	compressed := crypto.CompressPubkey(&ecdsaKey.PublicKey)
	var pubData [CompressedPublicKeyLength]byte
	copy(pubData[:], compressed)

	var key NeuronPrivateKey
	copy(key.data[:], raw)
	key.pubKey = NeuronPublicKey{data: pubData}
	return key, nil
}

// PublicKey returns the compressed secp256k1 public key derived from this private key.
// The public key is computed eagerly at construction time and cached; this method
// is safe for concurrent use without synchronization.
func (k *NeuronPrivateKey) PublicKey() NeuronPublicKey {
	return k.pubKey
}

// Bytes returns a COPY of the raw 32-byte private key material.
// Modifying the returned slice does not affect the internal state.
func (k *NeuronPrivateKey) Bytes() []byte {
	out := make([]byte, PrivateKeyLength)
	copy(out, k.data[:])
	return out
}

// Hex returns the private key as a lowercase hex string with "0x" prefix.
func (k *NeuronPrivateKey) Hex() string {
	return "0x" + hex.EncodeToString(k.data[:])
}

// IsZero returns true if the key has been zeroized via Zeroize().
func (k *NeuronPrivateKey) IsZero() bool {
	return k.zeroized
}

// Zeroize securely erases the private key material from memory and marks
// the key as unusable. After zeroization, ToBlockchainKey() will return an error
// and Bytes()/Hex() will return zeroed data.
func (k *NeuronPrivateKey) Zeroize() {
	for i := range k.data {
		k.data[i] = 0
	}
	runtime.KeepAlive(&k.data)
	k.zeroized = true
}

// ToBlockchainKey converts the NeuronPrivateKey back to a go-ethereum *ecdsa.PrivateKey.
// Returns an error if the key has been zeroized.
func (k *NeuronPrivateKey) ToBlockchainKey() (*ecdsa.PrivateKey, error) {
	const op = "ToBlockchainKey"

	if k.zeroized {
		return nil, NewKeyError(
			ErrZeroValue, op,
			"key has been zeroized",
		)
	}

	ecdsaKey, err := crypto.ToECDSA(k.data[:])
	if err != nil {
		return nil, NewSDKError(op, err)
	}
	return ecdsaKey, nil
}

// isHexChar reports whether r is a valid hexadecimal character.
func isHexChar(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// isZeroBytes reports whether all bytes in b are zero.
func isZeroBytes(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// Sign produces a deterministic ECDSA signature over the given message using
// this private key.
//
// Implementation:
//  1. Check the key has not been zeroized.
//  2. Keccak256-hash the message.
//  3. Convert to *ecdsa.PrivateKey.
//  4. Sign with crypto.Sign (uses RFC 6979 deterministic nonce).
//  5. Parse the 65-byte result into a Signature.
//
// FR-013: Sign MUST use Keccak256 hashing and RFC 6979 deterministic nonces.
func (k *NeuronPrivateKey) Sign(message []byte) (Signature, error) {
	const op = "Sign"

	if k.zeroized {
		return Signature{}, NewKeyError(
			ErrZeroValue, op,
			"key has been zeroized",
		)
	}

	hash := crypto.Keccak256(message)

	ecdsaKey, err := crypto.ToECDSA(k.data[:])
	if err != nil {
		return Signature{}, NewSDKError(op, err)
	}

	sigBytes, err := crypto.Sign(hash, ecdsaKey)
	if err != nil {
		return Signature{}, NewSDKError(op, err)
	}

	// crypto.Sign returns 65-byte R||S||V where V is 0 or 1.
	return SignatureFromBytes(sigBytes)
}

// NewNeuronPrivateKey generates a new random secp256k1 private key using
// cryptographically secure randomness from crypto/rand.
//
// The function reads 32 bytes from crypto/rand and validates that the result
// is a valid secp256k1 scalar (non-zero, in [1, N-1]). If the random bytes
// happen to fall outside the valid range, it retries (astronomically unlikely,
// but handled for correctness).
//
// FR-001: Private key MUST be exactly 32 bytes (secp256k1 scalar).
func NewNeuronPrivateKey() (NeuronPrivateKey, error) {
	const op = "NewNeuronPrivateKey"
	const maxRetries = 16 // astronomically unlikely to need even 2

	for range maxRetries {
		raw := make([]byte, PrivateKeyLength)
		if _, err := rand.Read(raw); err != nil {
			return NeuronPrivateKey{}, NewSDKError(op, fmt.Errorf("crypto/rand: %w", err))
		}

		key, err := neuronPrivateKeyFromValidatedBytes(raw, op)
		if err != nil {
			// Zero or out-of-range scalar; retry.
			continue
		}
		return key, nil
	}

	return NeuronPrivateKey{}, NewKeyError(
		ErrInvalidKey, op,
		"failed to generate valid key after maximum retries",
	)
}
