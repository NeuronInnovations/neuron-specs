package keylib

import (
	"crypto/subtle"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// Signature is the immutable, type-safe wrapper for a secp256k1 ECDSA signature
// in R(32) || S(32) || V(1) canonical format (65 bytes total).
//
// The V value follows go-ethereum convention: 0 or 1 for the recovery identifier.
// EthereumV() returns the legacy Ethereum format (V + 27).
//
// FR-012: Signature MUST be exactly 65 bytes (R||S||V).
// FR-013: Sign MUST use RFC 6979 deterministic nonces via go-ethereum.
type Signature struct {
	r [32]byte
	s [32]byte
	v byte
}

// SignatureFromBytes constructs a Signature from a 65-byte R(32)||S(32)||V(1) slice.
// The input MUST be exactly 65 bytes; any other length is rejected.
func SignatureFromBytes(b []byte) (Signature, error) {
	const op = "SignatureFromBytes"

	if len(b) != SignatureLength {
		return Signature{}, NewKeyError(
			ErrInvalidLength, op,
			fmt.Sprintf("expected %d bytes, got %d", SignatureLength, len(b)),
		)
	}

	v := b[64]
	if v != 0 && v != 1 && v != 27 && v != 28 {
		return Signature{}, NewKeyError(
			ErrInvalidFormat, op,
			fmt.Sprintf("invalid recovery identifier V=%d, must be 0, 1, 27, or 28", v),
		)
	}

	// FR-A10a: Normalize V to canonical form {0, 1}.
	// Accept 27/28 from Ethereum legacy sources, store as 0/1.
	if v >= 27 {
		v -= 27
	}

	var sig Signature
	copy(sig.r[:], b[0:32])
	copy(sig.s[:], b[32:64])
	sig.v = v
	return sig, nil
}

// Bytes returns the 65-byte R(32)||S(32)||V(1) representation of the signature.
// The returned slice is a fresh copy; modifying it does not affect the Signature.
func (sig *Signature) Bytes() []byte {
	out := make([]byte, SignatureLength)
	copy(out[0:32], sig.r[:])
	copy(out[32:64], sig.s[:])
	out[64] = sig.v
	return out
}

// R returns a copy of the 32-byte R component.
func (sig *Signature) R() []byte {
	out := make([]byte, 32)
	copy(out, sig.r[:])
	return out
}

// S returns a copy of the 32-byte S component.
func (sig *Signature) S() []byte {
	out := make([]byte, 32)
	copy(out, sig.s[:])
	return out
}

// V returns the recovery identifier in canonical form (0 or 1).
// Per FR-A10a, V is normalized to {0, 1} at construction time.
func (sig *Signature) V() byte {
	return sig.v
}

// StandardV returns the recovery identifier normalized to 0 or 1.
// Since FR-A10a normalizes at construction, this is identical to V().
// Retained for backward compatibility.
func (sig *Signature) StandardV() byte {
	return sig.v
}

// EthereumV returns the recovery identifier in Ethereum legacy format (V + 27).
// Since V is always stored as 0 or 1 per FR-A10a, this returns 27 or 28.
func (sig *Signature) EthereumV() byte {
	return sig.v + 27
}

// Verify checks that this signature was produced by the holder of the given public key
// over the given message.
//
// Implementation:
//  1. Keccak256-hash the message.
//  2. Recover the signer's uncompressed public key from the hash and signature.
//  3. Constant-time compare the recovered key against the provided pubkey's
//     uncompressed form.
//
// Returns true if the recovered key matches, false otherwise (including on any
// internal error).
func (sig *Signature) Verify(message []byte, pubkey NeuronPublicKey) bool {
	hash := crypto.Keccak256(message)

	// Build the 65-byte sig with standard V (0 or 1) for Ecrecover.
	sigBytes := sig.bytesWithStandardV()

	recovered, err := crypto.Ecrecover(hash, sigBytes)
	if err != nil {
		return false
	}

	// The recovered key is 65-byte uncompressed (0x04 || X || Y).
	// Compare against the uncompressed form of the provided public key.
	uncompressed := uncompressPublicKey(pubkey)
	if uncompressed == nil {
		return false
	}

	return subtle.ConstantTimeCompare(recovered, uncompressed) == 1
}

// RecoverPublicKey recovers the signer's NeuronPublicKey from the message
// that was signed.
//
// Implementation:
//  1. Keccak256-hash the message.
//  2. Recover the 65-byte uncompressed public key via crypto.Ecrecover.
//  3. Compress and wrap as NeuronPublicKey.
func (sig *Signature) RecoverPublicKey(message []byte) (NeuronPublicKey, error) {
	const op = "RecoverPublicKey"

	hash := crypto.Keccak256(message)

	// Build the 65-byte sig with standard V (0 or 1) for Ecrecover.
	sigBytes := sig.bytesWithStandardV()

	recovered, err := crypto.Ecrecover(hash, sigBytes)
	if err != nil {
		return NeuronPublicKey{}, NewSDKError(op, err)
	}

	// recovered is 65-byte uncompressed key (0x04 prefix).
	// Unmarshal to *ecdsa.PublicKey, then compress.
	ecPub, err := crypto.UnmarshalPubkey(recovered)
	if err != nil {
		return NeuronPublicKey{}, NewSDKError(op, err)
	}

	compressed := crypto.CompressPubkey(ecPub)

	var pubData [CompressedPublicKeyLength]byte
	copy(pubData[:], compressed)
	return NeuronPublicKey{data: pubData}, nil
}

// bytesWithStandardV returns the 65-byte signature with V normalized to 0 or 1.
// crypto.Ecrecover requires V in {0, 1}, not Ethereum legacy {27, 28}.
func (sig *Signature) bytesWithStandardV() []byte {
	out := make([]byte, SignatureLength)
	copy(out[0:32], sig.r[:])
	copy(out[32:64], sig.s[:])
	out[64] = sig.StandardV()
	return out
}

// uncompressPublicKey decompresses a NeuronPublicKey to 65-byte uncompressed form.
// Returns nil if decompression fails.
func uncompressPublicKey(pubkey NeuronPublicKey) []byte {
	ecPub, err := crypto.DecompressPubkey(pubkey.data[:])
	if err != nil {
		return nil
	}
	return crypto.FromECDSAPub(ecPub)
}
