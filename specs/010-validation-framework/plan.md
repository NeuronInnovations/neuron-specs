# Implementation Plan: Validation Framework

**Branch**: `010-validation-framework` | **Date**: 2026-03-31 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/010-validation-framework/spec.md`

## Summary

Spec 010 defines the cross-cutting validation framework for evidence-based, validator-autonomous compliance assessment across the Neuron protocol. It introduces: the `neuron-validator` service type for validator agent registration, a `validationEvidence` TopicMessage payload type (evidence envelope — 8 mandatory fields, 2 optional, canonical JSON per 006), a three-outcome oracle verdict schema (compliant/non-compliant/inconclusive mapped to Validation Registry codes 1/2/3), an evidence lifecycle (stdOut publication → optional on-chain anchoring), and the Zero-to-Heartbeat reference cross-spec scenario. This is a framework spec (like 006) — it defines protocol-wide concerns, not a single SDK module. The Go implementation produces validation utility types in `internal/validation/`.

## Technical Context

**Language/Version**: Go 1.25+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**: `neuron-go-sdk/internal/keylib` (Keccak256 hashing, signing), `neuron-go-sdk/internal/topic` (TopicMessage, TopicAdapter, stdOut publishing), `neuron-go-sdk/internal/registry` (AgentURI, service parsing, Identity Registry lookup), `neuron-go-sdk/internal/health` (HeartbeatPayload — for Zero-to-Heartbeat scenario tests), `go-ethereum` (Keccak256)
**Storage**: N/A — stateless; evidence published to topics, verdicts anchored on-chain
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First)
**Target Platform**: Cross-platform (Go SDK library)
**Project Type**: Single library package (`internal/validation/`)
**Performance Goals**: N/A — protocol correctness over throughput; envelope serialization < 1ms
**Constraints**: All envelopes canonical JSON (006 FR-W01–W10); all signatures deterministic (006 FR-A07); evidence chain hashes must be reproducible across implementations (SC-V01)
**Scale/Scope**: 24 functional requirements (FR-V01–V24), 8 success criteria (SC-V01–V08), 4 evidence rules (VR-VF-01–04), 4 user stories

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | Feature spec exists under `specs/` with all mandatory sections | PASS — spec.md complete with User Scenarios, Requirements, Success Criteria, Evidence & Validation |
| II | Independently Testable Stories | Each user story has Given/When/Then and is independently testable | PASS — 4 user stories (2×P1, 2×P2) with acceptance scenarios |
| III | Clarification Before Plan | No unresolved [NEEDS CLARIFICATION] markers in spec | PASS — 2 clarification sessions recorded, zero markers remain |
| IV | High-Level Types | Data models use semantic types (not primitives) | PASS — EvidenceEnvelope, ValidatorService, OracleVerdict, CompositeValidation; fields use UnsignedInt256, HexBytes, UnixTimestamp |
| V | Traceability | FR-*, SC-*, and Key Entities are present and aligned | PASS — FR-V01–V24, SC-V01–V08, 4 key entities, VR-VF-01–04 |
| VI | Golang-First SDK | Plan targets Go with idiomatic layout | PASS — targets `internal/validation/` package |
| VII | Strict Spec Compliance | Every task traces to an FR-* or SC-* requirement | PASS — will trace in tasks.md |
| VIII | Hedera Transport Binding | First adapter integration test targets HCS | PASS — evidence envelopes are TopicMessage payloads; stdOut publishing uses existing TopicAdapter (004) which targets HCS first |
| IX | Test-First Development | Test tasks precede implementation tasks in each phase | PASS — will enforce in tasks.md |
| X | Deterministic Signing | Signing tasks include determinism verification tests | PASS — evidence envelopes are signed TopicMessages; determinism inherited from 004 FR-T03 + 006 FR-A07; SC-V01 requires byte-identical serialization |
| XI | Verifiable Execution | Spec includes Evidence & Validation section with verification tier, observable signals, evidence rules, non-observable areas, and suggested evidence recipes | PASS — VR-VF-01–04 defined, tier = topic-observable + on-chain-only, 3 non-observable areas, 1 evidence recipe |

## Project Structure

### Documentation (this feature)

```text
specs/010-validation-framework/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output (below)
├── data-model.md        # Phase 1 output (below)
├── contracts/           # Phase 1 output (below)
│   ├── evidence-envelope.md
│   └── validator-service-schema.md
├── checklists/
│   └── requirements.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
impl/golang/internal/validation/
├── doc.go                    # Package-level godoc
├── evidence_envelope.go      # EvidenceEnvelope type, builder, serialization
├── evidence_envelope_test.go # Tests for envelope construction, serialization, validation
├── verdict.go                # OracleVerdict type, verdict constants, code mapping
├── verdict_test.go           # Tests for verdict validation, code mapping
├── validator_service.go      # ValidatorService type, agentURI parsing
├── validator_service_test.go # Tests for service construction, parsing
├── composite.go              # CompositeValidation, compositeRefs computation
├── composite_test.go         # Tests for composite linking, hash verification
└── conformance_test.go       # Cross-implementation test vectors (SC-V01)
```

**Structure Decision**: Single Go package `internal/validation/` following the existing pattern (`internal/keylib/`, `internal/health/`, `internal/payment/`). Although 010 is a cross-cutting framework spec, the Go SDK benefits from a dedicated package for validation utility types (envelope builders, verdict constants, service parsers). The package does NOT implement validator logic — that is validator-sovereign. It provides the protocol types.

## Complexity Tracking

> No violations. All gates pass.
