# Spec 005 — Health: Requirements Checklist

**Feature Branch**: `005-health`
**Generated**: 2026-02-24
**Status**: All items verified

---

## Functional Requirements Traceability

| FR ID | Requirement Summary | Source Document | Section | Status |
|-------|---------------------|-----------------|---------|--------|
| FR-H01 | HeartbeatPayload definition as TopicMessage payload | heartbeat-protocol.md | §2.1–2.2 | Done |
| FR-H02 | Mandatory fields (type, version, nextHeartbeatDeadline, role) | heartbeat-protocol.md | §2.3 | Done |
| FR-H03 | Optional fields (capabilities, location, peers) | heartbeat-protocol.md | §2.3 | Done |
| FR-H04 | Deterministic field ordering | heartbeat-protocol.md | §2.4 | Done |
| FR-H05 | Role enumeration (buyer, seller, relay; validator reserved) | heartbeat-protocol.md | §2.3 | Done |
| FR-H06 | MIN_DEADLINE_DELTA = 10s | heartbeat-protocol.md | §1.3 | Done |
| FR-H07 | MAX_DEADLINE_DELTA = 86400s | heartbeat-protocol.md | §1.3 | Done |
| FR-H08 | GRACE_PERIOD = 30s | heartbeat-protocol.md | §1.3 | Done |
| FR-H09 | SUSPECT_TO_DEAD = 120s | heartbeat-protocol.md | §1.3 | Done |
| FR-H10 | Deadline must be future relative to consensus timestamp | heartbeat-protocol.md | §4.1 INV-P02 | Done |
| FR-H11 | Delta bounds enforcement | heartbeat-protocol.md | §4.1 INV-P03 | Done |
| FR-H12 | SHUTDOWN_SENTINEL = 0 | heartbeat-protocol.md | §1.2 | Done |
| FR-H13 | Publisher validation algorithm (V-PUB-01..07) | heartbeat-protocol.md | §3.1 | Done |
| FR-H14 | Rate limiting (one per MIN_DEADLINE_DELTA) | heartbeat-protocol.md | §4.1 INV-P06 | Done |
| FR-H15 | Observer validation algorithm (V-OBS-01..06) | heartbeat-protocol.md | §3.2 | Done |
| FR-H16 | Consensus timestamp authority | heartbeat-protocol.md | §3.2 note, §4.2 INV-O01 | Done |
| FR-H17 | Highest-sequence rule | heartbeat-protocol.md | §4.2 INV-O06 | Done |
| FR-H18 | Five liveness states | heartbeat-protocol.md | §3.4 | Done |
| FR-H19 | State transition rules | heartbeat-protocol.md | §3.3–3.4 | Done |
| FR-H20 | Liveness evaluation algorithm | heartbeat-protocol.md | §3.3 | Done |
| FR-H21 | Recovery always possible | heartbeat-protocol.md | §4.2 INV-O04 | Done |
| FR-H22 | stdOut exclusivity | heartbeat-protocol.md | §4.3 INV-S01 | Done |
| FR-H23 | FIRE_AND_FORGET recommendation | transport-gap-analysis.md | §4.5 | Done |
| FR-H24 | Deadline scheduling after publish | transport-gap-analysis.md | §4.3 | Done |
| FR-H25 | Failure handling (log, skip, self-heal) | transport-gap-analysis.md | §4.4 | Done |
| FR-H26 | No encryption (public channel) | heartbeat-protocol.md | §4.3 INV-S04 | Done |
| FR-H27 | Peers field not for trust | heartbeat-protocol.md | §4.2 INV-O05 | Done |
| FR-H28 | Version compatibility (1.x.y accept, 2.x.y reject) | heartbeat-protocol.md | §6.3 | Done |
| FR-H29 | Payload size budget (256 bytes mandatory, trim order) | transport-gap-analysis.md | §5.2, architecture.md §7 | Done |

## Success Criteria Mapping

| SC ID | Criterion Summary | Maps to FRs | Status |
|-------|-------------------|-------------|--------|
| SC-H01 | End-to-end publish + retrieve on 2+ backends | FR-H01, FR-H22, FR-H23 | Done |
| SC-H02 | Publisher validation rejects 100% invalid | FR-H13 (V-PUB-01..07) | Done |
| SC-H03 | Observer validation rejects 100% invalid | FR-H15 (V-OBS-01..06) | Done |
| SC-H04 | Deterministic liveness transitions | FR-H18, FR-H19, FR-H20 | Done |
| SC-H05 | Consensus timestamp for ALL deadline arithmetic | FR-H16 | Done |
| SC-H06 | Shutdown → OFFLINE → ALIVE round-trip | FR-H12, FR-H21 | Done |
| SC-H07 | Full cadence range (10s to 86400s) | FR-H06, FR-H07, FR-H11 | Done |
| SC-H08 | Minimum viable heartbeat < 256 bytes | FR-H29 | Done |
| SC-H09 | Version forward-compatibility | FR-H28 | Done |
| SC-H10 | Deterministic JSON serialization round-trip | FR-H04 | Done |
| SC-H11 | External observer on non-Hedera chain | FR-H15, FR-H22 | Done |
| SC-H12 | Highest-sequence heartbeat used | FR-H17 | Done |

## User Stories Coverage

| US # | Title | Priority | Acceptance Scenarios | Status |
|------|-------|----------|---------------------|--------|
| US1 | Publish a Signed Heartbeat to stdOut | P1 | 5 Given/When/Then | Done |
| US2 | Observe and Evaluate Peer Liveness | P1 | 6 Given/When/Then | Done |
| US3 | Graceful Shutdown Signaling | P2 | 3 Given/When/Then | Done |
| US4 | Self-Tuning Heartbeat Cadence | P2 | 4 Given/When/Then | Done |
| US5 | Health Status Broadcast | P2 | 4 Given/When/Then | Done |
| US6 | Cross-Chain Heartbeat Verification | P3 | 3 Given/When/Then | Done |

## Edge Cases Coverage

| # | Edge Case | Covered | Source |
|---|-----------|---------|--------|
| 1 | Past deadline | Yes | V-OBS-05 / heartbeat-protocol.md §5.3 |
| 2 | Immortal deadline | Yes | V-OBS-06 / heartbeat-protocol.md §5.4 |
| 3 | Payload size overflow | Yes | FR-H29 / transport-gap-analysis.md §3.1 B5 |
| 4 | Sequence conflicts | Yes | FR-H17 / heartbeat-protocol.md §4.2 INV-O06 |
| 5 | Publish failure | Yes | FR-H25 / transport-gap-analysis.md §4.4 |
| 6 | Subscription gaps | Yes | FR-T25 / transport-gap-analysis.md §4.3 |
| 7 | Replay attacks | Yes | heartbeat-protocol.md §7.5 |
| 8 | Shutdown forgery | Yes | heartbeat-protocol.md §7.7 |
| 9 | Version 2.x.y rejection | Yes | FR-H28 / heartbeat-protocol.md §6.3 |

## Constitution v1.2.0 Compliance

| Requirement | Status |
|-------------|--------|
| Related specs section present | Done |
| Mermaid diagrams in appendix | Done (5 diagrams: ER, state machine, sequence, boundary map, cost-aware) |
| Blockchain compatibility notes | Done (Hedera, EVM, Kafka sections) |
| Semantic types used | Done (PayloadType, SemVer, UnixTimestamp, NodeRole, NATType, etc.) |
| No [NEEDS CLARIFICATION] markers | Done — none present |
| FR prefix convention followed | Done — FR-H01 through FR-H29 |
| SC prefix convention followed | Done — SC-H01 through SC-H12 |
| Given/When/Then acceptance scenarios | Done — all 6 user stories |
| Out of Scope section present | Done — 11 items |
| Clarifications section present | Done — 10 Q&A pairs |
| Purpose section present | Done — 2 paragraphs |

## Summary

- **29 Functional Requirements**: FR-H01 through FR-H29, all traceable to source documents
- **12 Success Criteria**: SC-H01 through SC-H12, all mapped to specific FRs
- **6 User Stories**: US1 through US6, all with Given/When/Then acceptance scenarios (25 total)
- **9 Edge Cases**: All covered in spec with resolution
- **5 Mermaid Diagrams**: ER, state machine, sequence, boundary map, cost-aware design
- **0 [NEEDS CLARIFICATION] markers**: Spec is complete
