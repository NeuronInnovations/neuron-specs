package keylib

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// NeuronPublicKey is the immutable, type-safe wrapper for a compressed secp256k1 public key.
// Once constructed through a validated factory, the internal key material cannot be modified
// externally. All accessor methods return defensive copies.
//
// The key is stored internally in 33-byte compressed form (0x02/0x03 prefix + 32-byte X coordinate).
// Uncompressed form (65 bytes: 0x04 prefix + 64-byte X||Y) is derived on demand via decompression.
//
// FR-004: Public key MUST be stored in compressed form (33 bytes).
// FR-005: Conversion between compressed and uncompressed forms MUST be supported.
// T008: All construction paths MUST validate curve point membership.
type NeuronPublicKey struct {
	data [CompressedPublicKeyLength]byte
}

// NeuronPublicKeyFromHex constructs a NeuronPublicKey from a hexadecimal string.
// It accepts compressed (33 bytes = 66 hex chars) or uncompressed (65 bytes = 130 hex chars)
// representations, with or without the "0x" prefix. If the input is uncompressed, it is
// compressed internally before storage.
//
// Validation order:
//  1. Strip optional 0x/0X prefix
//  2. Validate hex characters (report exact position of first invalid char)
//  3. Validate length (must be 66 or 130 hex characters)
//  4. Decode hex to bytes
//  5. Validate secp256k1 curve point membership
//  6. If uncompressed, compress to 33-byte form
func NeuronPublicKeyFromHex(hexStr string) (NeuronPublicKey, error) {
	const op = "NeuronPublicKeyFromHex"

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
			return NeuronPublicKey{}, NewKeyError(
				ErrInvalidHex, op,
				fmt.Sprintf("invalid hex character '%c' at position %d", c, i+prefixOffset),
			)
		}
	}

	// Validate length: must be 66 hex chars (33 bytes compressed) or 130 hex chars (65 bytes uncompressed).
	expectedCompressed := CompressedPublicKeyLength * 2  // 66
	expectedUncompressed := UncompressedPublicKeyLen * 2 // 130
	if len(cleaned) != expectedCompressed && len(cleaned) != expectedUncompressed {
		return NeuronPublicKey{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d or %d hex characters, got %d",
				expectedCompressed, expectedUncompressed, len(cleaned)),
		)
	}

	// Decode hex to bytes.
	raw, err := hex.DecodeString(cleaned)
	if err != nil {
		// Should not happen after our character validation, but handle defensively.
		return NeuronPublicKey{}, NewSDKError(op, err)
	}

	return neuronPublicKeyFromValidatedBytes(raw, op)
}

// NeuronPublicKeyFromBytes constructs a NeuronPublicKey from raw bytes.
// Accepts 33 bytes (compressed) or 65 bytes (uncompressed). If the input is 65 bytes,
// it is compressed internally before storage.
//
// Validation order:
//  1. Validate length (must be 33 or 65 bytes)
//  2. Validate secp256k1 curve point membership
//  3. If uncompressed, compress to 33-byte form
func NeuronPublicKeyFromBytes(b []byte) (NeuronPublicKey, error) {
	const op = "NeuronPublicKeyFromBytes"

	if len(b) != CompressedPublicKeyLength && len(b) != UncompressedPublicKeyLen {
		return NeuronPublicKey{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d or %d bytes, got %d",
				CompressedPublicKeyLength, UncompressedPublicKeyLen, len(b)),
		)
	}

	return neuronPublicKeyFromValidatedBytes(b, op)
}

// NeuronPublicKeyFromBlockchainKey constructs a NeuronPublicKey from a go-ethereum
// *ecdsa.PublicKey. The key's curve MUST be secp256k1; any other curve is rejected
// with ErrUnsupportedKeyType.
func NeuronPublicKeyFromBlockchainKey(key *ecdsa.PublicKey) (NeuronPublicKey, error) {
	const op = "NeuronPublicKeyFromBlockchainKey"

	if key == nil {
		return NeuronPublicKey{}, NewKeyError(
			ErrInvalidKey, op,
			"public key is nil",
		)
	}

	// Verify the curve is secp256k1 (S256).
	curveName := key.Curve.Params().Name
	s256Name := crypto.S256().Params().Name
	if curveName != s256Name {
		return NeuronPublicKey{}, NewKeyError(
			ErrUnsupportedKeyType, op,
			fmt.Sprintf("expected secp256k1 curve, got %s", curveName),
		)
	}

	// Compress the public key.
	compressed := crypto.CompressPubkey(key)
	var pubData [CompressedPublicKeyLength]byte
	copy(pubData[:], compressed)
	return NeuronPublicKey{data: pubData}, nil
}

// neuronPublicKeyFromValidatedBytes is the internal constructor that validates a curve
// point from already length-validated bytes, optionally compressing uncompressed input.
func neuronPublicKeyFromValidatedBytes(raw []byte, op string) (NeuronPublicKey, error) {
	var compressed [CompressedPublicKeyLength]byte

	switch len(raw) {
	case CompressedPublicKeyLength:
		// Validate the compressed key is a valid secp256k1 point by decompressing it.
		ecdsaPub, err := crypto.DecompressPubkey(raw)
		if err != nil {
			return NeuronPublicKey{}, NewKeyError(
				ErrInvalidKey, op,
				"compressed key is not a valid secp256k1 curve point",
			)
		}

		// Re-compress to ensure canonical form.
		copy(compressed[:], crypto.CompressPubkey(ecdsaPub))

	case UncompressedPublicKeyLen:
		// Validate the uncompressed key is a valid secp256k1 point.
		ecdsaPub, err := crypto.UnmarshalPubkey(raw)
		if err != nil {
			return NeuronPublicKey{}, NewKeyError(
				ErrInvalidKey, op,
				"uncompressed key is not a valid secp256k1 curve point",
			)
		}

		// Compress the validated uncompressed key.
		copy(compressed[:], crypto.CompressPubkey(ecdsaPub))

	default:
		// This should not be reached given callers validate length, but handle defensively.
		return NeuronPublicKey{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d or %d bytes, got %d",
				CompressedPublicKeyLength, UncompressedPublicKeyLen, len(raw)),
		)
	}

	return NeuronPublicKey{data: compressed}, nil
}

// Compressed returns a COPY of the 33-byte compressed public key (0x02/0x03 prefix + 32-byte X).
func (k NeuronPublicKey) Compressed() []byte {
	out := make([]byte, CompressedPublicKeyLength)
	copy(out, k.data[:])
	return out
}

// Uncompressed returns the 65-byte uncompressed public key (0x04 prefix + 32-byte X + 32-byte Y).
// The uncompressed form is derived on demand by decompressing the stored compressed key.
//
// Invariant: This function returns nil only if the internal compressed key data is invalid.
// All NeuronPublicKey constructors (NeuronPublicKeyFromBytes, NeuronPublicKeyFromHex,
// NeuronPrivateKey.PublicKey) validate curve point membership at construction time,
// so the error path is unreachable for any validly constructed instance. The nil return
// is retained as a defensive guard rather than panicking.
func (k NeuronPublicKey) Uncompressed() []byte {
	ecdsaPub, err := crypto.DecompressPubkey(k.data[:])
	if err != nil {
		return nil
	}

	return crypto.FromECDSAPub(ecdsaPub)
}

// Bytes returns a COPY of the raw 33-byte compressed public key.
// This is an alias for Compressed().
func (k NeuronPublicKey) Bytes() []byte {
	return k.Compressed()
}

// Hex returns the compressed public key as a lowercase hex string with "0x" prefix.
func (k NeuronPublicKey) Hex() string {
	return "0x" + hex.EncodeToString(k.data[:])
}

// ToBlockchainKey converts the NeuronPublicKey to a go-ethereum *ecdsa.PublicKey.
func (k NeuronPublicKey) ToBlockchainKey() (*ecdsa.PublicKey, error) {
	const op = "NeuronPublicKey.ToBlockchainKey"

	ecdsaPub, err := crypto.DecompressPubkey(k.data[:])
	if err != nil {
		return nil, NewKeyError(
			ErrInvalidKey, op,
			"failed to decompress public key to blockchain key",
		)
	}

	return ecdsaPub, nil
}
