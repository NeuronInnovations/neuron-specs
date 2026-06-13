# Implementation Plan: Payment

**Branch**: `008-payment` | **Date**: 2026-03-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/008-payment/spec.md`

## Summary

Spec 008 defines the commerce protocol layer for Neuron agent-to-agent service transactions. It introduces the `neuron-commerce` service type in agentURI (service offerings with delivery, settlement, and pricing), a six-message negotiation sub-protocol over topics (serviceRequest, serviceResponse, connectionSetup, escrowCreated, invoice, invoiceAck), an agreement lifecycle state machine (10 states), an abstract EscrowAdapter interface (6 operations with hedera-native and evm-escrow bindings), and trust-gated engagement. The implementation targets Go first (Constitution VI), follows TDD (Constitution IX), and produces deterministic signed messages (Constitution X).

## Technical Context

**Language/Version**: Go 1.25+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**: `go-ethereum` (Keccak256, EIP-55), `neuron-go-sdk/internal/keylib` (signing), `neuron-go-sdk/internal/topic` (TopicMessage, TopicAdapter), `neuron-go-sdk/internal/registry` (AgentURI, services), `neuron-go-sdk/internal/account` (PaymentAddress, SharedAccount)
**Storage**: N/A — stateless protocol layer; escrow state is on-chain
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First)
**Target Platform**: Cross-platform (Go SDK library)
**Project Type**: Single library package (`internal/payment/`)
**Performance Goals**: N/A — protocol correctness over throughput; message serialization < 1ms
**Constraints**: All messages must be canonical JSON (006 FR-W01–W10); all signatures deterministic (006 FR-A07); agreement state must be deterministic given same message sequence (SC-P04)
**Scale/Scope**: 44 functional requirements (FR-P01 through FR-P35), 11 success criteria (SC-P01–P11), 7 validator rules (VR-PAY-01–07)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | Feature spec exists under `specs/` with all mandatory sections | PASS — spec.md complete with User Scenarios, Requirements (A–H), Success Criteria, Third-Party Validation |
| II | Independently Testable Stories | Each user story has Given/When/Then and is independently testable | PASS — 6 user stories (P1–P3) with acceptance scenarios |
| III | Clarification Before Plan | No unresolved [NEEDS CLARIFICATION] markers in spec | PASS — 2 clarification sessions recorded (2026-03-19, 2026-03-24), zero markers remain |
| IV | High-Level Types | Data models use semantic types (not primitives) | PASS — NeuronCommerceService, DeliveryDescriptor, EscrowRef, AgreementState, EscrowAdapter defined |
| V | Traceability | FR-*, SC-*, and Key Entities are present and aligned | PASS — FR-P01–P35, SC-P01–P11, 6 key entities, VR-PAY-01–07 |
| VI | Golang-First SDK | Plan targets Go with idiomatic layout | PASS — targets `internal/payment/` package |
| VII | Strict Spec Compliance | Every task traces to an FR-* or SC-* requirement | PASS — will trace in tasks.md |
| VIII | Hedera Transport Binding | First adapter integration test targets HCS | PASS — hedera-native is a co-equal first binding per spec |
| IX | Test-First Development | Test tasks precede implementation tasks in each phase | PASS — will enforce in tasks.md |
| X | Deterministic Signing | Signing tasks include determinism verification tests | PASS — SC-P03 requires deterministic signed TopicMessages for all 6 payload types |
| XI | Verifiable Execution | Spec includes Third-Party Validation section with VR-* checklist and verification tier | PASS — VR-PAY-01–07 defined, tier = topic-observable + on-chain-only |

## Project Structure

### Documentation (this feature)

```text
specs/008-payment/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── data-model.md        # Phase 1 output (below)
├── contracts/           # Phase 1 output (below)
│   ├── commerce-service-schema.md
│   ├── negotiation-payloads.md
│   └── escrow-adapter.md
├── checklists/
│   ├── requirements.md
│   ├── architecture-quality.md
│   └── funding-compliance.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code

```text
impl/golang/internal/
├── payment/                    # NEW — Spec 008 primary package
│   ├── doc.go                  # Package godoc
│   ├── commerce_service.go     # NeuronCommerceService, DeliveryDescriptor, SettlementDescriptor, PricingDescriptor
│   ├── negotiation.go          # serviceRequest, serviceResponse, connectionSetup payloads
│   ├── escrow_payloads.go      # escrowCreated, invoice, invoiceAck payloads
│   ├── agreement.go            # AgreementState machine, per-requestId tracking
│   ├── escrow.go               # EscrowAdapter interface, EscrowRef, ReleaseRequestRef
│   ├── trust.go                # Trust gating (SHOULD-level liveness/reputation checks)
│   ├── errors.go               # NEURON-PAYMENT-* error taxonomy
│   └── *_test.go               # Tests per source file
├── registry/                   # MODIFIED — AgentURI integration
│   ├── agent_uri.go            # Add commerceServices field, MarshalJSON/UnmarshalJSON
│   ├── service.go              # Add ServiceTypeNeuronCommerce constant
│   └── errors.go               # Add InvalidDeliveryRef error
└── account/
    └── payment.go              # EXISTING — PaymentAddress() (FR-P23/P24, already done)
```

**Structure Decision**: New `internal/payment/` package for the bulk of Spec 008 logic (negotiation, lifecycle, escrow). The `NeuronCommerceService` type is placed in `payment/` but also integrated into `registry/agent_uri.go` for serialization. This follows the pattern where 004 defines topic types in `topic/` but 003's registry consumes them.

## Phase 0: Research

No NEEDS CLARIFICATION markers remain. Two clarification sessions were completed (2026-03-19, 2026-03-24). Key resolved decisions:

| Decision | Resolution | Rationale |
|----------|-----------|-----------|
| Payload encryption | Public (FR-P12b) | Enables validator observability; exception for connectionSetup.encryptedMultiaddrs (FR-P34) |
| Balance thresholds | Service-level, not protocol (FR-P24–P27) | Different dApps need different funding models |
| Concurrent agreements | Yes, per requestId (FR-P13a) | Same buyer-seller pair may have multiple active agreements |
| Negotiation timeout | negotiationDeadline in serviceRequest (FR-P07a) | Auto-reject on deadline expiry |
| Delivery descriptor | delivery.mode in neuron-commerce (FR-P01a) | Bridges control plane to data plane |
| Confirmation mode | SHOULD WaitForConsensus for escrow payloads (FR-P12c) | Ensures agreementHash/evidenceHash are provably anchored |

## Phase 1: Design & Contracts

### Data Model

See [data-model.md](data-model.md) (generated below).

### Architecture Decisions

**DD-P01**: Commerce service types live in `internal/payment/` but are consumed by `internal/registry/` for AgentURI serialization. The registry package imports payment types — this reverses the typical dependency direction. Alternative: define types in registry. Chosen approach: types belong in payment because they are Spec 008's domain; registry acts as a consumer.

**DD-P02**: Agreement state is tracked in-memory per requestId. No persistence layer — state is reconstructable from the topic message history (the messages on stdIn/stdOut are the source of truth). This matches the old SDK's pattern and avoids introducing storage dependencies.

**DD-P03**: EscrowAdapter is an interface, not a concrete implementation. The hedera-native and evm-escrow bindings are separate files that implement the interface. This follows the TopicAdapter pattern from Spec 004.

**DD-P04**: Negotiation payloads are pure data types with canonical JSON serialization. They do NOT embed signing logic — signing happens when the payload is wrapped in a TopicMessage (via topic.NewTopicMessage). This separates concerns: payment defines what to say, topic defines how to sign and send it.

---

## 2026-05-08 Amendment — Plan Addendum

This addendum extends Phase 1 design to cover the spec changes ratified in `spec.md` Clarifications Session 2026-05-08. Under `/speckit` discipline these would normally trigger a full plan regeneration; we are appending in-place because the changes are additive (no FR removal, no FR semantics change for the existing 6-message flow). A future `/speckit.plan` regeneration on a fresh feature branch will integrate this addendum into the main plan body.

### New Architectural Decisions

**DD-P05** (lifecycle messages): The three new payload types (`serviceStop`, `serviceCancel`, `serviceRenew`) live alongside the existing six in `internal/payment/negotiation.go`. They share the canonical-JSON envelope discipline of FR-P12 and do NOT introduce a new transport mechanism. State-machine transitions in `agreement.go` extend the existing event enum (add `EventStop`, `EventCancel`, `EventRenew`) rather than introducing a parallel state machine.

**DD-P06** (active-service persistence): Persistence is a NEW subsystem. Recommended Go layout: `internal/payment/persistence/` with an interface `ActiveServiceStore` and at minimum two implementations — `MemoryStore` (for tests) and `JSONFileStore` (for the edge demo and other simple deployments). The `internal/edgeapp/state.go` schema bumps version to include `activeServices[]` per the existing pattern of versioned `EdgeState`. Conformance is behavioral (FR-P43): a "kill seller, restart, observe resume" integration test, not a wire-format conformance vector.

**DD-P07** (degraded mode): Degraded mode is achieved by **removing** chain/topic-backend checks from the data-plane critical path, NOT by adding new branches. Audit `internal/edgeapp/seller.go` and `internal/delivery/libp2p_adapter.go` for any place where a `bus.Publish` failure or an `escrow.GetBalance` error currently aborts an active stream; remove or downgrade those paths to logging. The active-service entry from DD-P06 is the source of truth for "should I keep streaming?".

**DD-P08** (AdmissionPolicy interface): Define `AdmissionPolicy` in `internal/payment/admission.go` as a small synchronous interface returning a typed `Decision`. Provide one built-in implementation `AllowAll` in the same file. Concrete implementations (partner allowlists, denylists, fan-out fallback decisions) live in DApp packages (e.g., `internal/dapp/adsb/admission.go` for spec 016). The Core SDK's seller runtime accepts an `AdmissionPolicy` via constructor injection.

**DD-P09** (streams[] catalog wire format): Extend `ConnectionSetup` struct in `negotiation.go` with an OPTIONAL `Streams []StreamCatalogEntry` field. `StreamCatalogEntry` carries `Name`, `ProtocolID`, `Direction`, optional `Schema`. Marshal omits the field when empty (FR-W04). Back-compat: existing single-string `Protocol` continues to marshal alongside `Streams` when both are present. Receivers prefer `Streams` over `Protocol` when both are present.

### New File Surface (additions, not replacements)

```text
internal/payment/
  ├── admission.go             # NEW — AdmissionPolicy interface + AllowAll default (DD-P08)
  ├── persistence/
  │   ├── store.go             # NEW — ActiveServiceStore interface (DD-P06)
  │   ├── memory_store.go      # NEW — in-memory impl (tests)
  │   ├── json_file_store.go   # NEW — JSON-on-disk impl (edge demo)
  │   └── *_test.go
  ├── lifecycle.go             # NEW — serviceStop/serviceCancel/serviceRenew payload helpers
  └── (existing files updated: negotiation.go +Streams field; agreement.go +new events)
```

### Phase 1 Constitution Check Re-run

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | Spec amendment ratified before plan addendum | PASS |
| VI | Language-Neutral Protocol | New payloads canonical-JSON; conformance vectors required (Phase 10 task) | PASS pending vectors |
| VII | Strict Spec Compliance | All FR-P36/37/38/40-57 traced to tasks (see tasks.md addendum) | PASS |
| IX | Test-First | Persistence tasks lead with restart-and-resume integration test | PASS |
| X | Deterministic Signing | New payloads sign through existing TopicMessage path; no new signing logic | PASS |
| XI | Verifiable Execution | VR-PAY-08/09/10 added to spec; SC-P12-P16 added | PASS |
| XII | Layered Architecture | Spec 008 stays Core SDK: AdmissionPolicy is an interface only (no policy); fan-out is mentioned only as a delegate to DApps | PASS |
