# NormalizedTrack Wire-Format Contract — Demo-Grade Decoded-Track Fast Path

> **Status**: Normative contract document, landed 2026-05-14 to satisfy Constitution Principle I (Specification-First) for the BaseStation demo slice.
> **Authored**: 2026-05-14 (post-audit; implementation slice Phase 0).
> **Authority**: BaseStation fast-fusion audit (2026-05-14, verified verdict + schema sketch); Constitution v1.7.0 Principle XII; `docs/dapp-frame-format-precedent.md` (R-FF-02).
> **Companion**: `docs/fid-display-contract.md` (TaggedFrame envelope); `specs/016-adsb-dapp/spec.md`; `specs/017-remote-id-dapp/spec.md`; `docs/dapp-frame-format-precedent.md` (R-FF-02 normalized-JSON rule).
> **Audience**: implementers of `internal/dapp/adsb/`, `cmd/adsb-seller/`, `cmd/multistream-buyer/`, and `cmd/fid-display/` dispatch branches; reviewers of the BaseStation slice.
> **Scope**: NormalizedTrack is `additive` / `demo-grade` / `decoded-track shortcut` / `lower-fidelity than raw paths` / `not sufficient by itself for raw evidence claims`. These five labels are load-bearing — they are quoted verbatim in §1 and every evidence report consuming this format. Raw paths (`/jetvision/raw/1.0.0` BEAST, `/ds240/raw/1.0.0` `RemoteIdFrame`) and the DroneScout MQTT JSON live-source path are NOT touched by this contract.

---

## 1. Five-label classification

NormalizedTrack is positioned exactly as the audit positioned BaseStation — the wire format the seller emits IS a decoded text shape, so the same five labels apply verbatim:

| Label | Why |
|-------|-----|
| `additive` | NormalizedTrack rides on new stream catalog entries (`/jetvision/basestation/1.0.0`, future `/ds240/basestation/1.0.0`). Existing `/jetvision/raw/1.0.0` BEAST and `/ds240/raw/1.0.0` `RemoteIdFrame` consumers see no change. |
| `demo-grade` | Designed for fused-buyer demo iteration speed and for partners already speaking SBS-1; not designed for byte-perfect rollback or Validator-grade evidence. |
| `decoded-track shortcut` | Upstream decoder (JetVision firmware on port 30003 or BlueMark `modules/tcp_sbs_export.py`) has already lifted radio signal into named fields. A bug in that decoder is unrecoverable from the NormalizedTrack wire — no parity bits, no CRC, no raw payload survives. |
| `lower-fidelity than raw paths` | BEAST 0x1A preserves bit-exact Mode-S bodies including parity; ASTM F3411-22a `RemoteIdFrame` preserves regulatory variant + operator-system message details. NormalizedTrack discards all of that. The fast path is a thinner pipe by construction. |
| `not sufficient by itself for raw evidence claims` | TTRL claims that rest on Mode-S CRC verification, bit-perfect rollback, or `RemoteIdFrame.regulatorVariant` traceability cannot be substantiated from NormalizedTrack alone. Validators (010) MUST consume the raw paths when producing verdicts; they MAY use NormalizedTrack as a secondary cross-check signal only. |

---

## 2. Position in the architecture

NormalizedTrack is a **payload schema**, not a wire envelope. It rides **inside** the existing Spec 018 `TaggedFrame.frame` field per FR-F-02:

```
       TaggedFrame  (envelope; owned by Spec 018)
       ┌──────────────────────────────────────────────┐
       │ source        : "adsb" | "remote-id"         │  ← producer DApp tag (Principle XII)
       │ type          : "normalized-track"           │  ← this contract's discriminator
       │ sellerPeerID  : <libp2p PeerID>              │
       │ receivedAt    : <RFC 3339 buyer-side ts>     │
       │ frame         : NormalizedTrack { … }        │  ← THIS contract describes the inner shape
       └──────────────────────────────────────────────┘
```

Spec 018 FR-F-02 leaves `frame` as DApp-owned canonical JSON; the `type` vocabulary is extensible per the producing DApp (FR-F-02 declares the v1 values `"aircraft"` and `"drone"` as examples, not exhaustive). NormalizedTrack uses `type = "normalized-track"` to discriminate from the legacy `type = "aircraft"` (which carries the edge-buyer's `AggregatedFrame` shape; BEAST bytes inside).

Spec 018 §Out of Scope explicitly rejects merged cross-DApp frames:

> Cross-DApp protocol-layer fusion — TaggedFrames are independently typed per source. No spec defines a "merged" frame that combines fields from multiple DApps; 018 explicitly rejects that direction as outside Constitution Principle XII (DApps own their frame schemas).

NormalizedTrack respects this. A NormalizedTrack with `TaggedFrame.source = "adsb"` carries an **aircraft** track (one ICAO24-keyed entity). A NormalizedTrack with `TaggedFrame.source = "remote-id"` carries a **drone** track (one DroneID-keyed entity, or one BlueMark fake-ICAO). There is never a single envelope carrying both an aircraft and a drone. Source identity is end-to-end; the producing DApp owns the contents of the NormalizedTrack payload it emits, and the display layer dispatches by `source` first, then reads `NormalizedTrack.entityType` for confirmation and rendering hints.

Frame-format precedent rule that applies: **R-FF-02 (normalized canonical JSON)** per `docs/dapp-frame-format-precedent.md:36–55`. The shape meets R-FF-02 criteria 1 (multi-vendor decoder ecosystems benefit from seller-side normalization), 4 (cross-DApp fusion is a primary use case), and 5 (upstream binary format coupling is undesirable).

---

## 3. Wire format

### 3.1 Field-by-field schema

Canonical JSON per Spec 006. Field order below is **normative**. Absent optional fields MUST be omitted (not `null`) per Spec 006 FR-W04.

| # | Field | Required | Type | Units | Notes |
|---|-------|----------|------|-------|-------|
| 1 | `type` | MUST | string | n/a | Discriminator. v1 value: `"normalized-track"`. |
| 2 | `version` | MUST | string | n/a | Semver. v1 value: `"1.0.0"`. Bumps when this contract amends. |
| 3 | `observedAt` | MUST | string | RFC 3339 §5.6 `date-time` (UTC, nanosecond precision when available) | Receiver-side / upstream-decoder timestamp. SBS-1 BaseStation lines carry split date+time fields; the seller MUST combine them and convert to RFC 3339 with the seller's process clock for sub-second precision when the upstream timestamp lacks it. |
| 4 | `source` | MUST | string | enum | Matches the wrapping `TaggedFrame.source` (Spec 018 FR-F-03) AND the producing DApp's `neuron-commerce.name` (016 FR-A01: `"adsb"`; 017 FR-R01: `"remote-id"`). |
| 5 | `entityType` | MUST | string | enum | Closed v1 enum: `"aircraft"` \| `"drone"`. New entity types (e.g., `"vessel"` for maritime AIS) introduced via minor-version bumps. |
| 6 | `entityID` | MUST | string | UPPERCASE | Identifier within `entityType`. For `aircraft`: ICAO24 hex, 6 uppercase characters (e.g., `"A1B2C3"`). For `drone`: BlueMark fake-ICAO when sourced via SBS export (`"FF"`-prefix per §6 below) OR raw DroneID serial when sourced via another path. The exact provenance shape is governed by the producing DApp; this contract pins the type (string) and the case (UPPERCASE). |
| 7 | `position` | OPTIONAL | object | metric | Decoded position. See §3.2. Omitted entirely when the upstream decoder has not produced a position fix for this entity yet. |
| 8 | `velocity` | OPTIONAL | object | metric | Decoded velocity. See §3.3. Omitted when not yet decoded. |
| 9 | `callsign` | OPTIONAL | string | UPPERCASE, ≤ 8 chars | Aircraft callsign (e.g., `"BAW178"`) from SBS-1 field [10]; OR BlueMark drone callsign (4+4 split of the drone serial number) when sourced via SBS export. Omitted when absent in the upstream. |
| 10 | `squawk` | OPTIONAL | string | 4-digit octal as decimal string | Aircraft transponder code (e.g., `"7000"`) from SBS-1 field [17]. Omitted for `entityType = "drone"`. |
| 11 | `quality` | OPTIONAL | object | mixed | Sensor-confidence metadata. See §3.4. Omitted entirely when no quality data is available. |

### 3.2 `position` sub-object

Order is normative. All fields metric SI.

| Field | Required | Type | Units | Notes |
|-------|----------|------|-------|-------|
| `lat` | MUST (within `position`) | number | degrees, signed | Latitude. Range [-90, 90]. |
| `lon` | MUST (within `position`) | number | degrees, signed | Longitude. Range [-180, 180]. |
| `altitudeM` | OPTIONAL | number | metres | Converted from SBS-1 field [11] (feet) via `× 0.3048`. Omitted when upstream altitude is missing or invalid. |

### 3.3 `velocity` sub-object

Order is normative. All fields metric SI.

| Field | Required | Type | Units | Notes |
|-------|----------|------|-------|-------|
| `groundSpeedMps` | OPTIONAL | number | m/s | Converted from SBS-1 field [12] (knots) via `× 0.514444`. Omitted when upstream ground speed is missing. |
| `headingDeg` | OPTIONAL | number | degrees | True-track in degrees, range [0, 360). From SBS-1 field [13]. Omitted when missing. |
| `verticalRateMps` | OPTIONAL | number | m/s | Converted from SBS-1 field [16] (fpm) via `× 0.00508`. Sign convention: positive = climbing, negative = descending. Omitted when missing. |

> **Producer note (vanilla SBS-1):** ground speed and track arrive in MSG **type-4** records while
> position arrives in **type-3** records — the two are disjoint per ICAO. The producer's source
> adapter merges them by ICAO before emission (per §10 — the producer owns its source adapter), so a
> single NormalizedTrack carries the latest-known position and velocity together. A velocity component
> that has never been observed for an entity is **omitted (not zero)**; consumers MUST render it as
> unknown rather than `0` (per the 2026-05-25 speed/heading-zero diagnosis).

### 3.4 `quality` sub-object

Order is normative.

| Field | Required | Type | Units | Notes |
|-------|----------|------|-------|-------|
| `receivers` | OPTIONAL | integer ≥ 0 | count | Number of contributing MLAT sensors (SBS-1 field [18], mlat-server variant only). Indicates triangulation confidence. |
| `horizErrM` | OPTIONAL | number ≥ 0 | metres | Horizontal positional uncertainty. From SBS-1 trailing `herr` column (field [22]) when present; mlat-server may also encode this in the `emerg` column (field [19]) as integer metres. |
| `fakePosition` | OPTIONAL | boolean | n/a | Mirrors Spec 018 FR-F-05 `frame.meta.positionFake` convention. `true` ⇒ the position fields above are deterministic stand-ins for a not-yet-implemented decoder, not real fixes. Default `false` when omitted. |

---

## 4. Worked example — ADS-B aircraft from JV port 30003

Source line (SBS-1, MSG type 3 position):

```
MSG,3,1,1,A1B2C3,1,2026/05/14,14:49:00.123,2026/05/14,14:49:00.123,BAW178,37000,,,51.4700,-0.4543,,,7000,0,0,0,0
```

Resulting NormalizedTrack (canonical JSON, field order normative):

```json
{
  "type": "normalized-track",
  "version": "1.0.0",
  "observedAt": "2026-05-14T14:49:00.123Z",
  "source": "adsb",
  "entityType": "aircraft",
  "entityID": "A1B2C3",
  "position": {
    "lat": 51.47,
    "lon": -0.4543,
    "altitudeM": 11277.6
  },
  "callsign": "BAW178",
  "squawk": "7000"
}
```

Wrapped in a TaggedFrame on the wire (`/jetvision/basestation/1.0.0`):

```json
{"source":"adsb","type":"normalized-track","sellerPeerID":"12D3KooW...","receivedAt":"2026-05-14T14:49:00.567Z","frame":{"type":"normalized-track","version":"1.0.0","observedAt":"2026-05-14T14:49:00.123Z","source":"adsb","entityType":"aircraft","entityID":"A1B2C3","position":{"lat":51.47,"lon":-0.4543,"altitudeM":11277.6},"callsign":"BAW178","squawk":"7000"}}
```

Note the doubled `source` and `type` — once on the outer TaggedFrame envelope (Spec 018 FR-F-02 routing) and once on the inner NormalizedTrack (this contract's self-description for downstream consumers that receive a NormalizedTrack without its TaggedFrame wrapper). The redundancy is deliberate; consistency between the two layers is REQUIRED per §5 below.

---

## 5. Worked example — Remote ID drone via BlueMark SBS export (illustrative)

The Remote ID basestation path is illustrated for symmetry so implementers see the cross-DApp shape consistency; the shipping Remote ID path is `/ds240/raw/1.0.0`.

Source line (SBS-1 from BlueMark `tcp_sbs_export.py`, drone with `FF`-prefix fake-ICAO):

```
MSG,3,1,1,FFA1B2,1,2026/05/14,14:49:00.456,2026/05/14,14:49:00.456,B12345CD,200,,,51.4682,-0.4517,,,,,,
```

Per the BlueMark README: ICAO `FFA1B2` = `FF` namespace + 4-hex hash of the drone's serial number; callsign `B12345CD` = first-4 + last-4 digits of the serial number. The `FF` ICAO prefix is BlueMark's convention to keep drones renderable in third-party SBS consumers (Virtual Radar Server, FlightAirMap) that lack source-tag awareness.

Resulting NormalizedTrack:

```json
{
  "type": "normalized-track",
  "version": "1.0.0",
  "observedAt": "2026-05-14T14:49:00.456Z",
  "source": "remote-id",
  "entityType": "drone",
  "entityID": "FFA1B2",
  "position": {
    "lat": 51.4682,
    "lon": -0.4517,
    "altitudeM": 61.0
  },
  "callsign": "B12345CD"
}
```

The outer TaggedFrame carries `source = "remote-id"`, `type = "normalized-track"`. End-to-end source identity is preserved; the `FF` ICAO prefix is incidental information for defence-in-depth, NOT the load-bearing routing signal.

---

## 6. Consistency rules

The following rules are NORMATIVE for any producer or consumer implementation:

- **R1 — Self-consistent source/type**: A NormalizedTrack's `source` field MUST equal its wrapping `TaggedFrame.source`. A NormalizedTrack's `type` field MUST equal its wrapping `TaggedFrame.type` (i.e., always `"normalized-track"` in v1). Mismatched envelopes MUST be rejected with a descriptive error and SHOULD be logged as evidence-grade discipline failures (not silently coerced).
- **R2 — entityType ↔ source pairing**: `source = "adsb"` MUST pair with `entityType = "aircraft"`. `source = "remote-id"` MUST pair with `entityType = "drone"`. A NormalizedTrack with `source = "adsb"` and `entityType = "drone"` is a malformed frame; the consumer rejects per R1.
- **R3 — entityID format**: `entityID` is always UPPERCASE. For `entityType = "aircraft"`, MUST be 6-character hex (the ICAO24 of the aircraft transponder). For `entityType = "drone"`, format is producer-DApp-defined; the v1 BlueMark convention is `FF`-prefix + 4-hex hash, length 6 — equivalent in shape to an ICAO24 but namespace-segregated by the `FF` prefix.
- **R4 — Canonical JSON field order**: Top-level keys in the order listed in §3.1 (1 through 11). Nested objects in the order listed in §3.2 / §3.3 / §3.4. Optional fields absent → key omitted entirely (NOT `null`).
- **R5 — Position precision**: `lat` and `lon` MUST preserve at least 5 decimal places when present in the upstream (~1.1m resolution at the equator). Conversions feet→metres / knots→m/s / fpm→m/s use the constants in §3.2/§3.3; rounding policy: round-half-to-even at the limit of float64 representation (Go default).
- **R6 — Placeholder marker propagation**: When the producer DApp's heartbeat advertises `feedSource = "placeholder"` per 016 FR-A18 / 017 FR-R15, every NormalizedTrack emitted from that source MUST set `quality.fakePosition = true`. The two signals are complementary; the heartbeat is the auditable trail, the per-frame field is the fast-path indicator.
- **R7 — Sensor unit conversion is the SELLER's responsibility**: the wire format is metric SI. The producer DApp converts at emission time; consumers receive metric SI and do not perform unit conversion.

---

## 7. Relationship to Spec 018 TaggedFrame

NormalizedTrack does NOT amend Spec 018. The envelope (`TaggedFrame`) is untouched. NormalizedTrack uses the existing extensibility:

- Spec 018 FR-F-02 declares `type` vocabulary is owned by the producing DApp. Adding `"normalized-track"` is a producer-side decision; no 018 amendment needed.
- Spec 018 FR-F-02 declares `frame` is DApp-owned canonical JSON. NormalizedTrack is a new DApp-owned canonical JSON shape; no 018 amendment needed.
- Spec 018 §Out of Scope rejection of "merged cross-DApp frames" is preserved: each NormalizedTrack carries one entity, one DApp source, one type.

The fid-display dispatcher gains a third arm (`source == "adsb" && type == "normalized-track"` → render aircraft with real lat/lon) without touching the existing `source == "adsb" && type == "aircraft"` arm (which continues to render BEAST-derived aircraft with placeholder positions via `placeholderPositionFromICAO`).

---


## 8. Relationship to Spec 016 ADS-B and Spec 017 Remote ID

This contract does NOT amend either spec in v1. Implementers add the new stream catalog entry (`/jetvision/basestation/1.0.0`) inside their DApp's `streams[]` advertisement per existing 016 FR-A03 / 017 FR-R03 extension patterns ("MAY additionally include ..."). The schema URL field of the stream catalog entry SHOULD point at this contract document:

```jsonc
{
  "name": "basestation",
  "protocolID": "/jetvision/basestation/1.0.0",
  "direction": "seller-initiates",
  "schema": "https://specs.neuron.network/contracts/normalized-track.md"  // points at this file
}
```

Heartbeat `feedSource` taxonomy: per implementation-slice decision 2026-05-14, BaseStation sources use `feedSource = "live"` (existing 016 FR-A18 / 017 FR-R15 vocabulary; no spec amendment) with `feedSourceConfig.upstream = "basestation:<host:port>"` (e.g., `"basestation:30003"`) describing the upstream decoder lineage.

Producer-side per-DApp operational disclosure follows the FR-R21 shape (sellerEVM, sellerPeerID, serviceName, topicBackend, escrowBackend, agentURISha256, optional degraded). The implementation slice ships the FR-R21-shape sub-object for both DApps.

---

## 9. Reference — where in the code

| Concern | File |
|---------|------|
| NormalizedTrack Go struct | `impl/golang/internal/dapp/adsb/frame.go` (Phase 2) |
| `FromSBSTrack` converter | `impl/golang/internal/dapp/adsb/frame.go` (Phase 2) |
| Canonical-JSON `MarshalJSON` | `impl/golang/internal/dapp/adsb/frame.go` (Phase 2) |
| Stream catalog entry | `impl/golang/internal/dapp/adsb/catalog.go` (Phase 2) |
| Seller-side emission pipeline | `impl/golang/internal/dapp/adsb/seller.go` (Phase 2) |
| Buyer-side TaggedFrame emission | `impl/golang/cmd/multistream-buyer/main.go` |
| fid-display dispatch branch | `impl/golang/cmd/fid-display/main.go` `applyNormalizedTrack` (Phase 5) |
| Conformance tests | `impl/golang/internal/dapp/adsb/conformance_test.go` `Test_FR_A05_BasestationStreamCanonicalShape` (Phase 2) |

---


## 10. Anti-scope of THIS contract

NormalizedTrack is NOT:

- A replacement for `/jetvision/raw/1.0.0` BEAST opaque frames.
- A replacement for `/ds240/raw/1.0.0` canonical-JSON `RemoteIdFrame`.
- A merged cross-DApp frame.
- An evidence-grade source for Validator (010) verdicts (Validators consume the raw paths).
- A formal Spec Kit specification — this is a contract document.

NormalizedTrack does NOT prescribe:

- HOW the producer DApp obtains its decoded tracks (the producing DApp owns its source adapter — SBS-1 TCP, replay, synthetic, etc.).
- HOW the consumer renders tracks (the display layer is implementation-defined; the contract pins only the wire shape).
- WHICH evidence reports MAY cite NormalizedTrack as a primary signal (none — it is always secondary at most).

End of contract.
