# Tasks: Key Library (Spec 002)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `002-key-library` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

---

## Phase 1: Setup

**Purpose**: Project scaffolding, Go module initialization, and dependency wiring. All subsequent phases depend on this.

- [x] T001 Initialize Go module for `internal/keylib/` with `go.mod`; add primary dependencies: `go-ethereum/crypto`, `go-ethereum/common`, `go-libp2p/core/peer`, `go-libp2p/core/crypto`, `tyler-smith/go-bip39`, `tyler-smith/go-bip32`, `mr-tron/base58`, `golang.org/x/crypto/argon2`, `stretchr/testify`. File: `internal/keylib/go.mod` (FR-003, FR-010)
- [x] T002 [P] Create package directory structure matching plan.md layout with placeholder files for all source and test files. Files: `internal/keylib/*.go` directory scaffold (FR-001)
- [x] T003 [P] Create `internal/keylib/doc.go` with package documentation describing the Key Library purpose, secp256k1-only constraint, adapter pattern architecture (no reimplementation of crypto primitives), and immutability guarantees (FR-001, FR-003, FR-022)

**Checkpoint**: `go build ./internal/keylib/...` compiles with zero errors; all dependencies resolve.

---

## Phase 2: Foundational

**Purpose**: Error types, constants, and shared types that block ALL user story phases. These MUST be completed before any US phase begins.

### Tests (Red phase)

- [x] T004 [P] Write failing tests for all 11 error kinds: `InvalidFormat`, `InvalidLength`, `InvalidHex` (with character position), `InvalidKey`, `ZeroValue`, `KeyMismatch`, `Encryption`, `Mnemonic`, `Derivation`, `UnsupportedKeyType`, `SDKError` (wraps underlying error). Verify `Error()` returns descriptive messages, `Unwrap()` preserves wrapped errors, and error messages never contain private key material. File: `internal/keylib/errors_test.go` (FR-008, FR-008a, SEC-003, SEC-005, SC-002)

### Implementation (Green phase)

- [x] T005 [P] Implement structured error types with error kind enum, descriptive message format including context (operation name, exact invalid character position for hex errors), `Error()` string method, `Unwrap()` for SDKError wrapping, and `Is()`/`As()` support. File: `internal/keylib/errors.go` (FR-008, FR-008a, SEC-003, SEC-005)
- [x] T006 [P] Define shared constants: secp256k1 key lengths (32-byte private, 33-byte compressed public, 65-byte uncompressed public, 20-byte address), signature length (65 bytes), encryption parameters (16-byte salt, 12-byte nonce, 48-byte ciphertext), Argon2id v1 defaults (time=1, memory=64*1024 KiB, threads=4), BIP-44 default path `m/44'/60'/0'/0/0`, multicodec secp256k1-pub varint bytes `0xe7 0x01`, DID:key prefix `did:key:z`. File: `internal/keylib/constants.go` (FR-001, FR-005, FR-006a, FR-013, FR-014, FR-015)

**Checkpoint**: `go test ./internal/keylib/... -run TestError` passes; all 11 error kinds instantiate with correct messages.

---

## Phase 3: US1 -- Type-Safe Key Operations (P1)

**Goal**: Developers can work with cryptographic keys using strong types instead of error-prone string handling. NeuronPrivateKey, NeuronPublicKey, EVMAddress, PeerID, DID:key, Signature, and type-safe matching functions are all available.

**Independent Test**: Create a NeuronPrivateKey from a hex string, derive NeuronPublicKey, EVMAddress, PeerID, and DID:key, sign a message, verify the signature, recover the public key, and verify all relationships using type-safe matching functions. All operations use types not strings. Invalid inputs return descriptive errors.

### Tests (Red phase)

#### NeuronPrivateKey

- [x] T007 [P] [US1] Write failing tests for NeuronPrivateKey construction and core operations: `NeuronPrivateKeyFromHex` with/without `0x` prefix, `NeuronPrivateKeyFromBytes` from raw 32 bytes, `NeuronPrivateKeyFromBlockchainKey` from `*ecdsa.PrivateKey`, Ed25519 rejection returns `UnsupportedKeyType` error, invalid hex character rejection with exact position returns `InvalidHex`, wrong-length rejection returns `InvalidLength`, zero-value rejection returns `ZeroValue`, `PublicKey()` derivation returns valid NeuronPublicKey, immutability (no setters exist), thread-safety for concurrent `PublicKey()` calls, `Zeroize()` zeroes memory, post-zeroize key is unusable. File: `internal/keylib/private_key_test.go` (FR-001, FR-002, FR-003, FR-004, FR-004a, FR-008, FR-009, FR-010, FR-011, FR-018, FR-021, FR-022, SEC-003, SEC-005, SC-001, SC-002, SC-006, SC-008)

#### NeuronPublicKey

- [x] T008 [P] [US1] Write failing tests for NeuronPublicKey construction and format methods: `NeuronPublicKeyFromHex` from compressed hex (33 bytes), uncompressed hex (65 bytes), `NeuronPublicKeyFromBytes` from raw bytes (33 or 65), `NeuronPublicKeyFromBlockchainKey` from `*ecdsa.PublicKey`, `Compressed()` returns 33 bytes with `0x02`/`0x03` prefix, `Uncompressed()` returns 65 bytes with `0x04` prefix, invalid input rejection, immutability after construction. File: `internal/keylib/public_key_test.go` (FR-001, FR-002, FR-010, FR-018, FR-022, SC-001, SC-006)

#### EVMAddress

- [x] T009 [P] [US1] Write failing tests for EVMAddress: derivation from NeuronPublicKey (Keccak256 of uncompressed[1:64] -> last 20 bytes), `Hex()` returns EIP-55 checksummed output, `LowercaseHex()` returns lowercase, `EVMAddressFromHex` accepts both lowercase and EIP-55, `EVMAddressFromBytes` from raw 20 bytes, `Bytes()` returns raw 20 bytes, invalid hex rejection, wrong-length rejection. File: `internal/keylib/evm_address_test.go` (FR-005, FR-008, FR-019, SC-001, SC-003, SC-007)

#### PeerID

- [x] T010 [P] [US1] Write failing tests for PeerID: derivation from NeuronPublicKey (compressed -> libp2p `crypto.Secp256k1PublicKey` -> `peer.IDFromPublicKey()` -> PeerID), `String()` returns base58btc-encoded multihash matching `12D3KooW...` prefix pattern, deterministic derivation (same key always produces same PeerID). File: `internal/keylib/peer_id_test.go` (FR-006, FR-019, SC-001, SC-003)

#### DID:key

- [x] T011 [P] [US1] Write failing tests for DID:key derivation: `DIDKey()` on NeuronPublicKey returns string with `did:key:zQ3s` prefix, multicodec varint `0xe7 0x01` prepended to compressed key, base58btc encoding with `z` multibase prefix, deterministic derivation (same key always produces same DID:key). File: `internal/keylib/did_key_test.go` (FR-006a, SC-001)

#### Signature

- [x] T012 [P] [US1] Write failing tests for Signature type: construction from 65-byte R||S||V, `Bytes()` returns 65 bytes, `R()`/`S()`/`V()` accessors return correct 32/32/1 byte components, `StandardV()` returns 0 or 1, `EthereumV()` returns 27 or 28, `Verify(message, pubkey)` returns true for valid signature and false for tampered message/wrong key, `RecoverPublicKey(message)` returns correct NeuronPublicKey (uncompressed recovery then elevate), recovery with wrong message returns different key. File: `internal/keylib/signature_test.go` (FR-014, FR-017, SC-009, SC-010)

#### Sign method

- [x] T013 [US1] Write failing tests for `NeuronPrivateKey.Sign(message)`: produces valid 65-byte Signature in R||S||V format, uses Keccak256 hashing, signature verifies against derived public key, recovered public key matches derived public key. File: `internal/keylib/private_key_test.go` (append Sign tests) (FR-014, FR-017, SC-009, SC-010)

#### Matching Functions

- [x] T014 [P] [US1] Write failing tests for matching functions: `NeuronPrivateKey.MatchesPublicKey(NeuronPublicKey)` true for matching key and false for non-matching, `NeuronPrivateKey.MatchesEVMAddress(EVMAddress)` true/false, `NeuronPublicKey.MatchesPeerID(PeerID)` true/false, `NeuronPublicKey.MatchesEVMAddress(EVMAddress)` true/false, no false negatives with correct keys, no false positives with wrong keys. File: `internal/keylib/matching_test.go` (FR-016, SEC-004, SC-005, SC-007)

### Implementation (Green phase)

#### NeuronPrivateKey

- [x] T015 [US1] Implement NeuronPrivateKey struct with internal `[32]byte`, factory methods `NeuronPrivateKeyFromHex`, `NeuronPrivateKeyFromBytes`, `NeuronPrivateKeyFromBlockchainKey` (elevating transformations), `PublicKey()` derivation (computation) via `go-ethereum/crypto`, Ed25519 detection and rejection, input validation (hex chars with position reporting, length, zero-value, secp256k1 curve membership), `Zeroize()` method, immutability (no setters/mutators). API naming MUST distinguish computations (deriving) from transformations (elevating). File: `internal/keylib/private_key.go` (FR-001, FR-002, FR-003, FR-004, FR-004a, FR-008, FR-009, FR-010, FR-011, FR-018, FR-020, FR-021, FR-022, SEC-002, SEC-003, SEC-005, SEC-007)

#### NeuronPublicKey

- [x] T016 [P] [US1] Implement NeuronPublicKey struct with internal compressed `[33]byte`, factory methods `NeuronPublicKeyFromHex`, `NeuronPublicKeyFromBytes`, `NeuronPublicKeyFromBlockchainKey`, `Compressed()` returning 33-byte slice, `Uncompressed()` returning 65-byte slice via curve decompression, input validation (accepts 33 or 65 byte formats), immutability. File: `internal/keylib/public_key.go` (FR-001, FR-002, FR-004a, FR-009, FR-010, FR-011, FR-018, FR-022, SEC-005)

#### EVMAddress

- [x] T017 [P] [US1] Implement EVMAddress struct with internal `[20]byte`, factory methods `EVMAddressFromHex` (accepts lowercase or EIP-55), `EVMAddressFromBytes` (elevating transformations), `Hex()` with EIP-55 checksumming via `go-ethereum/common`, `LowercaseHex()`, `Bytes()`, derivation method `EVMAddress()` on NeuronPublicKey (computation) using Keccak256(uncompressed[1:]) -> last 20 bytes. File: `internal/keylib/evm_address.go` (FR-005, FR-008, FR-019, FR-020, SEC-005)

#### PeerID

- [x] T018 [P] [US1] Implement PeerID struct wrapping `peer.ID`, derivation method `PeerID()` on NeuronPublicKey using compressed key -> `libp2pcrypto.UnmarshalSecp256k1PublicKey` -> `peer.IDFromPublicKey()`, `String()` for base58btc output. File: `internal/keylib/peer_id.go` (FR-006, FR-019)

#### DID:key

- [x] T019 [P] [US1] Implement `DIDKey()` method on NeuronPublicKey: get compressed bytes, prepend multicodec varint `0xe7 0x01`, base58btc encode via `mr-tron/base58`, return `"did:key:z" + encoded`. File: `internal/keylib/did_key.go` (FR-006a)

#### Signature

- [x] T020 [US1] Implement Signature struct with `[32]byte` R, `[32]byte` S, `byte` V, constructor from 65 bytes, `Bytes()`, `R()`/`S()`/`V()` accessors, `StandardV()`, `EthereumV()`, `Verify(message, pubkey)` using Keccak256 + ecrecover + `crypto/subtle.ConstantTimeCompare`, `RecoverPublicKey(message)` using ecrecover -> uncompressed 65-byte key -> elevate to NeuronPublicKey. File: `internal/keylib/signature.go` (FR-014, FR-017, SEC-002, SEC-004)

#### Sign method

- [x] T021 [US1] Implement `Sign(message []byte) Signature` on NeuronPrivateKey: `Keccak256(message)` -> `crypto.Sign(hash, ecdsaKey)` (RFC 6979 deterministic nonce) -> parse into Signature{R, S, V} and return. File: `internal/keylib/private_key.go` (append Sign method) (FR-014, FR-017, SEC-002)

#### Matching Functions

- [x] T022 [US1] Implement matching functions using `crypto/subtle.ConstantTimeCompare`: `MatchesPublicKey` derives public key and constant-time compares compressed bytes, `MatchesEVMAddress` derives address and compares raw bytes, `MatchesPeerID` derives PeerID and compares string representation, `MatchesEVMAddress` on NeuronPublicKey derives and compares. File: `internal/keylib/matching.go` (FR-016, SEC-004)

**Checkpoint**: `go test ./internal/keylib/... -run "TestPrivateKey|TestPublicKey|TestEVMAddress|TestPeerID|TestDIDKey|TestSignature|TestMatch"` all pass. A NeuronPrivateKey can be created from hex, derive all types, sign a message, verify, and recover the signer. SC-001, SC-002, SC-006, SC-007, SC-009, SC-010 verifiable.

---

## Phase 4: US2 -- Cross-Ecosystem Key Conversions (P2)

**Goal**: Developers building cross-ecosystem applications get seamless bidirectional conversion between Neuron keys and blockchain SDK keys, and can verify that an EVMAddress and PeerID belong to the same underlying key.

**Independent Test**: Generate a NeuronPrivateKey, convert to `*ecdsa.PrivateKey` and back, verify byte-equal. Derive EVMAddress and PeerID from the same NeuronPublicKey, verify cross-format matching confirms they belong to the same key. All derivations are deterministic (derive twice, compare).

### Tests (Red phase)

- [x] T023 [P] [US2] Write failing tests for bidirectional blockchain key conversion: `NeuronPrivateKey.ToBlockchainKey()` returns `*ecdsa.PrivateKey`, converting back via `NeuronPrivateKeyFromBlockchainKey` produces byte-equal NeuronPrivateKey; `NeuronPublicKey.ToBlockchainKey()` returns `*ecdsa.PublicKey`, converting back produces identical NeuronPublicKey; all derived values (EVMAddress, PeerID, DID:key) match after round-trip. File: `internal/keylib/private_key_test.go` (append conversion tests) (FR-011, SC-008)
- [x] T024 [P] [US2] Write failing tests for cross-format verification: given EVMAddress and PeerID both derived from the same NeuronPublicKey, `EVMAddressMatchesPeerID(address, peerID, pubkey)` returns true; given EVMAddress from key A and PeerID from key B, returns false. File: `internal/keylib/matching_test.go` (append cross-format tests) (FR-007, SC-003, SC-005)
- [x] T025 [US2] Write failing test for full derivation chain determinism: generate NeuronPrivateKey, derive NeuronPublicKey, EVMAddress, PeerID, DID:key, derive all again, assert all pairs are equal; verify all matching functions confirm same-key relationships; generate second key, verify all cross-key matches return false. File: `internal/keylib/matching_test.go` (append full chain test) (FR-005, FR-006, FR-006a, FR-007, FR-016, SC-001, SC-003, SC-005)

### Implementation (Green phase)

- [x] T026 [US2] Implement `ToBlockchainKey() *ecdsa.PrivateKey` on NeuronPrivateKey and `ToBlockchainKey() *ecdsa.PublicKey` on NeuronPublicKey, using `go-ethereum/crypto` for conversion. File: `internal/keylib/private_key.go` and `internal/keylib/public_key.go` (append ToBlockchainKey methods) (FR-011, SEC-007)
- [x] T027 [US2] Implement `EVMAddressMatchesPeerID(address EVMAddress, peerID PeerID, pubkey NeuronPublicKey) bool` helper that derives both from the pubkey and checks both match using constant-time comparison. File: `internal/keylib/matching.go` (append cross-format helper) (FR-007, SEC-004)

**Checkpoint**: `go test ./internal/keylib/... -run "TestBlockchainConversion|TestCrossFormat|TestDerivationChain"` all pass. Blockchain key round-trips produce identical keys. Cross-format verification works. SC-003, SC-005, SC-008 verifiable.

---

## Phase 5: US3 -- Key Generation and Recovery (P3)

**Goal**: Developers can generate new keys with secure randomness, restore keys from BIP-39 mnemonic phrases with BIP-44 derivation paths, and encrypt/decrypt private keys with Argon2id/AES-256-GCM for secure storage.

**Independent Test**: Generate a new NeuronPrivateKey, create a mnemonic, restore the key from the mnemonic and verify identical. Encrypt the key with a password, decrypt it, verify it matches the original. All operations use type-safe interfaces.

### Tests (Red phase)

#### Key Generation

- [x] T028 [P] [US3] Write failing tests for `NewNeuronPrivateKey()`: returned key is non-zero and on secp256k1 curve, two generated keys are different (randomness verification), generated key can derive public key/EVMAddress/PeerID/DID:key, generated key can sign and verify. File: `internal/keylib/private_key_test.go` (append generation tests) (FR-012, FR-018, SC-004)

#### Mnemonic

- [x] T029 [P] [US3] Write failing tests for mnemonic operations: `GenerateMnemonic()` produces valid BIP-39 phrase (12 or 24 words), `NeuronPrivateKeyFromMnemonic(mnemonic, "")` uses default path `m/44'/60'/0'/0/0` and produces valid key, `NeuronPrivateKeyFromMnemonic(mnemonic, "m/44'/60'/0'/0/1")` produces different key, restoring from same mnemonic+path produces identical key (deterministic), invalid mnemonic checksum rejected with `Mnemonic` error kind, invalid derivation path rejected with `Derivation` error kind. File: `internal/keylib/mnemonic_test.go` (FR-013, FR-008, SC-004)

#### Encryption

- [x] T030 [P] [US3] Write failing tests for encryption/decryption: `Encrypt(key, password, nil)` returns EncryptedPrivateKey with version=1, 16-byte salt, 12-byte nonce, 48-byte ciphertext; `Decrypt(encrypted, password)` returns byte-identical NeuronPrivateKey; wrong password returns `Encryption` error (no information leakage about whether password or key was wrong); EncryptedPrivateKey JSON marshal/unmarshal round-trip; version 2 with custom Argon2 params (time, memory, threads) stored and used; version 2 JSON includes `time`/`memory`/`threads` fields. File: `internal/keylib/encrypted_key_test.go` (FR-015, FR-008, SEC-006, SC-004)

### Implementation (Green phase)

#### Key Generation

- [x] T031 [US3] Implement `NewNeuronPrivateKey() (NeuronPrivateKey, error)`: generate 32 bytes from `crypto/rand`, validate on secp256k1 curve (retry if invalid), return sealed immutable NeuronPrivateKey. File: `internal/keylib/private_key.go` (append NewNeuronPrivateKey) (FR-012, SEC-002)

#### Mnemonic

- [x] T032 [US3] Implement `GenerateMnemonic() (string, error)` using `go-bip39` entropy generation and `NeuronPrivateKeyFromMnemonic(mnemonic string, path string) (NeuronPrivateKey, error)` using BIP-39 seed derivation + BIP-44 HD key derivation via `go-bip32`, default path constant `m/44'/60'/0'/0/0`, validation for mnemonic checksum and path format. File: `internal/keylib/mnemonic.go` (FR-013, SEC-002, SEC-006)

#### Encryption

- [x] T033 [US3] Implement EncryptedPrivateKey struct with JSON tags (Version, Salt, Nonce, Ciphertext, Time, Memory, Threads), `Encrypt(key NeuronPrivateKey, password string, opts ...EncryptOption) (EncryptedPrivateKey, error)` using `argon2.IDKey` for KDF + `crypto/aes` + `crypto/cipher` for AES-256-GCM, `Decrypt(encrypted EncryptedPrivateKey, password string) (NeuronPrivateKey, error)` with version dispatching (v1 hardcoded params, v2 stored params), generic decryption failure error without leaking password/key info. File: `internal/keylib/encrypted_key.go` (FR-015, SEC-006)

**Checkpoint**: `go test ./internal/keylib/... -run "TestNewNeuronPrivateKey|TestMnemonic|TestEncrypt|TestDecrypt"` all pass. Key generation produces valid unique keys. Mnemonic restore is deterministic. Encrypt/decrypt round-trip preserves key. SC-004 verifiable.

---

## Phase 6: US4 -- Multisig Key Interoperability (P4)

**Goal**: Developers get type-safe MultisigKey support for both standard secp256k1 key aggregation and blockchain-specific threshold implementations. The unified type enables integration with the Neuron SDK while clearly documenting limitations.

**Independent Test**: Create a MultisigKey from multiple NeuronPrivateKeys (secp256k1-aggregated), verify protocol/threshold/total. Create a MultisigKey from a mock blockchain threshold key (hedera-threshold), verify EVMAddress/PeerID return `UnsupportedKeyType` error. Convert to/from blockchain key and verify round-trip.

### Tests (Red phase)

- [x] T034 [P] [US4] Write failing tests for MultisigKey construction and protocol tracking: `NewMultisigKey(keys, threshold)` creates MultisigKey with `Protocol()` = `"secp256k1-aggregated"`, correct `Threshold()` and `TotalKeys()`; validation rejects threshold=0, threshold > len(keys), empty keys slice; `MultisigKeyFromBlockchainKey(key, "hedera-threshold")` creates blockchain-specific MultisigKey with correct protocol; `EVMAddress()` on `"hedera-threshold"` returns `UnsupportedKeyType` error; `PeerID()` on `"hedera-threshold"` returns `UnsupportedKeyType` error; protocol query returns correct identifier for `"frost"`, `"bls"`. File: `internal/keylib/multisig_key_test.go` (FR-023, FR-024, FR-008, SC-002)
- [x] T035 [P] [US4] Write failing tests for MultisigKey bidirectional blockchain conversion: `ToBlockchainKey()` returns underlying blockchain key, `MultisigKeyFromBlockchainKey(blockchainKey, protocol)` -> `ToBlockchainKey()` round-trip preserves key and protocol. File: `internal/keylib/multisig_key_test.go` (append conversion tests) (FR-023, SC-008)

### Implementation (Green phase)

- [x] T036 [US4] Implement MultisigKey struct with fields (protocol string, threshold uint, totalKeys uint, keys []NeuronPrivateKey, blockchainKey interface{}), `NewMultisigKey(keys []NeuronPrivateKey, threshold uint) (MultisigKey, error)` with validation, `MultisigKeyFromBlockchainKey(key interface{}, protocol string) (MultisigKey, error)`, `Protocol() string`, `Threshold() uint`, `TotalKeys() uint`, `EVMAddress()` returning error for non-secp256k1-aggregated and GAP-005 note for aggregated, `PeerID()` with same gating. File: `internal/keylib/multisig_key.go` (FR-023, FR-024, FR-004a, FR-008)
- [x] T037 [US4] Implement `ToBlockchainKey() (interface{}, error)` on MultisigKey for bidirectional conversion — returns stored keys for secp256k1-aggregated, returns wrapped blockchain key for other protocols. File: `internal/keylib/multisig_key.go` (append ToBlockchainKey) (FR-023)

**Checkpoint**: `go test ./internal/keylib/... -run "TestMultisigKey"` all pass. MultisigKey construction, protocol tracking, limitation errors, and blockchain conversion all work. SC-002, SC-008 verifiable.

---

## Phase 7: Polish

**Purpose**: Deterministic signing verification (Constitution Principle X), full lifecycle integration tests, cross-type validation, and quickstart scenario verification.

### Deterministic Signing (Constitution Principle X -- MANDATORY)

- [x] T038 Write deterministic signing test: sign the same message twice with the same NeuronPrivateKey, assert byte-equal R||S||V signatures (identical R, identical S, identical V); sign a different message with the same key, assert signatures differ; sign the same message with a different key, assert signatures differ. Validates RFC 6979 deterministic nonce generation. File: `internal/keylib/signing_test.go` (FR-014, FR-017, SC-010, Constitution X)

### Integration Tests

- [x] T039 [P] Write full key lifecycle integration test: `NewNeuronPrivateKey()` -> derive `NeuronPublicKey` -> derive `EVMAddress` -> derive `PeerID` -> derive `DID:key` -> `Sign(message)` -> `sig.Verify(message, pubkey)` == true -> `sig.RecoverPublicKey(message)` -> assert recovered matches original pubkey -> `Encrypt(key, password)` -> `Decrypt(encrypted, password)` -> assert decrypted matches original key -> `Zeroize()` -> verify key unusable. File: `internal/keylib/integration_test.go` (FR-001, FR-005, FR-006, FR-006a, FR-012, FR-014, FR-015, FR-017, FR-021, SC-001, SC-003, SC-004, SC-006, SC-009)

- [x] T040 [P] Write mnemonic-to-signing integration test: `GenerateMnemonic()` -> `NeuronPrivateKeyFromMnemonic(mnemonic, "")` -> `Sign(message)` -> `sig.Verify(message, pubkey)` == true -> restore from same mnemonic with default path -> assert byte-identical key -> sign same message -> assert byte-identical signature (deterministic via RFC 6979). File: `internal/keylib/integration_test.go` (append mnemonic integration) (FR-012, FR-013, FR-014, FR-017, SC-004, Constitution X)

- [x] T041 [P] Write cross-type matching integration test: generate NeuronPrivateKey, derive NeuronPublicKey, EVMAddress, PeerID, verify all `Matches*` functions return true; generate a second NeuronPrivateKey, verify all cross-key `Matches*` return false. File: `internal/keylib/integration_test.go` (append matching integration) (FR-016, SC-005, SC-007)

- [x] T042 [P] Write blockchain round-trip integration test: `NewNeuronPrivateKey()` -> `ToBlockchainKey()` -> `NeuronPrivateKeyFromBlockchainKey()` -> assert byte-equal to original; derive EVMAddress from both and assert equal; derive PeerID from both and assert equal; derive DID:key from both and assert equal. File: `internal/keylib/integration_test.go` (append blockchain round-trip) (FR-011, SC-008)

- [x] T043 [P] Write MultisigKey integration test: create MultisigKey from 3 generated NeuronPrivateKeys with threshold=2, verify `Protocol()` == `"secp256k1-aggregated"`, verify `Threshold()` == 2, verify `TotalKeys()` == 3, attempt `EVMAddress()` derivation (expect GAP-005 error or deferred behavior); create MultisigKey from mock blockchain threshold key with `"hedera-threshold"` protocol, verify `EVMAddress()` returns `UnsupportedKeyType`, verify `PeerID()` returns `UnsupportedKeyType`. File: `internal/keylib/integration_test.go` (append multisig integration) (FR-023, FR-024, SC-002)

- [x] T044 [P] Write error handling integration test: exercise all 11 error kinds with realistic invalid inputs -- `InvalidHex` (non-hex chars in key string), `InvalidLength` (31-byte key), `InvalidKey` (key exceeds curve order), `ZeroValue` (all-zero 32 bytes), `UnsupportedKeyType` (Ed25519 key), `KeyMismatch` (mismatched key pair), `Encryption` (wrong password decrypt), `Mnemonic` (invalid checksum phrase), `Derivation` (malformed BIP-44 path), `InvalidFormat` (unrecognized key format), `SDKError` (wrapped SDK error). Verify each returns the correct error kind and descriptive message. Verify no error message contains private key material. File: `internal/keylib/integration_test.go` (append error integration) (FR-008, FR-008a, SEC-003, SEC-005, SC-002)

### Quickstart Validation

- [x] T045 Validate all code examples in `specs/002-key-library/quickstart.md` against implemented API: verify function signatures match, verify usage patterns compile, verify expected outputs are accurate. File: `internal/keylib/integration_test.go` (append quickstart validation) (SC-001, SC-006)

**Checkpoint**: `go test ./internal/keylib/... -v` all pass (0 failures). All 10 success criteria (SC-001 through SC-010) verified. Deterministic signing confirmed. Full lifecycle works end-to-end.

---

## Dependencies & Execution Order

### Phase Dependency Graph

```
Phase 1 (Setup)
  T001 → T002 + T003 (parallel after module init)
    │
    v
Phase 2 (Foundational) ── BLOCKS all user story phases
  T004 + T005 + T006 (all parallelizable)
    │
    ├─────────────────────┬────────────────────┬────────────────────┐
    v                     v                    v                    v
Phase 3 (US1, P1)    Phase 4 (US2, P2)   Phase 5 (US3, P3)   Phase 6 (US4, P4)
  T007..T022            T023..T027          T028..T033           T034..T037
    │                     │                    │                    │
    │  US2 depends on     │                    │                    │
    │  US1 core types     │                    │                    │
    │─────────────────────│                    │                    │
    │                                          │                    │
    │  US3 depends on US1 core types           │                    │
    │──────────────────────────────────────────│                    │
    │                                                               │
    │  US4 depends on US1 NeuronPrivateKey type                     │
    │───────────────────────────────────────────────────────────────│
    │
    v
Phase 7 (Polish)
  T038..T045 ── depend on ALL prior phases
```

### User Story Dependencies

| User Story | Priority | Depends On | Can Parallel With |
|------------|----------|------------|-------------------|
| US1 (Type-Safe Key Ops) | P1 | Phase 2 only | -- (first story) |
| US2 (Cross-Ecosystem) | P2 | US1 core types (T015-T022) | US3, US4 (after US1 complete) |
| US3 (Key Gen & Recovery) | P3 | US1 NeuronPrivateKey (T015) | US2, US4 (after US1 complete) |
| US4 (Multisig Interop) | P4 | US1 NeuronPrivateKey (T015) | US2, US3 (after US1 complete) |

### Within-Story Ordering Rules (TDD: Red-Green-Refactor)

Every component follows strict ordering within each user story phase:

1. **Red**: Write `*_test.go` tests that define expected behavior -- tests MUST FAIL (no implementation yet)
2. **Green**: Implement `*.go` source to make all tests pass
3. **Refactor**: Clean up implementation while keeping tests green

| Test Task | Implementation Task | Reason |
|-----------|-------------------|--------|
| T007 (PrivateKey tests) | T015 (PrivateKey impl) | TDD: test before implementation |
| T008 (PublicKey tests) | T016 (PublicKey impl) | TDD: test before implementation |
| T009 (EVMAddress tests) | T017 (EVMAddress impl) | TDD: test before implementation |
| T010 (PeerID tests) | T018 (PeerID impl) | TDD: test before implementation |
| T011 (DIDKey tests) | T019 (DIDKey impl) | TDD: test before implementation |
| T012 (Signature tests) | T020 (Signature impl) | TDD: test before implementation |
| T013 (Sign tests) | T021 (Sign impl) | T021 also depends on T020 (Signature type) |
| T014 (Matching tests) | T022 (Matching impl) | TDD: test before implementation |
| T023 (Conversion tests) | T026 (Conversion impl) | TDD: test before implementation |
| T024 (Cross-format tests) | T027 (Cross-format impl) | TDD: test before implementation |
| T028 (KeyGen tests) | T031 (KeyGen impl) | TDD: test before implementation |
| T029 (Mnemonic tests) | T032 (Mnemonic impl) | TDD: test before implementation |
| T030 (Encrypt tests) | T033 (Encrypt impl) | TDD: test before implementation |
| T034 (Multisig tests) | T036 (Multisig impl) | TDD: test before implementation |
| T035 (Multisig conv tests) | T037 (Multisig conv impl) | TDD: test before implementation |

### Parallel Execution Examples

**Example 1: Phase 2 -- All foundational tasks in parallel**

```text
Developer A: T004 (error tests) → T005 (error impl)
Developer B: T006 (constants)
```

**Example 2: Phase 3 US1 -- After NeuronPrivateKey pair (T007→T015)**

```text
# After T015 (NeuronPrivateKey) is complete, launch parallel component pairs:
Developer A: T008→T016 (PublicKey)    then T012→T020 (Signature)    then T013→T021 (Sign)
Developer B: T009→T017 (EVMAddress)   then T010→T018 (PeerID)       then T011→T019 (DID:key)
Developer C: T014→T022 (Matching) -- can start after core types exist
```

**Example 3: After US1 complete -- US3 and US4 in parallel**

```text
# US2 depends on all US1 types, but US3 and US4 only need NeuronPrivateKey
Developer A: Phase 4 US2 (T023→T027) -- cross-ecosystem conversions
Developer B: Phase 5 US3 (T028→T033) -- key gen, mnemonic, encryption
Developer C: Phase 6 US4 (T034→T037) -- multisig interoperability
```

**Example 4: Phase 7 -- All integration tests in parallel**

```text
T038 (deterministic signing), T039 (lifecycle), T040 (mnemonic),
T041 (matching), T042 (blockchain), T043 (multisig), T044 (errors), T045 (quickstart)
-- all independent, all parallelizable
```

### Implementation Strategy

#### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T006)
3. Complete Phase 3: US1 -- Type-Safe Key Ops (T007-T022)
4. **STOP and VALIDATE**: Create NeuronPrivateKey from hex, derive all types, sign, verify, match. SC-001, SC-002, SC-006, SC-007, SC-009, SC-010 pass.
5. Deploy/demo if ready.

#### Incremental Delivery

1. **Setup + Foundational** -> Foundation ready (T001-T006)
2. **Add US1** (Type-Safe Key Ops) -> Validate SC-001, SC-002, SC-006, SC-007, SC-009, SC-010 -> **MVP!**
3. **Add US2** (Cross-Ecosystem) -> Validate SC-003, SC-005, SC-008 -> Full conversion pipeline
4. **Add US3** (Key Gen & Recovery) -> Validate SC-004 -> Production-ready key management
5. **Add US4** (Multisig Interop) -> Validate SC-002 (multisig errors), SC-008 (multisig round-trip) -> Complete type system
6. **Polish** -> T038 (Constitution X deterministic signing), T039-T045 (integration) -> All 10 SCs verified

#### TDD Cadence (Constitution Principle IX)

Every component follows strict Red-Green-Refactor:
1. **Red**: Write `*_test.go` with failing tests that define expected behavior
2. **Green**: Implement `*.go` source to make all tests pass
3. **Refactor**: Clean up implementation while keeping tests green

---

## File-to-Requirement Traceability Matrix

| File | FRs Covered | SECs Covered | SCs Covered |
|------|-------------|-------------|-------------|
| `errors.go` / `errors_test.go` | FR-008, FR-008a | SEC-003, SEC-005 | SC-002 |
| `constants.go` | FR-001, FR-005, FR-006a, FR-013, FR-014, FR-015 | -- | -- |
| `private_key.go` / `private_key_test.go` | FR-001..004a, FR-008..012, FR-014, FR-017, FR-018, FR-020, FR-021, FR-022 | SEC-002, SEC-003, SEC-005, SEC-007 | SC-001, SC-002, SC-004, SC-006, SC-008 |
| `public_key.go` / `public_key_test.go` | FR-001, FR-002, FR-004a, FR-009..011, FR-018, FR-022 | SEC-005, SEC-007 | SC-001, SC-006 |
| `evm_address.go` / `evm_address_test.go` | FR-005, FR-008, FR-019, FR-020 | SEC-005 | SC-001, SC-003, SC-007 |
| `peer_id.go` / `peer_id_test.go` | FR-006, FR-019 | -- | SC-001, SC-003 |
| `did_key.go` / `did_key_test.go` | FR-006a | -- | SC-001 |
| `signature.go` / `signature_test.go` | FR-014, FR-017 | SEC-002, SEC-004 | SC-009, SC-010 |
| `matching.go` / `matching_test.go` | FR-007, FR-016 | SEC-004 | SC-003, SC-005, SC-007 |
| `mnemonic.go` / `mnemonic_test.go` | FR-013 | SEC-002, SEC-006 | SC-004 |
| `encrypted_key.go` / `encrypted_key_test.go` | FR-015 | SEC-006 | SC-004 |
| `multisig_key.go` / `multisig_key_test.go` | FR-023, FR-024, FR-004a, FR-008 | -- | SC-002, SC-008 |
| `signing_test.go` | FR-014, FR-017 | -- | SC-010, Constitution X |
| `integration_test.go` | FR-001, FR-005..008a, FR-011..017, FR-021, FR-023, FR-024 | SEC-003, SEC-005 | SC-001..SC-009 |

## Success Criteria Coverage

| SC | Primary Tasks | Verified In |
|----|--------------|-------------|
| SC-001 (type-safe conversions) | T007-T011, T015-T019 | T039 (lifecycle integration) |
| SC-002 (descriptive errors) | T004, T034, T044 | T044 (error integration) |
| SC-003 (deterministic cross-ecosystem) | T009, T010, T024, T025 | T039, T042 |
| SC-004 (key gen/mnemonic/encrypt) | T028-T030 | T039, T040 |
| SC-005 (matching functions) | T014, T024, T025 | T041 (matching integration) |
| SC-006 (no string-based ops) | T007, T008 | T039 (lifecycle integration) |
| SC-007 (type confusion prevention) | T009, T014 | T041 (matching integration) |
| SC-008 (blockchain round-trip) | T023, T035 | T042 (blockchain round-trip) |
| SC-009 (public key recovery) | T012, T013 | T039, T040 |
| SC-010 (signature verification) | T012, T038 | T038 (deterministic signing) |

## Risk Notes

- **GAP-005**: MultisigKey `"secp256k1-aggregated"` aggregation algorithm is deferred. `EVMAddress()` and `PeerID()` on aggregated MultisigKey will return a "not yet implemented" error until the algorithm is specified. Tests T034 and T043 must account for this.
- **Argon2id Performance**: Encryption tests (T030) with default v1 parameters (64 MiB memory) may be slow (~100ms per operation). Consider using reduced parameters in test mode or `testing.Short()` gating.
- **External Dependencies**: `go-libp2p` and `go-ethereum` version compatibility must be verified during T001. Pin exact versions in `go.mod`.
- **Ed25519 Detection**: FR-004 requires detecting Ed25519 keys from external sources. The implementation in T015 must handle Go `ed25519.PrivateKey` and `*ecdsa.PrivateKey` with non-secp256k1 curve parameters.

## Total Task Count

| Phase | Tasks | Range |
|-------|-------|-------|
| Phase 1 (Setup) | 3 | T001-T003 |
| Phase 2 (Foundational) | 3 | T004-T006 |
| Phase 3 (US1 Type-Safe Key Ops) | 16 | T007-T022 |
| Phase 4 (US2 Cross-Ecosystem) | 5 | T023-T027 |
| Phase 5 (US3 Key Gen & Recovery) | 6 | T028-T033 |
| Phase 6 (US4 Multisig Interop) | 4 | T034-T037 |
| Phase 7 (Polish) | 8 | T038-T045 |
| **Total** | **45** | T001-T045 |
