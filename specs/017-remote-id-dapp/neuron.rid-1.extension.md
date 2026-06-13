# Neuron Remote-ID SAPIENT Extension (`rid/1`)

**Status**: Normative Annex A of spec 017 · **Schema id**: `neuron.rid/1` · **Version**: 1.1.0 · **Applies to**: DroneScout SAPIENT DetectionReports

> **Annex A of spec 017 (promoted normative).** This document is the source of
> truth for the `rid.*` **value domains (§3)**, the **accuracy enum→bound tables
> (§5)**, and the **verification lifecycle (§4.1)**. It now describes the
> **current single-report model**; on any other point `017 spec.md` governs (see
> its *Mapping — reconciliations* table). The earlier two-object prototype —
> the second "UAS Operator" object, the `OP-`/`OP-OP-` object-id scheme, the
> serial-as-`object_id`, and the top-level `authentication` block — is
> **superseded** and no longer described here.

This document defines the **non-standard** parts of the SAPIENT DetectionReports
emitted by a DroneScout ASM — i.e. everything that is *not* a plain native
SAPIENT DetectionReport field. It is the contract an HLDMM must understand to
consume these reports, and it is what the ASM declares (by schema id, URI, and
content hash) in its SAPIENT **Registration** `extensions[]` (015 FR-S40).

> **Encoding caveat.** The reference implementation models SAPIENT as an
> ICD-shaped JSON object (camelCase); the canonical wire form is Protocol
> Buffers (BSI Flex 335 v2.0), snake_case. Field *names* here are the JSON
> model's; reconcile them against the compiled `.proto` before wire use. The
> *semantics* are stable.

---

## 1. Conventions

- **Namespace.** Every extension `objectInfo` `type` is prefixed `rid.`. An
  HLDMM that does not recognise the prefix can safely ignore the entry.
- **Values are strings.** Per SAPIENT `objectInfo`, every `value` is a string;
  numbers are decimal text, booleans are `"true"`/`"false"`.
- **Units are static-per-key, not carried on the wire.** The v2.0
  `TrackObjectInfo` is `{type, value, error}` — it has **no `units` field**. The
  "Units" columns below (and the schema's optional `units`) document each key's
  fixed unit (`m`, `m/s`, `s`, `dBm`); that unit is implied by the key and
  surfaced only in the JSON inspection view, never as a per-entry wire field
  (spec 017 §"Encoding on the wire").
- **Absent ≠ zero, except where noted.** Fields that are part of a present ODID
  message are emitted even when `0` (a real `0` is faithful — no-suppression).
  Accuracy fields are the exception: their ODID `0` means *unknown*, so they are
  **omitted** rather than emitted as a false `0` bound.
- **One object per contact.** One Remote ID broadcast yields **one** craft
  `DetectionReport`. Operator data rides on it (§3.7); it is not a second object.

---

## 2. Native SAPIENT fields used (for reference, not part of this extension)

`reportID`, the envelope `timestamp` and `node_id`, `object_id` (**ULID**), `id` (= RID serial),
`location` (+ `error`), `detection_confidence`, `velocity_oneof` (**ENUVelocity**),
`classification`, `signal` (`amplitude` + `centre_frequency`), and the
`object_info` container itself. (`location.altDatum` — `geometric`|`barometric` —
is a Neuron clarifier on the native location, since standard SAPIENT location
does not tag the altitude datum.)

---

## 3. `rid.*` ObjectInfo namespace

### 3.1 Identity

| Key | Value domain | Units | ODID source | Cardinality |
|---|---|---|---|---|
| `rid.idType` | `None`,`SerialNumber`,`CAARegistration`,`UTMAssignedUUID`,`SpecificSessionID` | — | `BasicID[0].IDType` | always |
| `rid.uasId` | string | — | `BasicID[0].UASID` | always |
| `rid.uaType` | `Aeroplane`,`Multirotor`,`Gyroplane`,`HybridLift`,`Ornithopter`,`Glider`,`Kite`,`FreeBalloon`,`CaptiveBalloon`,`Airship`,`FreeFallParachute`,`Rocket`,`TetheredPowered`,`GroundObstacle`,`Other` | — | `BasicID[0].UAType` | when known |
| `rid.idType2` | as `rid.idType` | — | `BasicID[1].IDType` | when 2nd ID present |
| `rid.uasId2` | string | — | `BasicID[1].UASID` | when 2nd ID present |
| `rid.uaType2` | as `rid.uaType` | — | `BasicID[1].UAType` | when 2nd ID + known |

### 3.2 Self-ID

| Key | Value domain | ODID source | Cardinality |
|---|---|---|---|
| `rid.selfId` | string | `SelfID.Description` | when SelfID present |
| `rid.selfIdType` | `Text`,`Emergency`,`ExtendedStatus`,`Type<n>` | `SelfID.DescType` | when SelfID present |

### 3.3 Location extras (present whenever the ODID Location message is)

| Key | Value | Units | ODID source |
|---|---|---|---|
| `rid.status` | `Undeclared`,`Ground`,`Airborne`,`Emergency`,`RemoteIDFailure` | — | `Location.Status` |
| `rid.baroAltitudeM` | number | `m` | `Location.AltitudeBaroM` |
| `rid.heightM` | number | `m` | `Location.HeightM` |
| `rid.heightType` | `AGL`,`AboveTakeoff` | — | `Location.HeightType` |
| `rid.timestampSecsThisHour` | number | `s` | `Location.TimestampSecsThisHour` |

*(Geometric altitude and lat/lon are in native `location`; speed/heading/vertical
rate are in the native `velocity_oneof` ENU vector — not here.)*

### 3.4 Accuracy bounds (omitted when the ODID enum is `0` = unknown)

| Key | Value | Units | ODID source |
|---|---|---|---|
| `rid.speedAccuracyMps` | number | `m/s` | `Location.SpeedAccuracy` enum → bound |
| `rid.baroAccuracyM` | number | `m` | `Location.BarometricAccuracy` enum → bound |
| `rid.timestampAccuracySec` | number | `s` | `Location.TimestampAccuracy` × 0.1 |

*(Horizontal/vertical position accuracy are in the native `location.error`
envelope.)* Enum→bound mappings are in §5.

### 3.5 Regulatory & operation (from the System message)

| Key | Value | Units | ODID source | Cardinality |
|---|---|---|---|---|
| `rid.classificationType` | `EU` | — | `System.ClassificationType` | when EU |
| `rid.categoryEU` | integer | — | `System.CategoryEU` | when EU |
| `rid.classEU` | integer | — | `System.ClassEU` | when EU |
| `rid.areaCount` | integer | — | `System.AreaCount` | when > 1 |
| `rid.areaRadiusM` | integer | `m` | `System.AreaRadiusM` | when areaCount > 1 |
| `rid.areaCeilingM` | number | `m` | `System.AreaCeilingM` | when areaCount > 1 |
| `rid.areaFloorM` | number | `m` | `System.AreaFloorM` | when areaCount > 1 |
| `rid.systemTimestampSecsSince2019` | integer | `s` | `System.TimestampSecsSince2019` | when present |

### 3.6 Capture metadata (from the MQTT envelope, not ODID)

| Key | Value | Units | Source | Cardinality |
|---|---|---|---|---|
| `rid.macAddress` | string | — | envelope `MAC address` | when present |
| `rid.channel` | integer | — | envelope `channel` (the radio channel number, **not** a frequency; the DS240 reports `0` for BLE — verified, 017 CL-R4) | always |
| `rid.transport` | `BLE legacy`,`BLE long range`,`WiFi NaN`,`WiFi beacon` | — | envelope `type` | when present |
| `rid.rssiDbm` | integer | `dBm` | envelope `RSSI` | when no native `signal` block — **always for BLE**, where `channel = 0` does not resolve (017 CL-R4) |

*(The envelope `sensor ID` has no separate native field in SAPIENT v2.0; for a
single-sensor DS240 it corresponds to the ASM's own `node_id`. RSSI and the
derived `centre_frequency` live in the native `signal` block **for WLAN**; for
**BLE** the DS240 reports `channel = 0` (it does not expose the advertising
channel — verified by live capture, 017 CL-R4), so no frequency resolves, the
`signal` block is omitted, and `rid.rssiDbm` is the normal BLE strength carrier
— see FR-R-M08.)*

### 3.7 Operator (carried on the craft report — single-report model)

| Key | Value | ODID source | Cardinality |
|---|---|---|---|
| `rid.operatorLocationType` | `TakeOff`,`Dynamic`,`Fixed`,`Unknown` | `System.OperatorLocationType` | when System present |
| `rid.operatorLatDeg` | number | `System.OperatorLatitudeDeg` (passed through incl. `0`) | when System present |
| `rid.operatorLonDeg` | number | `System.OperatorLongitudeDeg` (passed through incl. `0`) | when System present |
| `rid.operatorAltM` | number | `System.OperatorAltitudeGeoM` | when System present |
| `rid.operatorId` | string | `OperatorID.OperatorID` | when present |
| `rid.operatorIdType` | `OperatorID`,`Type<n>` | `OperatorID.IDType` | when OperatorID present |

*(If a deployment genuinely tracks the operator as an independently moving
object, it MAY instead emit a second `DetectionReport` linked via the native
`associated_detection` (`association_type = SIBLING`) — spec 017 note O1. That
is not the default path and does not use an `OP-` id.)*

### 3.8 Authentication (`rid.auth.*`)

The broadcast Remote ID signature status. SAPIENT v2.0 has no native
authentication field, so it rides in `object_info`; an HLDMM that does not
implement this extension ignores the entries.

| Key | Value | ODID source |
|---|---|---|
| `rid.auth.authType` | `None`,`UASIDSignature`,`OperatorIDSignature`,`MessageSetSignature`,`NetworkRemoteID`,`SpecificMethod`,`Type<n>` | `Authentication.AuthType` |
| `rid.auth.signaturePresent` | `true`/`false` | derived (`len(Data) > 0`) |
| `rid.auth.verification` | `unsigned`,`unverified`,`verified`,`failed` | computed (see §4.1) |
| `rid.auth.signatureB64` | base64 string | `Authentication.Data` (when present) |
| `rid.auth.timestampSecsSince2019` | integer | `Authentication.Timestamp` (when ≠ 0) |
| `rid.auth.length` | integer | `Authentication.Length` (when ≠ 0) |
| `rid.auth.pages` | integer | `Authentication.PageCount` (when ≠ 0) |

---

## 4. Authentication semantics

### 4.1 Verification lifecycle

`rid.auth.verification` reflects **who has checked the signature**, not just its
presence:

- `unsigned` — no signature was broadcast (`authType = None`).
- `unverified` — a signature is present but **this ASM has not checked it**. The
  ASM is not the verification authority; it forwards the bytes.
- `verified` / `failed` — set by the **HLDMM (the verification sink)** after
  checking `rid.auth.signatureB64` against the signer's public key in the
  EIP-8004 trusted-agent registry, bound to the ASM's libp2p PeerID (015 FR-S16).

Because verification happens at the HLDMM, `rid.auth.signatureB64` is **always
carried when present** — the sink needs the bytes. The ASM emits
`unsigned`/`unverified` only; downstream rewrites `verification` after checking.

---

## 5. Accuracy enum → bound mappings

ODID reports accuracy as enums; this extension converts them to the metre/second
upper bound (the conversion is bijective for valid codes — the bound identifies
the code). Code `0` = unknown → the field is omitted.

**Horizontal** (`location.error.horizontalM`): 1=18520, 2=7408, 3=3704, 4=1852,
5=926, 6=555.6, 7=185.2, 8=92.6, 9=30, 10=10, 11=3, 12=1 (m).

**Vertical / Barometric** (`location.error.verticalM`, `rid.baroAccuracyM`):
1=150, 2=45, 3=25, 4=10, 5=3, 6=1 (m).

**Speed** (`rid.speedAccuracyMps`): 1=10, 2=3, 3=1, 4=0.3 (m/s).

**Timestamp** (`rid.timestampAccuracySec`): bound = enum × 0.1 s.

---

## 6. Object id & RF frequency conventions

- **`object_id`** is a **ULID** assigned by the ASM and stable per drone track,
  keyed on the broadcast Remote-ID identity **`BasicID[0].UASID`** — **not** the
  MQTT MAC (the MAC is the transient radio/merge key and rotates under BLE MAC
  randomisation, so it MUST NOT fragment a track; spec 017 FR-R-D02). The ASM
  declares `mode_definition.tracking_type = TRACKING_TYPE_TRACK` (persist an
  `object_id` even across broken tracks). The RID serial is **not** the
  object_id — it is carried as `rid.uasId` and mirrored to the native `id` field.
- **`signal.centre_frequency`** (reported in **Hz**; the channel plan below is in MHz for readability) is derived from `(transport, channel)` via the
  Bluetooth / IEEE 802.11 channel plans (spec 017 FR-R-M08):
  - `BLE legacy`/`BLE long range`: the DS240 reports `channel = 0` (advertising channel not exposed; verified by live capture, 017 CL-R4) → `(BLE, 0)` does not resolve → the `signal` block is omitted and RSSI rides in `rid.rssiDbm`. *(Were a sensor to expose the advertising channel: `37`→2402, `38`→2426, `39`→2480.)*
  - `WiFi NaN`/`WiFi beacon`, ch 1–13: `2407 + 5·channel`; ch 14: `2484`;
    ch ≥ 32 (5 GHz): `5000 + 5·channel`.
  - If `(transport, channel)` does not resolve, the native `signal` block is
    **omitted** and the strength is carried in `rid.rssiDbm` instead.

---

## 7. Registration

An ASM emitting these reports MUST declare this extension in its SAPIENT
Registration `extensions[]` (015 FR-S40): the schema id (`neuron.rid/1`), the
resolvable URI, and the content hash, covering the `rid.*` taxonomy above
(including `rid.operator*` and `rid.auth.*`). Consumers use the declared schema
to interpret `object_info` entries; anything not recognised is ignored per
SAPIENT's extension rules.
