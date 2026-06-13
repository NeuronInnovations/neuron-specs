// Package keylib provides a type-safe cryptographic key library built on the secp256k1 curve.
//
// keylib is the foundational cryptographic module for the Neuron SDK. It provides
// immutable, validated key types with deterministic derivation chains:
//
//   NeuronPrivateKey → NeuronPublicKey → EVMAddress
//                                      → PeerID
//                                      → DID:key
//
// # Design Principles
//
//   - secp256k1 only: All operations use the secp256k1 elliptic curve.
//     Ed25519 and other key types are explicitly rejected with descriptive errors.
//   - Adapter pattern: keylib wraps battle-tested libraries (go-ethereum/crypto,
//     go-libp2p/core) rather than reimplementing cryptographic primitives.
//   - Immutability: All key types are immutable after construction. No setters or
//     mutators exist. Keys are valid by construction — invalid inputs are rejected
//     at construction time with structured errors.
//   - Type safety: Strong types (NeuronPrivateKey, NeuronPublicKey, EVMAddress,
//     PeerID, Signature) prevent type confusion and make APIs self-documenting.
//   - Deterministic derivation: The same private key always produces the same
//     public key, EVMAddress, PeerID, and DID:key.
//   - Deterministic signing: RFC 6979 nonces ensure the same (key, message) pair
//     always produces the same signature (R||S||V).
//
// # Key Operations
//
// Key generation:
//
//	key, err := keylib.NewNeuronPrivateKey()
//
// Key restoration:
//
//	key, err := keylib.NeuronPrivateKeyFromHex("0x...")
//	key, err := keylib.NeuronPrivateKeyFromMnemonic(mnemonic, "")
//
// Derivation chain:
//
//	pubkey := key.PublicKey()
//	addr   := pubkey.EVMAddress()
//	peerID := pubkey.PeerID()
//	did    := pubkey.DIDKey()
//
// Signing and verification:
//
//	sig    := key.Sign(message)
//	ok     := sig.Verify(message, pubkey)
//	recovered, err := sig.RecoverPublicKey(message)
//
// Key encryption:
//
//	encrypted, err := keylib.Encrypt(key, password)
//	restored, err  := keylib.Decrypt(encrypted, password)
//
// Blockchain interoperability:
//
//	ecdsaKey := key.ToBlockchainKey()
//	key, err := keylib.NeuronPrivateKeyFromBlockchainKey(ecdsaKey)
package keylib
