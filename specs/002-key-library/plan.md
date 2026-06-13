# Implementation Plan: Key Library

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `002-key-library` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/002-key-library/spec.md`

## Summary

Spec 002 defines a type-safe cryptographic key library built on secp256k1. The library provides NeuronPrivateKey and NeuronPublicKey as immutable, validated types with deterministic derivation to EVMAddress, PeerID, and DID:key. It includes ECDSA signing (Keccak256 + RFC 6979), signature recovery, BIP-39/44 mnemonic support, Argon2id/AES-256-GCM encryption, and a MultisigKey type for threshold signing interoperability. The reference implementation targets Go per Constitution Principle VI, using `go-ethereum/crypto` for secp256k1 operations and `go-libp2p` for PeerID derivation.

## Technical Context

**Language/Version**: Go 1.22+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**: `github.com/ethereum/go-ethereum` (secp256k1, Keccak256, EIP-55, ecrecover), `github.com/libp2p/go-libp2p` (PeerID derivation), `github.com/tyler-smith/go-bip39` (BIP-39 mnemonics), `github.com/tyler-smith/go-bip32` (BIP-44 derivation), `github.com/mr-tron/base58` (DID:key base58btc), `golang.org/x/crypto/argon2` (Argon2id KDF)
**Storage**: N/A — all types are in-memory. EncryptedPrivateKey is JSON-serializable for external storage.
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First). 10 success criteria (SC-001..SC-010). Deterministic signing verified per Principle X.
**Target Platform**: Linux/macOS/Windows (Go cross-compilation).
**Project Type**: Go module — `internal/keylib/` package within the neuron-go-sdk module.
**Performance Goals**: Key generation < 10ms. Signing < 1ms. Address derivation < 1ms. Encryption (Argon2id) depends on parameters (default v1: ~100ms).
**Constraints**: secp256k1 only (no Ed25519). MultisigKey "secp256k1-aggregated" algorithm deferred (GAP-005). Keys are immutable after construction (FR-022).
**Scale/Scope**: Single-key operations are the primary path. MultisigKey is for interoperability only.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md exists with mandatory sections (Purpose, User Scenarios, Requirements, Success Criteria) | **PASS** |
| II | Independently Testable Stories | Stories prioritized (P1/P2/P3/P4), Given/When/Then acceptance scenarios, each independently testable | **PASS** — 4 stories, 15 scenarios |
| III | Clarification Before Plan | No [NEEDS CLARIFICATION] markers; underspecified areas resolved before plan | **PASS** — 0 markers. GAP-005 (MultisigKey aggregation) explicitly deferred with no impact on core implementation |
| IV | High-Level Types | Semantic types used (NeuronPrivateKey, NeuronPublicKey, EVMAddress, PeerID, Signature, MultisigKey, etc.) | **PASS** |
| V | Traceability | FRs, SCs, Key Entities present and aligned | **PASS** — 24 FRs, 8 SECs, 3 OBS, 10 SCs, 7 entities, all cross-referenced |
| VI | Golang-First SDK | Plan targets Go 1.22+ with `internal/keylib/` layout, `*_test.go` colocated tests, `go test` tooling | **PASS** |
| VII | Strict Spec Compliance | Every task traces to FR-* or SC-* requirements; no silent deviations | **PASS** — 24 FRs + 8 SECs mapped to tasks |
| VIII | Hedera Transport Binding | Not directly applicable — Key Library is transport-agnostic. Blockchain SDK interop (FR-011) includes Hedera SDK keys | **PASS** (N/A) |
| IX | Test-First Development | Test tasks (Red phase) precede implementation tasks in each phase; 100% MUST-level coverage | **PASS** — `*_test.go` tasks before each implementation phase |
| X | Deterministic Signing | Signing tasks include determinism verification (sign twice, assert byte-equal R\|\|S\|\|V) | **PASS** — determinism test task included (FR-014, FR-017) |
| — | Related specs section | Present with internal (001, 003, 004) cross-references in key entities | **PASS** |
| — | Mermaid diagrams | Not present in spec — Key Library spec uses Type Hierarchy prose instead | **PASS** (N/A — type hierarchy is clear without diagrams) |
| — | Blockchain compatibility | secp256k1 is Ethereum-native; Hedera SDK interop via FR-011 | **PASS** |

**Gate result**: All gates PASS. Proceeding to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/002-key-library/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output — library selection decisions
├── data-model.md        # Phase 1 output — entity model
├── quickstart.md        # Phase 1 output — SDK integration guide
├── contracts/           # Phase 1 output — API contracts
│   ├── key-operations.md    # Core key operations contract
│   ├── encryption.md        # Encrypt/decrypt contract
│   └── multisig.md          # MultisigKey contract
└── checklists/
    └── requirements.md  # Requirements traceability checklist
```

### Source Code (Go module — Constitution VI)

```text
internal/keylib/
├── private_key.go         # NeuronPrivateKey type + factory methods + Sign()
├── private_key_test.go    # Construction, validation, signing, zeroize
├── public_key.go          # NeuronPublicKey type + Compressed/Uncompressed + derivations
├── public_key_test.go     # Construction, format conversions, derivations
├── evm_address.go         # EVMAddress type + EIP-55 checksum
├── evm_address_test.go    # Derivation, EIP-55, parsing
├── peer_id.go             # PeerID type + libp2p derivation
├── peer_id_test.go        # Derivation, string format
├── did_key.go             # DID:key generation (multicodec + base58btc)
├── did_key_test.go        # DID:key format, prefix validation
├── signature.go           # Signature type + R||S||V + Verify + RecoverPublicKey
├── signature_test.go      # Signing, verification, recovery, V format
├── encrypted_key.go       # EncryptedPrivateKey + Argon2id/AES-256-GCM
├── encrypted_key_test.go  # Encrypt/decrypt round-trip, version compat
├── mnemonic.go            # BIP-39/BIP-44 mnemonic + derivation
├── mnemonic_test.go       # Generation, restoration, custom paths
├── multisig_key.go        # MultisigKey type + protocol tracking
├── multisig_key_test.go   # Construction, protocol queries, error paths
├── errors.go              # Structured error types (11 error kinds)
├── errors_test.go         # Error kind validation
├── matching.go            # Type-safe matching functions (constant-time)
├── matching_test.go       # All matching variants, false negatives/positives
├── signing_test.go        # Deterministic signing verification (Constitution X)
└── integration_test.go    # Full key lifecycle: generate → derive → sign → verify → encrypt → restore
```

**Structure Decision**: Go package (`internal/keylib/`) within the neuron-go-sdk module. Tests colocated as `*_test.go` per Go conventions. `internal/` ensures the keylib package is not importable by external modules — only the SDK's public API surface in `pkg/` exposes key functionality. Each entity maps to its own file pair (source + test) for clear separation and parallel development.

## Complexity Tracking

No constitution violations requiring justification.
