# API Contract: Health Publisher

**Source**: spec.md FR-H01..H14, FR-H22..H25, FR-H29

---

## BuildHeartbeatPayload

Constructs a HeartbeatPayload JSON object from input parameters.

**Input**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `nextHeartbeatDeadline` | UnixTimestamp | MUST | Future Unix timestamp (nanoseconds, per 006 FR-W02a). `0` for shutdown. |
| `role` | NodeRole | MUST | `"buyer"`, `"seller"`, `"relay"`. `"validator"` is reserved for future use |
| `capabilities` | Capabilities | SHOULD | Node capabilities object |
| `location` | Location | MAY | Geographic position |
| `peers` | AbbreviatedAddress[] | MAY | Connected peer list (last 4 hex chars) |

**Output**: HeartbeatPayload (JSON object)

**Behavior**:
- Sets `type` = `"heartbeat"` and `version` = `"1.0.0"` automatically
- Serializes fields in canonical order (FR-H04)
- Does NOT validate — call `ValidateOutboundHeartbeat` separately

---

## ValidateOutboundHeartbeat

Publisher-side validation before signing. Maps to V-PUB-01 through V-PUB-07.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `payload` | HeartbeatPayload | The payload to validate |
| `senderClock` | UnixTimestamp | Publisher's current wall clock |

**Output**: `(valid: Boolean, error: String or absent)`

**Validation steps** (FR-H13):

| Step | Check | Error on failure |
|------|-------|-----------------|
| V-PUB-01 | `payload.type == "heartbeat"` | `"invalid payload type"` |
| V-PUB-02 | `payload.version` is recognized | `"unsupported heartbeat version"` |
| V-PUB-03 | If `payload.nextHeartbeatDeadline == 0`: return valid | (bypass — shutdown) |
| V-PUB-04 | `payload.nextHeartbeatDeadline > senderClock` | `"deadline must be in the future"` |
| V-PUB-05 | `delta >= MIN_DEADLINE_DELTA (10)` | `"deadline too soon: minimum delta is 10 seconds"` |
| V-PUB-06 | `delta <= MAX_DEADLINE_DELTA (86400)` | `"deadline too far: maximum delta is 86400 seconds"` |
| V-PUB-07 | `payload.role` in allowed set | `"unrecognized role"` |

---

## PublishHeartbeat

Orchestrates payload construction, validation, and publication to stdOut.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `payload` | HeartbeatPayload | Validated payload |
| `stdOutTopicRef` | TopicRef | Publisher's stdOut topic (from Spec 003 agentURI) |
| `adapter` | TopicAdapter | Spec 004 adapter instance |

**Output**: `PublishResult` (from Spec 004 FR-T22)

**Behavior** (FR-H22..H25):
1. Check payload size against `adapter.maxMessageSize()` (FR-T27)
   - If too large: trim `peers`, then `capabilities`, then `location` (FR-H29)
2. Wrap payload into TopicMessage envelope (FR-T02) — adapter handles signing (FR-T03)
3. Publish via `adapter.publish(stdOutTopicRef, topicMessage, {mode: FIRE_AND_FORGET})` (FR-H23)
4. On success: schedule next heartbeat based on PublishResult
5. On failure: log error, skip — do NOT retry this heartbeat (FR-H25)

**Confirmation mode**: `FIRE_AND_FORGET` recommended (FR-H23)

---

## ScheduleNextHeartbeat

Computes the next publication time after a successful publish.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `publishResult` | PublishResult | Result from publish call |
| `chosenDelta` | UnsignedInt64 (seconds) | Publisher's desired cadence (10..86400) |
| `submitWallClock` | UnixTimestamp | Wall clock at time of submit |

**Output**: `nextPublishTime: UnixTimestamp`

**Behavior** (FR-H24):
1. If `publishResult.confirmed == true`: use `publishResult.consensusTimestamp` as reference
2. If `publishResult.confirmed == false`: use `submitWallClock` as reference
3. Return `referenceTime + chosenDelta`

*FR-T22 (PublishResult) added to Spec 004 as part of the Publish/Subscribe Execution Binding subsection.*
