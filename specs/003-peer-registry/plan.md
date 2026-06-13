# Implementation Plan: Peer Registry (EIP-8004 Registration)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `003-peer-registry` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/003-peer-registry/spec.md`

## Summary

Spec 003 defines peer registration via EIP-8004 smart contract registries. A Child account (identified by EVM address from 001) registers in a registry and receives an extended NFT (ERC-721). The registration carries mandatory services: three `neuron-topic` service objects (stdIn, stdOut, stdErr per 004 FR-T14) and one `neuron-p2p-exchange` service (peerID, protocol, topicRef per 004 FR-T17). Registration operations (create, update, revoke) are signed by the Child's NeuronPrivateKey as proof-of-control (FR-R06). The registry MAY enforce an admission policy anchored to the Parent's DID (FR-R09). Implementation requires: (1) registration lifecycle (register, update, revoke) via EIP-8004 contract interaction, (2) agentURI construction and validation with mandatory services, (3) registry resolution (lookup by EVM address or registry binding), (4) service schema validation delegated to 004, and (5) DID service support (FR-R13, FR-R14). The reference implementation targets Go per Constitution Principle VI, using `go-ethereum` for contract interaction and signing.

## Technical Context

**Language/Version**: Go 1.22+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**: `internal/keylib` (Spec 002 — NeuronPrivateKey signing, NeuronPublicKey, EVMAddress, PeerID, DID:key), `internal/account` (Spec 001 — Child identity, RegistryBinding, PaymentAddress), `github.com/ethereum/go-ethereum` (ethclient for RPC, abigen for contract ABI bindings, accounts/abi/bind for transaction signing, common types), Spec 004 service schemas (neuron-topic per FR-T14, neuron-p2p-exchange per FR-T17)
**Storage**: N/A — registration state lives on-chain (EIP-8004 smart contract). SDK types are in-memory representations of on-chain data.
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First). 3 success criteria (SC-R01..SC-R03). Deterministic signing verified per Constitution Principle X (FR-R06 requires NeuronPrivateKey signing of registration transactions).
**Target Platform**: Linux/macOS/Windows (Go cross-compilation).
**Project Type**: Go module — `internal/registry/` package within the neuron-go-sdk module.
**Performance Goals**: Registration submission < 500ms (excluding block confirmation). Lookup/resolution < 200ms (excluding network I/O). AgentURI construction and validation < 10ms.
**Constraints**: EIP-8004 contract ABI must be generated via `abigen` from the contract's Solidity ABI. Same contract model on Hedera and Ethereum. One registration per Child per registry instance (FR-R05). NFT ownership MUST be the Child's EVMAddress (FR-R10). Admission policy mechanism is out of scope (FR-R09).
**Scale/Scope**: Per-Child: at most one registration per registry. Per-application: manage N registrations across M registries. No persistent local storage — all state is on-chain.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md exists with mandatory sections (Purpose, User Scenarios, Requirements, Success Criteria) | **PASS** |
| II | Independently Testable Stories | Stories prioritized (P1/P2), Given/When/Then acceptance scenarios, each independently testable | **PASS** — 3 stories, 7 scenarios |
| III | Clarification Before Plan | No [NEEDS CLARIFICATION] markers; underspecified areas resolved before plan | **PASS** — 0 markers. Admission policy mechanism explicitly out of scope (FR-R09). Revocation semantics explicitly out of scope (FR-R12e). |
| IV | High-Level Types | Semantic types used (Registration, AgentURI, NeuronTopicService, NeuronP2PExchangeService, DIDService, AdmissionPolicy, RegistrationResult) | **PASS** |
| V | Traceability | FRs, SCs, Key Entities present and aligned | **PASS** — 14 FRs, 3 SCs, 5 key entities, all cross-referenced |
| VI | Golang-First SDK | Plan targets Go 1.22+ with `internal/registry/` layout, `*_test.go` colocated tests, `go test` tooling | **PASS** |
| VII | Strict Spec Compliance | Every task traces to FR-* or SC-* requirements; no silent deviations | **PASS** — 14 FRs mapped to tasks |
| VIII | Hedera Transport Binding | Registry is a smart contract (EIP-8004) on both Hedera and Ethereum (FR-R01). Same contract model applies on both chains. | **PASS** |
| IX | Test-First Development | Test tasks (Red phase) precede implementation tasks in each phase; 100% MUST-level coverage | **PASS** — `*_test.go` tasks before each implementation phase |
| X | Deterministic Signing | Registration transactions MUST be signed by the Child's NeuronPrivateKey (FR-R06). Deterministic signing (RFC 6979) inherited from 002. signing_test.go verifies determinism for registration payloads. | **PASS** |
| — | Related specs section | Present with internal (001, 002, 004) and external (EIP-8004) cross-references | **PASS** |
| — | Mermaid diagrams | Not present in spec — spec uses Access Control Model prose and appendix service shape instead | **PASS** (N/A — access control model and service shape are clear without diagrams) |
| — | Blockchain compatibility | EIP-8004 contract model applies on both Hedera and Ethereum (FR-R01) | **PASS** |

**Gate result**: All gates PASS. Proceeding to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/003-peer-registry/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output — library selection and design decisions
├── data-model.md        # Phase 1 output — entity model
├── quickstart.md        # Phase 1 output — SDK integration guide
├── contracts/           # Phase 1 output — API contracts
│   └── registration.md  # Registration lifecycle + resolution + validation contract
└── checklists/
    └── requirements.md  # Requirements traceability checklist
```

### Source Code (Go module — Constitution VI)

```text
internal/registry/
├── registration.go        # Registration type + Register() + UpdateRegistration() + RevokeRegistration()
├── registration_test.go   # Registration lifecycle: create, update, revoke, proof-of-control
├── service.go             # NeuronTopicService, NeuronP2PExchangeService, DIDService types + builders
├── service_test.go        # Service construction, mandatory field validation, topicRef cross-validation
├── agent_uri.go           # AgentURI type + construction + completeness validation (FR-R08)
├── agent_uri_test.go      # AgentURI construction, mandatory services check, JSON serialization
├── validation.go          # ValidateRegistrationCompleteness + admission policy interface + role checks
├── validation_test.go     # Completeness rules, role boundary enforcement (FR-R11), allowlist behavior (FR-R12)
├── resolver.go            # LookupRegistration() + ResolveTopics() + ResolveP2PExchange()
├── resolver_test.go       # Lookup by EVM address, lookup by registry binding, resolution of services
├── errors.go              # Structured error types (RegistryUnavailable, NotFound, IncompletRegistration, etc.)
├── errors_test.go         # Error kind validation
├── signing_test.go        # Deterministic signing verification for registration transactions (Constitution X)
└── integration_test.go    # Full lifecycle: create Child → register → lookup → resolve topics → resolve p2p
```

**Structure Decision**: Go package (`internal/registry/`) within the neuron-go-sdk module. Depends on `internal/keylib` (Spec 002) and `internal/account` (Spec 001). Tests colocated as `*_test.go` per Go conventions. `internal/` ensures the registry package is not importable by external modules. Each concern (registration lifecycle, service types, agentURI, validation, resolution) in its own file pair for clear separation and parallel development.

**EIP-8004 Contract Binding**: The EIP-8004 contract ABI is generated via `abigen` into a separate `internal/registry/contract/` package. The registry package uses generated bindings to interact with the on-chain contract. The generated code is not hand-maintained.

```text
internal/registry/contract/
├── identity_registry.go   # abigen-generated Go bindings for EIP-8004 Identity Registry
└── identity_registry.abi  # Source ABI file (from Solidity compilation)
```

## Complexity Tracking

No constitution violations requiring justification.

---

## 2026-05-08 Amendment — No Plan/Tasks Changes

The 2026-05-08 amendment to `spec.md` is **clarification-only**: a new Clarifications session confirms no normative change is required, and an informative appendix subsection ("Path-based protocol IDs and multi-service catalogs") shows an example agentURI advertising both ADS-B (016) and Remote ID (017) DApp services.

No new FRs were added; no existing FRs were modified. The 003 plan and tasks remain unchanged. Constitution Principle XII (added in v1.7.0) is satisfied: 003 stays Core SDK, defers `services[]` schema authority to consuming specs (008/010/016/017), and does not encode any DApp-specific semantics.
