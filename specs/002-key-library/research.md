# Research: Key Library

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `002-key-library` | **Date**: 2026-02-25 | **Source**: spec.md

---

## R1: Go secp256k1 / Ethereum Crypto Library

**Decision**: Use `github.com/ethereum/go-ethereum/crypto` (geth) for all secp256k1 operations.

**Rationale**: geth's `crypto` package provides production-grade secp256k1 key generation, ECDSA signing (RFC 6979 deterministic nonces), Keccak256 hashing, EVM address derivation, and signature recovery. It is the de facto standard in the Go/Ethereum ecosystem and directly implements the operations required by FR-005 (EVM derivation), FR-014 (R||S||V signing), FR-017 (Keccak256 + verification + recovery).

**Alternatives considered**:
- `github.com/btcsuite/btcd/btcec`: Lower-level secp256k1, no EVM address derivation built-in. Would require more glue code.
- `crypto/ecdsa` (stdlib): Uses generic elliptic curves; no Keccak256, no recovery ID, no deterministic nonces without additional libraries.

---

## R2: Go libp2p PeerID Library

**Decision**: Use `github.com/libp2p/go-libp2p/core/peer` and `github.com/libp2p/go-libp2p/core/crypto` for PeerID derivation.

**Rationale**: The canonical Go library for libp2p PeerID generation. Accepts secp256k1 public keys and produces standard PeerID (multihash-encoded) per FR-006. Used across the IPFS and libp2p ecosystem.

**Alternatives considered**:
- Manual multihash encoding: Error-prone, no ecosystem validation.
- `go-libp2p-crypto` (deprecated): Merged into `go-libp2p/core/crypto`.

---

## R3: DID:key Encoding (FR-006a)

**Decision**: Implement DID:key encoding manually using `github.com/mr-tron/base58` for base58btc encoding plus multicodec prefix bytes.

**Rationale**: The W3C did:key method for secp256k1 is straightforward: compressed pubkey (33 bytes) → prepend `0xe7 0x01` (secp256k1-pub multicodec varint) → base58btc encode → prefix with `did:key:z`. No full DID library needed; the encoding is ~10 lines of code. The `mr-tron/base58` library is widely used (dependency of go-libp2p).

**Alternatives considered**:
- `github.com/multiformats/go-multicodec`: Full multicodec table — overkill for two bytes.
- Full DID resolution library: Out of scope per spec (Key Library manages keys, not DID documents).

---

## R4: BIP-39 / BIP-44 Mnemonic Library

**Decision**: Use `github.com/tyler-smith/go-bip39` for mnemonic generation/validation and `github.com/tyler-smith/go-bip32` for HD key derivation (BIP-44 paths).

**Rationale**: Mature Go libraries implementing BIP-39 (mnemonic word list, entropy → mnemonic → seed) and BIP-32/44 (HD derivation paths). Default path m/44'/60'/0'/0/0 per FR-013.

**Alternatives considered**:
- `github.com/btcsuite/btcutil/hdkeychain`: Also mature but tightly coupled to btcutil ecosystem.
- Manual implementation: Not recommended for cryptographic key derivation.

---

## R5: Argon2id + AES-256-GCM (FR-015)

**Decision**: Use Go standard library: `golang.org/x/crypto/argon2` for Argon2id KDF and `crypto/aes` + `crypto/cipher` for AES-256-GCM.

**Rationale**: Go's `x/crypto` provides a production-grade Argon2id implementation. AES-GCM is in the standard library. No external dependencies needed for encryption. Version 1 uses hardcoded Argon2 defaults; version 2 stores custom parameters.

**Alternatives considered**:
- NaCl/secretbox: Uses XSalsa20-Poly1305, not AES-256-GCM as spec requires.
- `github.com/minio/sio`: Streaming encryption — overkill for 32-byte keys.

---

## R6: EIP-55 Checksum Implementation

**Decision**: Use `github.com/ethereum/go-ethereum/common` which provides `HexToAddress()` and `Address.Hex()` with EIP-55 checksumming built-in.

**Rationale**: geth's `common.Address` type already implements EIP-55 mixed-case checksum encoding in its `Hex()` method. No need to implement manually. This aligns with the FR-005 amendment.

**Alternatives considered**:
- Manual EIP-55: Keccak256(lowercase hex) → use nibbles to case-switch. Works but redundant when geth provides it.

---

## R7: Constant-Time Comparison (SEC-004)

**Decision**: Use Go standard library `crypto/subtle.ConstantTimeCompare` for all key material comparisons.

**Rationale**: SEC-004 requires constant-time comparisons for matching functions. Go's `crypto/subtle` package provides `ConstantTimeCompare` which is the standard approach. No external dependency needed.

**Alternatives considered**: None — `crypto/subtle` is the canonical Go solution.
