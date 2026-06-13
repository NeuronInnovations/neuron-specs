# Implementation Plan: NeuronAccount Module (TypeScript)

**Date**: 2026-03-17 | **Spec**: [spec.md](spec.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

> **Non-negotiable**: Derives from specs, NOT from `impl/golang/`.

## Summary

Spec 001 defines a hierarchical three-account-type system (Parent, Child, Shared) with fluent builder construction, type-safe validation, ledger attachment, and payment address resolution. The TypeScript implementation uses discriminated unions for type safety, native `bigint` for balance fields, and async interfaces for injected ledger/registry verification. Depends on spec 002 keylib for NeuronPublicKey, MultisigKey, EVMAddress, PeerID, and DID:key.

## Technical Context

**Language/Version**: TypeScript 5.7+ targeting ES2022
**Dependencies**: `@neuron-sdk/keylib` (spec 002 types), wire format utilities
**Testing**: `vitest`
**Key TS Adaptations**:
- Balance fields: `bigint | undefined` with FR-W07 string encoding in JSON
- Discriminated union: `NeuronAccount = ParentAccount | ChildAccount | SharedAccount`
- Verifier interfaces: async (Promise-based) for ledger/registry I/O
- Error codes: NEURON-ACCT-001..008 per 006 error-taxonomy.md

## Constitution Check

| # | Principle | Status |
|---|-----------|--------|
| I | Specification-First | **PASS** — spec.md exists with all mandatory sections |
| II | Testable Stories | **PASS** — 6 user stories with Given/When/Then |
| III | Clarification Before Plan | **PASS** — zero [NEEDS CLARIFICATION] |
| IV | Semantic Types | **PASS** — AccountType enum, NeuronDID, LedgerAttachment, RegistryBinding |
| V | Traceability | **PASS** — 26 FRs, 9 SCs mapped |
| VI | Golang-First | **PASS** — TS is subsequent implementation |
| VII | Strict Compliance | **PASS** |
| VIII | Hedera Binding | **PASS** (N/A — account module is transport-agnostic) |
| IX | Test-First | **PASS** — test tasks precede implementation |
| X | Deterministic Signing | **PASS** (N/A — no signing in account module) |
| XI | Verifiable Execution | **PASS** (noted — VR-* pending) |

## Source Structure

```text
impl/typescript/src/account/
├── index.ts              # Public exports
├── types.ts              # AccountType enum, NeuronDID, LedgerAttachment, RegistryBinding, LedgerAccountId
├── account.ts            # NeuronAccount discriminated union type
├── builder.ts            # ParentAccountBuilder, ChildAccountBuilder, SharedAccountBuilder
├── validation.ts         # V-PARENT-01..05, V-CHILD-01..05, V-SHARED-01..04 rules
├── payment.ts            # paymentAddress() resolution (FR-023, FR-024)
├── serialization.ts      # JSON serialize/deserialize (FR-015, FR-W07)
├── verifier.ts           # ParentChildVerifier, LedgerVerifier interfaces (FR-017, FR-019)
└── errors.ts             # AccountError, NEURON-ACCT-001..008
```

## Phases

### Phase 1: Foundational — Types + Errors
AccountType enum, NeuronDID, LedgerAttachment, RegistryBinding, error codes.

### Phase 2: US1 — Account Creation (P1)
Builder pattern: ParentAccountBuilder, ChildAccountBuilder, SharedAccountBuilder with validation.
**Gate**: SC-001 (Parent), SC-002 (Child) pass.

### Phase 3: US4 — Validation (P2)
Full validation rules V-PARENT-*, V-CHILD-*, V-SHARED-* with error collection.
**Gate**: SC-005 pass.

### Phase 4: US5 — Serialization (P3)
JSON serialize/deserialize with BigInt→string, canonical field order.
**Gate**: SC-007 roundtrip pass.

### Phase 5: US6 — Payment Address + Registry Binding (P4)
PaymentAddress resolution, verifier interfaces.
**Gate**: SC-011 (payment), SC-012 (registry) pass.
