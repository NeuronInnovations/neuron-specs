package keylib

import (
	"github.com/mr-tron/base58"
)

// DIDKey derives a did:key identifier from the compressed secp256k1 public key.
//
// Derivation (W3C did:key method, secp256k1):
//  1. Get compressed public key bytes (33 bytes)
//  2. Prepend multicodec varint 0xe7 0x01 (secp256k1-pub) to get 35 bytes
//  3. Base58btc encode the result
//  4. Return "did:key:z" + encoded string
//
// The result has prefix "did:key:zQ3s" for secp256k1 keys.
//
// T011: DID:key derivation MUST be deterministic for the same public key.
func (k NeuronPublicKey) DIDKey() string {
	compressed := k.Bytes()

	// Prepend multicodec varint for secp256k1-pub (0xe7, 0x01).
	prefix := MulticodecSecp256k1Pub()
	multicodecPrefixed := make([]byte, len(prefix)+len(compressed))
	copy(multicodecPrefixed, prefix)
	copy(multicodecPrefixed[len(prefix):], compressed)

	// Base58btc encode the multicodec-prefixed bytes.
	encoded := base58.Encode(multicodecPrefixed)

	return DIDKeyPrefix + encoded
}
