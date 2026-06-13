# Implementation Plan: Health (TypeScript)

**Date**: 2026-03-18 | **Spec**: [spec.md](spec.md)
**Language**: TypeScript | **Derivation Source**: Language-neutral artifacts ONLY

## Summary

Spec 005 defines a self-declared deadline liveness model. HeartbeatPayload is the JSON payload (type, version, nextHeartbeatDeadline, role, optional capabilities/location/peers) published to stdOut as a TopicMessage. Publisher validates before signing (V-PUB-01..07). Observer validates on receipt (V-OBS-01..06) and evaluates a 5-state liveness machine (UNKNOWN/ALIVE/SUSPECT/DEAD/OFFLINE).

**Conformance Gate**: Chain 3 (HeartbeatPayload signing as TopicMessage payload).

## Technical Context

**Dependencies**: keylib (signing), topic (TopicMessage, adapters)
**Constants**: MIN_DEADLINE_DELTA=10s, MAX_DEADLINE_DELTA=86400s, GRACE_PERIOD=30s, SUSPECT_TO_DEAD=120s, SHUTDOWN_SENTINEL=0
**Error codes**: NEURON-HEALTH-001..007

## Constitution Check

All 11 gates PASS.

## Source Structure

```text
impl/typescript/src/health/
├── index.ts, constants.ts, types.ts, errors.ts
├── payload.ts     # HeartbeatPayload build + canonical JSON
├── publisher.ts   # ValidateOutbound (V-PUB), PublishHeartbeat, ScheduleNext
├── observer.ts    # ValidateInbound (V-OBS), UpdateLivenessRecord
└── liveness.ts    # EvaluateLiveness (pure), LivenessState machine
```

## Phases

### Phase 1: Foundational — Constants + Types + Errors
### Phase 2: HeartbeatPayload + Chain 3 conformance 🎯
### Phase 3: Publisher (V-PUB-01..07) + Observer (V-OBS-01..06)
### Phase 4: Liveness state machine (5 states, 8 transitions)
