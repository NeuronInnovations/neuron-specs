# Tasks: NeuronAccount Module (Spec 001)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `001-neuron-account-module` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)
**Input**: Design documents from `/specs/001-neuron-account-module/` (spec.md, plan.md, data-model.md, contracts/account-builder.md, research.md, quickstart.md)

---

## Constitution Compliance

| # | Principle | Application |
|---|-----------|-------------|
| VI | Go-first | All file paths use Go layout: `internal/account/` with colocated `*_test.go` files |
| VII | Strict Spec Compliance | Every task references FR-* or SC-* it satisfies |
| IX | Test-First TDD | Test tasks appear BEFORE implementation tasks in every phase |
| X | No signing in this spec | N/A -- account module does not perform signing. Dependency on Spec 002 `internal/keylib` noted |
| VIII | HCS Transport Binding | N/A -- account module is transport-agnostic |

## Cross-Spec Dependency

This module depends on **Spec 002** (`internal/keylib`): `NeuronPublicKey`, `MultisigKey`, `EVMAddress`, `PeerID`, `DID:key` generation. All tasks using these types assume `internal/keylib` is implemented or that test stubs/mocks are available.

## Task Format

```
- [x] T### [P?] [US#?] Description with exact Go file path (FR-XXX, SC-XXX)
```

- **[P]**: Can run in parallel with other [P] tasks in the same phase (different files, no dependencies)
- **[US#]**: Which user story this task belongs to (US1, US2, US4, US5, US6 -- matching spec story numbers)
- Every task references FR-* or SC-* it satisfies (Constitution VII)
- Every task includes exact Go file path in `internal/account/`
- Tests use `go test` with `testify` assertions (Constitution IX)

---

## Phase 1: Setup (Go Module Structure, Shared Types/Enums)

**Purpose**: Initialize the Go package skeleton, directory structure, and dependency wiring. No logic -- only scaffolding.

- [x] T001 Create `internal/account/` package directory and placeholder source files per plan.md: `account.go`, `builder.go`, `validation.go`, `did.go`, `ledger.go`, `registry.go`, `payment.go`, `serialization.go` (FR-011, SC-008)
- [x] T002 [P] Add Go package declaration `package account` in all `.go` files under `internal/account/`; add import stubs for `internal/keylib` (Spec 002 dependency), `math/big`, `encoding/json`, `time` (FR-001, FR-002)
- [x] T003 [P] Create colocated test file stubs under `internal/account/`: `account_test.go`, `builder_test.go`, `validation_test.go`, `did_test.go`, `ledger_test.go`, `registry_test.go`, `payment_test.go`, `serialization_test.go`, `integration_test.go` (SC-008)

---

## Phase 2: Foundational (AccountType Enum, Error Types, Base NeuronAccount Struct)

**Purpose**: Define the core types that every user story depends on. MUST complete before any story phase begins.

**CRITICAL**: No user story work can begin until this phase is complete.

### Tests (Red)

- [x] T004 Write tests in `internal/account/account_test.go`: verify AccountType enum values (Parent=1, Child=2, Shared=3), verify Unspecified/0 is rejected as invalid, verify string representations (FR-013)
- [x] T005 [P] Write tests in `internal/account/validation_test.go`: verify ValidationError type has Field name, RuleCode (V-PARENT-*, V-CHILD-*, V-SHARED-*), and human-readable Message; verify error messages are actionable (FR-014)
- [x] T006 [P] Write tests in `internal/account/ledger_test.go`: verify AttachmentState enum (attached/detached), VerificationStatus enum (verified/unverified/failed), LedgerAttachment struct field presence including lastSyncedAt (FR-018, FR-016)

### Implementation (Green)

- [x] T007 Implement `AccountType` enum (Parent=1, Child=2, Shared=3) with Unspecified=0 rejection and String() method in `internal/account/account.go` (FR-013)
- [x] T008 [P] Implement `NeuronAccount` struct skeleton in `internal/account/account.go` with all fields per data-model.md: `accountType`, `publicKey`, `evmAddress`, `peerID`, `did`, `parentPubKey`, `multisigKey`, `currencySymbol`, `creditBalance`, `balanceAllocation`, `balance`, `ledgerAttachment`, `registryBinding`, `feePayer`, `p2pHost` (FR-001, FR-002, FR-007a, FR-008, FR-013, FR-020, FR-022, FR-026)
- [x] T009 [P] Implement `ValidationError` struct with `Field`, `RuleCode`, `Message` fields and `[]ValidationError` return type in `internal/account/validation.go` (FR-014)
- [x] T010 [P] Implement `LedgerAttachment` struct in `internal/account/ledger.go` with fields: `ledgerIdentifier` (string), `attachedAddress` (EVMAddress), `state` (AttachmentState), `verificationStatus` (VerificationStatus), `lastSyncedAt` (*time.Time) (FR-018, FR-016)
- [x] T011 [P] Implement `RegistryBinding` struct in `internal/account/registry.go` with fields: `registryIdentifier` (string), `externalID` (string) (FR-022)
- [x] T012 [P] Implement `NeuronDID` struct in `internal/account/did.go` with fields: `identifier` (string in `did:key:zQ3s...` format), `document` (DIDDocument) (FR-012)
- [x] T013 [P] Implement `LedgerVerifier` interface and `ParentChildVerifier` interface in `internal/account/ledger.go` with injectable verification signatures for ledger attachment and parent-child relationship verification (FR-017, FR-019)

**Checkpoint**: Foundation ready -- all shared types defined, enum values tested, error types available. User story implementation can now begin.

---

## Phase 3: US1 -- Create Parent Account with Identity and Financial Management (Priority: P1)

**Goal**: A developer can create a Parent account with cryptographic identity (NeuronPublicKey), DID document, currency symbol, and credit balance management using a fluent builder API. Parent accounts serve as the root identity and primary financial account holder.

**Independent Test**: Create a Parent account via builder, verify DID is present, verify credit balance capability exists (balance undefined before sync), verify no reachability/comms data. Covers SC-001, SC-008.

**Satisfies**: FR-001, FR-006, FR-008, FR-011, FR-011a, FR-012, FR-013, FR-016, FR-020, SC-001, SC-008

### Tests (Red)

- [x] T014 [US1] Write tests in `internal/account/did_test.go`: verify `GenerateDID(NeuronPublicKey)` produces `did:key:zQ3s...` format identifier from secp256k1 key via Spec 002 `DIDKey()`; verify invalid key input returns error (FR-012, FR-001)
- [x] T015 [US1] Write tests in `internal/account/builder_test.go` for `NewParentAccountBuilder()`: verify fluent chain `.WithPublicKey().WithDID().WithCurrency().Build()` returns valid NeuronAccount with `accountType=Parent`; verify `evmAddress` and `peerID` are derived from publicKey via Spec 002 (FR-001, FR-006, FR-008, FR-011)
- [x] T016 [US1] Write tests in `internal/account/builder_test.go` for Parent builder validation errors: verify `.Build()` fails without publicKey, fails without DID, fails without currency; verify error messages are actionable per FR-014 (FR-006, FR-014)
- [x] T017 [US1] Write tests in `internal/account/builder_test.go` for Parent builder negative cases: verify `.Build()` fails if parentPubKey is set (V-PARENT-04: Parent MUST NOT have parent reference), fails if multisigKey is set (V-PARENT-05: Parent MUST NOT have multisig key) (FR-006)
- [x] T018 [US1] Write tests in `internal/account/builder_test.go` for `BuildComplete()`: verify Parent is complete only when `ledgerAttachment` + `DID` + `currency` + credit balance capability are present; verify `BuildComplete()` fails when ledger attachment is missing with clear report of what is missing (FR-011a, SC-001)
- [x] T019 [US1] Write tests in `internal/account/account_test.go`: verify Parent account has no reachability/comms data stored; verify `creditBalance` is nil/undefined before ledger sync (balance is NEVER set at construction) (FR-001, FR-016)

### Implementation (Green)

- [x] T020 [US1] Implement `GenerateDID(NeuronPublicKey) (NeuronDID, error)` in `internal/account/did.go` using Spec 002 `DIDKey()` to produce `did:key:zQ3s...` format; validate input key (FR-012, FR-001)
- [x] T021 [US1] Implement `NewParentAccountBuilder()` returning `ParentAccountBuilder` in `internal/account/builder.go` with fluent methods: `.WithPublicKey(NeuronPublicKey)`, `.WithDID(NeuronDID)`, `.WithCurrency(string)`, `.WithLedgerAttachment(LedgerAttachment)` (FR-001, FR-006, FR-011, FR-020)
- [x] T022 [US1] Implement `ParentAccountBuilder.Build() (NeuronAccount, error)` in `internal/account/builder.go`: validate MUST fields (publicKey, DID, currency per V-PARENT-01..02), derive `evmAddress` and `peerID` from publicKey via Spec 002, reject parentPubKey (V-PARENT-04) and multisigKey (V-PARENT-05) if set, set `accountType=Parent`, initialize `creditBalance` as nil (FR-006, FR-008, FR-011, FR-016)
- [x] T023 [US1] Implement `ParentAccountBuilder.BuildComplete() (NeuronAccount, error)` in `internal/account/builder.go`: call `Build()` then additionally require `ledgerAttachment` is present; report missing attachments with clear error (FR-011a, SC-001)

**Checkpoint**: US1 complete -- Parent accounts can be created with DID, credit balance capability, and identity derivation. SC-001 and SC-008 are verifiable.

---

## Phase 4: US2 -- Create Child Account for Agents and Devices with Relationship Verification (Priority: P2)

**Goal**: A developer can create a Child account referencing a Parent's NeuronPublicKey, with p2pHost identity (= PeerID derived from same key), balance allocation, optional fee payer, registry binding, and parent-child verification against on-ledger data.

**Independent Test**: Create a Child account referencing a Parent, verify parent reference is stored, verify `p2pHost` identity = `peerID` (derived from same NeuronPublicKey), verify no DID required, verify parent-child verification against mock ledger returns verified/unverified/failed. Covers SC-002, SC-008.

**Satisfies**: FR-002, FR-007, FR-008, FR-011, FR-011a, FR-013, FR-016, FR-017, FR-020, FR-022, FR-026, SC-002, SC-008

### Tests (Red)

- [x] T024 [US2] Write tests in `internal/account/builder_test.go` for `NewChildAccountBuilder()`: verify fluent chain `.WithPublicKey().WithParentPublicKey().WithCurrency().Build()` returns valid NeuronAccount with `accountType=Child`; verify `p2pHost` = `peerID` derived from same publicKey (one key, no separate host identity); verify `evmAddress` derived from publicKey (FR-002, FR-007, FR-008, FR-011)
- [x] T025 [US2] Write tests in `internal/account/builder_test.go` for Child builder validation errors: verify `.Build()` fails without publicKey, fails without parentPubKey (V-CHILD-01), fails without currency; verify error messages are actionable per FR-014 (FR-007, FR-014)
- [x] T026 [US2] Write tests in `internal/account/builder_test.go` for Child builder negative cases: verify `.Build()` fails if DID is set (V-CHILD-04: Child MUST NOT have DID), fails if multisigKey is set (V-CHILD-05: Child MUST NOT have multisig key) (FR-007)
- [x] T027 [US2] Write tests in `internal/account/builder_test.go` for Child `BuildComplete()`: verify Child is complete only when `ledgerAttachment` + `parentPubKey` + `currency` + balance allocation capability + `registryBinding` are ALL present; verify `BuildComplete()` fails when any is missing with clear report (FR-011a, FR-022, SC-002)
- [x] T028 [US2] Write tests in `internal/account/builder_test.go` for `.WithFeePayer(LedgerAccountId)`: verify fee payer is stored and retrievable on the account; verify it is optional (account valid without it) (FR-026)
- [x] T029 [US2] Write tests in `internal/account/ledger_test.go` for `VerifyParentChild(child, verifier)`: verify returns `verified` when mock verifier confirms first-funder or creator relationship; verify returns `unverified` when ledger/registry unavailable; verify returns `failed` when relationship not found on ledger (FR-017)
- [x] T030 [US2] Write tests in `internal/account/account_test.go`: verify Child account does NOT store reachability/comms data; verify `balanceAllocation` is nil/undefined before ledger sync; verify `p2pHost` identity equals `peerID` (same key derivation) (FR-002, FR-016)

### Implementation (Green)

- [x] T031 [US2] Implement `NewChildAccountBuilder()` returning `ChildAccountBuilder` in `internal/account/builder.go` with fluent methods: `.WithPublicKey(NeuronPublicKey)`, `.WithParentPublicKey(NeuronPublicKey)`, `.WithCurrency(string)`, `.WithRegistryBinding(RegistryBinding)`, `.WithFeePayer(LedgerAccountId)`, `.WithLedgerAttachment(LedgerAttachment)` (FR-002, FR-007, FR-011, FR-020, FR-022, FR-026)
- [x] T032 [US2] Implement `ChildAccountBuilder.Build() (NeuronAccount, error)` in `internal/account/builder.go`: validate MUST fields (publicKey V-CHILD-02, parentPubKey V-CHILD-01, currency), derive `evmAddress`, `peerID`, and `p2pHost` (= peerID, same key) from publicKey via Spec 002, reject DID (V-CHILD-04) and multisigKey (V-CHILD-05) if set, set `accountType=Child`, initialize `balanceAllocation` as nil (FR-007, FR-008, FR-011, FR-016)
- [x] T033 [US2] Implement `ChildAccountBuilder.BuildComplete() (NeuronAccount, error)` in `internal/account/builder.go`: call `Build()` then additionally require `ledgerAttachment` and `registryBinding` (V-CHILD-03) are present; report missing attachments (FR-011a, FR-022, SC-002)
- [x] T034 [US2] Implement `VerifyParentChild(child NeuronAccount, verifier ParentChildVerifier) (VerificationResult, error)` in `internal/account/ledger.go`: delegate to injected verifier for first-funder or creator check; return verified/unverified/failed based on verifier response; return distinct `unverified` when ledger/registry unavailable (FR-017)

**Checkpoint**: US2 complete -- Child accounts can be created with parent reference, p2pHost identity, fee payer, and parent-child verification. SC-002 and SC-008 are verifiable.

---

## Phase 5: US4 -- Validate Account Structure and Relationships (Priority: P2)

**Goal**: A developer can validate any account type against its type-specific rules and receive clear, actionable error messages for ALL violations in a single call. Includes ledger attachment semantics verification (public key match for Parent/Child, multisig config match for Shared).

**Independent Test**: Create various invalid account configurations (missing DID for Parent, missing parent ref for Child, missing MultisigKey for Shared, semantics mismatch between Neuron and ledger), verify all violations are caught with specific RuleCode and actionable messages. Covers SC-005, SC-010.

**Satisfies**: FR-006, FR-007, FR-007a, FR-014, FR-019, SC-005, SC-009, SC-010

### Tests (Red)

- [x] T035 [US4] Write tests in `internal/account/validation_test.go` for Parent validation rules: V-PARENT-01 (has DID), V-PARENT-02 (has single NeuronPublicKey), V-PARENT-03 (has credit balance capability), V-PARENT-04 (no parent reference), V-PARENT-05 (no multisig key); verify each rule returns correct `RuleCode` and actionable message (FR-006, FR-014)
- [x] T036 [US4] Write tests in `internal/account/validation_test.go` for Child validation rules: V-CHILD-01 (has parent NeuronPublicKey ref), V-CHILD-02 (has single NeuronPublicKey), V-CHILD-03 (has registry binding), V-CHILD-04 (no DID required -- DID presence is a violation), V-CHILD-05 (no multisig key); verify each rule returns correct `RuleCode` and actionable message (FR-007, FR-014)
- [x] T037 [US4] Write tests in `internal/account/validation_test.go` for Shared validation rules: V-SHARED-01 (has MultisigKey with threshold), V-SHARED-02 (no DID), V-SHARED-03 (no parent reference), V-SHARED-04 (no single NeuronPublicKey); verify each rule returns correct `RuleCode` and actionable message (FR-007a, FR-014)
- [x] T038 [US4] Write tests in `internal/account/validation_test.go` for valid accounts: verify `Validate()` returns empty `[]ValidationError` for valid Parent, valid Child (with registry binding), valid Shared accounts (FR-006, FR-007, FR-007a, SC-005)
- [x] T039 [US4] Write tests in `internal/account/validation_test.go` for multi-violation detection: verify `Validate()` returns ALL violations in a single call (not just the first one); verify Unspecified accountType (0) is rejected as invalid (FR-013, FR-014, SC-005)
- [x] T040 [US4] Write tests in `internal/account/ledger_test.go` for `VerifyLedgerAttachment(verifier)`: verify public key semantics match check -- Parent/Child publicKey derivation (EVMAddress) MUST match ledger account; Shared multisig config MUST match ledger multisig structure; verify failure on mismatch with clear error (FR-019, SC-010)
- [x] T041 [US4] Write tests in `internal/account/ledger_test.go` for detached account verification: verify `VerifyLedgerAttachment` returns clear error when account is in detached state; verify balance is last-known cache when detached; verify `lastSyncedAt` is interpretable (FR-018, FR-016)

### Implementation (Green)

- [x] T042 [US4] Implement `Validate() []ValidationError` method on `NeuronAccount` in `internal/account/validation.go`: dispatch to type-specific validation based on `accountType`; reject Unspecified(0) as invalid; collect ALL violations (FR-006, FR-007, FR-007a, FR-013, FR-014)
- [x] T043 [US4] Implement Parent validation rules in `internal/account/validation.go`: V-PARENT-01 (has DID), V-PARENT-02 (has single NeuronPublicKey), V-PARENT-03 (has credit balance capability), V-PARENT-04 (no parent reference), V-PARENT-05 (no multisig key) per data-model.md validation table (FR-006)
- [x] T044 [US4] Implement Child validation rules in `internal/account/validation.go`: V-CHILD-01 (has parent NeuronPublicKey ref), V-CHILD-02 (has single NeuronPublicKey), V-CHILD-03 (has registry binding), V-CHILD-04 (no DID), V-CHILD-05 (no multisig key) per data-model.md validation table (FR-007)
- [x] T045 [US4] Implement Shared validation rules in `internal/account/validation.go`: V-SHARED-01 (has MultisigKey with threshold), V-SHARED-02 (no DID), V-SHARED-03 (no parent reference), V-SHARED-04 (no single NeuronPublicKey) per data-model.md validation table (FR-007a)
- [x] T046 [US4] Implement `VerifyLedgerAttachment(verifier LedgerVerifier) (VerificationResult, error)` method on `NeuronAccount` in `internal/account/ledger.go`: verify account exists on ledger, verify ownership proof via EVMAddress, verify public key semantics match (Parent/Child) or multisig config match (Shared), reject if detached with clear error (FR-019, SC-009, SC-010)

**Checkpoint**: US4 complete -- all 16 validation rules (V-PARENT-01..05, V-CHILD-01..05, V-SHARED-01..04, plus Unspecified rejection and multi-violation) enforced, all violations caught with actionable messages, ledger semantics verified. SC-005, SC-009, SC-010 are verifiable.

---

## Phase 6: US5 -- Create Shared Account with Multisig Threshold Keys (Priority: P3)

**Goal**: A developer can create a Shared account (multisig threshold account) with MultisigKey (e.g. 2-of-3), currency symbol, and balance capability, using a fluent builder. Shared accounts have no DID, no parent reference, no single NeuronPublicKey, and no reachability/comms.

**Independent Test**: Create a Shared account with 2-of-3 MultisigKey, verify no DID, no parent ref, no single publicKey, no reachability, passes validation, can attach to ledger and verify multisig config match. Covers SC-005 (Shared validation path), SC-009 (Shared ledger attachment), SC-010 (Shared multisig semantics).

**Satisfies**: FR-007a, FR-011, FR-011a, FR-013, FR-020, FR-021, SC-005

### Tests (Red)

- [x] T047 [US5] Write tests in `internal/account/builder_test.go` for `NewSharedAccountBuilder()`: verify fluent chain `.WithMultisigKey().WithCurrency().Build()` returns valid NeuronAccount with `accountType=Shared`; verify no DID, no parent ref, no single publicKey; verify `balance` is nil before sync (FR-007a, FR-011, FR-021)
- [x] T048 [US5] Write tests in `internal/account/builder_test.go` for Shared builder validation errors: verify `.Build()` fails without multisigKey (V-SHARED-01), fails without currency; verify error messages are actionable per FR-014 (FR-007a, FR-014)
- [x] T049 [US5] Write tests in `internal/account/builder_test.go` for Shared builder negative cases: verify `.Build()` fails if DID is set (V-SHARED-02), fails if parentPubKey is set (V-SHARED-03), fails if single publicKey is set (V-SHARED-04) (FR-007a)
- [x] T050 [US5] Write tests in `internal/account/builder_test.go` for Shared `BuildComplete()`: verify Shared is complete only when `ledgerAttachment` + `multisigKey` + `currency` + balance capability are present; verify `BuildComplete()` fails when ledger attachment is missing with clear report (FR-011a)

### Implementation (Green)

- [x] T051 [US5] Implement `NewSharedAccountBuilder()` returning `SharedAccountBuilder` in `internal/account/builder.go` with fluent methods: `.WithMultisigKey(MultisigKey)`, `.WithCurrency(string)`, `.WithLedgerAttachment(LedgerAttachment)` (FR-007a, FR-011, FR-020, FR-021)
- [x] T052 [US5] Implement `SharedAccountBuilder.Build() (NeuronAccount, error)` in `internal/account/builder.go`: validate MUST fields (multisigKey V-SHARED-01, currency), reject DID (V-SHARED-02), parentPubKey (V-SHARED-03), and publicKey (V-SHARED-04) if set, set `accountType=Shared`, initialize `balance` as nil (FR-007a, FR-011, FR-021)
- [x] T053 [US5] Implement `SharedAccountBuilder.BuildComplete() (NeuronAccount, error)` in `internal/account/builder.go`: call `Build()` then additionally require `ledgerAttachment` is present; report missing attachments with clear error (FR-011a)

**Checkpoint**: US5 complete -- Shared accounts can be created with MultisigKey threshold config. SC-005 (Shared path) is verifiable.

---

## Phase 7: US6 -- Registry Binding and Payment Address for Peer Registries (Priority: P3)

**Goal**: A developer can set a registry binding on a Child account and resolve the payment address (Parent's evmAddress) for use by peer registries (e.g. ERC-8004 compatible). Parent's payment address is own evmAddress. Shared returns error (no payment address defined).

**Independent Test**: Add registry binding to a Child, resolve Child's payment address to Parent's evmAddress, verify Parent's payment address is own evmAddress (attached address), verify Shared returns error. Covers SC-011, SC-012.

**Satisfies**: FR-022, FR-023, FR-024, FR-025, FR-026, SC-011, SC-012

### Tests (Red)

- [x] T054 [US6] Write tests in `internal/account/registry_test.go`: verify RegistryBinding stores `registryIdentifier` and `externalID`; verify binding is retrievable from a Child account; verify Child is NOT complete without binding (FR-011a check); verify empty registryIdentifier is rejected (FR-022, SC-012)
- [x] T055 [US6] Write tests in `internal/account/payment_test.go` for `PaymentAddress()` on Parent: verify returns Parent's own `evmAddress` (attached address when attached to ledger); verify returns error when Parent is not attached to ledger (FR-023, SC-011)
- [x] T056 [US6] Write tests in `internal/account/payment_test.go` for `PaymentAddress()` on Child: verify resolves to Parent's `evmAddress` from `parentPubKey`; verify returns error when Parent reference cannot be resolved; verify returns error when Parent has no attached address (not attached to ledger) (FR-024, SC-011)
- [x] T057 [US6] Write tests in `internal/account/payment_test.go` for `PaymentAddress()` on Shared: verify returns error -- Shared accounts have no payment address defined (FR-023, FR-024)
- [x] T058 [US6] Write tests in `internal/account/payment_test.go` for edge cases: verify payment address resolution when Parent is detached from ledger returns error; verify registry binding with empty registryIdentifier is rejected; verify registry binding with empty externalID is rejected (FR-022, FR-023)

### Implementation (Green)

- [x] T059 [US6] Implement `PaymentAddress() (EVMAddress, error)` method on `NeuronAccount` in `internal/account/payment.go`: Parent returns own `evmAddress` (attached address); Child resolves Parent's `evmAddress` from `parentPubKey`; Shared returns error with clear message (FR-023, FR-024, SC-011)
- [x] T060 [US6] Implement registry binding storage and retrieval on `NeuronAccount` in `internal/account/registry.go`: expose `RegistryBinding()` getter; validate non-empty registryIdentifier and externalID; integrate with Child completeness check in builder (FR-022, FR-025, SC-012)

**Checkpoint**: US6 complete -- registry binding and payment address resolution work for all account types. SC-011 and SC-012 are verifiable.

---

## Phase 8: Polish (Serialization, Ledger Attachment State, Integration Tests, Quickstart Validation)

**Purpose**: JSON serialization/deserialization round-trip, ledger attachment state management, full lifecycle integration tests, performance verification, and quickstart example validation.

**Satisfies**: FR-015, FR-016, FR-018, SC-007, SC-008

### Serialization Tests (Red) and Implementation (Green)

- [x] T061 Write tests in `internal/account/serialization_test.go`: verify `Serialize()` produces canonical JSON for Parent, Child, Shared accounts; verify NeuronPublicKey fields are compressed hex strings (Spec 002 FR-010); verify MultisigKey includes threshold and constituent keys; verify all fields round-trip correctly (FR-015, SC-007)
- [x] T062 Write tests in `internal/account/serialization_test.go`: verify `Deserialize()` reconstructs NeuronAccount from JSON; verify derived fields (`evmAddress`, `peerID`, `p2pHost`) are re-derived from deserialized publicKey; verify data integrity -- no loss or corruption (FR-015, SC-007)
- [x] T063 Write tests in `internal/account/serialization_test.go` for edge cases: verify serialization of account with nil `creditBalance`, nil `ledgerAttachment`, nil `registryBinding`, nil `feePayer`; verify deserialization handles unknown/extra fields gracefully (FR-015)
- [x] T064 Implement `Serialize() ([]byte, error)` (via `json.Marshal`) and `Deserialize([]byte) (NeuronAccount, error)` (via `json.Unmarshal`) in `internal/account/serialization.go`: canonical JSON format; custom `MarshalJSON`/`UnmarshalJSON` for NeuronPublicKey as compressed hex, EVMAddress as EIP-55, MultisigKey with threshold and keys; handle nilable *big.Int and *time.Time fields (FR-015, SC-007)

### Ledger Attachment State Tests (Red) and Implementation (Green)

- [x] T065 Write tests in `internal/account/ledger_test.go` for ledger attachment state transitions: verify attach (detached -> attached), verify detach (attached -> detached), verify `lastSyncedAt` is tracked and updated on sync, verify balance becomes stale/undefined when detached (FR-018, FR-016)
- [x] T066 Implement ledger attachment/detachment methods on `NeuronAccount` in `internal/account/ledger.go`: `AttachToLedger(LedgerAttachment)`, `DetachFromLedger()`, update `lastSyncedAt` on sync; balance becomes stale on detach; expose `lastSyncedAt` for caller interpretation (FR-018, FR-016)

### Integration Tests

- [x] T067 Write integration tests in `internal/account/integration_test.go`: full Parent lifecycle -- build via `NewParentAccountBuilder()` -> validate via `Validate()` -> attach to ledger -> verify attachment via `VerifyLedgerAttachment()` -> resolve payment address -> serialize to JSON -> deserialize from JSON -> re-validate (SC-001, SC-007, SC-008, SC-009, SC-011)
- [x] T068 Write integration tests in `internal/account/integration_test.go`: full Child lifecycle -- build via `NewChildAccountBuilder()` with parent ref and registry binding -> validate -> attach to ledger -> verify parent-child relationship via `VerifyParentChild()` -> resolve payment address (= Parent's evmAddress) -> serialize -> deserialize -> re-validate (SC-002, SC-007, SC-008, SC-009, SC-011, SC-012)
- [x] T069 Write integration tests in `internal/account/integration_test.go`: full Shared lifecycle -- build via `NewSharedAccountBuilder()` with MultisigKey -> validate -> attach to ledger -> verify multisig config match via `VerifyLedgerAttachment()` -> serialize -> deserialize -> re-validate (SC-007, SC-008, SC-009, SC-010)
- [x] T070 Write integration tests in `internal/account/integration_test.go`: verify account creation and validation complete in under 100ms for typical configurations (excluding network I/O) using Go benchmarks / `time.Since()` assertion (SC-008)
- [x] T071 Write integration tests in `internal/account/integration_test.go`: cross-type negative tests -- verify Parent cannot be built with Child fields (parentPubKey, registryBinding), Child cannot be built with Shared fields (multisigKey), Shared cannot be built with Parent fields (DID, publicKey); verify Unspecified accountType (0) is rejected everywhere (FR-013, SC-005)
- [x] T072 Write integration tests in `internal/account/integration_test.go`: quickstart validation -- reproduce the code examples from quickstart.md (Parent creation, Child creation with parent ref and registry binding, ledger attachment, validation, JSON serialization) and verify they produce expected results (SC-001, SC-002, SC-007)

**Checkpoint**: Polish complete -- all accounts can be serialized/deserialized without data loss, ledger attachment works, full lifecycle validated, quickstart examples verified. SC-007, SC-008, SC-009 are verifiable.

---

## Dependencies & Execution Order

### Phase Dependency Graph

```
Phase 1: Setup (T001-T003)
  |
  v
Phase 2: Foundational (T004-T013) ── BLOCKS all user story phases
  |
  ├──> Phase 3: US1 (P1) -- Create Parent Account (T014-T023)
  |      |
  |      ├──> Phase 4: US2 (P2) -- Create Child Account (T024-T034)
  |      |      |     (needs Parent builder for parentPubKey reference)
  |      |      |
  |      ├──> Phase 5: US4 (P2) -- Validate Account Structure (T035-T046)
  |      |           (needs account types from Phase 3 for validation inputs)
  |      |
  |      ├──> Phase 7: US6 (P3) -- Registry & Payment (T054-T060)
  |      |           (needs Parent + Child builders from Phase 3 + Phase 4)
  |      |
  |      v
  ├──> Phase 6: US5 (P3) -- Create Shared Account (T047-T053)
  |           (needs foundational types only; independent of US1 builder)
  |
  v
Phase 8: Polish (T061-T072) ── needs ALL user story phases complete
```

### Cross-Spec Blocking Dependencies

| Dependency | Source | Status | Used By |
|------------|--------|--------|---------|
| `NeuronPublicKey`, `EVMAddress`, `PeerID`, `DID:key` generation | Spec 002 `internal/keylib` | REQUIRED -- must be available or mocked | T014, T015, T020-T023, T024, T031-T032, T055-T056, T061-T064, T067-T069 |
| `MultisigKey` with threshold configuration | Spec 002 `internal/keylib` | REQUIRED for Shared accounts | T047-T053, T069 |
| Peer Registry (registration, reachability) | Spec 003 | NOT REQUIRED -- account module is upstream | N/A |

### User Story Dependencies

| Story | Priority | Depends On | Can Parallel With |
|-------|----------|------------|-------------------|
| US1 (Phase 3) | P1 | Foundational only | Nothing (first story) |
| US2 (Phase 4) | P2 | Foundational + US1 | US4 (different files: builder_test.go vs validation_test.go) |
| US4 (Phase 5) | P2 | Foundational + US1 | US2 (different files: validation_test.go vs builder_test.go) |
| US5 (Phase 6) | P3 | Foundational only | US1, US2, US4 (different builder type, independent of Parent/Child) |
| US6 (Phase 7) | P3 | US1 + US2 (needs Parent and Child builders) | US5 (different files: payment_test.go, registry_test.go) |

---

## Parallel Opportunities

### Phase 2 (Foundational)
- T005, T006 (test stubs for `validation_test.go`, `ledger_test.go`) -- different files, no deps
- T008, T009, T010, T011, T012, T013 (NeuronAccount struct, ValidationError, LedgerAttachment, RegistryBinding, NeuronDID, interfaces) -- all different files, no deps between them

### Phase 3 + Phase 6 (US1 + US5)
- US1 (Parent builder in `builder.go`) and US5 (Shared builder in `builder.go`) operate on the same file but are independent builder types. Can be parallelized if developers coordinate on `builder.go` sections.

### Phase 4 + Phase 5 (US2 + US4)
- US2 (Child builder in `builder.go`, ledger verification in `ledger.go`) and US4 (validation rules in `validation.go`) operate on mostly different files and can run in parallel after US1 completes.

### Phase 7 (US6)
- T054 (`registry_test.go`) and T055-T058 (`payment_test.go`) -- different files, can parallelize.

### Phase 8 (Polish)
- T061-T063 (serialization tests) and T065 (ledger tests) -- different files, can parallelize.
- T067-T072 (integration tests) -- all in same file but independent test functions.

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T013)
3. Complete Phase 3: US1 -- Create Parent Account (T014-T023)
4. **STOP and VALIDATE**: Build a Parent account, generate DID, verify credit balance capability. SC-001 and SC-008 pass.
5. Deploy/demo if ready.

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US1 (Parent) -> Validate SC-001, SC-008 -> **MVP!**
3. Add US2 (Child) + US4 (Validate) in parallel -> Validate SC-002, SC-005, SC-009, SC-010 -> Hierarchical model works
4. Add US5 (Shared) -> Validate SC-005 (Shared path) -> All account types work
5. Add US6 (Registry + Payment) -> Validate SC-011, SC-012 -> Registry-ready
6. Polish -> Validate SC-007 (serialization), SC-008 (perf) -> Production-ready
7. Integration tests + quickstart validation -> Full lifecycle verified

### Parallel Team Strategy (2 developers)

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - **Developer A**: US1 (Parent builder) -> US2 (Child builder) -> US6 (Payment + Registry)
   - **Developer B**: US5 (Shared builder) -> US4 (Validation rules) -> Polish (Serialization + Ledger state)
3. Integration tests after both converge

### TDD Cadence (Constitution IX)

Within every user story phase:
1. **Red**: Write ALL failing tests for that phase
2. **Green**: Implement to pass ALL tests for that phase
3. **Refactor**: Clean up implementation while keeping tests green
4. **Checkpoint**: Verify independently before moving to next phase

---

## Task Count Summary

| Phase | Description | Tasks | Range |
|-------|-------------|-------|-------|
| Phase 1 | Setup | 3 | T001-T003 |
| Phase 2 | Foundational | 10 | T004-T013 |
| Phase 3 | US1 -- Parent Account (P1) | 10 | T014-T023 |
| Phase 4 | US2 -- Child Account (P2) | 11 | T024-T034 |
| Phase 5 | US4 -- Validation (P2) | 12 | T035-T046 |
| Phase 6 | US5 -- Shared Account (P3) | 7 | T047-T053 |
| Phase 7 | US6 -- Registry & Payment (P3) | 7 | T054-T060 |
| Phase 8 | Polish | 12 | T061-T072 |
| **Total** | | **72** | T001-T072 |

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks within the same phase
- [US#] label maps task to specific user story for traceability
- Each user story is independently completable and testable after its dependencies
- All file paths are relative to `internal/account/` within the neuron-go-sdk Go module
- Spec 002 dependency: all tasks using `NeuronPublicKey`, `MultisigKey`, `EVMAddress`, `PeerID`, or `DID:key` require `internal/keylib` (Spec 002) to be implemented or mocked
- Constitution X (Deterministic Signing) does NOT apply -- this module does not sign
- Constitution VIII (HCS binding) does NOT apply -- this module is transport-agnostic
- Balance is NEVER set at construction -- only via ledger sync (FR-016)
- Reachability/comms are NOT part of account completeness -- see Peer Registry (Spec 003)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
