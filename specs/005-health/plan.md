# Implementation Plan: Health (Onchain Liveness & Health Status)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `005-health` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-health/spec.md`

## Summary

Spec 005 defines a self-declared deadline liveness model where agents publish signed heartbeat messages (HeartbeatPayload) to their stdOut topic via Spec 004's TopicAdapter. Implementation requires: (1) a HeartbeatPayload builder with publisher validation (V-PUB-01..07), (2) an observer-side validator with liveness state machine (UNKNOWN/ALIVE/SUSPECT/DEAD/OFFLINE), and (3) integration with the Spec 004 publish/subscribe pipeline. The reference implementation targets Go per Constitution Principle VI.

## Technical Context

**Language/Version**: Go 1.22+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**: Spec 002 (`internal/keylib` — NeuronPrivateKey, Signature); Spec 004 (`internal/topic` — TopicMessage, TopicAdapter, PublishResult, MessageDelivery); `encoding/json` (deterministic serialization)
**Storage**: Observer-local only. LivenessRecord (per-peer state) is in-memory; not persisted on-chain.
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First). 12 success criteria (SC-H01..H12), 25 acceptance scenarios.
**Target Platform**: Linux/macOS/Windows (Go cross-compilation). HCS validated end-to-end first (Constitution Principle VIII).
**Project Type**: Go module — `internal/health/` package within the neuron-go-sdk module.
**Performance Goals**: Heartbeat publish latency within backend-specific bounds (HCS ~100ms submit, EVM ~1-2s). Observer evaluation is a pure function of (timestamp, deadline, constants) — sub-millisecond.
**Constraints**: Mandatory HeartbeatPayload must serialize to < 256 bytes JSON (FR-H29). MIN_DEADLINE_DELTA = 10s floor. MAX_DEADLINE_DELTA = 86400s ceiling.
**Scale/Scope**: Per-observer: track N peers (one LivenessRecord each). Per-publisher: one heartbeat per MIN_DEADLINE_DELTA interval.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md exists with mandatory sections (Purpose, User Scenarios, Requirements, Success Criteria) | **PASS** |
| II | Independently Testable User Stories | Stories prioritized (P1/P2/P3), Given/When/Then acceptance scenarios, each independently testable | **PASS** — 6 stories, 25 scenarios |
| III | Clarification Before Plan | No [NEEDS CLARIFICATION] markers; underspecified areas resolved before plan | **PASS** — 0 markers. Two "Open question" items explicitly deferred (per-role delta, default delta) with no impact on v1.0.0 implementation |
| IV | High-Level Types | Semantic types used (PayloadType, SemVer, UnixTimestamp, NodeRole, NATType, GPSFixQuality, etc.) | **PASS** |
| V | Traceability | FRs, SCs, Key Entities present and aligned | **PASS** — 29 FRs, 12 SCs, 7 entities, all cross-referenced |
| — | Related specs section | Present with internal (001–004) and external (EIP-8004) refs | **PASS** |
| — | Mermaid diagrams | Appendix contains 5 diagrams (ER, state machine, sequence, boundary, cost) | **PASS** |
| — | Blockchain compatibility | Hedera, EVM, Kafka sections with per-backend notes | **PASS** |
| VI | Golang-First SDK | Plan targets Go 1.22+ with `internal/health/` layout, colocated `*_test.go` | **PASS** |
| VII | Strict Spec Compliance | Every task traces to FR-H* or SC-H*; no silent deviations | **PASS** |
| VIII | Hedera Transport Binding | HCS integration test (T072) precedes cross-chain tests; HCS validated first | **PASS** |
| IX | Test-First Development | Test tasks precede implementation tasks in each phase | **PASS** |
| X | Deterministic Signing | signing_test.go task (T073) verifies deterministic R\|\|S\|\|V for heartbeat signing | **PASS** |

**Gate result**: All gates PASS. Proceeding to Phase 0.

**Resolved dependency**: Spec 005 references FR-T22..T29 (PublishResult, MessageDelivery, confirmation modes, subscribe resumption, size limits, clock normalization). These have been added to Spec 004's spec.md as the "Publish/Subscribe Execution Binding" subsection. See research.md R1 for details.

## Project Structure

### Documentation (this feature)

```text
specs/005-health/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output — dependency resolution
├── data-model.md        # Phase 1 output — entity model
├── quickstart.md        # Phase 1 output — SDK integration guide
├── contracts/           # Phase 1 output — API contracts
│   ├── health-publisher.md   # Publisher API contract
│   └── health-observer.md    # Observer API contract
├── checklists/
│   └── requirements.md  # Requirements traceability checklist
│
# Source documents (internal research artifacts, not in the public tree):
# heartbeat-protocol.md, architecture.md, context-summary.md,
# transport-gap-analysis.md, extraction.md, Tech Standup notes
```

### Source Code (Go module — Constitution VI)

```text
internal/health/
├── doc.go               # Package documentation
├── constants.go         # Protocol constants (MIN/MAX delta, grace, S2D)
├── constants_test.go    # Constants validation
├── types.go             # Semantic types (NodeRole, NATType, GPSFixQuality, etc.)
├── types_test.go        # Type validation
├── errors.go            # Sentinel errors (ErrInvalidPayloadType, ErrSenderAddressMismatch, etc.)
├── errors_test.go       # Error type tests
├── payload.go           # HeartbeatPayload builder + deterministic JSON serialization
├── payload_test.go      # Serialization, determinism, size budget
├── publisher.go         # Publisher validation (V-PUB-01..07) + publish orchestration
├── publisher_test.go    # V-PUB-01..07 rejection paths
├── observer.go          # Observer validation (V-OBS-01..06) + liveness evaluator
├── observer_test.go     # V-OBS-01..06 rejection paths
├── liveness.go          # LivenessState enum + state machine + LivenessRecord
├── liveness_test.go     # State machine transitions (all 8 transitions)
├── signing_test.go      # Deterministic signing verification (Constitution X)
├── integration_test.go  # End-to-end publish → observe → evaluate (HCS first per VIII)
└── cross_chain_test.go  # SC-H11: non-Hedera chain verification
```

**Structure Decision**: Go package (`internal/health/`) within the neuron-go-sdk module. Depends on `internal/keylib` (Spec 002) and `internal/topic` (Spec 004). Each concern in its own file pair with colocated tests per Go convention.

## Complexity Tracking

No constitution violations requiring justification.

---

## 2026-05-08 Amendment — Plan Addendum

Adds plan coverage for FR-H30–H33 (degraded-mode liveness).

### New Architectural Decisions

**DD-H05** (degraded-mode observer): The observer state machine (FR-H18–H21) is unchanged. New behavior is an **advisory layer** that wraps the result. Implementation: a `LivenessObserver.Status()` method returns both the canonical `LivenessState` AND a `Confidence` value (`"high"` when control-plane healthy, `"degraded"` when control-plane stalled, `"data-plane-evidence"` when 009 reports an active channel). Application-level decisions consume `Status()`; the canonical state machine is preserved for byte-level conformance.

**DD-H06** (cached last-known-good): Add `LivenessRecord.persisted` field. On observer restart with a control-plane outage detected, the persisted last-known-good record is the baseline rather than `UNKNOWN`. This deviates from the existing "observer-local, not persisted" stance — the new behavior is OPTIONAL (FR-H32 says MAY) but RECOMMENDED.

### Phase 1 Constitution Check (Re-run)

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| VI | Language-Neutral Protocol | No wire-format changes | PASS |
| VII | Strict Spec Compliance | FR-H30–H33 traced to tasks | PASS |
| XI | Verifiable Execution | Degraded-mode behavior is observable; documented in spec.md SC-H13/SC-H14 | PASS |
| XII | Layered Architecture | Health is Core SDK; new FRs do not introduce DApp-specific behavior | PASS |
