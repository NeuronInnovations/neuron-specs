# Tasks: Key Library — TypeScript (Spec 002)

**Branch**: `007-identity-contract-2` | **Date**: 2026-03-17 | **Spec**: [spec.md](spec.md) | **Plan**: [ts-plan.md](ts-plan.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

> **Constitution Compliance Notes** (Principles VII, IX, X, XI):
> - **VII**: Every task traces to FR-* or SC-* requirement
> - **IX**: Test tasks (Red) precede implementation tasks (Green) within each phase
> - **X**: Signing tasks include determinism verification (sign twice, assert byte-equal)
> - **XI**: VR-* checklist pending for spec 002 (constitution TODO, non-blocking)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project scaffolding already complete. Verify existing infrastructure.

- [x] T001 Verify TypeScript project scaffold compiles: `cd impl/typescript && npx tsc --noEmit`. Files: `impl/typescript/package.json`, `impl/typescript/tsconfig.json` (FR-001)
- [x] T002 [P] Verify wire format utilities pass all tests: `cd impl/typescript && npx vitest run tests/conformance/wire-format.test.ts`. Files: `impl/typescript/src/wire/*.ts` (FR-W01..FR-W10)
- [x] T003 [P] Verify conformance test vector constants match spec 006 test-vectors.md: `impl/typescript/tests/conformance/vectors.ts` (FR-V)

**Checkpoint**: `npx tsc --noEmit` compiles, `npx vitest run tests/conformance/wire-format.test.ts` passes (32 tests).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Error types and constants that ALL user story phases depend on. MUST complete before any US phase begins.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Tests (Red phase)

- [x] T004 [P] Write failing tests for KeyError: all 14 NEURON-KEY-* codes instantiate correctly, `code` and `name` properties set, `message` is descriptive, `cause` wraps underlying errors for NEURON-KEY-012 (SDKError). Verify error messages never contain private key hex (SEC-003, SEC-005). File: `impl/typescript/tests/keylib/error.test.ts` (FR-008, FR-008a, SEC-003, SEC-005, SC-002)
- [x] T005 [P] Write failing tests for constants: PRIVATE_KEY_LENGTH=32, COMPRESSED_PUBLIC_KEY_LENGTH=33, UNCOMPRESSED_PUBLIC_KEY_LENGTH=65, SIGNATURE_LENGTH=65, EVM_ADDRESS_LENGTH=20, SECP256K1_ORDER matches 006 §1 hex value, MULTICODEC_SECP256K1_PUB=[0xe7, 0x01], DID_KEY_PREFIX="did:key:z", BIP44_DEFAULT_PATH="m/44'/60'/0'/0/0". File: `impl/typescript/tests/keylib/constants.test.ts` (FR-001, FR-005, FR-006a, FR-013, FR-014)

### Implementation (Green phase)

- [x] T006 [P] Implement KeyError class extending NeuronError with all 14 NEURON-KEY-* error codes. Each code has a named constant and factory function. Error messages MUST NOT contain key material. File: `impl/typescript/src/keylib/errors.ts` (FR-008, FR-008a, SEC-003, SEC-005)
- [x] T007 [P] Implement named constants from 006 algorithm-reference.md §1: key lengths, SECP256K1_ORDER as bigint, multicodec bytes, DID prefix, BIP-44 path, Argon2id v1 defaults (time=1, memory=65536, threads=4, salt=16, nonce=12, ciphertext=48). File: `impl/typescript/src/keylib/constants.ts` (FR-001, FR-005, FR-006a, FR-013, FR-014, FR-015)

**Checkpoint**: `npx vitest run tests/keylib/error.test.ts tests/keylib/constants.test.ts` passes.

---

## Phase 3: US1 — Type-Safe Key Operations (Priority: P1) 🎯 MVP

**Goal**: Developers can work with cryptographic keys using strong types instead of error-prone string handling. NeuronPrivateKey, NeuronPublicKey, EVMAddress, PeerID, DIDKey, Signature, and type-safe matching functions are all available.

**Independent Test**: Create a NeuronPrivateKey from hex, derive NeuronPublicKey, EVMAddress, PeerID, DID:key, sign a message, verify, recover, and verify all relationships using matching functions. All operations use types, not strings.

**Conformance Gate**: Chain 1 test vectors (key derivation) MUST pass after this phase.

### Tests (Red phase)

#### NeuronPrivateKey

- [ ] T008 [P] [US1] Write failing tests for NeuronPrivateKey: `fromHex` with/without 0x prefix, `fromBytes` from raw 32 bytes, invalid hex rejection with NEURON-KEY-004, wrong-length rejection with NEURON-KEY-003, zero-value rejection with NEURON-KEY-006, curve validation failure with NEURON-KEY-005 (k ≥ n), `publicKey()` derivation returns valid NeuronPublicKey, `toBytes()` returns defensive copy (mutating copy does not affect key), `zeroize()` zeroes internal bytes and marks key unusable. File: `impl/typescript/tests/keylib/private-key.test.ts` (FR-001, FR-002, FR-003, FR-008, FR-010, FR-018, FR-021, FR-022, SEC-003, SC-001, SC-002, SC-006)

#### NeuronPublicKey

- [ ] T009 [P] [US1] Write failing tests for NeuronPublicKey: `fromHex` from compressed hex (33 bytes, 0x02/0x03 prefix), uncompressed hex (65 bytes, 0x04 prefix), `fromBytes` from raw bytes (33 or 65), `toCompressedBytes()` returns 33 bytes, `toUncompressedBytes()` returns 65 bytes with 0x04 prefix, invalid prefix rejection with NEURON-KEY-002, invalid length rejection with NEURON-KEY-003, immutability after construction. File: `impl/typescript/tests/keylib/public-key.test.ts` (FR-001, FR-002, FR-004, FR-004a, FR-010, FR-018, FR-022, SC-001, SC-006)

#### EVMAddress

- [ ] T010 [P] [US1] Write failing tests for EVMAddress: derivation from NeuronPublicKey per 006 §3 (Keccak256 of uncompressed[1:64] → last 20 bytes), `toString()` returns EIP-55 checksummed output per 006 §4, `toLowercaseHex()` returns lowercase, `fromHex` accepts both lowercase and EIP-55, `toBytes()` returns raw 20 bytes, invalid hex rejection, wrong-length rejection. File: `impl/typescript/tests/keylib/evm-address.test.ts` (FR-005, FR-008, FR-019, SC-001, SC-003, SC-007)

#### PeerID

- [ ] T011 [P] [US1] Write failing tests for PeerID: derivation from NeuronPublicKey per 006 §5 (protobuf 0x08 0x02 0x12 0x21 + 33 compressed key → identity multihash 0x00 0x25 → base58btc), `toString()` returns string matching `12D3KooW` prefix, `protobufBytes()` returns 37 bytes matching Chain 1 test vector, `multihashBytes()` returns 39 bytes matching Chain 1 test vector, deterministic derivation (same key → same PeerID). File: `impl/typescript/tests/keylib/peer-id.test.ts` (FR-006, FR-019, SC-001, SC-003)

#### DIDKey

- [ ] T012 [P] [US1] Write failing tests for DIDKey: derivation from NeuronPublicKey per 006 §6 (multicodec 0xE7 0x01 + 33 compressed key → base58btc → "did:key:z" prefix), `toString()` returns `did:key:zQ3s...` string matching Chain 1 test vector, `multicodecBytes()` returns 35 bytes matching Chain 1 test vector, deterministic derivation. File: `impl/typescript/tests/keylib/did-key.test.ts` (FR-006a, SC-001)

#### Signature

- [ ] T013 [P] [US1] Write failing tests for Signature: construction from 65-byte R||S||V, `toBytes()` returns 65 bytes, `r()` returns 32 bytes, `s()` returns 32 bytes, `v()` returns 0 or 1, `toBase64()` matches FR-W03, `fromBytes` validation (wrong length → error), `fromBase64` round-trip. File: `impl/typescript/tests/keylib/signature.test.ts` (FR-014, SC-009, SC-010)

#### Sign + Verify + Recover

- [ ] T014 [US1] Write failing tests for NeuronPrivateKey.sign(message): produces valid 65-byte Signature in R||S||V, uses Keccak256 hashing (FR-017), verify against derived public key returns true, verify against wrong key returns false, recover from signature returns matching public key, Chain 2 test vector signature matches (signing_hash → signature_hex). File: `impl/typescript/tests/keylib/private-key.test.ts` (append sign tests) (FR-014, FR-017, SC-009, SC-010)

#### Matching Functions

- [ ] T015 [P] [US1] Write failing tests for matching: `NeuronPrivateKey.matchesPublicKey(pubkey)` true for matching/false for non-matching, `NeuronPrivateKey.matchesEvmAddress(addr)` true/false, `NeuronPublicKey.matchesPeerId(peerId)` true/false, `NeuronPublicKey.matchesEvmAddress(addr)` true/false. All comparisons constant-time. No false negatives with correct keys, no false positives with wrong keys. File: `impl/typescript/tests/keylib/matching.test.ts` (FR-016, SEC-004, SC-005, SC-007)

### Implementation (Green phase)

#### Shared Utilities

- [x] T016 [US1] Implement constant-time comparison function using Node.js `crypto.timingSafeEqual`. File: `impl/typescript/src/keylib/matching.ts` (FR-016, SEC-004)

#### EVMAddress

- [x] T017 [P] [US1] Implement EVMAddress class: internal Uint8Array(20), `fromPublicKey(pubkey)` per 006 §3 (keccak_256 of uncompressed key bytes[1:] → last 20 bytes), EIP-55 checksum per 006 §4 (keccak_256 of lowercase ASCII hex → nibble comparison → mixed-case), `fromHex(hex)` accepting lowercase or EIP-55, `toString()` returning EIP-55, `toLowercaseHex()`, `toBytes()`. File: `impl/typescript/src/keylib/evm-address.ts` (FR-005, FR-008, FR-019)

#### PeerID

- [x] T018 [P] [US1] Implement PeerID class: `fromPublicKey(pubkey)` per 006 §5 — construct protobuf bytes (0x08 0x02 0x12 0x21 + 33 compressed bytes = 37 bytes), construct identity multihash (0x00 0x25 + 37 protobuf bytes = 39 bytes), base58btc encode via `bs58.encode()`. Expose `protobufBytes()`, `multihashBytes()`, `toString()`. File: `impl/typescript/src/keylib/peer-id.ts` (FR-006, FR-019)

#### DIDKey

- [x] T019 [P] [US1] Implement DIDKey class: `fromPublicKey(pubkey)` per 006 §6 — prepend multicodec [0xE7, 0x01] to 33 compressed bytes = 35 bytes, base58btc encode via `bs58.encode()`, prepend "did:key:z". Expose `multicodecBytes()`, `toString()`. File: `impl/typescript/src/keylib/did-key.ts` (FR-006a)

#### Signature

- [x] T020 [US1] Implement Signature class: internal Uint8Array(65) for R||S||V, constructor from 65 bytes with validation, `r()` → Uint8Array(32), `s()` → Uint8Array(32), `v()` → 0 or 1, `toBytes()` defensive copy, `toBase64()` via wire/base64, `fromBytes(bytes)`, `fromBase64(str)`. Recovery: `recover(messageHash)` using `secp.Signature.fromCompact(rs).addRecoveryBit(v).recoverPublicKey(hash)` → elevate to NeuronPublicKey. Verify: `verify(messageHash, pubkey)` using recovery + constant-time compare. File: `impl/typescript/src/keylib/signature.ts` (FR-014, FR-017, SEC-004)

#### NeuronPublicKey

- [x] T021 [US1] Implement NeuronPublicKey class: internal Uint8Array(33) compressed, `fromHex(hex)` accepting 33 or 65 bytes, `fromBytes(bytes)` accepting 33 or 65, prefix byte validation (0x02/0x03 for compressed, 0x04 for uncompressed → compress internally; else reject with NEURON-KEY-002 per 006 §14), `toCompressedBytes()`, `toUncompressedBytes()` via `secp.ProjectivePoint.fromHex(compressed).toRawBytes(false)`, `evmAddress()` → EVMAddress, `peerId()` → PeerID, `didKey()` → DIDKey. Immutable after construction. File: `impl/typescript/src/keylib/public-key.ts` (FR-001, FR-002, FR-004, FR-004a, FR-010, FR-018, FR-022)

#### NeuronPrivateKey

- [x] T022 [US1] Implement NeuronPrivateKey class: internal Uint8Array(32), `fromHex(hex)` with 0x prefix stripping + hex validation (NEURON-KEY-004 with position) + length validation (NEURON-KEY-003) + zero-check (NEURON-KEY-006) + curve validation (NEURON-KEY-005: 1 ≤ k < n), `fromBytes(bytes)` with same validation, `publicKey()` via `secp.getPublicKey(key, true)` → NeuronPublicKey, `sign(messageHash)` via keccak_256 + `secp.sign(hash, key, {lowS: true})` → Signature (R||S||V with V=0/1 per 006 §10), `toBytes()` defensive copy, `zeroize()` fills internal array with 0 + marks unusable. Immutable. File: `impl/typescript/src/keylib/private-key.ts` (FR-001, FR-002, FR-003, FR-008, FR-009, FR-010, FR-014, FR-017, FR-018, FR-020, FR-021, FR-022, SEC-002, SEC-003)

#### Matching

- [x] T023 [US1] Implement matching methods on NeuronPrivateKey and NeuronPublicKey: `matchesPublicKey(pubkey)` — derive public key, constant-time compare compressed bytes; `matchesEvmAddress(addr)` — derive address, constant-time compare raw bytes; `matchesPeerId(peerId)` — derive PeerID, compare multihash bytes. All use `crypto.timingSafeEqual`. File: `impl/typescript/src/keylib/matching.ts` (append to T016) (FR-016, SEC-004)

#### Conformance Verification

- [x] T024 [US1] Run Chain 1 conformance tests and verify all intermediate hex values pass: `npx vitest run tests/conformance/chain1-key-derivation.test.ts`. Fix any failures. (FR-A01..FR-A06, SC-001, SC-003)

**Checkpoint**: Chain 1 golden test vectors pass. NeuronPrivateKey → NeuronPublicKey → EVMAddress/PeerID/DIDKey derivation chain is byte-identical to spec 006 test-vectors.md. Signing produces correct R||S||V. SC-001, SC-002, SC-005, SC-006, SC-007, SC-009, SC-010 verifiable.

---

## Phase 4: US3 — Key Generation and Recovery (Priority: P3)

**Goal**: Developers can generate new keys, restore from mnemonic phrases, and securely store/retrieve encrypted keys.

**Independent Test**: Generate a new NeuronPrivateKey, create mnemonic, restore from mnemonic, encrypt with password, decrypt and verify match.

**Conformance Gate**: Chain 4 test vectors (key encryption) MUST pass after this phase.

### Tests (Red phase)

- [x] T025 [P] [US3] Write failing tests for key generation: `NeuronPrivateKey.generate()` returns valid key (32 bytes, non-zero, on curve), generated key can derive all types, two generated keys are different. File: `impl/typescript/tests/keylib/private-key.test.ts` (append generation tests) (FR-012, SC-004)
- [x] T026 [P] [US3] Write failing tests for mnemonic: `NeuronPrivateKey.fromMnemonic(mnemonic)` restores key using default path m/44'/60'/0'/0/0 per 006 §12/§13, invalid mnemonic rejected with NEURON-KEY-010, invalid checksum rejected, custom derivation path supported. File: `impl/typescript/tests/keylib/private-key.test.ts` (append mnemonic tests) (FR-013, SC-004)
- [x] T027 [P] [US3] Write failing tests for encryption: `EncryptedPrivateKey.encrypt(keyBytes, password)` produces valid EncryptedPrivateKey, `decrypt(password)` recovers original key, wrong password returns NEURON-KEY-009, `toCanonicalJson()` matches 006 wire-format.md §2 field order (version → salt → nonce → ciphertext), `fromJson(json)` parses correctly, Chain 4 test vector values match (Argon2id derived key, AES ciphertext, round-trip). File: `impl/typescript/tests/keylib/encrypted-key.test.ts` (FR-015, FR-W03, FR-W05, SC-004)

### Implementation (Green phase)

- [x] T028 [US3] Implement `NeuronPrivateKey.generate()`: use `secp.utils.randomPrivateKey()` (cryptographically secure, validates curve membership), wrap in NeuronPrivateKey. File: `impl/typescript/src/keylib/private-key.ts` (append generate) (FR-012)
- [x] T029 [US3] Implement `NeuronPrivateKey.fromMnemonic(mnemonic, path?)`: validate mnemonic via `bip39.validateMnemonic(mnemonic, wordlist)` (reject with NEURON-KEY-010), derive seed via `bip39.mnemonicToSeedSync(mnemonic)` per 006 §12, derive key at path via `HDKey.fromMasterSeed(seed).derive(path)` per 006 §13, default path `m/44'/60'/0'/0/0`. File: `impl/typescript/src/keylib/private-key.ts` (append fromMnemonic) (FR-013)
- [x] T030 [US3] Implement EncryptedPrivateKey class: `static async deriveKey(password, salt, params)` → Argon2id via `hash-wasm` (password as UTF-8 Uint8Array, salt, iterations=params.time, memorySize=params.memory, parallelism=params.threads, hashLength=32) per 006 §11. `static async encrypt(keyBytes, password, salt?, nonce?)` → derive key + AES-256-GCM encrypt via Node `crypto.createCipheriv('aes-256-gcm', derivedKey, nonce)` → 48-byte ciphertext (32 + 16 tag). `async decrypt(password)` → derive key + AES-256-GCM decrypt. `toCanonicalJson()` per wire-format §2 field order. `static fromJson(json)` → parse. `ciphertextBytes()` → Uint8Array. File: `impl/typescript/src/keylib/encrypted-key.ts` (FR-015, FR-A11, SEC-006)

#### Conformance Verification

- [x] T031 [US3] Run Chain 4 conformance tests and verify all intermediate hex values pass: `npx vitest run tests/conformance/chain4-key-encryption.test.ts`. Fix any failures. (FR-A11, SC-004)

**Checkpoint**: Chain 4 golden test vectors pass. Key generation, mnemonic restoration, and encrypt/decrypt round-trip work. SC-004 verifiable.

---

## Phase 5: US4 — MultisigKey Interoperability (Priority: P4)

**Goal**: Type-safe MultisigKey support with protocol tracking. Aggregation algorithm deferred (GAP-005).

**Independent Test**: Create MultisigKey from protocol identifier and threshold, query protocol, verify non-secp256k1 protocols reject EVM address derivation.

### Tests (Red phase)

- [x] T032 [P] [US4] Write failing tests for MultisigKey: construction with protocol identifier and m-of-n threshold, `protocol()` returns correct string, `threshold()` and `totalKeys()` return correct values, EVMAddress derivation for non-secp256k1 protocol returns NEURON-KEY-002, PeerID derivation for non-secp256k1 protocol returns NEURON-KEY-002. File: `impl/typescript/tests/keylib/multisig-key.test.ts` (FR-023, FR-024)

### Implementation (Green phase)

- [x] T033 [US4] Implement MultisigKey class: protocol identifier string, threshold number, totalKeys number, `protocol()`, `threshold()`, `totalKeys()`, `evmAddress()` → reject with NEURON-KEY-002 for non-secp256k1-aggregated protocols (GAP-005: secp256k1-aggregated signing not yet available), `peerId()` → same rejection. Factory `fromConfig(protocol, threshold, totalKeys)` for type definitions. File: `impl/typescript/src/keylib/multisig-key.ts` (FR-023, FR-024)

**Checkpoint**: MultisigKey type-safe interface works. Protocol queries return correct identifiers. Unsupported operations return clear errors.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Integration tests, determinism verification, error sanitization, and public API exports.

- [x] T034 Write signing determinism tests: sign same message with same key twice, assert byte-identical R||S||V signatures. Sign same message with different key, assert different signatures. File: `impl/typescript/tests/keylib/signing-determinism.test.ts` (Constitution X, FR-014)
- [ ] T035 [P] Write error sanitization tests: for each error code (NEURON-KEY-001..014), trigger the error and verify `message` does NOT contain private key hex, raw key bytes, or password strings. File: `impl/typescript/tests/keylib/error.test.ts` (append sanitization tests) (SEC-003, SEC-005)
- [ ] T036 [P] Write full lifecycle integration test: generate key → derive public key → derive EVMAddress + PeerID + DIDKey → sign message → verify signature → recover public key → verify matching → encrypt → decrypt → verify key matches original. File: `impl/typescript/tests/keylib/integration.test.ts` (SC-001, SC-003, SC-004, SC-005, SC-006, SC-008)
- [x] T037 Create public exports barrel file: re-export all types, factory functions, and error classes. File: `impl/typescript/src/keylib/index.ts` (FR-001)
- [x] T038 Run all tests and verify zero failures: `npx vitest run`. File: all test files (SC-001..SC-010)

**Checkpoint**: All tests pass. All Chain 1 and Chain 4 conformance tests pass. Signing is deterministic. Error messages are sanitized.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — verify existing scaffold
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **Phase 3 (US1)**: Depends on Phase 2 — core type-safe key operations
- **Phase 4 (US3)**: Depends on Phase 3 — generation/recovery builds on key types
- **Phase 5 (US4)**: Depends on Phase 2 only — MultisigKey is independent of US1/US3
- **Phase 6 (Polish)**: Depends on Phase 3, 4, 5

### User Story Dependencies

- **US1 (P1)**: Foundation — no story dependencies
- **US3 (P3)**: Requires US1 key types for mnemonic restoration and encryption
- **US4 (P4)**: Independent of US1/US3 (type definitions only, no signing)

> Note: US2 (Cross-Ecosystem Conversions) from spec.md is subsumed by US1 in this TypeScript plan. The spec's US2 scenarios (deterministic derivation, cross-format matching) are covered by US1's EVMAddress/PeerID/DIDKey derivation and matching functions. In TypeScript, "blockchain SDK key conversion" (FR-011) maps to raw Uint8Array interchange, which is implicit in all factory methods.

### Within Each User Story

- Tests (Red) MUST be written and FAIL before implementation (Green)
- Foundation types before derived types: errors/constants → Signature → EVMAddress/PeerID/DIDKey → NeuronPublicKey → NeuronPrivateKey → matching
- Conformance gate MUST pass before proceeding to next phase

### Parallel Opportunities

**Phase 2** (all tasks [P]):
```
T004 (error tests) || T005 (constants tests)
T006 (errors impl) || T007 (constants impl)
```

**Phase 3 Tests** (all tasks [P]):
```
T008 (private-key tests) || T009 (public-key tests) || T010 (evm tests) || T011 (peer-id tests) || T012 (did-key tests) || T013 (signature tests) || T015 (matching tests)
```

**Phase 3 Implementation** (partial parallel):
```
T016 (matching util) || T017 (EVMAddress) || T018 (PeerID) || T019 (DIDKey)
T020 (Signature) — after T016
T021 (NeuronPublicKey) — after T017, T018, T019
T022 (NeuronPrivateKey) — after T020, T021
T023 (matching methods) — after T022
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Verify setup
2. Complete Phase 2: Errors + constants
3. Complete Phase 3: US1 — type-safe key operations
4. **STOP and VALIDATE**: Run Chain 1 conformance tests
5. If Chain 1 passes → MVP is complete

### Incremental Delivery

1. Phase 2 → Foundation ready
2. Phase 3 (US1) → Chain 1 passes → key derivation + signing works (MVP)
3. Phase 4 (US3) → Chain 4 passes → generation + encryption works
4. Phase 5 (US4) → MultisigKey type available
5. Phase 6 → All tests green, determinism verified, errors sanitized

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 38 |
| Phase 1 (Setup) | 3 |
| Phase 2 (Foundational) | 4 |
| Phase 3 (US1) | 17 |
| Phase 4 (US3) | 7 |
| Phase 5 (US4) | 2 |
| Phase 6 (Polish) | 5 |
| Parallel opportunities | 15 tasks marked [P] |
| Conformance gates | 2 (Chain 1, Chain 4) |
| FR coverage | 24/24 FRs traced |
| SEC coverage | 8/8 SECs traced |
| SC coverage | 10/10 SCs traced |
