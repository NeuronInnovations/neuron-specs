# Tasks: Health — TypeScript (Spec 005)

**Date**: 2026-03-18 | **Spec**: [spec.md](spec.md) | **Plan**: [ts-plan.md](ts-plan.md)

---

## Phase 1: Foundational

- [ ] T001 [P] Write tests + implement HealthError with NEURON-HEALTH-001..007 codes. Files: `src/health/errors.ts`, `tests/health/error.test.ts` (FR-H13, FR-H15)
- [ ] T002 [P] Write tests + implement constants: MIN_DEADLINE_DELTA=10, MAX_DEADLINE_DELTA=86400, GRACE_PERIOD=30, SUSPECT_TO_DEAD=120, SHUTDOWN_SENTINEL=0. Files: `src/health/constants.ts`, `tests/health/constants.test.ts` (FR-H06..H09, FR-H12)
- [ ] T003 [P] Write tests + implement types: NodeRole (buyer/seller/relay/validator), NATType, GPSFixQuality, LivenessState (5 states), HeartbeatPayload interface, Capabilities, Location, LivenessRecord. Files: `src/health/types.ts`, `tests/health/types.test.ts` (FR-H02, FR-H03, FR-H05, FR-H18)

---

## Phase 2: HeartbeatPayload + Chain 3 Conformance 🎯

- [ ] T004 [P] [US1] Write failing tests for HeartbeatPayload.build(): mandatory fields set (type="heartbeat", version="1.0.0"), toCanonicalJson() produces correct field order (type→version→nextHeartbeatDeadline→role→capabilities?→location?→peers?), absent optionals omitted, UnsignedInt64 as JSON strings (FR-W02), Float64 with .0 (FR-W10). File: `tests/health/payload.test.ts` (FR-H01, FR-H02, FR-H04, FR-W02, FR-W04, FR-W10)
- [ ] T005 [US1] Implement HeartbeatPayload class: `static build(opts)` auto-sets type/version, `toCanonicalJson()` using serializeCanonicalJson with correct field order including nested Capabilities and Location serialization. File: `src/health/payload.ts` (FR-H01, FR-H02, FR-H04)
- [ ] T006 [US1] Update and run Chain 3 conformance test: `tests/conformance/chain3-heartbeat-signing.test.ts`. HeartbeatPayload JSON must match test vector byte-for-byte. Signing hash and signature must match. File: `tests/conformance/chain3-heartbeat-signing.test.ts` (FR-A09)

---

## Phase 3: Publisher + Observer Validation

- [ ] T007 [P] [US2] Write failing tests for ValidateOutboundHeartbeat: V-PUB-01 (type check), V-PUB-02 (version), V-PUB-03 (shutdown bypass), V-PUB-04 (future deadline), V-PUB-05 (min delta), V-PUB-06 (max delta), V-PUB-07 (role). File: `tests/health/publisher.test.ts` (FR-H13, SC-H02, SC-H07)
- [ ] T008 [US2] Implement validateOutboundHeartbeat(payload, senderClock). File: `src/health/publisher.ts` (FR-H13)
- [ ] T009 [P] [US3] Write failing tests for ValidateInboundHeartbeat: V-OBS-01 (signature), V-OBS-02 (type), V-OBS-03 (version major=1), V-OBS-04 (shutdown bypass), V-OBS-05 (future deadline vs consensus), V-OBS-06 (delta range). File: `tests/health/observer.test.ts` (FR-H15, SC-H03)
- [ ] T010 [US3] Implement validateInboundHeartbeat(topicMessage, consensusTimestamp). File: `src/health/observer.ts` (FR-H15, FR-H16)

---

## Phase 4: Liveness State Machine

- [ ] T011 [P] [US3] Write failing tests for evaluateLiveness: null→UNKNOWN, deadline=0→OFFLINE, within grace→ALIVE, past grace→SUSPECT, past grace+suspect→DEAD. Recovery: any state + valid HB → ALIVE. File: `tests/health/liveness.test.ts` (FR-H18, FR-H19, FR-H20, FR-H21, SC-H04, SC-H06)
- [ ] T012 [US3] Implement evaluateLiveness(record, currentTime) pure function + updateLivenessRecord(record, heartbeat, consensusTimestamp, sequenceNumber) with sequence ordering (FR-H17). File: `src/health/liveness.ts` (FR-H18, FR-H19, FR-H20, FR-H21)

---

## Phase 5: Polish

- [ ] T013 Create barrel exports. File: `src/health/index.ts`
- [ ] T014 Run all health tests + Chain 3 conformance. All pass.

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 14 |
| FR coverage | 29/29 FR-H* traced |
| Conformance gates | Chain 3 (HeartbeatPayload signing) |
| Validation rules | 7 V-PUB + 6 V-OBS = 13 |
