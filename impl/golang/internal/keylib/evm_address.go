package keylib

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// EVMAddress is the immutable, type-safe wrapper for a 20-byte Ethereum address.
// It stores the raw address bytes internally and provides both EIP-55 checksummed
// and lowercase hex output.
//
// FR-009: EVMAddress MUST be derived from Keccak256(uncompressed_pubkey[1:65]),
// taking the last 20 bytes.
type EVMAddress struct {
	addr [EVMAddressLength]byte
}

// EVMAddressFromHex constructs an EVMAddress from a hexadecimal string.
// It accepts lowercase or EIP-55 mixed-case checksummed addresses,
// with or without the "0x" prefix.
//
// Validation order:
//  1. Strip optional 0x prefix
//  2. Validate hex characters (report exact position of first invalid char)
//  3. Validate length (must be exactly 40 hex characters = 20 bytes)
//  4. Validate non-zero (all-zero address is rejected)
func EVMAddressFromHex(hexStr string) (EVMAddress, error) {
	const op = "EVMAddressFromHex"

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
			return EVMAddress{}, NewKeyError(
				ErrInvalidHex, op,
				fmt.Sprintf("invalid hex character '%c' at position %d", c, i+prefixOffset),
			)
		}
	}

	// Validate length: must be exactly 40 hex chars (20 bytes).
	if len(cleaned) != EVMAddressLength*2 {
		return EVMAddress{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d hex characters, got %d", EVMAddressLength*2, len(cleaned)),
		)
	}

	// Decode hex to bytes.
	raw, err := hex.DecodeString(cleaned)
	if err != nil {
		// Should not happen after our character validation, but handle defensively.
		return EVMAddress{}, NewSDKError(op, err)
	}

	// Check for all-zero address.
	if isZeroBytes(raw) {
		return EVMAddress{}, NewKeyError(
			ErrZeroValue, op,
			"address is all zeros",
		)
	}

	// Use go-ethereum's common.HexToAddress for canonical parsing.
	addr := common.HexToAddress(hexStr)
	var evmAddr EVMAddress
	copy(evmAddr.addr[:], addr.Bytes())
	return evmAddr, nil
}

// EVMAddressFromBytes constructs an EVMAddress from raw 20 bytes.
//
// Validation order:
//  1. Validate length (must be exactly 20 bytes)
//  2. Validate non-zero
func EVMAddressFromBytes(b []byte) (EVMAddress, error) {
	const op = "EVMAddressFromBytes"

	if len(b) != EVMAddressLength {
		return EVMAddress{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d bytes, got %d", EVMAddressLength, len(b)),
		)
	}

	if isZeroBytes(b) {
		return EVMAddress{}, NewKeyError(
			ErrZeroValue, op,
			"address is all zeros",
		)
	}

	var addr EVMAddress
	copy(addr.addr[:], b)
	return addr, nil
}

// Hex returns the EIP-55 mixed-case checksummed address with "0x" prefix.
// This is the canonical human-readable representation.
func (a EVMAddress) Hex() string {
	addr := common.BytesToAddress(a.addr[:])
	return addr.Hex()
}

// LowercaseHex returns the address as a lowercase hex string with "0x" prefix.
func (a EVMAddress) LowercaseHex() string {
	return "0x" + hex.EncodeToString(a.addr[:])
}

// Bytes returns a COPY of the raw 20-byte address.
// Modifying the returned slice does not affect the internal state.
func (a EVMAddress) Bytes() []byte {
	out := make([]byte, EVMAddressLength)
	copy(out, a.addr[:])
	return out
}

// EVMAddress derives the Ethereum address from this public key.
//
// Derivation:
//  1. Decompress the 33-byte compressed public key to 65-byte uncompressed form
//  2. Take bytes [1:65] (skip the 0x04 uncompressed prefix)
//  3. Keccak256 hash the 64-byte public key coordinates
//  4. Take the last 20 bytes of the hash as the address
//
// This follows the standard Ethereum address derivation per the Yellow Paper.
// EVMAddress derives the Ethereum address from this public key.
//
// Invariant: The error path (returning zero EVMAddress) is unreachable for validly
// constructed NeuronPublicKey instances — all constructors validate curve point
// membership. The zero-address return is retained as a defensive guard.
func (k NeuronPublicKey) EVMAddress() EVMAddress {
	uncompressed, err := crypto.DecompressPubkey(k.data[:])
	if err != nil {
		return EVMAddress{}
	}

	// Marshal the uncompressed public key to get the 65-byte representation.
	// crypto.FromECDSAPub returns [0x04 || X(32) || Y(32)].
	uncompressedBytes := crypto.FromECDSAPub(uncompressed)

	// Keccak256 hash of the 64-byte public key (skip the 0x04 prefix byte).
	hash := crypto.Keccak256(uncompressedBytes[1:])

	// Take the last 20 bytes.
	var addr EVMAddress
	copy(addr.addr[:], hash[len(hash)-EVMAddressLength:])
	return addr
}
