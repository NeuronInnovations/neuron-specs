# `neuron.adsb/1` — SAPIENT `DetectionReport` extension for JetVision aircraft *(normative)*

Co-located normative artifact of **spec 016**. It carries the parts of the
JetVision Air!Squitter's `aircraftlist.json` that have **no native slot** in a
SAPIENT (BSI Flex 335 v2.0) `DetectionReport`. It is the air-traffic analogue of
017's `neuron.rid/1`. Everything here rides in the report's repeated
`object_info` (`TrackObjectInfo{type, value}`) under the **`adsb.*`** namespace;
an HLDMM that does not load this schema ignores `adsb.*` per SAPIENT extension
rules (015 FR-S40).

`object_info` has **no units field** — units are static per key (documented in
§3 and §4) and surfaced only in the JSON inspection view. Every `adsb.*` value
is a **string**. A key is emitted **only when its source field is present**; a
real `0` is kept (the no-suppression rule, 016 FR-A-D07) — only genuinely-absent
(`null`/missing) fields are dropped. The machine-readable form is
[`neuron.adsb-1.schema.json`](./neuron.adsb-1.schema.json); `adsb.*` `object_info`
MUST validate against it.

- **id / version**: `neuron.adsb/1`, schema `x-version: 1.0.0`.
- **appliesTo**: `DetectionReport` (declared in the agent-card `extensions[]`,
  016 FR-A-A03). It does **not** currently apply to `Task` (the altitude-band
  filter is pending, 016 CL-J11).

## 1. What maps to NATIVE SAPIENT fields (NOT carried here)

These are native `DetectionReport` fields, specified in 016 §C / *Encoding* —
listed so the boundary is unambiguous:

| `aircraftlist` | native SAPIENT field |
|---|---|
| `hex` | `object_id` (ULID, minted per ICAO hex) — and **also** `adsb.icao24` |
| `reg` (else `fli`) | `id` (the native "tail number" field) |
| `lat` / `lon` | `location.y` / `location.x` (`coordinate_system = LAT_LNG_DEG_M`) |
| `altg` (else `alt`) | `location.z` (datum `WGS84_E` geometric / `WGS84_G` barometric) |
| `nacp` → metres (A.1) / MLAT → ~300 m | `location.x_error` = `location.y_error` |
| `spd`, `trk`, `vrt` | `enu_velocity` (East/North/Up, m/s) — only when **both** `spd`+`trk` present (016 FR-A-D03; partial kinematics → §3.5) |
| `cat`, `src` | `classification` (top-level type + nested `sub_class` + per-source confidence) |
| `dbm` + source frequency | `signal` (amplitude dBm + `centre_frequency` Hz) — **both proto-mandatory**: omitted when the source is unknown **or** `dbm` is absent (016 FR-A-M07) |
| *(none — ASM-minted)* | `report_id` — a **fresh ULID per report** (mandatory + `is_ulid`; 016 FR-A-D02a), distinct from the per-track `object_id` |
| `uti`, `ns` | `SapientMessage.timestamp` (nanosecond FPGA time) |
| (track eviction) | `state = "lost"` (016 FR-A-G01) |

## 2. Source codes & provenance *(normative — reproduced from 016 §B / Annex A.2)*

`src` is a single letter naming the **selected** data source. Each resolves a
centre frequency and a provenance:

| `src` | source | `centre_frequency` | `adsb.provenance` |
|---|---|---|---|
| `A` | ADS-B (1090 + 978 UAT) | 1090 MHz (978 if `cla` 8–13) | `local` |
| `M` | Multilateration (JetVision MLAT network) | 1090 MHz (locally heard Mode-S) | **`network`** |
| `L` | OGN Decoder (local) | 868 MHz | `local` |
| `F` | piAware / FlightAware MLAT (unobserved) | 1090 MHz | **`network`** |
| `S` | FLARM SkyLens FLARM | 868 MHz | `local` |
| `D` | FLARM SkyLens ADS-L | 868 MHz | `local` |
| `O` | OGN **Server Connection** (network) | 868 MHz | **`relayed`** |
| `?` / unlisted | undetermined | — (omit `signal`) | `unknown` |

**Provenance semantics (016 CL-J4/J5, FR-A-M03):**

- **`local`** — detected **and** positioned by this box's own receivers
  (ADS-B/UAT on 1090/978, the OGN decoder and SkyLens on 868).
- **`network`** — the aircraft **is locally heard** (the box decodes its
  1090 MHz Mode-S and uploads it to a cooperative MLAT network —
  `jetvision.de` for `M`, FlightAware via piAware for `F`) but the
  **position** is computed server-side from ≥3 receivers and returned. Inside
  this sensor's coverage → **emitted by default**.
- **`relayed`** — `src=O` is pulled from the global OGN server; the box never
  heard the aircraft (coverage is continent-wide). A SAPIENT ASM reports its
  own coverage, so the ASM **drops `src=O` by default** (016 FR-A-D05;
  `--include-relayed` re-admits it).

Every emitted report carries `adsb.provenance` regardless, so a consumer can
filter for itself. Frequencies are emitted on the wire in **Hz** (`MHz × 10⁶`,
016 CL-J12).

## 3. The `adsb.*` `object_info` keys *(normative)*

### 3.1 Source & identity
| key | units | from | value domain |
|---|---|---|---|
| `adsb.icao24` | — | `hex` | hex string (the track join key) |
| `adsb.source` | — | `src` | `A` \| `M` \| `L` \| `F` \| `O` \| `S` \| `D` \| `?` |
| `adsb.provenance` | — | `src` | `local` \| `network` \| `relayed` \| `unknown` (§2) |
| `adsb.availability` | — | `ava` | subset of the source letters |
| `adsb.subclass` | — | `cla` | numeric (0=Mode-S, 8–13=UAT, 14=FLARM) — often absent |
| `adsb.callsign` | — | `fli` | string |
| `adsb.registration` | — | `reg` | string |
| `adsb.typeCode` | — | `typ` | ICAO type designator |
| `adsb.operator` | — | `opr` | operator ICAO |
| `adsb.originIcao` | — | `org` | airport ICAO |
| `adsb.destIcao` | — | `dst` | airport ICAO |
| `adsb.country` | — | `cou` | string |
| `adsb.countryIso` | — | `ciso` | ISO country code |
| `adsb.emitterCategory` | — | `cat` | raw category (`A0`–`C5`, `F0`–`F15`) — also drives native `classification` |

### 3.2 Altitude & air/ground
| key | units | from | value domain |
|---|---|---|---|
| `adsb.baroAltFt` | ft | `alt` | numeric (raw barometric; geometric is in `location.z`) |
| `adsb.geoAltFt` | ft | `altg` | numeric (raw geometric) |
| `adsb.airGround` | — | `gda` | `airborne` \| `ground` \| `unknown` |
| `adsb.airGroundTisB` | — | `gda` (lowercase) | `true` — present only when the air/ground state is TIS-B derived |

### 3.3 Surveillance status
| key | units | from | value domain |
|---|---|---|---|
| `adsb.squawk` | — | `squ` | Mode-A squawk (4-digit octal string) |
| `adsb.emergency` | — | `squ` | `hijack` (7500) \| `radioFailure` (7600) \| `general` (7700) — present only when set |
| `adsb.alert` | — | `alr` | numeric 0–3 |
| `adsb.spi` | — | `spi` | `true` \| `false` |
| `adsb.tcasMode` | — | `tcm` | numeric 0/1/2 |
| `adsb.autopilotEngaged` | — | `ape` | `true` \| `false` |

### 3.4 Position-quality / integrity *(the ADS-B set ASTERIX CAT021 would also carry)*
| key | units | from | value domain |
|---|---|---|---|
| `adsb.nacp` | — | `nacp` | numeric (also → `location.error`, A.1) |
| `adsb.sil` | — | `sil` | numeric (Source Integrity Level) |
| `adsb.sda` | — | `sda` | numeric (System Design Assurance) |
| `adsb.pic` | — | `pic` | numeric (Position Integrity Category) |
| `adsb.mopsVersion` | — | `mop` | numeric 0/1/2 |
| `adsb.trustCount` | — | `tru` | numeric (DF-11/17/18 frames decoded; a weak integrity proxy) |
| `adsb.posAgeProxy` | — | `lla` | numeric (position-age proxy; low on ADS-B, high on MLAT) |
| `adsb.signalDbm` | dBm | `dbm` | numeric — **only when there is no native `signal` block** (unknown source) |

### 3.5 Kinematics fallback *(only when no native `enu_velocity` is emitted)*

A native `ENUVelocity` needs **both** `spd` and `trk` (the proto marks
`east_rate`/`north_rate` mandatory; zeros would fabricate a hover — 016
FR-A-D03). When it cannot be emitted, whatever kinematics ARE present still
ride here (no-suppression):

| key | units | from | value domain |
|---|---|---|---|
| `adsb.groundSpeedKt` | kt | `spd` | numeric |
| `adsb.trackDeg` | deg | `trk` | numeric |
| `adsb.verticalRateFpm` | ft/min | `vrt` | numeric |

### 3.6 Mode-S Enhanced Surveillance (BDS 4,0) & Meteorological (BDS 4,4)
| key | units | from | value domain |
|---|---|---|---|
| `adsb.selectedAltFt` | ft | `alts` | numeric (MCP/FCU selected altitude) |
| `adsb.selectedHeadingDeg` | deg | `hdgs` | numeric |
| `adsb.qnhHpa` | hPa | `qnhs` | numeric (QNH pressure setting) |
| `adsb.oatC` | °C | `tmp` | numeric (outside air temperature; synthetic, not always reliable) |
| `adsb.windSpeedKt` | kt | `wsp` | numeric (synthetic) |
| `adsb.windDirDeg` | deg | `wdi` | numeric (synthetic) |

### 3.7 MLAT diagnostics *(`src=M` only, firmware-dependent)*
| key | units | from | value domain |
|---|---|---|---|
| `adsb.mlatPest` | — | `pest` | numeric (position estimations/sec) |
| `adsb.mlatNoclients` | — | `nocl` | numeric (sensors contributing) |

## 4. Design rules *(where this differs from a naive mapping)*

- **MLAT = network position, local signal.** The MLAT *position* is computed by
  the JetVision MLAT server (`jetvision.de`, ≥3 sharing receivers) from
  Mode-S data this box uploads — so `adsb.provenance = network`. The *signal*
  is the box's **own** 1090 MHz Mode-S reception of that aircraft, so the
  native `signal{1090 MHz, dbm}` block **is emitted** (016 FR-A-M07/FR-A-D08;
  the wiki source table assigns `M` → 1090 MHz). Only an **unknown** source
  fails to resolve a frequency — then `signal` is omitted and the level rides
  in `adsb.signalDbm`.
- **MLAT has no NACp.** NACp is self-reported by the aircraft; the MLAT
  position is computed by the receiver network. So instead of "unknown" (false
  precision) MLAT gets a conservative synthesised `location.error` (~300 m,
  016 FR-A-D08). ADS-B uses the NACp→metres table (A.1).
- **No operator, no authentication.** Unlike `neuron.rid/1`, manned aircraft
  broadcast no operator location and no signature; those paths are absent.
  Position **integrity** (not authenticity) is conveyed via `location.error` +
  the `adsb.*` quality set; the HLDMM weighs it (016 US4).
- **Real ICAO, never synthesised.** `object_id` is a ULID keyed on the real
  ICAO `hex`; the `hex` is also carried verbatim as `adsb.icao24`.
- **Mandatory-field honesty (016 FR-A-D02a/D03/M07).** Three DetectionReport
  rules follow from the proto's `is_mandatory`/`is_ulid` options: `report_id`
  is a **fresh ULID per report** (distinct from the per-track `object_id`);
  `enu_velocity` is emitted only when **both** `spd`+`trk` are present
  (east/north are mandatory — zeros would fabricate a hover; partial
  kinematics ride §3.5); and `signal` needs **both**
  `amplitude`+`centre_frequency` (a frequency-only block is never emitted —
  the frequency is recoverable from `adsb.source`).
- **No-suppression.** Every present field is carried even when `0`; only
  NaN/Inf (→ `0`) and genuinely-absent fields are dropped. The one contact the
  default path withholds is a **relayed** (`src=O`) one — a coverage decision,
  not a data-quality one (016 FR-A-D05/D07).

## 5. Refinements still tracked against this reference

- FLARM `F1`–`F15` are passed through as `"FLARM F<n>"` rather than decoded into
  specific labels; `C4`/`C5` obstacle categories are treated generically (016
  FR-A-A04 keeps these as the sub_class string).
- A native SAPIENT `Alert` on emergency-squawk transition is **deferred** (016
  CL-J10); emergency state is carried here as `adsb.emergency` + `adsb.squawk`.
- An altitude-band `Task` filter under `neuron.adsb/1` is **pending** (016
  CL-J11); until defined, `appliesTo` does **not** include `Task`.
