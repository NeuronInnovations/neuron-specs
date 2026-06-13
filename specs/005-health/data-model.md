# Data Model: Health (Onchain Liveness & Health Status)

**Branch**: `005-health` | **Date**: 2026-02-25 | **Source**: spec.md FR-H01..H05, Key Entities

---

## Entities

### HeartbeatPayload

The primary data structure published to stdOut as the TopicMessage payload.

| Field | Type | Required | Validation | Source FR |
|-------|------|----------|------------|----------|
| `type` | PayloadType | MUST | Exact match: `"heartbeat"` | FR-H02 |
| `version` | SemVer | MUST | Must be recognized (1.x.y accepted, 2.x.y rejected) | FR-H02, FR-H28 |
| `nextHeartbeatDeadline` | UnixTimestamp (UnsignedInt64) | MUST | `0` = shutdown sentinel; else `consensusTs + MIN_DELTA ≤ deadline ≤ consensusTs + MAX_DELTA` | FR-H02, FR-H10, FR-H11 |
| `role` | NodeRole | MUST | One of: `"buyer"`, `"seller"`, `"relay"`. `"validator"` is reserved for future use | FR-H02, FR-H05 |
| `capabilities` | Capabilities | SHOULD | If present, object with sub-fields below | FR-H03 |
| `location` | Location | MAY | If present, `lat` and `lon` MUST be present | FR-H03 |
| `peers` | AbbreviatedAddress[] | MAY | Array of 4-char hex strings. Informational only | FR-H03, FR-H27 |

**Serialization order** (FR-H04): `type` → `version` → `nextHeartbeatDeadline` → `role` → `capabilities` → `location` → `peers`

**Size budget** (FR-H29): Mandatory fields < 256 bytes. Trim order on overflow: `peers` → `capabilities` → `location`.

### Capabilities

| Field | Type | Required | Validation | Source FR |
|-------|------|----------|------------|----------|
| `natReachability` | Boolean | SHOULD | true or false | FR-H03 |
| `natType` | NATType | MAY | One of: `"no-nat"`, `"endpoint-independent"`, `"address-dependent"`, `"address-and-port-dependent"` | FR-H03 |
| `protocols` | ProtocolID[] | SHOULD | Array of protocol ID strings (e.g., `"/adsb/v1"`) | FR-H03 |

**Serialization order**: `natReachability` → `natType` → `protocols`

### Location

| Field | Type | Required | Validation | Source FR |
|-------|------|----------|------------|----------|
| `lat` | Float64 | Conditional | WGS84 latitude. MUST be present if `location` is present | FR-H03 |
| `lon` | Float64 | Conditional | WGS84 longitude. MUST be present if `location` is present | FR-H03 |
| `alt` | Float64 | MAY | Meters above WGS84 ellipsoid | FR-H03 |
| `fix` | GPSFixQuality | MAY | One of: `"none"`, `"2D"`, `"3D"` | FR-H03 |

**Serialization order**: `lat` → `lon` → `alt` → `fix`

### LivenessState (Enum)

| Value | Meaning | Entry Condition | Source FR |
|-------|---------|-----------------|----------|
| `UNKNOWN` | No heartbeat ever observed for this peer | Initial state (default) | FR-H18 |
| `ALIVE` | Peer is within its declared deadline + grace period | Valid heartbeat received; `now ≤ deadline + GRACE_PERIOD` | FR-H19, FR-H20 |
| `SUSPECT` | Peer missed its deadline but within suspect window | `deadline + GRACE_PERIOD < now ≤ deadline + GRACE_PERIOD + SUSPECT_TO_DEAD` | FR-H19, FR-H20 |
| `DEAD` | Peer has exceeded all tolerance windows | `now > deadline + GRACE_PERIOD + SUSPECT_TO_DEAD` | FR-H19, FR-H20 |
| `OFFLINE` | Peer published a graceful shutdown sentinel | `nextHeartbeatDeadline = 0` in last valid heartbeat | FR-H12, FR-H19 |

### LivenessRecord

Observer-local tracking state per observed peer. Not persisted on-chain.

| Field | Type | Description | Source FR |
|-------|------|-------------|----------|
| `senderAddress` | EVMAddress | Peer being observed | FR-H18 |
| `currentState` | LivenessState | Current liveness assessment | FR-H18 |
| `lastDeadline` | UnixTimestamp | `nextHeartbeatDeadline` from highest-seq heartbeat | FR-H17 |
| `lastSequence` | SequenceNumber | Highest valid sequence number seen | FR-H17 |
| `lastConsensusTimestamp` | UnixTimestamp | Consensus timestamp of highest-seq heartbeat | FR-H16 |

### ProtocolConstants

| Constant | Type | Value | Source FR |
|----------|------|-------|----------|
| `MIN_DEADLINE_DELTA` | UnsignedInt64 (seconds) | 10 | FR-H06 |
| `MAX_DEADLINE_DELTA` | UnsignedInt64 (seconds) | 86400 | FR-H07 |
| `GRACE_PERIOD` | UnsignedInt64 (seconds) | 30 | FR-H08 |
| `SUSPECT_TO_DEAD` | UnsignedInt64 (seconds) | 120 | FR-H09 |

> **Unit note**: Protocol constants are defined in seconds for readability. All UnixTimestamp fields (`nextHeartbeatDeadline`, `lastDeadline`, `lastConsensusTimestamp`) are in nanoseconds per 006 FR-W02a. Implementations MUST multiply constants by 10^9 when comparing against nanosecond timestamps (e.g., `delta_seconds = (deadline - consensusTs) / 1_000_000_000`).
| `SHUTDOWN_SENTINEL` | UnsignedInt64 | 0 | FR-H12 |

---

## State Transitions

### LivenessState Machine

```
UNKNOWN ──[first valid HB]──► ALIVE
ALIVE ──[new valid HB]──► ALIVE (reset deadline)
ALIVE ──[now > deadline + GP]──► SUSPECT
ALIVE ──[deadline = 0]──► OFFLINE
SUSPECT ──[new valid HB]──► ALIVE
SUSPECT ──[now > deadline + GP + S2D]──► DEAD
DEAD ──[new valid HB]──► ALIVE
OFFLINE ──[new valid HB with deadline > 0]──► ALIVE
```

**Key invariant** (FR-H21): Recovery is always possible from any state.

---

## Relationships

```
HeartbeatPayload ──[carried in]──► TopicMessage.payload (Spec 004 FR-T02)
HeartbeatPayload ──[signed via]──► Signature (Spec 002 FR-014)
HeartbeatPayload ──[published to]──► stdOut topic (Spec 004 FR-T07)
HeartbeatPayload ──[discovered via]──► agentURI (Spec 003 FR-R08)
TopicMessage.senderAddress ──[derived from]──► EVMAddress (Spec 001 FR-008)
LivenessRecord ──[tracks]──► HeartbeatPayload (highest-sequence per sender)
LivenessRecord ──[uses clock from]──► MessageDelivery.consensusTimestamp (Spec 004 FR-T24)

* FR-T24 pending addition to Spec 004
```
