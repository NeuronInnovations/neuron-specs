# Implementation Plan: Key Library (TypeScript)

**Branch**: `007-identity-contract-2` | **Date**: 2026-03-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/002-key-library/spec.md`
**Language**: TypeScript (subsequent implementation per Constitution VI)
**Derivation Source**: Language-neutral artifacts ONLY (spec.md, data-model.md, contracts/, 006 contracts)

> **Non-negotiable**: This plan derives from specs, NOT from `impl/golang/`. Do not port, mirror, or reverse-engineer Go logic.

## Summary

Spec 002 defines a type-safe cryptographic key library built on secp256k1. The TypeScript implementation provides NeuronPrivateKey and NeuronPublicKey as immutable, validated types with deterministic derivation to EVMAddress, PeerID, and DID:key. It includes ECDSA signing (Keccak256 + RFC 6979), signature recovery, BIP-39/44 mnemonic support, Argon2id/AES-256-GCM encryption, and a MultisigKey type for threshold signing interoperability. Conformance is validated against Spec 006 golden test vectors (Chain 1: key derivation, Chain 4: key encryption).

## Technical Context

**Language/Version**: TypeScript 5.7+ targeting ES2022 (native BigInt support)
**Runtime**: Node.js ≥ 18.0.0 (required for `crypto.timingSafeEqual`, `crypto.subtle`)
**Primary Dependencies**:
- `@noble/secp256k1` v2 — secp256k1 curve operations, RFC 6979 deterministic signing, low-S normalization, recovery ID
- `@noble/hashes` — `keccak_256` (Ethereum-compatible Keccak256, NOT NIST SHA-3)
- `@scure/bip39` — BIP-39 mnemonic generation and validation
- `@scure/bip32` — BIP-44 HD key derivation
- `bs58` v6 — Base58btc encoding for PeerID and DID:key
- `hash-wasm` — Argon2id KDF (cross-platform WASM)
- Node.js `crypto` — AES-256-GCM encryption/decryption, `timingSafeEqual`
**Storage**: N/A — all types are in-memory. EncryptedPrivateKey is JSON-serializable for external storage.
**Testing**: `vitest` with assertion-based testing (Constitution Principle IX: Test-First)
**Target Platform**: Node.js 18+ (browser support deferred — Argon2id WASM and crypto.subtle differences)
**Project Type**: TypeScript package — `impl/typescript/src/keylib/` within the neuron-ts-sdk
**Constraints**: secp256k1 only (no Ed25519). MultisigKey aggregation deferred (GAP-005). Keys immutable after construction (FR-022).

### TypeScript-Specific Adaptations

| Spec Requirement | Go Behavior | TypeScript Adaptation | Rationale |
|-----------------|-------------|----------------------|-----------|
| FR-021 (Zeroize) | `runtime.Zeroize([]byte)` | `Uint8Array.fill(0)` + mark unusable | JS GC prevents deterministic memory clearing of strings/BigInt. Key material stored ONLY in Uint8Array. |
| FR-022 (Thread safety) | Goroutine-safe via immutable struct | Trivially satisfied — JS single-threaded | No Worker support in v1. |
| FR-011 (Blockchain key interop) | `*ecdsa.PrivateKey` ↔ NeuronPrivateKey | `Uint8Array` (raw bytes) ↔ NeuronPrivateKey | No single "blockchain SDK key type" in TS. Raw bytes are the universal interchange format. ethers.js SigningKey conversion optional. |
| FR-008 (Error types) | Go error interface with `KeyError` struct | `KeyError extends NeuronError` with `code`/`name` | Per 006 error-taxonomy.md: 14 NEURON-KEY-* codes. TypeScript extends Error class. |
| SEC-004 (Constant-time) | `crypto/subtle.ConstantTimeCompare` | `crypto.timingSafeEqual` (Node.js) | Browser polyfill via bitwise OR accumulation if needed later. |
| FR-W02 (UnsignedInt64) | Native uint64 | `bigint` with JSON string encoding | Per 006 wire-format.md §3. JSON.stringify cannot handle bigint natively. |
| C-2 (UTF-8) | Native UTF-8 strings | `TextEncoder().encode()` before all hashing | JS uses UTF-16 internally. 006 wire-format.md §9 mandates explicit UTF-8 conversion. |

### Spec Gap: FR-004 Ed25519 Detection in TypeScript

006 algorithm-reference.md §14 describes Ed25519 detection via OID, protobuf KeyType, or DER encoding. In TypeScript, raw bytes are ambiguous (32 bytes could be Ed25519 public key OR secp256k1 private key). The implementation MUST:
1. Accept typed input where possible (e.g., objects with curve metadata)
2. For raw 33-byte input: validate prefix byte (0x02/0x03 = secp256k1 compressed, else reject)
3. For raw 65-byte input: validate prefix byte (0x04 = secp256k1 uncompressed, else reject)
4. For raw 32-byte input: require explicit type indicator or treat as private key with secp256k1 validation

This follows 006 §14 step 4: "Ambiguous raw bytes MUST NOT be silently accepted."

## Constitution Check

*GATE: Must pass before implementation. Re-check after design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md exists with all mandatory sections (Purpose, User Scenarios, Requirements, Success Criteria) | **PASS** |
| II | Independently Testable Stories | 4 stories prioritized (P1-P4), Given/When/Then acceptance scenarios, each independently testable | **PASS** |
| III | Clarification Before Plan | Zero [NEEDS CLARIFICATION] markers. GAP-005 (MultisigKey aggregation) explicitly deferred. | **PASS** |
| IV | High-Level Types | Semantic types used (NeuronPrivateKey, NeuronPublicKey, EVMAddress, PeerID, Signature, MultisigKey) | **PASS** |
| V | Traceability | 24 FRs, 8 SECs, 10 SCs present and aligned. data-model.md cross-references all FRs. | **PASS** |
| VI | Golang-First SDK | Go is reference; TS MAY follow but MUST NOT drive spec design. This plan targets TS as a conforming implementation. | **PASS** (TS is a subsequent implementation per VI) |
| VII | Strict Spec Compliance | Every task will trace to FR-* or SC-*. No silent deviations. | **PASS** |
| VIII | Hedera Transport Binding | Not applicable — Key Library is transport-agnostic. | **PASS** (N/A) |
| IX | Test-First Development | Test tasks (Red phase) precede implementation tasks. Conformance tests (Chain 1, Chain 4) already written. | **PASS** |
| X | Deterministic Signing | Signing uses RFC 6979 via @noble/secp256k1. Determinism tests assert byte-identical signatures on repeated signing. | **PASS** |
| XI | Verifiable Execution | No VR-* checklist yet for spec 002 (constitution TODO). Non-blocking for implementation. | **PASS** (noted) |

**Gate result**: All gates PASS. Proceeding.

## Project Structure

### Documentation (existing, language-neutral)

```text
specs/002-key-library/
├── spec.md              # Feature specification (language-neutral, authoritative)
├── data-model.md        # Entity model (language-neutral)
├── contracts/           # API contracts (language-neutral)
│   ├── key-operations.md
│   ├── encryption.md
│   └── multisig.md
├── plan.md              # Go implementation plan (NOT used for TS)
├── tasks.md             # Go implementation tasks (NOT used for TS)
├── ts-plan.md           # THIS FILE — TypeScript implementation plan
└── ts-tasks.md          # TypeScript implementation tasks (generated by /speckit.tasks)
```

### Source Code (TypeScript)

```text
impl/typescript/src/keylib/
├── index.ts              # Public exports
├── constants.ts          # Named constants (key lengths, curve order, multicodec bytes)
├── errors.ts             # KeyError extending NeuronError (NEURON-KEY-001..014)
├── private-key.ts        # NeuronPrivateKey — immutable, fromHex, fromBytes, fromMnemonic, generate, sign, zeroize
├── public-key.ts         # NeuronPublicKey — compressed/uncompressed, fromHex, fromBytes, evmAddress, peerId, didKey
├── evm-address.ts        # EVMAddress — Keccak256 derivation, EIP-55 checksum
├── peer-id.ts            # PeerID — protobuf + identity multihash + base58btc
├── did-key.ts            # DIDKey — multicodec 0xE7 0x01 + base58btc
├── signature.ts          # Signature — R||S||V (65 bytes), verify, recover, base64
├── encrypted-key.ts      # EncryptedPrivateKey — Argon2id + AES-256-GCM, JSON serialization
├── multisig-key.ts       # MultisigKey — protocol tracking, type definitions (GAP-005)
└── matching.ts           # Constant-time matching functions (crypto.timingSafeEqual)
```

### Test Structure

```text
impl/typescript/tests/
├── conformance/
│   ├── vectors.ts                      # Golden hex values from 006 test-vectors.md
│   ├── chain1-key-derivation.test.ts   # All Chain 1 intermediate values
│   └── chain4-key-encryption.test.ts   # All Chain 4 intermediate values
└── keylib/
    ├── private-key.test.ts             # Construction, validation, signing, zeroize
    ├── public-key.test.ts              # Formats, derivations
    ├── evm-address.test.ts             # EIP-55, parsing
    ├── peer-id.test.ts                 # Protobuf, multihash, base58btc
    ├── did-key.test.ts                 # Multicodec, prefix validation
    ├── signature.test.ts               # R||S||V, verify, recover
    ├── encrypted-key.test.ts           # Argon2id, AES-GCM, round-trip
    ├── multisig-key.test.ts            # Protocol tracking, error paths
    ├── matching.test.ts                # Constant-time, false negatives/positives
    ├── signing-determinism.test.ts     # Constitution X: sign twice, assert byte-equal
    └── error.test.ts                   # All 14 NEURON-KEY-* codes
```

## Dependency Map (Spec 006 Algorithms → TypeScript Libraries)

| 006 Algorithm | FR | Library | Function |
|--------------|-----|---------|----------|
| §1 secp256k1 Key Generation | FR-A01 | `@noble/secp256k1` | `secp.utils.randomPrivateKey()` |
| §2 Point Compression | FR-A02 | `@noble/secp256k1` | `secp.getPublicKey(key, compressed)` |
| §3 EVM Address Derivation | FR-A03 | `@noble/hashes` | `keccak_256(uncompressed[1:])` → last 20 bytes |
| §4 EIP-55 Checksum | FR-A04 | `@noble/hashes` | `keccak_256(lowercase_ascii_hex)` → nibble comparison |
| §5 PeerID Derivation | FR-A05 | `bs58` | Manual protobuf (0x08 0x02 0x12 0x21) + identity multihash (0x00 0x25) + `bs58.encode()` |
| §6 DID:key Construction | FR-A06 | `bs58` | Multicodec (0xE7 0x01) + compressed key + `bs58.encode()` + "did:key:z" prefix |
| §7 RFC 6979 Signing | FR-A07 | `@noble/secp256k1` | `secp.sign(hash, key)` — deterministic, returns `{r, s, recovery}` |
| §8 Keccak256 Pre-Image | FR-A08 | `@noble/hashes` | `keccak_256(timestamp_be8 \|\| seqnum_be8 \|\| payload)` |
| §10 Signature Encoding | FR-A10 | `@noble/secp256k1` | `.toCompactRawBytes()` (64 bytes R\|\|S) + V byte. Low-S: `{lowS: true}`. V = 0 or 1. |
| §11 Argon2id | FR-A11 | `hash-wasm` | `argon2id({password, salt, iterations: 1, memorySize: 65536, parallelism: 4, hashLength: 32})` |
| §11 AES-256-GCM | FR-A11 | Node `crypto` | `createCipheriv('aes-256-gcm', key, nonce)` / `createDecipheriv` |
| §12 BIP-39 | FR-A12 | `@scure/bip39` | `mnemonicToSeedSync(mnemonic, passphrase)` |
| §13 BIP-44 | FR-A13 | `@scure/bip32` | `HDKey.fromMasterSeed(seed).derive("m/44'/60'/0'/0/0")` |
| §14 Ed25519 Rejection | FR-A14 | (validation logic) | Prefix byte check: accept 0x02/0x03/0x04 only |

## Phases

### Phase 1: Setup

Create TypeScript project scaffold (already done: package.json, tsconfig.json, vitest.config.ts, eslint.config.js, wire format utilities at `src/wire/`).

### Phase 2: Foundational (Error types + Constants)

Error types per 006 error-taxonomy.md (14 codes: NEURON-KEY-001..014). Named constants from 006 algorithm-reference.md §1.

### Phase 3: US1 — Type-Safe Key Operations (P1)

NeuronPrivateKey, NeuronPublicKey, EVMAddress, PeerID, DIDKey, Signature, matching functions. TDD: tests first (Red), then implementation (Green).

**Conformance gate**: Chain 1 test vectors must pass after this phase.

### Phase 4: US3 — Key Generation and Recovery (P3)

Generate, mnemonic (BIP-39/44), encrypt/decrypt (Argon2id + AES-256-GCM).

**Conformance gate**: Chain 4 test vectors must pass after this phase.

### Phase 5: US4 — MultisigKey Interoperability (P4)

MultisigKey type definitions and protocol tracking. Aggregation algorithm deferred (GAP-005).

### Phase 6: Polish

Integration tests, signing determinism tests (Constitution X), error sanitization tests (SEC-003).

## Conformance Gates

| Phase | Gate | Test Vectors | Pass Criteria |
|-------|------|-------------|---------------|
| Phase 3 | Chain 1 Key Derivation | `tests/conformance/chain1-key-derivation.test.ts` | All intermediate hex values match 006 test-vectors.md |
| Phase 4 | Chain 4 Key Encryption | `tests/conformance/chain4-key-encryption.test.ts` | Argon2id derived key, AES ciphertext, round-trip all match |
| Phase 6 | Determinism | `tests/keylib/signing-determinism.test.ts` | Sign same message twice → byte-identical R\|\|S\|\|V |
| Phase 6 | Error Sanitization | `tests/keylib/error.test.ts` | No key material in error messages (SEC-003, SEC-005) |

## Complexity Tracking

No constitution violations requiring justification.

## Artifacts Generated

- `specs/002-key-library/ts-plan.md` — THIS FILE
- Existing language-neutral artifacts reused: `spec.md`, `data-model.md`, `contracts/key-operations.md`, `contracts/encryption.md`, `contracts/multisig.md`
- Next step: `/speckit.tasks` to generate `specs/002-key-library/ts-tasks.md`
