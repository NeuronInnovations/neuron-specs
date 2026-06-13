# Implementation Plan: NeuronAccount Module

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `001-neuron-account-module` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-neuron-account-module/spec.md`

## Summary

Spec 001 defines a hierarchical account system with three account types (Parent, Child, Shared) built on Spec 002's NeuronPublicKey. Implementation requires: (1) a fluent builder API for type-safe account construction with completeness validation, (2) type-specific validation rules (Parent: DID+balance+key, Child: parentRef+registryBinding+key, Shared: MultisigKey), (3) ledger attachment model with injectable verification, (4) payment address resolution (Parent→own EVMAddress, Child→Parent's EVMAddress), and (5) JSON serialization. The reference implementation targets Go per Constitution Principle VI.

## Technical Context

**Language/Version**: Go 1.22+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**: Spec 002 (`internal/keylib` — NeuronPublicKey, MultisigKey, EVMAddress, PeerID, DID:key); `math/big` (balance amounts); `encoding/json` (serialization)
**Storage**: N/A — all account types are in-memory. Ledger interaction is injected via interfaces.
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First). 9 success criteria (SC-001..SC-012). No signing operations — Constitution X not directly applicable.
**Target Platform**: Linux/macOS/Windows (Go cross-compilation).
**Project Type**: Go module — `internal/account/` package within the neuron-go-sdk module.
**Performance Goals**: Account creation and validation < 100ms (SC-008, excluding network I/O).
**Constraints**: No ledger interaction built-in — verification is injectable. No money transfers. No reachability/comms (see Spec 003).
**Scale/Scope**: Per-application: manage N accounts (Parent + Children). No persistent storage required.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md exists with mandatory sections (Purpose, User Scenarios, Requirements, Success Criteria) | **PASS** |
| II | Independently Testable Stories | Stories prioritized (P1/P2/P3), Given/When/Then acceptance scenarios, each independently testable | **PASS** — 5 stories, 20 scenarios |
| III | Clarification Before Plan | No [NEEDS CLARIFICATION] markers; underspecified areas resolved | **PASS** — 0 markers |
| IV | High-Level Types | Semantic types used (NeuronPublicKey, EVMAddress, PeerID, AccountType, NeuronDID, etc.) | **PASS** |
| V | Traceability | FRs, SCs, Key Entities present and aligned | **PASS** — 22 FRs, 9 SCs, 4 entities |
| VI | Golang-First SDK | Plan targets Go 1.22+ with `internal/account/` layout, `*_test.go` colocated tests | **PASS** |
| VII | Strict Spec Compliance | Every task traces to FR-* or SC-* requirements; no silent deviations | **PASS** |
| VIII | Hedera Transport Binding | Not directly applicable — Account module is transport-agnostic. LedgerVerifier supports Hedera | **PASS** (N/A) |
| IX | Test-First Development | Test tasks precede implementation tasks in each phase | **PASS** |
| X | Deterministic Signing | Not directly applicable — Account module does not perform signing | **PASS** (N/A) |

**Gate result**: All gates PASS.

## Project Structure

### Documentation (this feature)

```text
specs/001-neuron-account-module/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output — design decisions
├── data-model.md        # Phase 1 output — entity model
├── quickstart.md        # Phase 1 output — SDK integration guide
├── contracts/           # Phase 1 output — API contracts
│   └── account-builder.md   # Builder + methods contract
└── checklists/
    └── requirements.md  # Requirements traceability checklist
```

### Source Code (Go module — Constitution VI)

```text
internal/account/
├── account.go            # NeuronAccount struct + AccountType enum
├── account_test.go       # Account creation, field access, derived values
├── builder.go            # Fluent builder API (Parent/Child/Shared builders)
├── builder_test.go       # Builder validation, completeness, error paths
├── validation.go         # Type-specific validation rules (V-PARENT-*, V-CHILD-*, V-SHARED-*)
├── validation_test.go    # All validation rules, invalid configs, error messages
├── did.go                # NeuronDID type + GenerateDID() via 002 DIDKey()
├── did_test.go           # DID generation, format validation
├── ledger.go             # LedgerAttachment + LedgerVerifier interface + verification
├── ledger_test.go        # Attachment state, verification with mock verifier
├── registry.go           # RegistryBinding type
├── registry_test.go      # Binding storage, completeness checks
├── payment.go            # PaymentAddress() resolution (Parent→own, Child→Parent's)
├── payment_test.go       # Resolution for all account types
├── serialization.go      # JSON marshal/unmarshal with 002 key hex encoding
├── serialization_test.go # Round-trip, data integrity (SC-007)
└── integration_test.go   # Full lifecycle: build → validate → attach → serialize → deserialize
```

**Structure Decision**: Go package (`internal/account/`) within the neuron-go-sdk module. Depends on `internal/keylib` (Spec 002). Each concern (builder, validation, ledger, serialization) in its own file pair for parallel development.

## Complexity Tracking

No constitution violations requiring justification.
