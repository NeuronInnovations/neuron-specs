# Tasks: NeuronAccount Module — TypeScript (Spec 001)

**Date**: 2026-03-17 | **Spec**: [spec.md](spec.md) | **Plan**: [ts-plan.md](ts-plan.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

> **Constitution Compliance**: VII (FR traceability), IX (TDD Red→Green), XI (VR-* pending)

---

## Phase 1: Foundational (Blocking)

### Tests (Red)

- [x] T001 [P] Write failing tests for AccountError: all 8 NEURON-ACCT-* codes instantiate correctly, code/name properties set, messages descriptive. File: `impl/typescript/tests/account/error.test.ts` (FR-014, 006 error-taxonomy.md)
- [x] T002 [P] Write failing tests for types: AccountType enum (Parent=1, Child=2, Shared=3), invalid values rejected. NeuronDID constructed from did:key string. LedgerAttachment with ledgerIdentifier, attachedAddress (EVMAddress), state, verificationStatus, lastSyncedAt. RegistryBinding with registryIdentifier and externalId. File: `impl/typescript/tests/account/types.test.ts` (FR-013, FR-018, FR-022)

### Implementation (Green)

- [x] T003 [P] Implement AccountError extending NeuronError with NEURON-ACCT-001..008 codes and factory functions. File: `impl/typescript/src/account/errors.ts` (FR-014, 006 error-taxonomy.md)
- [x] T004 [P] Implement types: AccountType enum, NeuronDID (string identifier + validation for did:key:zQ3s prefix), LedgerAttachment interface, RegistryBinding interface, LedgerAccountId type alias, VerificationResult type. File: `impl/typescript/src/account/types.ts` (FR-013, FR-018, FR-020, FR-022)

**Checkpoint**: Error and type tests pass.

---

## Phase 2: US1 — Account Creation (Priority: P1) 🎯 MVP

**Goal**: Developers can create Parent, Child, and Shared accounts using fluent builders.

### Tests (Red)

- [x] T005 [P] [US1] Write failing tests for ParentAccountBuilder: `.withPublicKey().withDID().withCurrency().build()` creates valid Parent, missing DID throws NEURON-ACCT-002, missing publicKey throws NEURON-ACCT-002, parentPubKey set throws NEURON-ACCT-003, multisigKey set throws NEURON-ACCT-003. File: `impl/typescript/tests/account/builder.test.ts` (FR-001, FR-006, FR-011, SC-001, SC-005)
- [x] T006 [P] [US1] Write failing tests for ChildAccountBuilder: `.withPublicKey().withParentPublicKey().withCurrency().withRegistryBinding().build()` creates valid Child, missing parentPubKey throws NEURON-ACCT-002, DID set allowed (not error), multisigKey set throws NEURON-ACCT-003. `.buildComplete()` without registryBinding throws NEURON-ACCT-007. File: `impl/typescript/tests/account/builder.test.ts` (append) (FR-002, FR-007, FR-011, FR-022, SC-002, SC-005, SC-012)
- [x] T007 [P] [US1] Write failing tests for SharedAccountBuilder: `.withMultisigKey().withCurrency().build()` creates valid Shared, missing multisigKey throws NEURON-ACCT-002, DID set throws NEURON-ACCT-003, publicKey set throws NEURON-ACCT-003, parentPubKey set throws NEURON-ACCT-003. File: `impl/typescript/tests/account/builder.test.ts` (append) (FR-007a, FR-011, FR-021, SC-005)

### Implementation (Green)

- [x] T008 [US1] Implement NeuronAccount as discriminated union type with accountType discriminator. Immutable after construction. Derived fields: evmAddress() and peerId() delegate to publicKey (spec 002). File: `impl/typescript/src/account/account.ts` (FR-008, FR-013, FR-018, FR-022)
- [x] T009 [US1] Implement builders (ParentAccountBuilder, ChildAccountBuilder, SharedAccountBuilder) with fluent `.withX()` chaining. `.build()` validates type-specific rules (V-PARENT-01..05, V-CHILD-01..05, V-SHARED-01..04), collects all errors, returns NeuronAccount or throws. `.buildComplete()` adds completeness checks (FR-011a). File: `impl/typescript/src/account/builder.ts` (FR-001, FR-002, FR-006, FR-007, FR-007a, FR-011, FR-011a, FR-021)

**Checkpoint**: All three builders create valid accounts. Validation errors for missing/forbidden fields. SC-001, SC-002 pass.

---

## Phase 3: US4 — Validation (Priority: P2)

**Goal**: Full validation rule enforcement with error collection.

### Tests (Red)

- [x] T010 [P] [US4] Write failing tests for validation: account.validate() returns empty array for valid accounts, returns array of all violations for invalid (not just first). Test V-PARENT-01..05, V-CHILD-01..05, V-SHARED-01..04 individually. File: `impl/typescript/tests/account/validation.test.ts` (FR-006, FR-007, FR-007a, FR-014, SC-005)

### Implementation (Green)

- [x] T011 [US4] Implement validate() on NeuronAccount returning ValidationError[] per type-specific rules. Must collect ALL violations, not stop at first. File: `impl/typescript/src/account/validation.ts` (FR-006, FR-007, FR-007a, FR-014, SC-005)

**Checkpoint**: SC-005 (all violations reported) passes.

---

## Phase 4: US5 — Serialization (Priority: P3)

**Goal**: JSON serialize/deserialize without data loss.

### Tests (Red)

- [x] T012 [P] [US5] Write failing tests for serialization: Parent account → JSON → deserialize → equal. Child account roundtrip. Shared account roundtrip. BigInt balance fields serialized as strings (FR-W07). publicKey serialized as compressed hex (spec 002 FR-010). Optional fields omitted when absent (FR-W04). File: `impl/typescript/tests/account/serialization.test.ts` (FR-015, FR-W04, FR-W07, SC-007)

### Implementation (Green)

- [x] T013 [US5] Implement serialize(account) → JSON string with canonical field order and deserialize(json) → NeuronAccount with full validation on construction. BigInt → string per FR-W07. Public keys as compressed hex. File: `impl/typescript/src/account/serialization.ts` (FR-015, FR-W04, FR-W07, SC-007)

**Checkpoint**: SC-007 (roundtrip) passes.

---

## Phase 5: US6 — Payment Address + Registry Binding (Priority: P4)

**Goal**: Payment address resolution and verifier interfaces.

### Tests (Red)

- [x] T014 [P] [US6] Write failing tests for paymentAddress(): Parent returns own evmAddress (FR-023), Child returns parent's evmAddress derived from parentPubKey (FR-024), Shared throws error. File: `impl/typescript/tests/account/payment.test.ts` (FR-023, FR-024, SC-011)
- [x] T015 [P] [US6] Write failing tests for verifier interfaces: ParentChildVerifier.verifyRelationship called with correct args, LedgerVerifier.verifyAttachment returns VerificationResult. File: `impl/typescript/tests/account/verifier.test.ts` (FR-017, FR-019, SC-009, SC-010)

### Implementation (Green)

- [x] T016 [US6] Implement paymentAddress() on NeuronAccount: Parent → own evmAddress, Child → EVMAddress.fromPublicKeyBytes(parentPubKey.toUncompressedBytes()), Shared → throw NEURON-ACCT-001. File: `impl/typescript/src/account/payment.ts` (FR-023, FR-024, SC-011)
- [x] T017 [US6] Implement verifier interfaces: ParentChildVerifier, LedgerVerifier as TypeScript interfaces with async methods. File: `impl/typescript/src/account/verifier.ts` (FR-017, FR-019)

**Checkpoint**: SC-011 (payment), SC-012 (registry binding) pass.

---

## Phase 6: Polish

- [x] T018 Create public exports barrel file. File: `impl/typescript/src/account/index.ts` (FR-001)
- [x] T019 Run all account tests: `npx vitest run tests/account/`. All pass. (SC-001..SC-012)

---

## Dependencies

- **Phase 1**: No dependencies
- **Phase 2**: Depends on Phase 1
- **Phase 3**: Depends on Phase 2 (needs valid accounts to validate)
- **Phase 4**: Depends on Phase 2 (needs accounts to serialize)
- **Phase 5**: Depends on Phase 2 (needs accounts with pubkeys)
- **Phase 6**: Depends on all

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 19 |
| FR coverage | 26/26 |
| SC coverage | 9/9 |
| Conformance gates | Validation rules (V-PARENT/CHILD/SHARED), JSON roundtrip |
