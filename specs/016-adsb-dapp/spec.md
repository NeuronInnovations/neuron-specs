# Feature Specification: JetVision Air!Squitter ADS-B ASM (DApp)

**Feature Branch**: `016-adsb-dapp`
**Created**: 2026-06-11
**Status**: Draft

> **Scope of this revision.** A sensor node is a SAPIENT **ASM**; a consumer node is a SAPIENT **HLDMM**; **015** defines the session, lane binding, registration, tasking, and fan-out mechanics once. This spec describes only **the JetVision vendor specifics** — its service, classification taxonomy, the `adsb.*` extension namespace, and the `aircraftlist.json`-to-`DetectionReport` mapping.

> **This DApp is a SAPIENT *translator*, not a seller.** Per 015's transparent-proxy model (015 FR-S90–S97), the SAPIENT-forwarding **Seller Proxy** is generic and defined once in 015. The JetVision side specified here is the **Neuron-blind modality→SAPIENT translator** that runs *behind* that proxy: it polls the Air!Squitter and emits standard SAPIENT (`Registration`/`DetectionReport`/`StatusReport`) over a conformant BSI Flex 335 edge, and it ingests SAPIENT (`Task`/`RegistrationAck`). It holds **no Neuron identity, key, wallet, or lane logic** — the Seller Proxy owns the EIP-8004 identity and `node_id`→PeerID binding (015 FR-S94), the lane routing (015 FR-S92), the end-to-end registration handshake and 003 agent-card publication (015 FR-S96), and the admission gates (015 FR-S80–S84). So "the JetVision ASM" below denotes *this translator together with the generic Seller Proxy*; 016 specifies only the translator half.

## Layer

**DApp (vendor ASM)** — composes the **Shared Application Profile 015** and, through it, the Core SDK (001–013). Per Constitution Principle XII (three-tier model). This spec MUST NOT redefine any 015 or Core requirement; it references them. Concretely it defines a **Neuron-blind SAPIENT translator** that runs behind 015's generic **Seller Proxy** (015 FR-S90–S97); all Neuron-side concerns — identity, signing, lane routing, registration handshake, 003 publication, admission — belong to that proxy, not here.

This spec is **vendor-named** (the JetVision Air!Squitter / Radarcape sensor), not modality-named: the registration identifies a JetVision sensor and the air-surveillance modality it provides, rather than a modality in the abstract. Modality and standard semantics (ADS-B/Mode-S DO-260B/ED-102A, UAT DO-282B, FLARM/OGN) are **input contracts**, not redefined here.

**The single-sensor, multi-source shape (the difference from 017).** 017's DS240 has one RF source (broadcast Remote-ID). The Air!Squitter decodes **four** sources into **one** `aircraftlist.json`, and each record names its *selected* source in `src` (`A` ADS-B 1090/978, `M` MLAT, `L`/`O` OGN, `F` piAware, `S`/`D` SkyLens). This is still **one ASM, one SAPIENT `Registration`, one service** — the source diversity is carried *inside* the report (native `signal.centre_frequency`, native `classification.confidence`, and `adsb.source`/`adsb.provenance`), **not** as separate services. (Splitting raw Mode-S out as a *separate non-SAPIENT* feed is the one exception; see *Out of Scope* / the BEAST boundary.)

## Related Specs

**Shared Application Profile (the substrate this DApp stands on):**
- **015 SAPIENT Sensor Interop Profile** — the SAPIENT message set & directions, lane binding (`DetectionReport`/`AssociatedFile`→009 p2p; `Registration`/`RegistrationAck`/`Task`/`TaskAck`/`StatusReport`/`Alert`→004 auditable), the pointer-model Registration↔agent-card, identity binding, tasking, fan-out (one ASM→many HLDMM), the `object_info` extension mechanism, the **service** concept, and encoding (protobuf normative, JSON projection optional). **All session mechanics live there.** 015 FR-S70 / FR-S95 explicitly defer to **this spec** the question of how a JetVision node's **SAPIENT** (`adsb`) identity relates to a separate **BEAST** (raw Mode-S) identity (CL-16) — discharged in *Out of Scope* below.

**Core SDK (consumed via 015, not restated):** 001/007 identity, 002 keys, 003 registry, 004 topics, 005 health, 006 determinism, 008 payment (the `adsb` service is an 008 agreement), 009 p2p, 010 validation, 011 relay, 013 connectivity profiles.

**Sibling DApp:** **017 DroneScout Remote-ID ASM** — the single-source sibling. 016 deliberately mirrors 017's structure (vendor-named ASM, single SAPIENT service, `neuron.<modality>/1` extension, no fusion); the differences are intrinsic to the modality (multi-source, manned aircraft, poll-only ingest), enumerated in *Clarifications* and *Mapping*.

**Consumer side:** **018 CoT Display Consumer** — the lightweight HLDMM (SAPIENT→CoT behind the generic Buyer Proxy, static affiliation policy, aggregation without association); fusion is the reserved **019**. A consumer MAY buy this ASM's `adsb` service and consume it with **no fusion** (015 FR-S71).

**Reference implementation (informative, not normative; external repository — not part of this source release):**
- `neuron-jv-bridge/internal/sapient/` — `detection.go` (the `aircraftlist`→`DetectionReport` renderer), `classify.go` (the source/category/NACp lookup tables), `wire.go` (the protobuf/JSON encoder), `EXTENSION.md` + `neuron.adsb-1.schema.json` (the `neuron.adsb/1` namespace), `agent-card.json` (the ICD sketch); `internal/jetvision/` (the `aircraftlist.json` model + HTTP poller); `internal/state/` (the ULID track cache + change-detection). The SAPIENT protobuf is generated from the vendored DSTL protos (`internal/sapientpb`). **Unlike 017's `detection.go` (which predated the verified v2.0 protos), the JetVision reference impl was written against the verified protos and this spec, so it already conforms** — the *Mapping* table records only the handful of spec requirements that exceed the current build.
- `neuron-jv-bridge/AIRCRAFTLIST.md` — the `aircraftlist.json` field reference, transcribed from the vendor wiki. Its content is promoted into **§B** here so the translation is buildable without the wiki or the reference code.

**External (input contract):** RTCA DO-260B / EUROCAE ED-102A (1090 MHz ADS-B), DO-282B (978 MHz UAT), Mode-S Enhanced Surveillance (ICAO Annex 10 Vol IV, BDS registers), FLARM/OGN — referenced for field semantics, **not** redefined. **ULID** ([github.com/ulid/spec](https://github.com/ulid/spec)) — the identifier format SAPIENT's `is_ulid` fields require (`object_id`, `report_id`; FR-A-D02/D02a). The JetVision Air!Squitter / Radarcape **`aircraftlist.json` JSON Service** (`http://<box>/aircraftlist.json`) is **documented normatively in §B** (from the vendor wiki) so the bridge is implementable without the manual or the reference code.

## Purpose

Specify a **JetVision Air!Squitter** (and Radarcape) sensor node acting as a SAPIENT **ASM** that offers a single Neuron service — **`adsb`** — emitting SAPIENT `DetectionReport`s for aircraft decoded from its unified `aircraftlist.json` (ADS-B 1090, MLAT, UAT 978, FLARM/OGN 868), carrying the full air-traffic payload via the `neuron.adsb/1` `object_info` extension. The ASM registers, streams, status-reports, and accepts tasking exactly as 015 prescribes; this spec defines only what is JetVision-specific.

## Out of Scope

- **The SAPIENT session, lane binding, registration handshake, tasking, fan-out, encoding** — all owned by **015**. This spec references them.
- **ADS-B / Mode-S / UAT / FLARM message-field layout** (the air-interface wire formats decoded by the box) — external normative (DO-260B/ED-102A, DO-282B, ICAO Annex 10); **referenced, not redefined**. The `aircraftlist.json` *envelope* (every field, the `src`/`cla`/`cat` tables) IS specified — §B / FR-A-M* — so a bridge is buildable from this spec plus the named standards alone.
- **Raw Mode-S / BEAST (the "settled in 016" pointer from 015 FR-S70 / FR-S95).** This spec is the **assembled, position-bearing** SAPIENT `adsb` service. Raw Mode-S frames (time-of-arrival-preserving, for downstream MLAT) are **not** carried as a position-relaxed SAPIENT `DetectionReport`: a stock SAPIENT HLDMM cannot consume a location-less raw report, and SAPIENT does not preserve the FPGA-nanosecond time-of-arrival that MLAT needs (015 CL-16). Raw Mode-S is therefore a **separate, non-SAPIENT BEAST feed on its own BEAST proxy pair** (a sibling profile, not written here), advertised with a BEAST-interface agent-card and a non-`/sapient/*` protocol ID (015 FR-S11). **Identity relationship (discharging 015 FR-S70/S95):** the SAPIENT `adsb` service and any BEAST `modes-raw` feed of the same physical box are **two distinct Neuron node identities — one per protocol** (015 FR-S95: "a vendor that also emits BEAST runs a separate BEAST Seller Proxy"); they are not multiplexed onto one `Registration`. They MAY share a common `config_data` vendor identity (manufacturer/model/serial) so a consumer can recognise them as the same hardware, but each rides its own proxy pair, its own agent-card, and its own EIP-8004 entry. The BEAST feed's full shape is deferred to its own spec.
- **UAT, FLARM/OGN as *separate services*.** They are not separate feeds — the box folds them into the one `aircraftlist.json` and names them in `src`/`cla`. They surface inside the `adsb` report (native `signal.centre_frequency` = 978/868 MHz, `adsb.source`, `adsb.subclass`), not as sibling services. *(015's DApp-decomposition note listed `uat`/`flarm-ogn` as illustrative possible services; on this hardware they are sources within `adsb`, per CL-J1.)*
- **Network-relayed contacts as this sensor's coverage.** `src=O` (OGN *Server Connection*) is pulled from the global OGN network, not RF-received by this box; it is excluded by default (FR-A-D05) and is not part of the sensor's advertised coverage.
- **The box's native SBS-1 output (`:30003`).** The Air!Squitter emits SBS natively; this bridge does **no** SBS translation (the RID bridge's SBS path has no analogue here).
- **Fusion, plot-to-track association, derived affiliation / friend-foe** — fusion-HLDMM concern (**019**, reserved); **CoT/TAK output and the static affiliation policy** — display-consumer concern (**018**); ASTERIX/map UI — unassigned/product.
- **Sensor inventory** (count, locations, IPs, partner mapping) — deployment configuration, not protocol semantics.

## Clarifications

### Session 2026-06-11

- Q: One service or many for a multi-source box? (CL-J1) → A: **One SAPIENT service `adsb`**, mirroring 017's single-service shape. The Air!Squitter's four sources (ADS-B 1090, MLAT, UAT 978, FLARM/OGN 868) all arrive in **one** `aircraftlist.json`; the source is carried *inside* each report (`signal.centre_frequency`, `classification.confidence`, `adsb.source`/`adsb.provenance`), not as separate services. UAT and FLARM/OGN are **sources, not services** on this hardware. The one genuine split — raw Mode-S as a separate **non-SAPIENT BEAST** feed — is *Out of Scope* here (a separate proxy pair, separate identity; discharges 015 FR-S70/S95).
- Q: Vendor or modality named, and the directory? (CL-J2) → A: **Vendor** (JetVision Air!Squitter); directory `016-adsb-dapp` (parallel to `017-remote-id-dapp`).
- Q: `object_id`? (CL-J3) → A: SAPIENT v2.0 requires a **ULID**; the ASM mints one **per ICAO `hex`** (the airframe address — both the ingest key and the track identity, unlike 017 where the MAC and the identity differ). `tracking_type = TRACKING_TYPE_TRACK`. The real `hex` rides as `adsb.icao24`; the native `id` ("tail number") is the registration (`reg`), else the callsign (`fli`).
- Q: How is the multi-source nature reflected natively? (CL-J12) → A: Per-source **`signal.centre_frequency`** (1090 / 978 / 868 MHz, reported in **Hz (SI)** — `MHz × 10⁶`) and per-source **confidence** on `detection_confidence` / `classification.confidence`; the selected source code rides as `adsb.source`.
- Q: MLAT (`src=M`)? (CL-J4) → A: **MLAT is a network-computed position for a locally-heard aircraft.** The box uploads its locally received Mode-S data to the JetVision MLAT server (`jetvision.de`; ≥3 sharing receivers — Radarcape version history v170521), which returns the multilaterated positions associated to that data. So the *detection* is local (the box decodes the aircraft's 1090 MHz Mode-S; `dbm` is the box's own measurement of those frames — the wiki source table assigns `M` → 1090 MHz) but the *position* is a server product. Consequences: `adsb.provenance = network` (a third value — neither `local` nor `relayed`); the native `signal` block **IS emitted** (`centre_frequency` = 1090 MHz, `amplitude` = `dbm`); and because a network solve carries **no NACp**, a conservative **~300 m** horizontal `location.error` is **synthesised** (honest "known-poor", not a false "unknown"). MLAT contacts are **emitted by default** — the aircraft is inside this sensor's RF coverage. *(Corrects an earlier draft + the reference impl, which tagged `M` as `local` and omitted the `signal` block on a "no single carrier" rationale — disproven by the vendor docs.)* `src=F` (FlightAware MLAT, returned via the piAware feeder — per the wiki `ava` gloss + version history "FlightAware no compromise feed including multilateration results") is the same shape and classifies identically; it is unobserved on current deployments.
- Q: Relayed OGN (`src=O`)? (CL-J5) → A: A SAPIENT ASM reports **its own coverage**. `src=O` (OGN *Server Connection*) is pulled from the global OGN network — the box never heard the aircraft — so it is **excluded by default**; `--include-relayed` re-admits it (deployment config). `src=L` (OGN *Decoder*) and `S`/`D` (SkyLens) are the box's own 868 MHz reception and are **local**. Every emitted report carries `adsb.provenance ∈ {local, network, relayed, unknown}` so a consumer can filter regardless. Only `relayed` is dropped by default (`local`/`network`/`unknown` all emit). This is a **provenance/coverage** filter, *not* a data-quality filter — the no-suppression rule (FR-A-D07) is preserved.
- Q: How does a poll-only source become a stream? (CL-J6) → A: The `aircraftlist.json` is **poll-only**; the ASM polls on an interval and forwards a contact **only on a meaningful change** vs the last one emitted (change-detection), deduping identical repeats. Thresholds sit at/below the sensor's own quantisation (pos 3 m, alt 25 ft, speed 1 kt, track 1°, vertical rate 64 ft/min) plus any change to identity/squawk/source/status/integrity-enum — so **no real motion is swallowed**. **No altitude biasing, no aircraft cap** (no-suppression; the HLDMM judges).
- Q: What tasking does the `adsb` service accept? (CL-J7) → A: **Match 017** — `region_definition` + `class_filter_definition` + the mandatory `CONTROL_STOP`/`CONTROL_START` stream baseline (015 FR-S29 / FR-A-D10). **No `detection_report_rate` decimation** (kept parallel to 017 for now). The Air!Squitter is `NODE_TYPE_PASSIVE_RF` (non-pointable, non-mobile), so its `TaskDefinition` advertises **no** node-global `CommandType`; 015 FR-S28's "refuse node-global while multiplexed" rule never triggers and fan-out is unconditionally faithful.
- Q: Emergencies and aircraft leaving coverage? (CL-J8 / CL-J9) → A: **Lost-track is normative** — when a track is evicted (no sighting within the TTL) the ASM MUST emit **one** final `DetectionReport` with native **`state = "lost"`** (FR-A-G01). **Emergency `Alert` is deferred** — emergency squawks (7500/7600/7700) ride as `adsb.emergency` + `adsb.squawk` faithful pass-through; a native SAPIENT `Alert` on the *transition into* emergency is an open item (CL-J10).
- Q: Altitude datum? (CL-J13) → A: Geometric/GNSS altitude (`altg`) → **`LOCATION_DATUM_WGS84_E`** (ellipsoid — exact) and is **preferred** for `location.z`; barometric (`alt`) → **`LOCATION_DATUM_WGS84_G`** (geoid — the closest-available label; barometric is not literally geoid-referenced, so both raw altitudes also ride verbatim in `adsb.geoAltFt`/`adsb.baroAltFt`).
- Q: Native `id` (tail number)? (CL-J14) → A: **`reg` (registration), else `fli` (callsign), else omit** — a human-facing label only; the ULID (seeded on `hex`) is the real key and `hex` is also carried as `adsb.icao24`.
- Q: Are all proto-mandatory `DetectionReport` fields covered, so the spec alone yields a conformant message? (CL-J15) → A: Audited the DSTL `is_mandatory`/`is_ulid` field options across `SapientMessage`/`DetectionReport`/`Location`/`ENUVelocity`/`Classification`/`Signal`/`TrackObjectInfo`. Three gaps found and closed: **`report_id`** is mandatory + `is_ulid` → a fresh ULID per report (FR-A-D02a); **`ENUVelocity.east_rate`/`north_rate`** are mandatory → native velocity only when both `spd`+`trk` are present, partial kinematics via `adsb.*` fallback keys (FR-A-D03); **`Signal.amplitude`** is mandatory (not just `centre_frequency`) → a `signal` block needs both, never frequency-only (FR-A-M07). All other mandatory fields (`Location.x/y/coordinate_system/datum`, `Classification.type`, `TrackObjectInfo.type/value`, the envelope `timestamp`/`node_id`/`content`) were already always populated.

### Open clarifications

- **CL-J10 [OPEN]**: Native SAPIENT `Alert` on emergency-squawk transition. 016 carries emergency state only as `adsb.emergency`/`adsb.squawk` (faithful pass-through). Emitting a native `Alert` message on the *transition* into a 7500/7600/7700 squawk (per 015 FR-S01's `Alert` support, on the auditable lane) is a documented upgrade, **not** specified here. Resolve by defining the trigger (edge-detect on `squ`), the `Alert` fields, and the de-bounce, then promoting it to a normative FR. Until then an ASM MUST NOT emit `Alert` for emergencies (the HLDMM reads `adsb.emergency`).
- **CL-J11 [OPEN]**: Altitude-band tasking filter — 016 intends to accept an altitude-band constraint as a `neuron.adsb/1` `object_info` extension filter carried in the `Task` (015 FR-S26a). The filter key + schema are **not yet defined** in Annex A, and the agent-card does **not** declare `neuron.adsb/1` as `appliesTo: Task`. Until both are in place, the ASM MUST reject an altitude-band `Task` (015 FR-S26a → `UnsupportedTask`). *(This mirrors 017's open CL-R7; altitude-band filtering is especially valuable for dense air traffic — low altitudes are the signal — so it is the most likely first extension to land.)* Resolve by defining the filter key in `neuron.adsb-1.extension.md` + schema and adding the `Task` declaration to the agent-card.

### Resolved clarifications

- **CL-J1 [RESOLVED 2026-06-11]**: One SAPIENT service `adsb` (multi-source: 1090/978/868/MLAT from one `aircraftlist.json`); source diversity carried inside the report, not as sibling services; raw Mode-S/BEAST is a separate non-SAPIENT feed (Out of Scope; discharges 015 FR-S70/S95).
- **CL-J2 [RESOLVED 2026-06-11]**: Vendor-named (JetVision Air!Squitter); directory `016-adsb-dapp`.
- **CL-J3 [RESOLVED 2026-06-11]**: `object_id` = ASM-minted ULID keyed on ICAO `hex`; `tracking_type = TRACKING_TYPE_TRACK`; `hex` also in `adsb.icao24`; native `id` = `reg`→`fli`.
- **CL-J4 [RESOLVED 2026-06-11; corrected same day per the vendor docs]**: MLAT (`M`, and `F` FlightAware MLAT) = **network-computed position for a locally-heard aircraft** (`jetvision.de`, ≥3 sharing receivers — version history v170521). `adsb.provenance = network`; native `signal` **emitted** (1090 MHz + local `dbm` — wiki source table); synthesised ~300 m horizontal `location.error` (a network solve has no NACp); **emitted by default** (inside this sensor's coverage). Corrects the earlier "no single carrier → omit signal, provenance local" draft and the reference impl.
- **CL-J5 [RESOLVED 2026-06-11]**: Relayed OGN (`src=O`, Server Connection — the box never heard the aircraft) excluded by default (coverage honesty); `L`/`S`/`D` 868 MHz local reception stays local; `adsb.provenance ∈ {local, network, relayed, unknown}` always tagged, only `relayed` dropped; `--include-relayed` = deployment config; a provenance filter, not data-quality.
- **CL-J6 [RESOLVED 2026-06-11]**: Poll-only `aircraftlist.json` → change-detection stream; thresholds at/below sensor quantisation; no biasing, no cap (no-suppression).
- **CL-J7 [RESOLVED 2026-06-11]**: Tasking = region + class filters + mandatory STOP/START; no rate decimation; `NODE_TYPE_PASSIVE_RF`, no node-global commands (matches 017).
- **CL-J8 [RESOLVED 2026-06-11]**: Lost-track = a native `DetectionReport.state = "lost"` final report on eviction (FR-A-G01); normative (exceeds the current reference build, which evicts silently).
- **CL-J9 [RESOLVED 2026-06-11]**: Emergency = `adsb.emergency` + `adsb.squawk` faithful pass-through; native `Alert` deferred (CL-J10).
- **CL-J12 [RESOLVED 2026-06-11]**: `signal.centre_frequency` reported in **Hz (SI)** — 1090/978/868 MHz × 10⁶ (no unit enum on SAPIENT `Signal`, so SI Hz is the interoperable default — same resolution as 017's CL).
- **CL-J13 [RESOLVED 2026-06-11]**: Datum — geometric `altg` → `WGS84_E` (ellipsoid, preferred for `location.z`); barometric `alt` → `WGS84_G` (geoid, closest-available label); raw values also in `adsb.geoAltFt`/`baroAltFt`.
- **CL-J14 [RESOLVED 2026-06-11]**: Native `id` = `reg` else `fli` else omit; ULID (hex-seeded) is the key.
- **CL-J15 [RESOLVED 2026-06-11]**: Proto-mandatory audit — three buildability gaps closed: `report_id` = fresh ULID per report (FR-A-D02a); `enu_velocity` only when `spd`+`trk` both present, partials → `adsb.groundSpeedKt`/`trackDeg`/`verticalRateFpm` (FR-A-D03); `signal` needs both `amplitude`+`centre_frequency` (FR-A-M07). Reference impl fixed in lockstep (Mapping M5/M6). The same `report_id` gap exists in **017** (fixed there as FR-R-D02a) and in 015's FR-S41 native-field checklist (amended).

## User Scenarios & Testing *(mandatory)*

### User Story 1 — JetVision sells multi-source aircraft detections to a consumer over SAPIENT (Priority: P1)

A JetVision sensor node registers an `adsb` service and, once a consumer connects (per 015), streams SAPIENT `DetectionReport`s for each tracked aircraft over the p2p lane — every contact in the unified `aircraftlist.json` (ADS-B, MLAT, UAT, FLARM/OGN) on one feed, each tagged with its source and provenance. This is the MVP and the second SAPIENT-over-Neuron sensor dApp (the air-traffic sibling of 017).

**Why this priority**: Without JetVision emitting fuse-ready SAPIENT air-traffic detections on the lanes, there is no ADS-B data product to exercise 015's multi-source ASM pattern.

**Independent Test**: Run the reference ASM (the `neuron-jv-bridge` renderer behind a 015 ASM runtime) against a recorded `aircraftlist.json` capture (feed-source `replay`) and a stub HLDMM over Profile F; assert `Registration`(service `adsb`)→`RegistrationAck`, then `DetectionReport`s on `/sapient/detection/2.0.0` each with a ULID `object_id` **and a fresh ULID `report_id`**, `adsb.icao24` = the ICAO hex, a populated `location` with an `error` envelope, an `ENUVelocity`, a `classification` derived from the emitter category, a `signal.centre_frequency` (1090/978/868 MHz in Hz) for frequency-resolving sources, and a correct `adsb.provenance` (`local` for own-receiver contacts, `network` for MLAT).

**Acceptance Scenarios**:
1. **Given** a 1090 MHz ADS-B aircraft (hex, position, NACp 9 → 30 m, geometric altitude, emitter category `A3`), **When** rendered, **Then** the `DetectionReport` has `location.error.horizontalM = 30`, `datum = WGS84_E`, `classification[0] = {type:"Air Vehicle", sub_class:[{type:"Large"}]}`, `signal.centre_frequency = 1090000000`, `detection_confidence = 1.0`, and the on-lane `node_id` resolves to the ASM's EIP-8004-bound identity (bound by the Seller Proxy, 015 FR-S94).
2. **Given** an MLAT contact (`src=M`, position, no NACp, signal level −80 dBm), **When** rendered, **Then** the native `signal` block carries `centre_frequency = 1090000000` Hz and `amplitude = -80` (the box's own Mode-S reception), `adsb.provenance = network`, `location.error.horizontalM ≈ 300` (synthesised — the position is a server solve), and `classification` confidence is conservative (no self-declared category).
3. **Given** a Mode-S contact with identity but no merged position, **When** rendered, **Then** no report is emitted on the assembled stream (withheld until a position arrives, 015 FR-S35 / FR-A-D09).

### User Story 2 — Consumer discovers JetVision capabilities from the registry (Priority: P1)

A consumer resolves the Air!Squitter's agent-card (the SAPIENT `Registration`, pointer model per 015) from the registry and learns: vendor identity (JetVision / Air!Squitter), `NodeType = PASSIVE_RF`, the `adsb` service, the air/surface/UAV taxonomy, coverage (RF reception footprint), the declared `StatusReport` interval, accepted tasks, and the `neuron.adsb/1` extension schema — before connecting.

**Acceptance Scenarios**:
1. **Given** a registered Air!Squitter, **When** the consumer does `003 LookupRegistration`, **Then** the agent-card's `config_data` carries manufacturer "JetVision" + model "Air!Squitter", `node_definition.node_type = NODE_TYPE_PASSIVE_RF`, the `adsb` service, and `extensions[]` containing `neuron.adsb/1` (id + URI + hash).
2. **Given** the resolved card, **When** a later report carries `adsb.*` `object_info`, **Then** the consumer interprets it using only `neuron.adsb/1` (015 FR-S40), zero JetVision-specific code.

### User Story 3 — Source provenance & coverage honesty (Priority: P2)

The unified list mixes three provenances: contacts the box's own receivers detected and positioned (`local`), MLAT contacts the box heard but whose position came back from the cooperative MLAT network (`network`), and OGN-server contacts the box never heard at all (`relayed`). The ASM tags every report with `adsb.source` and `adsb.provenance`, and **excludes only the relayed contacts by default** so the feed reflects this sensor's own coverage — not a continent-wide network view it didn't observe.

**Acceptance Scenarios**:
1. **Given** an `src=A` (local ADS-B) and an `src=O` (relayed OGN) contact in the same poll, **When** rendered with default settings, **Then** the ADS-B contact is emitted with `adsb.provenance = local` and the relayed one is **dropped**; **When** rendered with `--include-relayed`, **Then** both are emitted and the OGN one carries `adsb.provenance = relayed`.
2. **Given** an `src=M` (MLAT) contact, **When** rendered with default settings, **Then** it **is emitted** (the aircraft is locally heard — inside this sensor's coverage) with `adsb.provenance = network`.
3. **Given** an unknown source code (`src=?` or unlisted), **When** rendered (and not relayed-filtered), **Then** `adsb.provenance = unknown` and the contact is still emitted (no-suppression).

### User Story 4 — Position integrity conveyed for HLDMM trust weighting (Priority: P2)

ADS-B/Mode-S broadcasts are **unauthenticated and spoofable**; this ASM asserts no signature. Instead it conveys *position integrity* natively (the `location.error` envelope) and via the `adsb.*` quality set (NACp/SIL/SDA/PIC/MOPS, trust count), so the HLDMM — the trust authority (015 FR-S16) — can weigh each contact. This is the air-traffic analogue of 017's broadcast-authentication story: the ASM forwards the evidence; the consumer judges.

**Acceptance Scenarios**:
1. **Given** an ADS-B contact with NACp 7, SIL 3, SDA 2, MOPS 2, **When** rendered, **Then** `location.error.horizontalM = 185.2` and `adsb.nacp=7`/`adsb.sil=3`/`adsb.sda=2`/`adsb.mopsVersion=2` are all carried.
2. **Given** a contact with a very low `tru` (trust) count, **When** rendered, **Then** it is **still emitted** (`adsb.trustCount` carried; the HLDMM judges plausibility — no-suppression), never dropped by the ASM.
3. **Given** any contact, **When** rendered, **Then** the ASM asserts **no** "verified"/"trusted" position flag — integrity is reported, not adjudicated (the HLDMM's role).

### User Story 5 — Lost-track signalling (Priority: P2)

When an aircraft leaves the list (out of range / landed / transponder off) and its track ages out, the ASM emits one final `DetectionReport` with native `state = "lost"`, carrying the last-known position — so the HLDMM can close the track cleanly rather than infer disappearance from a silence.

**Acceptance Scenarios**:
1. **Given** a track with no sighting for longer than the eviction TTL, **When** the ASM evicts it, **Then** exactly **one** final `DetectionReport` is emitted with the same `object_id`, `state = "lost"`, and the last-known `location`, and no further reports for that `object_id` follow (until a fresh detection mints a new ULID).
2. **Given** a `Task{CONTROL_STOP}` is in effect for a session, **When** a track ages out, **Then** **no** `lost` report is emitted to that stopped session (lost-track follows the same emission gate as ordinary reports).

### User Story 6 — Feed-source disclosure for TEVV (Priority: P3)

The ASM advertises whether its data plane is `live`, `replay`, `synthetic`, or `placeholder` so a validator (010) can tell operational from fixture evidence.

**Acceptance Scenarios**:
1. **Given** any seller invocation, **When** it emits a SAPIENT `StatusReport` (the sensor health/coverage/mode beat — 015 FR-S31, **distinct** from the Neuron 005 liveness heartbeat), **Then** it advertises `feedSource ∈ {live, replay, synthetic, placeholder}` (absent ⇒ `live`); a non-`live` value marks the run as fixture evidence.

### Edge Cases

- **No position** → no report on the assembled stream (withheld, FR-A-D09); a Mode-S identity decoded before its first fix waits.
- **No identifier (`hex` empty)** → no track, no report (FR-A-M05).
- **MLAT** → emitted (`adsb.provenance = network`); native `signal{1090 MHz, dbm}` (the box's own Mode-S reception); ~300 m synthesised error (FR-A-D08).
- **Relayed OGN (`src=O`)** → excluded by default (the box never heard it); provenance tagged (FR-A-D05).
- **NaN/Inf** in a decoded value → sanitised to 0 (no other mutation); never dropped (no-suppression).
- **A real `0`** (squawk `0000`, vertical rate `0`) → emitted faithfully; only genuinely-absent (`null`/missing) fields are omitted.
- **Emergency squawk (7500/7600/7700)** → `adsb.emergency` + `adsb.squawk` carried; no native `Alert` (deferred, CL-J10).
- **HTTP poll fails / times out** → skipped, retried next interval; not a mid-stream error (FR-A-I01).

## Requirements *(mandatory)*

### Functional Requirements (FR-A-*)

**A. Service & vendor identity**

- **FR-A-A01**: The JetVision ASM MUST register exactly one service named **`adsb`** (008), discoverable in the 003 registry per 015 FR-S70. The `adsb` service MUST carry an 008 `neuron-commerce` `pricing` object (008 FR-P01/P03); 016 fixes neither the unit nor the amount — both are deployment config — and a **free** deployment sets `amount:"0"`. The four decoded sources are **sources within this one service**, not separate services (CL-J1).
- **FR-A-A02**: The agent-card (= SAPIENT `Registration`, 015 FR-S20) MUST set `config_data` `manufacturer = "JetVision"`, `model = "Air!Squitter"` (or `"Radarcape"`), and `node_definition.node_type = NODE_TYPE_PASSIVE_RF` (ADS-B/Mode-S/UAT/FLARM reception is passive RF interception). `capabilities` MUST describe the RF reception footprint as coverage (aircraft are received, not ranged — no steerable FoV). The single `mode_definition` MUST declare `scan_type = SCAN_TYPE_FIXED` (fixed/omnidirectional reception) and `tracking_type = TRACKING_TYPE_TRACK` (a stable `object_id` persists across detections of the same airframe, keyed on the ICAO `hex`, FR-A-D02). A fill-in-the-blanks template is co-located: **`agent-card.example.json`** (illustrative; normative requirements are here + 015 FR-S20). What goes **on-chain (007)** vs **off-chain (resolved via `agentURI`)** is specified in **015 FR-S24**. The translator **declares this SAPIENT-capability content in its `Registration`**; the **Seller Proxy synthesises and publishes the 003 agent-card from it and fills the Neuron-side blocks** (the 003 `services[]` — `neuron-topic`×3 + `neuron-p2p-exchange` + the 008 `neuron-commerce` service — plus the `identity` binding, per 003/004, 015 FR-S20a) — the blind translator does not publish to 003 itself (015 FR-S96).
- **FR-A-A03**: The agent-card MUST declare the `neuron.adsb/1` extension (id + resolvable URI + content hash) in `extensions[]` (015 FR-S40), `appliesTo: DetectionReport`, covering the `adsb.*` namespace.
- **FR-A-A04** *(classification taxonomy)*: The taxonomy the ASM declares and emits is derived from the ADS-B/FLARM **emitter category** (`cat`): top-level type ∈ **{`Air Vehicle`, `Surface Vehicle`, `UAV`}** with the specific category as a nested native `sub_class`. Mapping: `A0–A7`, `B1–B5`, `B7` → `Air Vehicle`; **`B6` → `UAV`** (the one genuinely-unmanned ADS-B row); `C1–C3` → `Surface Vehicle`; `F0–F15` (FLARM) → `Air Vehicle` (raw `F<n>` as the sub_class). An absent/unknown category resolves to `Air Vehicle` (this sensor sees only flying/surface contacts) — total, never failing. There is **no friend/foe / affiliation** here (downstream: 018's static policy / 019's derived affiliation), per SAPIENT.
- **FR-A-A05** *(admission — all three gates per 015 §M / FR-S80–S84)*: This deployment exercises **all three admission gates**, with concrete policy as deployment config (no partner list in this spec, per 015 FR-S81): (a) **network entry** — a permissioned 007 Identity Registry gate (FR-S84); (b) **seller-side** — the ASM MAY allowlist which consumers may buy `adsb` via 008 `AdmissionPolicy`, denials surfaced as `AdmissionDenied` (FR-S83); (c) **consumer-side** — an HLDMM MAY allowlist which sensors it accepts, refusals via `RegistrationAck.acceptance = false` → `RegistrationRejected` (FR-S82). Both peer gates evaluate the counterpart's EIP-8004-bound identity, never an address. The two registrations — **ledger/EIP-8004 registration** and **SAPIENT `Registration`** — MUST be named distinctly per 015 FR-S23. **All three gates are enforced by the Seller Proxy / 008 / the consumer's `RegistrationAck`, not by the Neuron-blind translator**; a 007/008 denial the blind translator cannot see surfaces as a SAPIENT `Error` (015 FR-S97).

**B. `aircraftlist.json` ingest — source format** *(the input contract, specified so a bridge is implementable without the reference code. ADS-B/Mode-S/UAT/FLARM field semantics are external; everything `aircraftlist`-specific is normative here. Source: JetVision Air!Squitter wiki, "Aircraftlist JSON Service".)*

- **FR-A-M01** *(transport — poll-only)*: The Air!Squitter serves aircraft state as a **bare JSON array** of aircraft objects at **`http://<box>/aircraftlist.json`** (port 80). It is **poll-only** (no push, no stream). An ASM MUST poll it on an interval (reference default `1s`, configurable) with a bounded per-request timeout, and MUST tolerate a poll that fails or times out by **skipping that interval and retrying the next** (FR-A-I01) — never aborting the stream. Unlike 017's MQTT push, there is no broker.
- **FR-A-M02** *(record schema)*: Each array element is an aircraft object. Fields for which no data exist may be **absent or `null`** — an ASM MUST treat both as "not present" and MUST model every numeric field as optional so a real **`0` is distinguishable from absent** (the no-suppression rule). An ASM MUST tolerate unknown extra keys. The normative field set (Air!Squitter wiki):

  | key | type | meaning |
  |---|---|---|
  | `uti` / `ns` | int | observation time: UNIX seconds / nanoseconds-within-second (FPGA clock) |
  | `hex` | string | ICAO 24-bit address (or FLARM id), hex — **the track key** (FR-A-M05) |
  | `fli` | string | callsign / flight id |
  | `src` | string | **selected** data source (one letter — FR-A-M03) |
  | `ava` | string | available sources (subset of the source letters) |
  | `cla` | int | ADS-B subclass (FR-A-M03; often absent on current firmware) |
  | `cat` | string | emitter category (`A0`–`C5`, FLARM `F0`–`F15`) → `classification` (FR-A-A04) |
  | `lat` / `lon` | float | WGS84 position (degrees) |
  | `alt` / `altg` | float | barometric / geometric (GNSS) altitude (ft) |
  | `spd` / `trk` / `vrt` | float | ground speed (kt) / true track (deg) / vertical rate (ft/min) |
  | `gda` | string | `A`=airborne `G`=ground `U`=unknown (lowercase ⇒ TIS-B-derived) |
  | `squ` | string | Mode-A squawk (4-digit octal) |
  | `alr`/`spi`/`tcm`/`ape` | int/bool | alert status / SPI ident / TCAS mode / autopilot engaged |
  | `nacp`/`sil`/`sda`/`pic`/`mop`/`tru` | int | integrity set (NACp, SIL, SDA, PIC, MOPS version, trust count) |
  | `reg`/`typ`/`opr`/`org`/`dst`/`cou`/`ciso` | string | DB-derived registration / type / operator / route / country |
  | `dbm` | float | approximate signal level (dBm) |
  | `lla` | float | age of last position (s) — a position-age proxy |
  | `alts`/`hdgs`/`qnhs` | float | Mode-S Enhanced Surveillance (BDS 4,0) selected values |
  | `tmp`/`wsp`/`wdi` | float | meteorological (BDS 4,4): outside-air-temp / wind speed / wind direction |
  | `pest`/`nocl` | int | MLAT diagnostics (`src=M` only, firmware-dependent) |

- **FR-A-M03** *(source & subclass discrimination — the multi-source key)*: The `src` letter names the **selected** source and resolves both a **centre frequency** and a **provenance** (Annex A.2; reproduced from the wiki "Data Source Identifiers"):

  | `src` | source | frequency | provenance |
  |---|---|---|---|
  | `A` | ADS-B (1090 + 978 UAT) | 1090 MHz (978 if `cla` 8–13) | local |
  | `M` | Multilateration (JetVision MLAT network) | 1090 MHz (the locally heard Mode-S) | **network** |
  | `L` | OGN **Decoder** (locally received) | 868 MHz | local |
  | `F` | piAware / FlightAware MLAT (unobserved on current deployments) | 1090 MHz | **network** |
  | `S` | FLARM SkyLens FLARM | 868 MHz | local |
  | `D` | FLARM SkyLens ADS-L | 868 MHz | local |
  | `O` | OGN **Server Connection** (from the OGN network) | 868 MHz | **relayed** |
  | `?` / unlisted | undetermined | — | unknown |

  The `cla` subclass (when present: `0`=Mode-S, `8–13`=978 UAT, `14`=FLARM) refines the ADS-B frequency for `src=A`. *(Current firmware often omits `cla`; an ASM MUST NOT depend on it — default `src=A` to 1090 MHz when `cla` is absent.)* **Provenance semantics (CL-J4/J5):** `local` = detected **and** positioned by this box's own receivers; **`network`** = the aircraft **is locally heard** (the box decodes its Mode-S and contributes it to a cooperative MLAT network) but the **position** is computed server-side and returned; `relayed` = the box never heard the aircraft at all (pulled from a network feed). Only `relayed` is outside this sensor's coverage.
- **FR-A-M04** *(poll → stream via change-detection)*: Because the source is poll-only and largely repetitive, an ASM MUST convert it to a stream by **emitting a contact only when it has meaningfully changed** versus the last record emitted for that `hex`, deduping byte-for-byte / sub-resolution repeats. A change is "meaningful" when **any** of: identity/text fields (`fli`/`reg`/`typ`/`opr`/`cat`/`src`/`ava`/`gda`/`squ`) change; enum/bool fields (`alr`/`tcm`/`mop`/`nacp`/`spi`/`ape`) change; or a kinematic field crosses its quantum — **position ≥ 3 m**, **altitude (`alt`/`altg`) ≥ 25 ft**, **speed ≥ 1 kt**, **track ≥ 1°**, **vertical rate ≥ 64 ft/min** (thresholds sit at/below the sensor's own quantisation so no real motion is swallowed). Fields that would force an emit every poll — `uti`/`ns`, `dbm`, `tru`, `lla`, met, MLAT diagnostics — MUST **not** trigger a change on their own. There MUST be **no altitude biasing and no aircraft cap**: every meaningful change is forwarded and the HLDMM judges (no-suppression). The **comparison method** is implementation-defined provided the thresholds are honoured (the reference impl snaps each kinematic to its quantum — `round(v/q)` — and uses great-circle distance for position); two conforming implementations therefore need not produce byte-identical streams from the same poll sequence — the stream-shaping contract is the thresholds, not the algorithm. *(A deployment polling for inspection MAY also offer a snapshot-every-poll mode, but the on-lane default is change-detection.)*
- **FR-A-M05** *(track identity & eviction)*: Aircraft state MUST be keyed by the ICAO **`hex`** (uppercased/trimmed); each `hex` gets a **stable ULID `object_id`** minted once and held for the life of the track (FR-A-D02). A record with an empty `hex` yields no track and no report. A track not seen within an **eviction TTL** (reference `60s`, configurable) MUST be evicted (after emitting its lost-track report, FR-A-G01); an airframe reappearing after the TTL is a **new track** with a fresh ULID. *(Unlike 017's DS240, the `aircraftlist.json` record is a **complete per-aircraft snapshot** — there is no sub-message merge; `hex` is both the ingest key and the identity key.)*
- **FR-A-M06** *(observation time)*: The report time base MUST be the FPGA observation time `uti` (seconds) + `ns` (nanoseconds-within-second) → `SapientMessage.timestamp` (nanosecond resolution). When `uti` is absent or implausible (`≤ 0`) the ASM MUST fall back to its wall clock. *(The FPGA timestamp is strictly better than ASTERIX CAT021's time-of-applicability; preserving it is a JetVision advantage.)*
- **FR-A-M07** *(RF `signal` — both fields mandatory, deterministic per source)*: The DSTL proto marks **both** `Signal.amplitude` **and** `Signal.centre_frequency` mandatory, so a `signal` block MUST be emitted **only when both are available**: the frequency resolved **deterministically** from `(src, cla)` per FR-A-M03 — `A` → 1090 MHz (978 MHz when `cla ∈ 8–13`); **`M`/`F` → 1090 MHz** (the MLAT position is server-computed, but the *signal* the box measures is its own 1090 MHz Mode-S reception of that aircraft — the wiki source table assigns both to 1090; CL-J4); `L`/`O`/`S`/`D` → 868 MHz — **and** a reported level (`amplitude` from `dbm`). When the frequency does not resolve (an **unknown** source, `?`/unlisted) but `dbm` is present, the level MUST ride `adsb.signalDbm`; when `dbm` is absent, the `signal` block MUST be omitted even though the frequency resolves (it is recoverable from `adsb.source`) — never emit a one-field block, a fabricated frequency, or a fabricated `0 dBm`. **`centre_frequency` MUST be reported in Hz (SI)** — `MHz × 10⁶` (e.g. 1090 MHz → `1090000000`); SAPIENT v2.0 carries no unit enum on `Signal`, so SI Hz is the interoperable default (CL-J12).

**C. `aircraftlist` → DetectionReport mapping**

- **FR-A-D01**: The ASM MUST emit, per `aircraftlist` record with a **usable position**, exactly **one** `DetectionReport`. A record with no position (e.g. a Mode-S identity decoded before its first fix) MUST be withheld (FR-A-D09), not emitted with a null `location`.
- **FR-A-D02** *(object_id & track identity)*: `object_id` MUST be a **ULID** assigned by the ASM and held **stable per airframe track**, consistent with `tracking_type = TRACKING_TYPE_TRACK`. The **track key MUST be the ICAO `hex`**; the `hex` MUST also be carried verbatim as `adsb.icao24`. The native `DetectionReport.id` ("tail number") MUST be the registration `reg`, else the callsign `fli`, else omitted (CL-J14). *(No MAC-rotation problem as in 017 — `hex` is stable per airframe and is both ingest and identity key.)*
- **FR-A-D02a** *(report_id — mandatory, fresh ULID)*: Every `DetectionReport` MUST carry a `report_id` that is a **fresh ULID minted per report** — the DSTL proto marks `report_id` both `is_mandatory` and `is_ulid` ("ULID for the message"). It is distinct from the per-track `object_id` (FR-A-D02): `object_id` is stable for the life of the track, `report_id` is unique per emission. Any derived/composite form (e.g. `<hex>@<nanos>`) is **non-conformant**. *(ULID = the canonical 128-bit identifier of [github.com/ulid/spec](https://github.com/ulid/spec): 48-bit ms timestamp + 80-bit entropy, 26 Crockford-base32 chars — the same standard `object_id` uses.)*
- **FR-A-D03** *(native fields)*: The ASM MUST map: position → `location` (`X` = `lon`, `Y` = `lat`, `Z` = altitude; `coordinate_system = LAT_LNG_DEG_M`), preferring **geometric** altitude `altg` (`datum = WGS84_E`, ellipsoid) and falling back to **barometric** `alt` (`datum = WGS84_G`, geoid — closest-available label, CL-J13); `location.error` from NACp (FR-A-D03a) — a radial horizontal bound applied to **both** `x_error` and `y_error`; kinematics → `velocity_oneof` as **`ENUVelocity`** (E = `spd`·sin(`trk`), N = `spd`·cos(`trk`), U = `vrt`), all in SI m/s — emitted **only when both `spd` and `trk` are present**: the proto marks `east_rate`/`north_rate` mandatory, and fabricating `E = N = 0` for an aircraft whose speed/track dropped out would assert a hover. `vrt` fills the **optional** `up_rate` when present (an implementation MAY emit `up_rate = 0` or omit it when `vrt` is absent). When no native velocity can be emitted, present kinematics MUST still be carried (no-suppression) as **`adsb.groundSpeedKt`/`adsb.trackDeg`/`adsb.verticalRateFpm`** (Annex A / extension §3.5). Also: a `classification` per FR-A-A04; a `detection_confidence` per FR-A-D04; and, for sources resolving a frequency, an RF `signal` per FR-A-M07. Unit conversions are SI (kt → m/s ×0.514444, ft → m ×0.3048, ft/min → m/s ×0.00508).
- **FR-A-D03a** *(NACp → horizontal error)*: For cooperative sources the horizontal 95% bound MUST come from the ADS-B **NACp** enum via the table in Annex A.1 (`9`→30 m, `10`→10 m, `11`→3 m, … `1`→18520 m). **NACp `0`/absent = unknown ⇒ omit `location.error`** (never a false `0` bound). For **MLAT** use the synthesised bound of FR-A-D08 instead.
- **FR-A-D04** *(per-source confidence)*: `detection_confidence` and the `classification` confidence MUST reflect the source's nature: cooperative self-announced ADS-B (`A`) detects with certainty (≈1.0) and classifies well when a category is self-declared; FLARM/OGN (`L`/`O`/`S`/`D`) is nearly as strong; **MLAT (`M`, and `F` FlightAware MLAT) detects well (≈0.9 — a network solve from ≥3 receivers) but cannot self-declare a category, so its classification confidence is low (≈0.4)**; an unknown source is treated conservatively. The per-source values are tabulated in Annex A.3. None of this **filters** — a low confidence is emitted and the HLDMM judges (no-suppression).
- **FR-A-D05** *(provenance & coverage honesty)*: Every report MUST carry `adsb.provenance ∈ {local, network, relayed, unknown}` derived from `src` (Annex A.2; semantics in FR-A-M03). A SAPIENT ASM reports **its own coverage**, so **relayed** contacts (`src=O`, the OGN *Server Connection* — aircraft the box never heard) MUST be **excluded by default**; a deployment MAY re-admit them (reference `--include-relayed`). **`network` contacts (`M`/`F` MLAT) are emitted by default** — the aircraft is locally heard and inside this sensor's RF coverage; only its *position* was computed by the cooperative MLAT network this box contributes to. This is a **provenance/coverage** decision, distinct from data-quality filtering (FR-A-D07): excluded contacts are ones the box did **not** RF-observe, not ones with poor data.
- **FR-A-D06** *(adsb.* extension)*: The ASM MUST carry the remaining `aircraftlist` payload as `object_info` under the `neuron.adsb/1` namespace per **Annex A** (`neuron.adsb-1.extension.md` + schema, co-located & normative): source/availability/subclass (`adsb.source`/`availability`/`subclass`), identity & route (`adsb.callsign`/`registration`/`typeCode`/`operator`/`originIcao`/`destIcao`/`country`/`countryIso`/`emitterCategory`), both raw altitudes (`adsb.baroAltFt`/`geoAltFt`) and air/ground (`adsb.airGround`/`airGroundTisB`), surveillance status (`adsb.squawk`/`emergency`/`alert`/`spi`/`tcasMode`/`autopilotEngaged`), the integrity set (`adsb.nacp`/`sil`/`sda`/`pic`/`mopsVersion`/`trustCount`/`posAgeProxy`, and `adsb.signalDbm` only when no native `signal`), the kinematics fallback (`adsb.groundSpeedKt`/`trackDeg`/`verticalRateFpm`, only when no native `enu_velocity` — FR-A-D03), Mode-S Enhanced Surveillance (`adsb.selectedAltFt`/`selectedHeadingDeg`/`qnhHpa`), meteorological (`adsb.oatC`/`windSpeedKt`/`windDirDeg`), and MLAT diagnostics (`adsb.mlatPest`/`mlatNoclients`).
- **FR-A-D07** *(no-suppression)*: The ASM MUST emit every present field even when its value is `0` (faithful to the wire); the ONLY exceptions are accuracy enums where `0`=unknown (omitted, FR-A-D03a) and NaN/Inf (sanitised to `0`, never dropped). It MUST NOT filter contacts on **data quality** (low trust count, missing integrity, `0,0` position, unknown source all emit). The coverage exclusion of FR-A-D05 is a *provenance* decision, not a data-quality one, and is the only contact the default path withholds.
- **FR-A-D08** *(MLAT special-case — network position, local signal)*: For the network-position sources (`src=M`, and `F` when observed) the ASM MUST: (a) emit the native `signal` block with `centre_frequency` = 1090 MHz and `amplitude` = `dbm` — the box's **own** measurement of the aircraft's Mode-S transmissions (FR-A-M07 / CL-J4); (b) tag `adsb.provenance = network` (FR-A-D05); and (c) **synthesise** a conservative horizontal `location.error` — the position is a server-side multilateration solve carrying no NACp, and "unknown accuracy" there really means "known to be poor": a stated large bound is more honest to the HLDMM than none. The bound value is deployment-configurable with a **normative default of 300 m**; a deployment choosing another value MUST keep it conservative (≥ the typical MLAT solve error for its receiver geometry). MLAT diagnostics ride as `adsb.mlatPest`/`adsb.mlatNoclients`.

**D. Position-relaxed handling**

- **FR-A-D09**: An identity-only / position-less contact MUST NOT be emitted on the assembled `/sapient/detection/2.0.0` stream (where 015 FR-S35 requires `location_oneof`). The JetVision ASM, which is single-service (`adsb` = assembled), MUST **withhold** the report until a position is available (default), OR (if a deployment opts in) advertise a **separate** position-relaxed service per 015 FR-S35. *(ADS-B normally gets a position quickly; a position-less Mode-S contact is rare and transient. Note: raw Mode-S time-of-arrival for MLAT is a **BEAST** feed, not a position-relaxed SAPIENT one — Out of Scope, 015 CL-16.)*

**E. Lifecycle, tasking, fan-out, health** — all per **015** (FR-S10–S32). This spec adds no new session mechanics. Tasking the ASM (region-of-interest via `region_definition`, class via `class_filter_definition`) uses 015 FR-S25/26; the `adsb` service accepts **no** `detection_report_rate` decimation (CL-J7, kept parallel to 017). An altitude-band task would use the 015 FR-S26a `object_info` extension filter — **not yet defined for `neuron.adsb/1`** (pending, CL-J11); until it is declared, the ASM MUST reject an altitude-band `Task` per FR-S26a.

- **FR-A-D10** *(mandatory stream STOP for the `adsb` service — binds 015 FR-S29)*: The ASM MUST declare `CONTROL_STOP` + `CONTROL_START` as accepted per-session controls in the `adsb` service's `mode_definition.task` and honour them per 015 FR-S29. On a `Task{CONTROL_STOP}` it MUST **cease emitting `DetectionReport`s** (including lost-track reports, FR-A-G01) on `/sapient/detection/2.0.0` for that session within its declared cease window and MUST `TaskAck` on the auditable lane; it MUST NOT reject a well-formed STOP, and a STOP from one consumer MUST NOT interrupt any other consumer's stream. `CONTROL_START` resumes emission to that session **without** a new SAPIENT `Registration`. The ASM keeps polling and tracking internally while stopped — STOP gates *emission to that consumer*, not the sensor's reception — so resume is immediate. If the `adsb` service is billed per-`DetectionReport` (an 008/deployment choice), a STOP halts billing to that consumer for the stopped interval. A consumer of `adsb` MUST be able to issue STOP and process the `TaskAck` (015 FR-S29 buyer obligation) and SHOULD STOP before disconnecting.

**F. Lost-track**

- **FR-A-G01** *(lost-track final report — normative)*: When a track is evicted (no sighting within the eviction TTL, FR-A-M05), the ASM MUST emit exactly **one** final `DetectionReport` for that `object_id` with the native field **`DetectionReport.state = "lost"`** and the **last-known `location`** (so the report still satisfies the mandatory `location_oneof`, 015 FR-S35) and the last-known `object_info`, then drop the track. No further report for that `object_id` follows until a fresh detection mints a new ULID. The lost report follows the same per-session emission gate as ordinary reports (it is suppressed to a STOPped session, FR-A-D10). *(The reference build currently TTL-evicts silently; this requirement exceeds the current build — see Mapping M1.)*

**G. Feed-source variations (TEVV)**

- **FR-A-E01**: The ASM's data plane MUST be produced from exactly one of `live` (real Air!Squitter poll), `replay` (recorded `aircraftlist.json` capture), `synthetic` (deterministic generator), or `placeholder` (valid record, deterministic stand-in). Default for production = `live`.
- **FR-A-E02**: The ASM MUST advertise its `feedSource` in its SAPIENT `StatusReport` (sensor health/coverage/mode, on the auditable lane) — **not** in the separate Neuron 005 liveness heartbeat (015 FR-S31). **Carrier:** SAPIENT v2.0 `StatusReport` has **no** `object_info`-style extension slot, so `feedSource` MUST ride as a **native repeated `StatusReport.status[]` entry** — `status_type = STATUS_TYPE_OTHER`, `status_level = STATUS_LEVEL_INFORMATION_STATUS`, `status_value = "neuron.feedSource=<live|replay|synthetic|placeholder>"` — strictly-conformant SAPIENT needing **no** extension declaration; a consumer that does not recognise the namespaced value ignores it. Absent ⇒ `live`. A validator (010) observing `feedSource ≠ live` MUST treat the run as fixture evidence and disclose the value. *(Same generic `status[]` carrier as 017 FR-R-E02.)*

**H. Errors**

- **FR-A-F01**: JetVision-specific errors MUST use a `NEURON-DAPP-ADSB-*` domain (006): at minimum `AircraftListMalformed` (invalid JSON array body), `NoUsablePosition` (informational; produces no report — FR-A-D01), `PollFailed` (HTTP error/timeout — **non-fatal**, the interval is skipped and retried, FR-A-M01), and `UnknownSource` (a `src` not in the FR-A-M03 table — informational; `adsb.provenance = unknown`, the contact still emits). The skip-class ingest codes (`PollFailed`) are non-fatal and MUST NOT abort the stream. Session/profile errors use the 015 `NEURON-SAPIENT-*` domain.

### Key Entities

- **JetVision ASM** — a JetVision Air!Squitter / Radarcape node, one SAPIENT identity, one `Registration`, one service `adsb`; `NodeType = PASSIVE_RF`; multi-source (1090/978/868/MLAT from one `aircraftlist.json`).
- **DetectionReport (aircraft)** — the single per-aircraft product; ULID `object_id` (per `hex`), `adsb.icao24` = the hex, native `id` = registration/callsign, emitter-category classification, per-source signal/confidence/provenance.
- **`neuron.adsb/1`** — the `object_info` extension namespace (source/availability, identity & route, altitudes, surveillance status, the ADS-B integrity set, EHS selected values, met, MLAT diagnostics). Defined by **Annex A**: `neuron.adsb-1.extension.md` + `neuron.adsb-1.schema.json` (co-located, normative).
- **Aircraft record (input)** — one complete per-aircraft snapshot from `aircraftlist.json` (no sub-message merge); `hex` is both the ingest and identity key.

## Mapping — reconciliations with the reference implementation *(normative over `detection.go`)*

Unlike 017's `detection.go` (written before the verified DSTL v2.0 protos and the single-report model), the `neuron-jv-bridge` reference impl was written **against** the verified protos, so it conforms to most of the native-field mapping (ULID `object_id` per hex; `Location.{y,x,z}` + WGS84_E/_G datum; `ENUVelocity`; recursive `classification.sub_class`; Hz `centre_frequency`; `adsb.*` `object_info`). The reconciliations are the spec requirements that exceed or **correct** the current build:

| # | Reference impl. (current build) | This spec (normative) | Rationale |
|---|---|---|---|
| M1 | TTL-evicts a stale track **silently** | a final `DetectionReport` with native **`state = "lost"`** on eviction (FR-A-G01) | the HLDMM should be told a track ended, not infer it from silence; `state` is the native slot ("e.g. object lost") |
| M2 | emergency carried as `adsb.emergency`/`adsb.squawk` only | **kept** (pass-through) — native `Alert` on emergency-transition is **deferred** (CL-J10) | MVP parity; `Alert` needs edge-detect + de-bounce design before it is normative |
| M3 | `--include-relayed` default **off**; `adsb.provenance` tagged | **normative** (FR-A-D05): relayed excluded by default, provenance always tagged | "an ASM reports its own coverage" — promote the impl default to a requirement |
| M4 | MLAT (`src=M`) tagged `provenance = local`; native `signal` **omitted** ("no single carrier"), `dbm` → `adsb.signalDbm` | `provenance = **network**`; native **`signal{1090 MHz, dbm}` emitted** (FR-A-D08/FR-A-M07) | the MLAT *position* is computed by the JetVision MLAT server (`jetvision.de`, ≥3 receivers) — neither local nor relayed — while the *signal* is the box's own 1090 MHz Mode-S reception (the wiki source table assigns `M` → 1090 MHz); the original impl predated this architecture correction (CL-J4); **the reference impl was fixed in lockstep on 2026-06-11** — deployed older binaries may still show the old behaviour |
| M5 | `report_id = "<hex>@<unixnano>"` (derived composite) | `report_id` = **fresh ULID per report** (FR-A-D02a) | the proto marks `report_id` mandatory + `is_ulid`; a composite string is not a ULID; **impl fixed in lockstep 2026-06-11** |
| M6 | `ENUVelocity` emitted with fabricated `E=N=0` when only `vrt` known; `signal` emitted frequency-only when `dbm` absent | native blocks emitted **only when their proto-mandatory fields are honestly fillable**: velocity needs `spd`+`trk` (partials → `adsb.groundSpeedKt`/`trackDeg`/`verticalRateFpm`); signal needs `dbm` + a resolved frequency (FR-A-D03/FR-A-M07) | `east_rate`/`north_rate` and `amplitude` are `is_mandatory` in the proto; zeros/one-field blocks fabricate data (a hover claim, an empty signal); **impl fixed in lockstep 2026-06-11** |

Everything else in `detection.go` / `classify.go` / `wire.go` is an informative realisation of FR-A-D03…D08 + Annex A; where any of it conflicts with this spec, **the spec governs** (015 FR-S41).

## Encoding on the wire & inspection view (reference ASM)

Encoding is owned by **015 FR-S50** (Protocol Buffers normative; canonical-JSON projection OPTIONAL, reconcilable field-for-field with the `.proto`). This section records how the JetVision reference ASM realises it — the DApp-specific concretes, not new policy.

- **Wire (default).** A `DetectionReport` is carried inside a SAPIENT `SapientMessage` envelope (`timestamp` + `node_id` (UUID) + `content.detection_report`) and serialised as **BSI Flex 335 v2.0 Protocol Buffers**, framed with a **4-byte little-endian length prefix** on the conformant SAPIENT connection (the edge to the Seller Proxy, 015 FR-S91) — the framing the DSTL test harness and Apex middleware read and write (SAPIENT ICD §2.1.2; `struct.pack("<I", …)`). Go bindings are generated from the vendored DSTL protos.
- **Inspection view.** `--sapient-format=json` (or a parallel `--sapient-json-listen` port) emits the OPTIONAL canonical-JSON projection (015 FR-S50) as `protojson` of the **same** `SapientMessage` — so what you inspect is exactly the wire message, never a parallel hand-rolled shape. Default is `protobuf`.
- **Field mapping (on top of the *Mapping — reconciliations* table).** `lat/lon/altitude → Location.{y,x,z}` with `coordinate_system = LOCATION_COORDINATE_SYSTEM_LAT_LNG_DEG_M`; `datum = LOCATION_DATUM_WGS84_E` for geometric (`altg`) altitude, `LOCATION_DATUM_WGS84_G` for barometric (`alt`); the NACp/MLAT horizontal bound → **both** `Location.x_error` and `y_error`; `spd`/`trk`/`vrt` → `ENUVelocity.{east_rate,north_rate,up_rate}`; `cat` → recursive `classification.sub_class` (each level numbered); the `adsb.*` extension → `object_info` (`TrackObjectInfo{type,value}`). **`TrackObjectInfo` has no `units` field**, so `adsb.*` units are static-per-key (declared in the `neuron.adsb/1` schema) and surfaced only in the JSON view. `signal.centre_frequency` is reported in **Hz** (the proto field is `float32`, so a ~GHz value is ~hundreds-of-Hz precise — fine for an RF centre frequency; compare with tolerance in tests). `state = "lost"` (FR-A-G01) sets the native `DetectionReport.state` field.
- **`node_id`.** The envelope's mandatory `node_id` UUID is the ASM's SAPIENT identity. The Neuron-blind translator MAY set any local UUID (the reference bridge sets it from `--node-id`); the **Seller Proxy binds the on-lane `node_id` to its EIP-8004-bound PeerID** (015 FR-S94), so a consumer observes the crypto-bound network identity regardless of the translator's local value.

**Verification.** A captured frame decodes with stock `protoc --decode=sapient_msg.bsi_flex_335_v2_0.SapientMessage` against the DSTL v2.0 schema into a `SapientMessage` whose `detection_report` carries a ULID `object_id` and a fresh ULID `report_id` (FR-A-D02a), `adsb.icao24` = the hex, an `ENUVelocity` (when `spd`+`trk` are present), a `Signal` with a Hz `centre_frequency` (omitted when the source is unknown or `dbm` is absent), and the `adsb.*` `object_info`; the `--sapient-format=json` projection of the same run is field-identical (VR-A-05).

## Success Criteria *(mandatory)*

- **SC-A01**: A JetVision ASM (replay feed) and a stub HLDMM complete `Registration(adsb)`→`RegistrationAck`→assembled `DetectionReport` stream over Profile F, each report with a ULID `object_id` **and a fresh ULID `report_id`** (FR-A-D02a), `adsb.icao24` = the hex, a populated `location` (with error or a synthesised MLAT bound), an `ENUVelocity` (when `spd`+`trk` are present), and an emitter-category `classification`.
- **SC-A02**: A consumer parses every `adsb.*` field using only the registry-declared `neuron.adsb/1` schema, zero JetVision-specific code.
- **SC-A03**: Across a poll containing all source types, each report's `signal.centre_frequency` matches its source (A→1090 MHz, UAT `cla`8–13→978 MHz, MLAT→1090 MHz, FLARM/OGN→868 MHz, all in Hz), MLAT carries `adsb.provenance = network` and a ~300 m synthesised `location.error`, and `adsb.provenance` is correct per `src` for every source.
- **SC-A04**: With default settings a relayed (`src=O`) contact is **dropped** and a local one is emitted; with `--include-relayed` both are emitted and the relayed one carries `adsb.provenance = relayed`.
- **SC-A05**: Across a test set (every emitter category, `0,0` positions, low trust counts, missing integrity, every NACp enum), zero contacts are dropped on data-quality grounds; NACp→bound is exact (9→30 m, 10→10 m, 11→3 m).
- **SC-A06**: A track that ages out yields exactly **one** final `DetectionReport` with `state = "lost"` carrying the last-known `location`; none follow for that `object_id`.
- **SC-A07**: A SAPIENT `StatusReport` advertises a valid `feedSource`; a non-`live` run is tagged fixture-evidence by a 010 validator.
- **SC-A08**: A streaming `adsb` session receives `Task{CONTROL_STOP}`; the ASM `TaskAck`s and reports (incl. lost-track) cease on `/sapient/detection/2.0.0` within the declared cease window; `CONTROL_START` resumes with no re-`Registration`; a second concurrent consumer's stream is unaffected; the agent-card declares `CONTROL_STOP`/`CONTROL_START` in the `adsb` `mode_definition.task` (015 FR-S29 / FR-A-D10).
- **SC-A09**: The JetVision ASM interoperates with 018 (HLDMM) over 015 with no 015-level change — only the `adsb` service + `neuron.adsb/1` differ from another vendor.

## Evidence & Validation *(per Principle XI)*

**Verification Tier**: `topic-observable` for registration/tasking/status (004 via 015); `proof-required` for the in-stream `DetectionReport` data plane (private 009 QUIC).

**Observable Signals**: registry agent-card (003) with `config_data` = JetVision/Air!Squitter, the `adsb` service, and `neuron.adsb/1`; auditable-lane `Task`/`TaskAck`/`StatusReport`; the `/sapient/detection/2.0.0` protocol ID advertised in the 009 catalog; `feedSource` in `StatusReport`s.

**Evidence Rules (`VR-A-*`)**: **VR-A-01** agent-card carries the `adsb` service + vendor identity + resolvable `neuron.adsb/1`. **VR-A-02** observe `feedSource` (∈ enum or absent⇒live); record verbatim. **VR-A-03** (data-plane, proof-required) DetectionReports on the negotiated stream carry a ULID `object_id` + `adsb.icao24` + `adsb.provenance`. **VR-A-04** invoice cadence: ADS-B is continuous in daylight/busy airspace; a quiet period (no aircraft in range) yielding no invoice is **inconclusive**, not non-compliant. **VR-A-05** the `--sapient-format=json` projection is field-identical to the decoded protobuf frame (encoding non-drift).

**Non-Observable Areas**: report contents (aircraft positions) are off-chain — a validator verifies `evidenceHash` linkage, not ground truth; whether a `feedSource=live` claim is truthful is a disclosure-discipline matter, not a runtime check. ADS-B/Mode-S broadcasts are unauthenticated — the ASM conveys *integrity* (NACp/SIL/SDA + the synthesised MLAT bound) but never asserts authenticity (US4).

## Layering Compliance Check *(per Principle XII as-amended)*

- ✅ DApp tier: composes 015 (Shared Application Profile) + Core; redefines neither. Declares only JetVision specifics (service `adsb`, the air/surface/UAV taxonomy, `neuron.adsb/1`, the `aircraftlist`→`DetectionReport` mapping).
- ✅ No CoT mapping or affiliation policy here (018); no fusion (019).
- ✅ No SAPIENT session mechanics restated — all referenced to 015.
- ✅ Satisfies **015 FR-S41** (explicit conversion mapping): the `aircraftlist`→`DetectionReport` conversion is specified normatively and is buildable without the reference code — §B ingest (the field/source/subclass tables), §C mapping, Annex A value domains + NACp/category/confidence tables (air-interface byte layout referenced to DO-260B/ED-102A et al., FR-A-M02).
- ✅ Discharges **015 FR-S70 / FR-S95**: the SAPIENT/BEAST identity relationship for a JetVision node is settled here (Out of Scope — one identity per protocol; raw Mode-S is a separate non-SAPIENT BEAST feed).
- ✅ Vendor-named, count-agnostic (sensor inventory = deployment config).
- ℹ️ Depends on the Principle XII three-tier model (defined in 015 and its `amendments/`).

## Superseded Artifacts

- No prior `016` artifact exists in this repository (this directory is created fresh). The bespoke `BeastFrame`/fusion-at-display concept referenced in 015's founding notes is **superseded** by this SAPIENT formulation, exactly as 017 superseded the bespoke `RemoteIdFrame`.
- `plan.md`, `tasks.md`, and a conformance checklist are to be generated against this specification (`/speckit.plan` + `/speckit.tasks`), gated on the same Principle XII amendment as 015/017.

## Annex A — `neuron.adsb/1` extension *(normative)*

Promoted into this spec so the `aircraftlist`→SAPIENT translation is buildable **without the reference application**. Two co-located, normative artifacts carry the full extension:

- **`neuron.adsb-1.extension.md`** — the `adsb.*` value domains, the source/provenance table, the NACp→metres table, the emitter-category→classification table, and the per-source confidence table.
- **`neuron.adsb-1.schema.json`** — machine-readable JSON Schema; `adsb.*` `object_info` MUST validate against it.

**Precedence.** These artifacts are normative for the `adsb.*` **value domains, frequency/provenance derivation, NACp bounds, and confidence weighting**. Should any conflict arise, **this `spec.md` governs**.

### A.1 NACp enum → horizontal bound *(reproduced; the numbers most at risk if the reference repo is deleted)*

ADS-B reports position accuracy as the NACp enum; map each to its **upper 95% horizontal bound** (EPU radius, metres). **Code `0`/absent = unknown ⇒ omit `location.error`** (never a false `0`). MLAT uses the synthesised bound (A.2), not this table.

`1`=18520, `2`=7408, `3`=3704, `4`=1852, `5`=926, `6`=555.6, `7`=185.2, `8`=92.6, `9`=30, `10`=10, `11`=3 (m).

### A.2 Source → frequency & provenance *(reproduced from §B / FR-A-M03)*

| `src` | source | `centre_frequency` | `adsb.provenance` |
|---|---|---|---|
| `A` | ADS-B (1090 / 978 UAT) | 1090 MHz (978 if `cla` 8–13) | `local` |
| `M` | Multilateration (JetVision MLAT network) | 1090 MHz (locally heard Mode-S; ~300 m synthesised error) | **`network`** |
| `L` | OGN Decoder (local) | 868 MHz | `local` |
| `F` | piAware / FlightAware MLAT (unobserved) | 1090 MHz (~300 m synthesised error) | **`network`** |
| `S` | FLARM SkyLens FLARM | 868 MHz | `local` |
| `D` | FLARM SkyLens ADS-L | 868 MHz | `local` |
| `O` | OGN Server Connection (network) | 868 MHz | **`relayed`** (excluded by default, FR-A-D05) |
| `?` / unlisted | undetermined | — (omit `signal`) | `unknown` |

Frequencies are reported on the wire in **Hz** (`MHz × 10⁶`, CL-J12). Provenance semantics (FR-A-M03): `local` = detected + positioned by this box; `network` = locally heard, position computed by a cooperative MLAT network; `relayed` = never heard by this box. **Only `relayed` is excluded by default**; `local`/`network`/`unknown` contacts are emitted.

### A.3 Emitter category → classification & per-source confidence

**Category → `{top-level type, sub_class}`** (FR-A-A04): `A0`→`{Air Vehicle, Unknown}`, `A1`→`{Air Vehicle, Light}`, `A2`→`{Air Vehicle, Small}`, `A3`→`{Air Vehicle, Large}`, `A4`→`{Air Vehicle, High-Vortex Large}`, `A5`→`{Air Vehicle, Heavy}`, `A6`→`{Air Vehicle, High-Performance}`, `A7`→`{Air Vehicle, Rotorcraft}`, `B1`→`{Air Vehicle, Glider/Sailplane}`, `B2`→`{Air Vehicle, Lighter-than-Air}`, `B3`→`{Air Vehicle, Parachutist/Skydiver}`, `B4`→`{Air Vehicle, Ultralight/Hang-glider/Paraglider}`, **`B6`→`{UAV, Unmanned Aerial Vehicle}`**, `B7`→`{Air Vehicle, Space/Transatmospheric}`, `C1`→`{Surface Vehicle, Emergency Vehicle}`, `C2`→`{Surface Vehicle, Service Vehicle}`, `C3`→`{Surface Vehicle, Point Obstacle}`; FLARM `F<n>`→`{Air Vehicle, "FLARM F<n>"}`; absent/unknown→`{Air Vehicle, <raw>}` (total, never failing).

**Per-source confidence** `{detection, classification}` (FR-A-D04): `A` (ADS-B) → `{1.0, 0.95}` with a self-declared category, `{1.0, 0.50}` without; `L`/`O`/`S`/`D` (FLARM/OGN family) → `{0.95, 0.85}` / `{0.95, 0.50}`; **`M`/`F` (MLAT — JetVision / FlightAware network solve) → `{0.90, 0.40}`** (detects well, cannot self-classify); `?` (unknown) → `{0.80, 0.40}` / `{0.80, 0.30}`. These weight, never filter (no-suppression).

### A.4 Value domains

The per-field `adsb.*` value domains (`adsb.source`, `adsb.provenance`, `adsb.airGround`, `adsb.emergency`, the boolean keys, and the numeric keys) are enumerated in `neuron.adsb-1.extension.md` and constrained by `neuron.adsb-1.schema.json`. An HLDMM MUST interpret `adsb.*` values using only those artifacts (declared in the agent-card `extensions[]`, 015 FR-S40) — never hard-coded vendor knowledge.
