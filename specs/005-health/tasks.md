# Tasks: Health (Onchain Liveness & Health Status)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `005-health` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: plan.md, spec.md, data-model.md, contracts/health-publisher.md, contracts/health-observer.md
**SDK Target**: Go (`neuron-go-sdk`) — all paths use `internal/health/`

**Dependencies**: Spec 002 (`internal/keylib` — NeuronPrivateKey, Signature), Spec 004 (`internal/topic` — TopicMessage, TopicAdapter, PublishResult, MessageDelivery)

**TDD Rule (Constitution IX)**: Within every phase, test tasks appear BEFORE implementation tasks. Write the failing test first, then make it pass.

---

## Format: `- [ ] T### [P?] [US#?] Description`

- `- [ ]` checkbox is mandatory
- **T###**: Sequential task ID (T001, T002, ...)
- **[P]**: Parallelizable — different files, no inter-task dependencies
- **[US#]**: User story label (US1 through US6, phases 3-8 only)
- Every task references the FR-H* or SC-H* requirements it satisfies
- Every task includes the exact Go file path in `internal/health/`

---

## Phase 1: Setup

**Purpose**: Go module init, dependency wiring, project skeleton.

- [x] T001 Initialize Go module and create `internal/health/` package directory with `doc.go` package comment at `internal/health/doc.go` — no FR (infrastructure)
- [x] T002 [P] Create stub files for the health package: `internal/health/constants.go`, `internal/health/types.go`, `internal/health/errors.go`, `internal/health/payload.go`, `internal/health/publisher.go`, `internal/health/observer.go`, `internal/health/liveness.go` — each with `package health` declaration — no FR (infrastructure)
- [x] T003 [P] Create stub test files: `internal/health/constants_test.go`, `internal/health/types_test.go`, `internal/health/errors_test.go`, `internal/health/payload_test.go`, `internal/health/publisher_test.go`, `internal/health/observer_test.go`, `internal/health/liveness_test.go` — each with `package health_test` declaration — no FR (infrastructure)
- [x] T004 [P] Add `go.mod` dependency declarations for `internal/keylib` (Spec 002) and `internal/topic` (Spec 004) — verify imports compile — no FR (infrastructure)

---

## Phase 2: Foundational

**Purpose**: Protocol constants, semantic types (NodeRole, NATType, GPSFixQuality), error types, and core data structures that ALL user stories depend on. MUST complete before any user story.

**TDD order**: Tests first, then implementation.

### Tests

- [x] T005 [P] Write tests for protocol constants in `internal/health/constants_test.go`: assert `MIN_DEADLINE_DELTA == 10`, `MAX_DEADLINE_DELTA == 86400`, `GRACE_PERIOD == 30`, `SUSPECT_TO_DEAD == 120`, `SHUTDOWN_SENTINEL == 0` (FR-H06, FR-H07, FR-H08, FR-H09, FR-H12)
- [x] T006 [P] Write tests for semantic types in `internal/health/types_test.go`: assert NodeRole enum values (`buyer`, `seller`, `relay`, `validator`), NATType enum values (`no-nat`, `endpoint-independent`, `address-dependent`, `address-and-port-dependent`), GPSFixQuality enum values (`none`, `2D`, `3D`), PayloadType constant (`heartbeat`) (FR-H02, FR-H03, FR-H05)
- [x] T007 [P] Write tests for error types in `internal/health/errors_test.go`: assert each publisher validation error (V-PUB-01..07) and observer validation error (V-OBS-01..06) has correct sentinel error value and message string (FR-H13, FR-H15)
- [x] T008 Write tests for LivenessState enum in `internal/health/liveness_test.go`: assert five states exist (`UNKNOWN`, `ALIVE`, `SUSPECT`, `DEAD`, `OFFLINE`) with correct string representations (FR-H18)

### Implementation

- [x] T009 [P] Define protocol constants in `internal/health/constants.go`: `MinDeadlineDelta = 10`, `MaxDeadlineDelta = 86400`, `GracePeriod = 30`, `SuspectToDead = 120`, `ShutdownSentinel = 0` (FR-H06, FR-H07, FR-H08, FR-H09, FR-H12)
- [x] T010 [P] Define semantic types in `internal/health/types.go`: `PayloadType` string type, `NodeRole` string type with constants (`RoleBuyer`, `RoleSeller`, `RoleRelay`, `RoleValidator`), `NATType` string type with constants, `GPSFixQuality` string type with constants, `AbbreviatedAddress` string type, `ProtocolID` string type (FR-H02, FR-H03, FR-H05)
- [x] T011 [P] Define error sentinel types in `internal/health/errors.go`: publisher errors (`ErrInvalidPayloadType`, `ErrUnsupportedVersion`, `ErrDeadlineNotFuture`, `ErrDeadlineTooSoon`, `ErrDeadlineTooFar`, `ErrUnrecognizedRole`) and observer errors (`ErrSignatureVerificationFailed`, `ErrSenderAddressMismatch`, `ErrNotHeartbeatMessage`, `ErrIncompatibleVersion`, `ErrDeadlineInPast`, `ErrDeltaBelowMinimum`, `ErrDeltaExceedsMaximum`) (FR-H13, FR-H15)
- [x] T012 Define `LivenessState` enum in `internal/health/liveness.go`: `StateUnknown`, `StateAlive`, `StateSuspect`, `StateDead`, `StateOffline` with `String()` method (FR-H18)
- [x] T013 Write tests for HeartbeatPayload struct and sub-structs in `internal/health/payload_test.go`: assert struct field types, JSON tags in canonical order, Capabilities sub-struct, Location sub-struct (FR-H01, FR-H02, FR-H03, FR-H04)
- [x] T014 Implement `HeartbeatPayload` struct in `internal/health/payload.go` with mandatory fields (`Type`, `Version`, `NextHeartbeatDeadline`, `Role`) and optional fields (`Capabilities`, `Location`, `Peers`), JSON struct tags in canonical serialization order (FR-H01, FR-H02, FR-H03, FR-H04)
- [x] T015 [P] Implement `Capabilities` struct in `internal/health/payload.go` with fields `NATReachability` (bool), `NATType` (NATType), `Protocols` ([]ProtocolID), JSON tags in order `natReachability` -> `natType` -> `protocols` (FR-H03)
- [x] T016 [P] Implement `Location` struct in `internal/health/payload.go` with fields `Lat` (float64), `Lon` (float64), `Alt` (*float64), `Fix` (GPSFixQuality), JSON tags in order `lat` -> `lon` -> `alt` -> `fix` (FR-H03)
- [x] T017 Implement `LivenessRecord` struct in `internal/health/liveness.go` with fields `SenderAddress` (EVMAddress), `CurrentState` (LivenessState), `LastDeadline` (uint64), `LastSequence` (uint64), `LastConsensusTimestamp` (uint64) (FR-H18)
- [x] T018 Write test for `BuildHeartbeatPayload` in `internal/health/payload_test.go`: assert auto-sets `type = "heartbeat"` and `version = "1.0.0"`, accepts remaining fields, returns well-formed struct (FR-H01, FR-H02)
- [x] T019 Implement `BuildHeartbeatPayload` constructor in `internal/health/payload.go`: auto-sets `Type = "heartbeat"`, `Version = "1.0.0"`, accepts `nextHeartbeatDeadline`, `role`, `capabilities`, `location`, `peers` as input (FR-H01, FR-H02)
- [x] T020 Write test for deterministic JSON serialization in `internal/health/payload_test.go`: serialize the same HeartbeatPayload twice, assert byte-equal output; serialize with all optional fields and verify canonical field order (FR-H04, SC-H10)
- [x] T021 Implement deterministic JSON serialization for HeartbeatPayload in `internal/health/payload.go`: custom `MarshalJSON()` method enforcing field order `type` -> `version` -> `nextHeartbeatDeadline` -> `role` -> `capabilities` -> `location` -> `peers`, omitting nil optional fields (FR-H04, FR-T21, SC-H10)

**Checkpoint**: Foundation ready. HeartbeatPayload can be constructed and serialized deterministically. All types, constants, and errors defined. User story work can begin.

---

## Phase 3: US1 — Publish a Signed Heartbeat to stdOut (Priority: P1)

**Goal**: Build, validate, and publish a HeartbeatPayload to stdOut via TopicAdapter.

**Independent Test**: Build a HeartbeatPayload, run publisher validation (V-PUB-01..07), wrap in TopicMessage, publish to stdOut via any adapter, verify message is persisted on-chain with correct fields. Covers SC-H02, SC-H08, SC-H10.

**Covers**: SC-H02, SC-H07, SC-H08, SC-H10

### Tests (TDD: write before implementation)

- [x] T022 [US1] Write tests for `ValidateOutboundHeartbeat` in `internal/health/publisher_test.go`: test all 7 validation paths — V-PUB-01 reject wrong type, V-PUB-02 reject unsupported version (e.g. "2.0.0"), V-PUB-03 accept shutdown sentinel (deadline=0) bypassing all delta checks, V-PUB-04 reject deadline not in future, V-PUB-05 reject delta < MIN_DEADLINE_DELTA (10s), V-PUB-06 reject delta > MAX_DEADLINE_DELTA (86400s), V-PUB-07 reject unrecognized role (FR-H13, SC-H02)
- [x] T023 [US1] Write test for mandatory payload size budget in `internal/health/payload_test.go`: build a HeartbeatPayload with mandatory fields only, serialize to JSON, assert length < 256 bytes (FR-H29, SC-H08)
- [x] T024 [US1] Write tests for payload size trimming in `internal/health/publisher_test.go`: build a payload exceeding maxMessageSize, assert trim order is `peers` first, then `capabilities`, then `location`; assert mandatory fields are NEVER trimmed (FR-H29)
- [x] T025 [US1] Write tests for `PublishHeartbeat` orchestration in `internal/health/publisher_test.go`: test happy path (mock adapter returns success PublishResult), test failure path (mock adapter returns error, assert no retry, assert error logged), test FIRE_AND_FORGET confirmation mode passed to adapter (FR-H22, FR-H23, FR-H25)
- [x] T026 [US1] Write tests for `ScheduleNextHeartbeat` in `internal/health/publisher_test.go`: test with confirmed PublishResult (uses consensusTimestamp + chosenDelta), test with unconfirmed PublishResult (uses submitWallClock + chosenDelta) (FR-H24)
- [x] T027 [US1] Write test for rate limiting guard in `internal/health/publisher_test.go`: assert publish attempts faster than MIN_DEADLINE_DELTA interval are rejected (FR-H14)

### Implementation

- [x] T028 [US1] Implement `ValidateOutboundHeartbeat(payload HeartbeatPayload, senderClock uint64) error` in `internal/health/publisher.go` with ordered validation steps V-PUB-01 through V-PUB-07, returning appropriate sentinel errors (FR-H13, SC-H02)
- [x] T029 [US1] Implement `TrimPayload(payload *HeartbeatPayload, maxSize int) error` in `internal/health/publisher.go`: check serialized size against maxSize, trim `Peers` first, then `Capabilities`, then `Location`; return error if mandatory fields alone exceed maxSize (FR-H29)
- [x] T030 [US1] Implement `PublishHeartbeat(payload HeartbeatPayload, stdOutTopicRef TopicRef, adapter TopicAdapter) (PublishResult, error)` in `internal/health/publisher.go`: validate payload size and trim if needed, wrap in TopicMessage envelope, publish via `adapter.Publish()` with FIRE_AND_FORGET mode, return PublishResult on success, log and return error on failure without retry (FR-H22, FR-H23, FR-H25)
- [x] T031 [US1] Implement `ScheduleNextHeartbeat(publishResult PublishResult, chosenDelta uint64, submitWallClock uint64) uint64` in `internal/health/publisher.go`: use `publishResult.ConsensusTimestamp` if confirmed, else `submitWallClock`, return `referenceTime + chosenDelta` (FR-H24)
- [x] T032 [US1] Implement rate limiting guard in `internal/health/publisher.go`: track last publish timestamp, reject publish attempts within MIN_DEADLINE_DELTA of last successful publish (FR-H14)

**Checkpoint**: US1 complete. Heartbeats can be built, validated, trimmed, and published. SC-H02, SC-H07, SC-H08, SC-H10 are verifiable.

---

## Phase 4: US2 — Observe and Evaluate Peer Liveness (Priority: P1)

**Goal**: Subscribe to a peer's stdOut, validate incoming heartbeats, maintain LivenessRecords, evaluate liveness via five-state machine.

**Independent Test**: Subscribe to a peer's stdOut, receive heartbeats, run observer validation (V-OBS-01..06), verify all liveness state transitions (UNKNOWN->ALIVE->SUSPECT->DEAD, DEAD->ALIVE). Covers SC-H03, SC-H04, SC-H05, SC-H12.

**Covers**: SC-H03, SC-H04, SC-H05, SC-H12

### Tests (TDD: write before implementation)

- [x] T033 [US2] Write tests for `ValidateInboundHeartbeat` in `internal/health/observer_test.go`: test all 6 validation paths — V-OBS-01 reject bad signature / sender address mismatch, V-OBS-02 reject non-heartbeat payload type, V-OBS-03 reject incompatible major version (e.g. "2.0.0"), V-OBS-04 accept shutdown sentinel (deadline=0) and return valid, V-OBS-05 reject deadline in the past relative to consensus timestamp, V-OBS-06 reject delta below minimum / above maximum (FR-H15, SC-H03)
- [x] T034 [US2] Write test for FR-H27a (Parent-signed heartbeat rejection) in `internal/health/observer_test.go`: construct a heartbeat signed by a Parent's NeuronPrivateKey where the Parent's EVMAddress differs from the Child's senderAddress in the TopicMessage envelope, assert V-OBS-01 rejects with `ErrSenderAddressMismatch` (FR-H27a)
- [x] T035 [US2] Write tests for `EvaluateLiveness` algorithm in `internal/health/liveness_test.go`: test all branches — nil record returns UNKNOWN, deadline==0 returns OFFLINE, currentTime <= deadline+GRACE_PERIOD returns ALIVE, currentTime <= deadline+GRACE_PERIOD+SUSPECT_TO_DEAD returns SUSPECT, else returns DEAD (FR-H20, SC-H04)
- [x] T036 [US2] Write tests for all 8 liveness state transitions in `internal/health/liveness_test.go`: UNKNOWN->ALIVE (first valid HB), ALIVE->ALIVE (new valid HB, reset deadline), ALIVE->SUSPECT (deadline+GP expired), ALIVE->OFFLINE (deadline=0), SUSPECT->ALIVE (new valid HB), SUSPECT->DEAD (deadline+GP+S2D expired), DEAD->ALIVE (new valid HB), OFFLINE->ALIVE (new valid HB with deadline>0) (FR-H19, SC-H04)
- [x] T037 [US2] Write tests for `UpdateLivenessRecord` in `internal/health/observer_test.go`: test nil record creates new (ALIVE or OFFLINE), test sequence ordering (higher seq wins, lower seq ignored per FR-H17), test shutdown transition to OFFLINE, test normal update to ALIVE with field updates, test recovery invariant from any state per FR-H21 (FR-H17, FR-H21, SC-H12)
- [x] T038 [US2] Write test for consensus timestamp authority in `internal/health/observer_test.go`: assert all deadline arithmetic in ValidateInboundHeartbeat and EvaluateLiveness uses consensusTimestamp, NEVER sender timestamp or local clock (FR-H16, SC-H05)
- [x] T039 [US2] Write tests for `SubscribeToHeartbeats` in `internal/health/observer_test.go`: test subscription setup via adapter, test non-heartbeat messages are skipped, test valid heartbeats update LivenessRecord, test subscribe resumption via fromSequence (FR-T25)

### Implementation

- [x] T040 [US2] Implement `ValidateInboundHeartbeat(topicMessage TopicMessage, consensusTimestamp uint64) error` in `internal/health/observer.go` with ordered validation steps V-OBS-01 through V-OBS-06: signature verification against senderAddress (delegates to Spec 002 keylib), payload type check, version compatibility (major==1), shutdown detection (deadline=0 returns nil), deadline vs consensus, delta bounds (FR-H15, FR-H27a, SC-H03)
- [x] T041 [US2] Implement `EvaluateLiveness(record *LivenessRecord, currentTime uint64) LivenessState` in `internal/health/liveness.go` as a pure function: nil->UNKNOWN, deadline==0->OFFLINE, currentTime<=deadline+GracePeriod->ALIVE, currentTime<=deadline+GracePeriod+SuspectToDead->SUSPECT, else->DEAD (FR-H20, SC-H04)
- [x] T042 [US2] Implement liveness state transition logic in `internal/health/liveness.go`: `TransitionState(record *LivenessRecord, newHeartbeat *HeartbeatPayload, consensusTimestamp uint64)` enforcing all 8 transitions from FR-H19, including recovery invariant (FR-H21) (FR-H19, FR-H21)
- [x] T043 [US2] Implement `UpdateLivenessRecord(record *LivenessRecord, heartbeat HeartbeatPayload, consensusTimestamp uint64, sequenceNumber uint64) *LivenessRecord` in `internal/health/observer.go`: create new record if nil, enforce sequence ordering (FR-H17), delegate state transition, update lastDeadline/lastSequence/lastConsensusTimestamp (FR-H17, FR-H18, SC-H12)
- [x] T044 [US2] Implement `SubscribeToHeartbeats(stdOutTopicRef TopicRef, adapter TopicAdapter, fromSequence uint64) (<-chan MessageDelivery, error)` in `internal/health/observer.go`: subscribe via adapter (FR-T25), filter heartbeat messages, run ValidateInboundHeartbeat, update LivenessRecord (FR-H15, FR-T25)
- [x] T045 [US2] Enforce consensus timestamp authority throughout `internal/health/observer.go`: all deadline arithmetic uses `MessageDelivery.ConsensusTimestamp`, never sender timestamp or local clock (FR-H16, FR-T24, FR-T29, SC-H05)

**Checkpoint**: US1 + US2 complete. Full publish -> observe -> evaluate lifecycle works. SC-H03, SC-H04, SC-H05, SC-H12 verifiable.

---

## Phase 5: US3 — Graceful Shutdown Signaling (Priority: P2)

**Goal**: Publish shutdown sentinel (deadline=0), observe OFFLINE transition, verify recovery.

**Independent Test**: Publish shutdown heartbeat, verify observer transitions to OFFLINE, publish recovery heartbeat, verify transition to ALIVE. Covers SC-H06.

**Covers**: SC-H06
**Dependencies**: Requires US1 (T028-T032) + US2 (T040-T045) complete.

### Tests (TDD: write before implementation)

- [x] T046 [US3] Write end-to-end test for shutdown sentinel in `internal/health/publisher_test.go`: build payload with `nextHeartbeatDeadline = 0`, validate via `ValidateOutboundHeartbeat`, assert V-PUB-03 bypass (no delta checks), publish succeeds (FR-H12, FR-H13 V-PUB-03)
- [x] T047 [US3] Write end-to-end test for OFFLINE transition in `internal/health/observer_test.go`: receive shutdown heartbeat (deadline=0), validate via `ValidateInboundHeartbeat` (V-OBS-04 accept), call `UpdateLivenessRecord`, assert state transitions to OFFLINE (FR-H12, FR-H19)
- [x] T048 [US3] Write end-to-end test for OFFLINE->ALIVE recovery in `internal/health/liveness_test.go`: from OFFLINE state, receive new valid heartbeat with non-zero deadline, assert transition to ALIVE (FR-H21, SC-H06)
- [x] T049 [US3] Write integrated shutdown/recovery cycle test in `internal/health/observer_test.go`: publish normal HB (ALIVE), publish shutdown (OFFLINE), publish recovery HB (ALIVE again) — full cycle end-to-end (SC-H06)

### Implementation

- [x] T050 [US3] Verify and harden shutdown sentinel path in `internal/health/publisher.go`: ensure `ValidateOutboundHeartbeat` returns nil error immediately when `NextHeartbeatDeadline == ShutdownSentinel`, no delta/future checks applied (FR-H12, FR-H13 V-PUB-03)
- [x] T051 [US3] Verify and harden OFFLINE transition path in `internal/health/observer.go`: ensure `ValidateInboundHeartbeat` returns nil for deadline=0 (V-OBS-04), and `UpdateLivenessRecord` transitions to OFFLINE (FR-H12, FR-H19)
- [x] T052 [US3] Verify and harden OFFLINE->ALIVE recovery in `internal/health/liveness.go`: ensure `TransitionState` transitions from OFFLINE to ALIVE on receipt of valid heartbeat with deadline > 0 (FR-H21)

**Checkpoint**: US3 complete. Graceful shutdown and recovery verified. SC-H06 verifiable.

---

## Phase 6: US4 — Self-Tuning Heartbeat Cadence (Priority: P2)

**Goal**: Full range delta acceptance (10s to 86400s), cadence change between heartbeats.

**Independent Test**: Publish heartbeats with varying deltas (10s, 60s, 3600s, 86400s), verify all pass publisher validation (V-PUB-05, V-PUB-06) and observer validation (V-OBS-06). Covers SC-H07.

**Covers**: SC-H07
**Dependencies**: Requires US1 (T028-T032) complete.

### Tests (TDD: write before implementation)

- [x] T053 [US4] Write boundary delta tests in `internal/health/publisher_test.go`: assert `ValidateOutboundHeartbeat` accepts exactly delta=10 (MIN_DEADLINE_DELTA), exactly delta=86400 (MAX_DEADLINE_DELTA), rejects delta=9, rejects delta=86401 (FR-H06, FR-H07, SC-H07)
- [x] T054 [US4] Write full-range delta tests in `internal/health/publisher_test.go`: table-driven test with deltas [10, 30, 60, 300, 3600, 43200, 86400] — all must pass validation (FR-H11, SC-H07)
- [x] T055 [US4] Write cadence change test in `internal/health/publisher_test.go`: publish heartbeat with delta=60, then delta=300, then delta=15 — assert all pass validation; nodes MAY change cadence at any time (FR-H11)
- [x] T056 [US4] Write `ScheduleNextHeartbeat` boundary tests in `internal/health/publisher_test.go`: test with delta=10 (minimum) and delta=86400 (maximum), with both confirmed and unconfirmed PublishResults (FR-H24, SC-H07)
- [x] T057 [US4] Write observer-side delta boundary tests in `internal/health/observer_test.go`: assert `ValidateInboundHeartbeat` accepts delta=10 and delta=86400, rejects delta=9 and delta=86401 via V-OBS-06 (FR-H11, FR-H15)

### Implementation

- [x] T058 [US4] Verify boundary acceptance in `internal/health/publisher.go`: confirm V-PUB-05 uses `<` (not `<=`) for MIN comparison and V-PUB-06 uses `>` (not `>=`) for MAX comparison, so exact boundary values pass (FR-H06, FR-H07)
- [x] T059 [US4] Verify boundary acceptance in `internal/health/observer.go`: confirm V-OBS-06 uses `<` for MIN and `>` for MAX, matching publisher behavior (FR-H11)

**Checkpoint**: US4 complete. Full cadence range validated. SC-H07 verifiable.

---

## Phase 7: US5 — Health Status Broadcast (Priority: P2)

**Goal**: Publish optional fields (capabilities, location, peers), verify observer parsing.

**Independent Test**: Publish heartbeats with full optional fields, verify serialization, verify observers can parse capabilities/location/peers, verify peers field is treated as informational only. Covers SC-H08.

**Covers**: SC-H08
**Dependencies**: Requires Phase 2 (T014-T021) + US1 (T028-T032) complete.

### Tests (TDD: write before implementation)

- [x] T060 [US5] Write tests for capabilities field handling in `internal/health/payload_test.go`: build payload with `Capabilities{NATReachability: true, NATType: "endpoint-independent", Protocols: ["/adsb/v1"]}`, verify serialization includes all sub-fields in canonical order (FR-H03)
- [x] T061 [US5] Write tests for location field handling in `internal/health/payload_test.go`: test with lat+lon (required when location present), test with lat+lon+alt+fix (all fields), test that location without lat or lon is rejected (FR-H03)
- [x] T062 [US5] Write tests for peers field handling in `internal/health/payload_test.go`: build payload with `Peers: ["a1b2", "c3d4"]` (4-char hex AbbreviatedAddress), verify serialization, verify format validation rejects non-4-char or non-hex values (FR-H03, FR-H27)
- [x] T063 [US5] Write test for peers trust warning in `internal/health/observer_test.go`: assert observer code does NOT use peers field for trust, routing, or identity decisions; verify peers data is passed through as informational only (FR-H27)

### Implementation

- [x] T064 [US5] Implement optional field validation in `internal/health/payload.go`: validate `Location` requires both `Lat` and `Lon` when present, validate `AbbreviatedAddress` is exactly 4 hex characters, validate `NATType` enum membership (FR-H03)
- [x] T065 [US5] Implement optional field serialization in `internal/health/payload.go`: ensure `MarshalJSON` includes `capabilities`, `location`, `peers` only when non-nil, in canonical order after mandatory fields (FR-H03, FR-H04)
- [x] T066 [US5] Add peers trust documentation and observer guard in `internal/health/observer.go`: code comment on `peers` field per FR-H27 — peers MUST NOT be used for trust, routing, or identity decisions (FR-H27)

**Checkpoint**: US5 complete. Health status broadcast with all optional fields works. SC-H08 verifiable.

---

## Phase 8: US6 — Cross-Chain Heartbeat Verification (Priority: P3)

**Goal**: External observer on a non-Hedera EVM chain can verify protocol compliance from chain data alone.

**Independent Test**: Deploy SDK on a non-Hedera EVM chain, publish heartbeats, have an independent observer verify (signature, senderAddress, stdOut topic, schema, deadline bounds) from chain data. Covers SC-H01, SC-H11.

**Covers**: SC-H01, SC-H11
**Dependencies**: Requires US1 (T028-T032) + US2 (T040-T045) complete. Requires Spec 004 multi-chain adapter support.

### Tests (TDD: write before implementation)

- [x] T067 [US6] Write test for backend-agnostic payload schema in `internal/health/payload_test.go`: serialize HeartbeatPayload, assert identical JSON structure regardless of target backend (HCS, EVM, Kafka) (SC-H01)
- [x] T068 [US6] Write test for EVM block.timestamp as consensus in `internal/health/observer_test.go`: construct a MessageDelivery with consensusTimestamp sourced from EVM `block.timestamp`, run `ValidateInboundHeartbeat` and `EvaluateLiveness`, assert correct behavior (FR-H16, FR-T29)
- [x] T069 [US6] Write cross-chain determinism test in `internal/health/liveness_test.go`: given identical (consensusTimestamp, nextHeartbeatDeadline, currentTime, constants), assert `EvaluateLiveness` produces the same LivenessState regardless of which backend supplied the timestamps (SC-H04, SC-H11)

### Implementation

- [x] T070 [US6] Verify chain-agnostic semantics in `internal/health/observer.go`: confirm `ValidateInboundHeartbeat` and `EvaluateLiveness` operate on abstract uint64 timestamps with no backend-specific logic (FR-H16, SC-H01)
- [x] T071 [US6] Document cross-chain verification properties in `internal/health/observer.go`: code comments explaining consensusTimestamp maps to HCS consensus timestamp, EVM `block.timestamp`, or Kafka anchor timestamp per FR-T29 (SC-H11)

**Checkpoint**: US6 complete. Cross-chain verification proven. SC-H01, SC-H11 verifiable.

---

## Phase 9: Polish

**Purpose**: HCS integration test (Constitution VIII), deterministic signing test (Constitution X), FR-H27a coverage, cross-chain test, version compatibility, final verification.

### Constitution VIII — HCS Binding (MUST precede cross-chain tests)

- [x] T072 Write HCS end-to-end integration test in `internal/health/integration_test.go`: publish a HeartbeatPayload via HCS TopicAdapter, subscribe and receive the MessageDelivery, run ValidateInboundHeartbeat with HCS consensus timestamp, run EvaluateLiveness, assert full lifecycle (UNKNOWN -> ALIVE) (Constitution VIII, SC-H01)

### Constitution X — Deterministic Signing

- [x] T073 Write deterministic signing test in `internal/health/signing_test.go`: sign the same HeartbeatPayload TopicMessage twice using the same NeuronPrivateKey (Spec 002), assert byte-equal R||S||V signatures (Constitution X, FR-T03)

### FR-H27a — Parent-Signed Heartbeat Rejection

- [x] T074 Write Parent-signed heartbeat rejection test in `internal/health/observer_test.go`: generate a Parent NeuronPrivateKey and a Child NeuronPrivateKey with different EVMAddresses, sign a heartbeat with the Parent's key but set `senderAddress` to the Child's EVMAddress in the TopicMessage envelope, assert `ValidateInboundHeartbeat` rejects with V-OBS-01 (`ErrSenderAddressMismatch`) because recovered signing address (Parent) != senderAddress (Child) (FR-H27a)

### FR-H22 — Observer-Side stdOut Enforcement

- [x] T075a Write test for FR-H22 observer-side enforcement in `internal/health/observer_test.go`: construct a valid heartbeat received on a non-stdOut topic (e.g., stdIn or stdErr TopicRef), assert `SubscribeToHeartbeats` or a dedicated guard rejects/filters it; verify that only heartbeats arriving on the sender's stdOut topic are accepted for liveness evaluation — "A heartbeat found on any other topic MUST be rejected by observers" (FR-H22)

### Version Compatibility

- [x] T075b [P] Write version compatibility tests in `internal/health/observer_test.go`: accept `version: "1.0.0"`, accept `version: "1.1.0"`, accept `version: "1.99.0"` (ignore unknown fields), reject `version: "2.0.0"`, reject `version: "0.9.0"` (FR-H28, SC-H09)
- [x] T076 [P] Implement version compatibility logic in `internal/health/observer.go`: parse SemVer, accept major==1 (any minor/patch), reject major>=2 or major==0 — satisfies FR-H28, SC-H09

### Security Documentation

- [x] T077 [P] Document security properties in `internal/health/doc.go`: heartbeats are NOT encrypted (FR-H26, public channel), peers field is gossip-grade (FR-H27), signature prevents forgery (Spec 002) (FR-H26, FR-H27)

### Deterministic Serialization Round-Trip

- [x] T078 Write deterministic serialization round-trip test in `internal/health/payload_test.go`: serialize -> deserialize -> re-serialize produces identical bytes for all HeartbeatPayload variants (mandatory-only, with capabilities, with location, with peers, with all fields) (SC-H10, FR-H04)

### Cross-Chain Integration (MUST follow T072 HCS integration test per Constitution VIII)

- [x] T079 Write cross-chain integration test in `internal/health/cross_chain_test.go`: publish the same HeartbeatPayload to two different backend adapters (e.g. HCS and EVM mock), retrieve from both, assert identical HeartbeatPayload content and identical liveness evaluation results (SC-H01, SC-H11)

### Quickstart Validation

- [x] T080 Validate quickstart.md scenarios: verify all code examples in `specs/005-health/quickstart.md` are accurate against implemented API signatures and behaviors — no FR (documentation accuracy)

---

## Dependencies & Execution Order

### Phase Dependencies (DAG)

```
Phase 1: Setup
    |
    v
Phase 2: Foundational  (BLOCKS all user stories)
    |
    +---> Phase 3: US1 (P1) ----+---> Phase 5: US3 (P2) [needs US1+US2]
    |                            |
    +---> Phase 4: US2 (P1) ----+---> Phase 6: US4 (P2) [needs US1]
    |                            |
    |                            +---> Phase 7: US5 (P2) [needs US1+Phase2]
    |                            |
    |                            +---> Phase 8: US6 (P3) [needs US1+US2]
    |
    +---> Phase 9: Polish [needs US1+US2; T072 before T079]
```

### Cross-Spec Blocking Dependencies

| Dependency | Source | Status |
|------------|--------|--------|
| `internal/keylib` (NeuronPrivateKey, Signature, ECDSA R\|\|S\|\|V) | Spec 002 | Required |
| `internal/topic` (TopicMessage, TopicAdapter, PublishResult, MessageDelivery, TopicRef) | Spec 004 | Required |
| FR-T22..T29 (Publish/Subscribe Execution Binding) | Spec 004 | Resolved (added to Spec 004 spec.md) |

### Intra-Phase Task Dependencies

| Phase | Sequential Chain | Rationale |
|-------|-----------------|-----------|
| Phase 2 | T005-T008 (tests) -> T009-T012 (impl) -> T013 (test) -> T014-T016 (impl) -> T017 -> T018 (test) -> T019 (impl) -> T020 (test) -> T021 (impl) | TDD: test before impl; structs before serialization |
| Phase 3 | T022-T027 (tests) -> T028-T032 (impl) | TDD: all test stubs first, then make them pass |
| Phase 4 | T033-T039 (tests) -> T040-T045 (impl) | TDD: all test stubs first, then make them pass |
| Phase 5 | T046-T049 (tests) -> T050-T052 (impl) | TDD: test shutdown/recovery paths |
| Phase 6 | T053-T057 (tests) -> T058-T059 (impl) | TDD: boundary tests first |
| Phase 7 | T060-T063 (tests) -> T064-T066 (impl) | TDD: optional field tests first |
| Phase 8 | T067-T069 (tests) -> T070-T071 (impl) | TDD: cross-chain tests first |
| Phase 9 | T072 (HCS integration) MUST precede T079 (cross-chain integration) per Constitution VIII |

---

## Parallel Opportunities

### Within Setup (Phase 1)
- T002, T003, T004 — different files, no dependencies

### Within Foundational (Phase 2)
- T005, T006, T007 — test files for constants, types, errors (different files)
- T009, T010, T011 — implementation of constants, types, errors (different files)
- T015, T016 — Capabilities and Location structs (same file, different structs)

### Between User Stories (After Phase 2)
- **US1 (Phase 3) || US2 (Phase 4)** — publisher.go vs observer.go, completely independent files
- **US4 (Phase 6) || US5 (Phase 7)** — cadence validation vs optional fields, independent concerns
- **US4 (Phase 6) || US3 (Phase 5)** — if US2 is already complete, US3 and US4 can run in parallel

### Within Polish (Phase 9)
- T075, T076 (version compat) || T077 (security docs) — independent concerns

### Two-Developer Parallel Strategy

```
Developer A (Publisher track):        Developer B (Observer track):
  Phase 1: Setup (shared)               Phase 1: Setup (shared)
  Phase 2: Foundational (shared)         Phase 2: Foundational (shared)
  Phase 3: US1 (publisher.go)            Phase 4: US2 (observer.go, liveness.go)
  Phase 6: US4 (cadence)                 Phase 7: US5 (optional fields)
  Phase 5: US3 (shutdown — shared)       Phase 5: US3 (shutdown — shared)
  Phase 8: US6 (cross-chain)            Phase 9: Polish
```

---

## Implementation Strategy

### 1. MVP First (US1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T021)
3. Complete Phase 3: US1 — Publish Heartbeat (T022-T032)
4. **STOP AND VALIDATE**: Build a heartbeat, validate it, publish to stdOut. SC-H02, SC-H08, SC-H10 pass.

### 2. Incremental Delivery

| Step | Phase | Validates | Milestone |
|------|-------|-----------|-----------|
| 1 | Setup + Foundational | Compiles, types exist | Foundation |
| 2 | US1 (Publish) | SC-H02, SC-H07, SC-H08, SC-H10 | **MVP** |
| 3 | US2 (Observe) | SC-H03, SC-H04, SC-H05, SC-H12 | Full lifecycle |
| 4 | US3 (Shutdown) | SC-H06 | Graceful offline |
| 5 | US4 (Cadence) + US5 (Status) | SC-H07 boundaries, SC-H08 | Production-ready |
| 6 | US6 (Cross-Chain) | SC-H01, SC-H11 | Suite portability acceptance test |
| 7 | Polish | SC-H09, Constitution VIII, X, FR-H27a | Ship |

### 3. FR/SC Traceability Summary

| Requirement | Task(s) |
|-------------|---------|
| FR-H01 | T013, T014, T018, T019 |
| FR-H02 | T006, T010, T013, T014, T018, T019 |
| FR-H03 | T006, T010, T015, T016, T060-T066 |
| FR-H04 | T020, T021, T065, T078 |
| FR-H05 | T006, T010 |
| FR-H06 | T005, T009, T053, T058 |
| FR-H07 | T005, T009, T053, T058 |
| FR-H08 | T005, T009, T035 |
| FR-H09 | T005, T009, T035 |
| FR-H10 | T022, T028 |
| FR-H11 | T022, T028, T054, T057 |
| FR-H12 | T005, T009, T046-T052 |
| FR-H13 | T007, T011, T022, T028 |
| FR-H14 | T027, T032 |
| FR-H15 | T007, T011, T033, T040 |
| FR-H16 | T038, T045, T068, T070 |
| FR-H17 | T037, T043 |
| FR-H18 | T008, T012, T017 |
| FR-H19 | T036, T042, T047 |
| FR-H20 | T035, T041 |
| FR-H21 | T037, T042, T048, T052 |
| FR-H22 | T025, T030, T075a |
| FR-H23 | T025, T030 |
| FR-H24 | T026, T031, T056 |
| FR-H25 | T025, T030 |
| FR-H26 | T077 |
| FR-H27 | T062, T063, T066 |
| FR-H27a | T034, T040, T074 |
| FR-H28 | T075, T076 |
| FR-H29 | T023, T024, T029 |
| SC-H01 | T067, T072, T079 |
| SC-H02 | T022, T028 |
| SC-H03 | T033, T040 |
| SC-H04 | T035, T036, T041, T042, T069 |
| SC-H05 | T038, T045 |
| SC-H06 | T046-T052 |
| SC-H07 | T053-T059 |
| SC-H08 | T023, T060-T065 |
| SC-H09 | T075, T076 |
| SC-H10 | T020, T021, T078 |
| SC-H11 | T068-T071, T079 |
| SC-H12 | T037, T043 |
| Constitution VIII | T072 (HCS integration, precedes T079) |
| Constitution IX | TDD order enforced in every phase |
| Constitution X | T073 (deterministic signing) |

---

## Notes

- All paths are Go-specific: `internal/health/*.go` and `internal/health/*_test.go`
- [P] tasks = different files, no dependencies on incomplete tasks
- [US#] label maps task to specific user story for traceability
- TDD (Constitution IX): test tasks always precede implementation tasks within each phase
- Constitution VIII: T072 (HCS integration) MUST complete before T079 (cross-chain integration)
- Constitution X: T073 tests deterministic R||S||V signing
- FR-H27a: T034 and T074 explicitly test Parent-signed heartbeat rejection
- Total: 81 tasks across 9 phases (T075a added for FR-H22 observer-side enforcement)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently

---

## Phase 10 — 2026-05-08 Amendment (Degraded-Mode Liveness)

- **T-NA-201 [test]** — Topic-adapter-fault test: with the observer's topic adapter simulated as unreachable for 5 minutes during which a peer's `nextHeartbeatDeadline` lapses, verify the observer does NOT advance the peer to DEAD solely on missed heartbeats. Trace: SC-H13.
- **T-NA-202** — Implement `LivenessObserver.Status()` returning canonical state + `Confidence` value. Trace: FR-H30, FR-H31.
- **T-NA-203 [test]** — Data-plane reachability advisory: an observer with both stalled heartbeats and an active 009 delivery channel reports `Confidence = "data-plane-evidence"`. Trace: SC-H14.
- **T-NA-204** — Wire 009 delivery-channel state into `LivenessObserver.Status()` as optional advisory input. Trace: FR-H31.
- **T-NA-205 [test]** — Cached last-known-good across restart: observer is persisted, killed during control-plane outage, restarted; verify it uses the cached record as baseline rather than declaring UNKNOWN. Trace: FR-H32.
- **T-NA-206** — Add `LivenessRecord.persisted` field and persistence hook (opt-in). Trace: FR-H32.
- **T-NA-207 [test]** — Publisher-side enforcement: simulate publisher's topic adapter recovery; verify the publisher resumes per FR-H22–H25 without skipping heartbeats. Trace: FR-H33.

### Phase 10 totals

- Test tasks: 4
- Implementation tasks: 3
- Total Phase 10: 7 tasks
- Cumulative project total: 81 (original) + 7 (Phase 10) = 88 tasks
