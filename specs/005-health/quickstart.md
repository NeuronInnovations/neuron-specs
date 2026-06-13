# Quickstart: Health (Onchain Liveness & Health Status)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `005-health` | **Date**: 2026-02-25 | **Phase**: 1

---

## Overview

The Health module adds onchain liveness signaling to the Neuron SDK. Agents publish signed **HeartbeatPayload** messages to their stdOut topic (Spec 004) with a self-declared `nextHeartbeatDeadline`. Observers subscribe to peers' stdOut topics, validate incoming heartbeats, and evaluate liveness using a five-state machine (UNKNOWN / ALIVE / SUSPECT / DEAD / OFFLINE).

This module is a **pure protocol library** — no persistence layer, no HTTP endpoints, no UI. It integrates directly with existing Specs 001–004 modules.

---

## Developer Prerequisites

Before you can use this module, ensure the following are in place:

| # | Prerequisite | Why | How to obtain |
|---|---|---|---|
| 1 | **A NeuronPrivateKey** | Signing HeartbeatPayload messages (via TopicMessage envelope) | Generate via `keylib.NewNeuronPrivateKey()` (Spec 002) |
| 2 | **A funded Hedera account** | Publishing heartbeats to stdOut costs HBAR per HCS message | Create at [portal.hedera.com](https://portal.hedera.com) (testnet) or fund via exchange (mainnet) |
| 3 | **A registered agent with stdOut topic** | Heartbeats are published to the agent's stdOut channel | Complete registration via Spec 003; stdOut topic created via Spec 004 |
| 4 | **Peer's agentURI** (observer role only) | Discovering which stdOut topic to subscribe to | Resolve via `registry.LookupRegistration()` (Spec 003) |

> **Publisher vs Observer:** Publishers need items 1-3. Observers need items 1 and 4. An agent that both publishes its own heartbeats and observes peers needs all four.

### SDK Dependencies

| Dependency | Module | What You Need |
|------------|--------|---------------|
| Spec 001 | Account | `EVMAddress` for identity |
| Spec 002 | Key Library | `NeuronPrivateKey` for signing heartbeats |
| Spec 003 | Peer Registry | `agentURI` to discover peer stdOut topics |
| Spec 004 | Topic System | `TopicAdapter`, `TopicMessage`, `TopicRef` for publish/subscribe |

---

## Integration Guide

### 1. Publish a Heartbeat (Publisher Role)

```text
// Step 1: Build the heartbeat payload
payload = BuildHeartbeatPayload(
    nextHeartbeatDeadline: now() + 60,    // 60-second cadence
    role: "seller",
    capabilities: { natReachability: true, protocols: ["/adsb/v1"] },
    location: { lat: 37.7749, lon: -122.4194 }
)

// Step 2: Validate before signing
(valid, error) = ValidateOutboundHeartbeat(payload, now())
if !valid: handle(error)

// Step 3: Publish to stdOut
result = PublishHeartbeat(payload, myStdOutTopicRef, adapter)

// Step 4: Schedule next heartbeat
nextTime = ScheduleNextHeartbeat(result, chosenDelta: 60, now())
scheduleAt(nextTime, publishHeartbeat)
```

**Key constraints**:
- `nextHeartbeatDeadline` must be 10–86400 seconds in the future (FR-H06, FR-H07)
- Set `nextHeartbeatDeadline = 0` for graceful shutdown (FR-H12)
- Mandatory fields serialize to < 256 bytes JSON (FR-H29)
- Use `FIRE_AND_FORGET` confirmation mode (FR-H23)
- Do NOT retry on publish failure — skip and wait for next cycle (FR-H25)

### 2. Observe Peer Liveness (Observer Role)

```text
// Step 1: Subscribe to peer's stdOut topic
stream = SubscribeToHeartbeats(peerStdOutTopicRef, adapter, fromSequence: 0)

// Step 2: Process incoming heartbeats
for delivery in stream:
    (valid, error) = ValidateInboundHeartbeat(delivery.message, delivery.consensusTimestamp)
    if valid:
        record = UpdateLivenessRecord(record, delivery.payload, delivery.consensusTimestamp, delivery.sequenceNumber)

// Step 3: Evaluate liveness at any time
state = EvaluateLiveness(peerAddress, now())
// Returns: UNKNOWN | ALIVE | SUSPECT | DEAD | OFFLINE
```

**Key constraints**:
- ALL deadline arithmetic uses `consensusTimestamp` from the ledger, never local clock (FR-H16)
- Only the highest-sequence heartbeat counts per sender (FR-H17)
- Liveness evaluation is a pure function of (time, lastDeadline, constants) (FR-H20)

### 3. Liveness State Machine

```text
States: UNKNOWN → ALIVE → SUSPECT → DEAD
                                ↕
                             OFFLINE

Transitions:
  UNKNOWN  + first valid HB           → ALIVE
  ALIVE    + new valid HB             → ALIVE (deadline reset)
  ALIVE    + now > deadline + 30s     → SUSPECT
  ALIVE    + deadline = 0             → OFFLINE
  SUSPECT  + new valid HB             → ALIVE
  SUSPECT  + now > deadline + 150s    → DEAD
  DEAD     + new valid HB             → ALIVE  (recovery always possible)
  OFFLINE  + new valid HB (deadline>0)→ ALIVE
```

Constants: `GRACE_PERIOD = 30s`, `SUSPECT_TO_DEAD = 120s`

---

## Module Structure

```text
health/
├── payload.{ext}        # HeartbeatPayload builder + JSON serialization
├── publisher.{ext}      # ValidateOutboundHeartbeat (V-PUB-01..07) + PublishHeartbeat
├── observer.{ext}       # ValidateInboundHeartbeat (V-OBS-01..06) + EvaluateLiveness
├── liveness.{ext}       # LivenessState enum + LivenessRecord + state transitions
├── constants.{ext}      # MIN_DEADLINE_DELTA, MAX_DEADLINE_DELTA, GRACE_PERIOD, SUSPECT_TO_DEAD
└── types.{ext}          # NodeRole, NATType, GPSFixQuality, Capabilities, Location
```

---

## API Reference

| Function | Role | Contract |
|----------|------|----------|
| `BuildHeartbeatPayload` | Publisher | [health-publisher.md](contracts/health-publisher.md) |
| `ValidateOutboundHeartbeat` | Publisher | [health-publisher.md](contracts/health-publisher.md) |
| `PublishHeartbeat` | Publisher | [health-publisher.md](contracts/health-publisher.md) |
| `ScheduleNextHeartbeat` | Publisher | [health-publisher.md](contracts/health-publisher.md) |
| `ValidateInboundHeartbeat` | Observer | [health-observer.md](contracts/health-observer.md) |
| `EvaluateLiveness` | Observer | [health-observer.md](contracts/health-observer.md) |
| `UpdateLivenessRecord` | Observer | [health-observer.md](contracts/health-observer.md) |
| `SubscribeToHeartbeats` | Observer | [health-observer.md](contracts/health-observer.md) |

---

## Cross-Spec Dependencies

```text
Spec 002 (Key Library)
  └─ NeuronPrivateKey.sign() → used by TopicAdapter to sign HeartbeatPayload

Spec 001 (Account)
  └─ EVMAddress → senderAddress in TopicMessage envelope

Spec 004 (Topic System)
  ├─ TopicAdapter.publish() → publishes HeartbeatPayload to stdOut
  ├─ TopicAdapter.subscribe() → receives heartbeats from peer stdOut
  ├─ TopicMessage → envelope carrying HeartbeatPayload
  └─ PublishResult, MessageDelivery → FR-T22..T29 (added to Spec 004)

Spec 003 (Peer Registry)
  └─ agentURI → resolves peer's stdOut TopicRef for observation
```

**Resolved dependency**: FR-T22..T29 have been added to Spec 004's spec.md. See [research.md](research.md) R1.
