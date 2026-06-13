# Research: NeuronAccount Module

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `001-neuron-account-module` | **Date**: 2026-02-25 | **Source**: spec.md

---

## R1: Account Type Model in Go

**Decision**: Use a single `NeuronAccount` struct with an `AccountType` discriminator (Parent=1, Child=2, Shared=3) and type-specific fields as optional/nilable.

**Rationale**: The spec defines NeuronAccount as a single type with an account-type discriminator (FR-013). In Go, this maps to a struct with optional fields (pointers or interfaces for type-specific data). A builder pattern (FR-011) constructs valid accounts. Validation (FR-006, FR-007, FR-007a) enforces type-specific rules post-construction.

**Alternatives considered**:
- Separate types per account kind (ParentAccount, ChildAccount, SharedAccount): Cleaner type safety but diverges from spec's single-entity model. Would require interface-based polymorphism.
- Sum type via sealed interface: Go lacks sum types. An interface + concrete types would work but adds complexity.

---

## R2: Builder Pattern (FR-011)

**Decision**: Implement a fluent builder (`AccountBuilder`) that chains method calls and returns `(NeuronAccount, error)` on `.Build()`.

**Rationale**: FR-011 requires a fluent builder API. Go idiom: `NewParentAccount().WithPublicKey(pk).WithDID(did).WithCurrency("ETH").Build()`. The builder validates completeness (FR-011a) at build time.

**Alternatives considered**:
- Functional options pattern: Common in Go but less "fluent" than method chaining. Better for configuration, not construction workflows.
- Constructor with config struct: Simpler but not fluent.

---

## R3: DID:key Generation (FR-012)

**Decision**: Delegate to Spec 002's `NeuronPublicKey.DIDKey()` method (FR-006a).

**Rationale**: Spec 002 already defines DID:key generation with multicodec secp256k1 encoding. The account module calls `publicKey.DIDKey()` to get the `did:key:zQ3s...` identifier. No additional DID library needed in the account module.

---

## R4: Ledger Attachment Model (FR-018, FR-019)

**Decision**: Define `LedgerAttachment` as a struct with ledger identifier, attached address (EVMAddress), attachment state (attached/detached), verification status, and last-synced timestamp. Verification (FR-019) is implemented as a `Verify()` method that accepts a `LedgerVerifier` interface — actual ledger queries are injected, not built-in.

**Rationale**: The spec explicitly states "Account creation on the ledger is out of scope" (FR-018). Ledger interaction must be injectable. A `LedgerVerifier` interface allows different backends (Ethereum RPC, Hedera SDK, mock) without coupling the account module to specific chains.

**Alternatives considered**:
- Hardcoded Ethereum RPC calls: Violates blockchain-agnostic principle.
- No verification at all: Doesn't satisfy FR-019.

---

## R5: Balance Management (FR-016)

**Decision**: Balance is represented as `*big.Int` (nilable — undefined until first sync). `LastSyncedAt` as `*time.Time`. Balance is read-only in the account module; sync operations are delegated to an injected `BalanceSyncer` interface.

**Rationale**: FR-016 states "balance is only set by syncing with the ledger." The account module caches the value but doesn't execute transfers. `big.Int` handles arbitrary-precision balances across all chains.

---

## R6: Registry Binding (FR-022)

**Decision**: Simple struct: `RegistryBinding{RegistryIdentifier string, ExternalID string}`. Required for Child completeness (FR-011a).

**Rationale**: The spec explicitly says "this module does not interpret registry semantics." The binding is opaque — just two strings.

---

## R7: JSON Serialization (FR-015)

**Decision**: Use Go's `encoding/json` with custom `MarshalJSON`/`UnmarshalJSON` methods. Public keys serialized as compressed hex strings per Spec 002 FR-010 (W-003 resolution).

**Rationale**: Go's standard JSON library is sufficient. Custom marshalers handle NeuronPublicKey → hex string and EVMAddress → EIP-55 checksummed string conversions.

---

## R8: Payment Address Resolution (FR-023, FR-024)

**Decision**: `PaymentAddress()` method on NeuronAccount. For Parent: returns own EVMAddress. For Child: follows parent reference to resolve Parent's EVMAddress. For Shared: not applicable (no payment address defined).

**Rationale**: Straightforward delegation. The Child's payment address is always the Parent's.
