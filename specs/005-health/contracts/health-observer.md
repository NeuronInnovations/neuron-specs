# API Contract: Health Observer

**Source**: spec.md FR-H15..H21, FR-H28

---

## ValidateInboundHeartbeat

Observer-side validation on receipt of a heartbeat TopicMessage. Maps to V-OBS-01 through V-OBS-06.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `topicMessage` | TopicMessage | Received message (Spec 004 FR-T02) |
| `consensusTimestamp` | UnixTimestamp | From `MessageDelivery.consensusTimestamp` (FR-T24) |

**Output**: `(valid: Boolean, error: String or absent)`

**Validation steps** (FR-H15):

| Step | Check | Error on failure |
|------|-------|-----------------|
| V-OBS-01 | Signature verifies against `senderAddress` (FR-T10) | `"signature verification failed"` / `"sender address mismatch"` |
| V-OBS-02 | `payload.type == "heartbeat"` | `"not a heartbeat message"` |
| V-OBS-03 | `payload.version` major == 1 (FR-H28) | `"incompatible heartbeat version"` |
| V-OBS-04 | If `payload.nextHeartbeatDeadline == 0`: return valid | (caller MUST transition to OFFLINE) |
| V-OBS-05 | `payload.nextHeartbeatDeadline > consensusTimestamp` | `"deadline is in the past relative to consensus time"` |
| V-OBS-06 | `MIN_DEADLINE_DELTA ≤ delta ≤ MAX_DEADLINE_DELTA` | `"deadline delta below minimum"` / `"deadline delta exceeds maximum"` |

**Clock authority** (FR-H16): ALL deadline arithmetic uses `consensusTimestamp`. NEVER use `topicMessage.timestamp` or observer's local clock.

---

## EvaluateLiveness

Continuous liveness evaluation for an observed peer. Pure function of time and last-known heartbeat.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `senderAddress` | EVMAddress | Peer being evaluated |
| `currentTime` | UnixTimestamp | Observer's current time (for evaluation tick) |

**Output**: `LivenessState` (UNKNOWN | ALIVE | SUSPECT | DEAD | OFFLINE)

**Algorithm** (FR-H20):

```
lastHB = GetLastValidHeartbeat(senderAddress)

IF lastHB is absent                         → UNKNOWN
IF lastHB.nextHeartbeatDeadline == 0        → OFFLINE
IF currentTime ≤ deadline + GRACE_PERIOD    → ALIVE
IF currentTime ≤ deadline + GRACE + S2D     → SUSPECT
ELSE                                        → DEAD
```

**Sequence rule** (FR-H17): `GetLastValidHeartbeat` returns the heartbeat with the highest valid `sequenceNumber` for this sender. Lower-sequence heartbeats are ignored.

---

## UpdateLivenessRecord

Updates the observer's per-peer tracking state when a new valid heartbeat arrives.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `record` | LivenessRecord | Existing record (or absent for first heartbeat) |
| `heartbeat` | HeartbeatPayload | Validated heartbeat payload |
| `consensusTimestamp` | UnixTimestamp | From MessageDelivery |
| `sequenceNumber` | SequenceNumber | From TopicMessage envelope |

**Output**: Updated `LivenessRecord`

**Behavior**:
1. If `record` is absent: create new record, set state to ALIVE (or OFFLINE if deadline == 0)
2. If `sequenceNumber <= record.lastSequence`: ignore (FR-H17)
3. If `heartbeat.nextHeartbeatDeadline == 0`: set state to OFFLINE
4. Else: set state to ALIVE, update `lastDeadline`, `lastSequence`, `lastConsensusTimestamp`

**Recovery invariant** (FR-H21): Any valid heartbeat transitions from ANY state to ALIVE (or OFFLINE). DEAD nodes can always recover.

---

## SubscribeToHeartbeats

Sets up observation of a peer's stdOut topic.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `stdOutTopicRef` | TopicRef | Peer's stdOut topic (from agentURI via Spec 003) |
| `adapter` | TopicAdapter | Spec 004 adapter instance |
| `fromSequence` | SequenceNumber | Resume from this sequence (FR-T25) |

**Output**: Message stream delivering `MessageDelivery` objects

**Behavior**:
1. Call `adapter.subscribe(stdOutTopicRef, {fromSequence})` (FR-T25)
2. For each `MessageDelivery`:
   a. If `payload.type != "heartbeat"`: skip (non-heartbeat messages on stdOut)
   b. Run `ValidateInboundHeartbeat(message, delivery.consensusTimestamp)`
   c. If valid: `UpdateLivenessRecord(record, payload, consensusTimestamp, sequenceNumber)`
3. On subscription gap: adapter backfills (FR-T25). Observer waits for backfill before evaluating.

*FR-T24 (MessageDelivery), FR-T25 (subscribe resumption) added to Spec 004 as part of the Publish/Subscribe Execution Binding subsection.*
