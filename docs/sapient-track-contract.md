# SapientTrackSnapshot contract — v1.1.0

**Producer**: `cmd/sapient-fid-consumer` (`--sapient-output`) · **Consumer**: `cmd/sapient-fid-display`
· **Status**: local SAPIENT demo (additive; sibling of `docs/normalized-track-contract.md`)

The rich SAPIENT display projection. One JSONL record per DetectionReport, wrapped in the standard
TaggedFrame envelope and sent to `cmd/sapient-fid-display`'s **own** TCP port (default `127.0.0.1:19194`)
— never into the legacy `cmd/fid-display` stream, which stays remote-id-only and **unmodified**.

```
{"source":"sapient","type":"track","sellerPeerID":"","receivedAt":"<RFC3339>","frame":{ <SapientTrackSnapshot> }}
```

`sellerPeerID` is empty by design: the generic Buyer Proxy strips the Neuron envelope (FR-S90/S93); the
consumer is Neuron-blind on the data path. Seller identity arrives instead via the **agent** block below
(sourced from the EIP-8004 evidence file, a registration artefact — not from parsing the stream).

## §1 Identity and keying

| Field | Source | Rule |
|---|---|---|
| `uid` | `DetectionReport.id` else `object_id` | REQUIRED. Display keys tracks by the **composite** `nodeId\|uid` (v1.1.0 multi-source rule — see below); the bare uid remains the per-source identity |
| `objectId` | `DetectionReport.object_id` | optional — ULID track id |
| `reportId` | `DetectionReport.report_id` | optional |
| `nodeId` | `SapientMessage.node_id` | optional — the seller's identity-bound node_id (re-stamped at the proxy boundary, FR-S94) |

**Composite keying (v1.1.0).** With two SAPIENT sources feeding one buyer (DroneScout RID + JetVision
ADS-B), two sellers can legitimately mint colliding `uid`/`object_id` values. The display therefore keys
its track map by `nodeId + "|" + uid`; a frame without a `nodeId` (legacy single-source producer) keeps
the bare `uid` key — exact pre-v1.1.0 behavior. Never key tracks by object identity alone.

## §2 Blocks (ALL optional — absent ≠ zero)

Every block is omitted when the source data is absent. The display stores `nil` and the UI renders "—";
**missing optional fields must never break state or UI**.

| Block | Fields | Source |
|---|---|---|
| `position` | `lat, lon, alt` | `Location` (proto Y=lat, X=lon, Z=alt) — required for a map marker; tracks without it still apply to state |
| `velocity` | `speedMps, trackDeg` | `enu_velocity` → `hypot(e,n)`, `atan2(e,n)` bearing; `trackDeg:0` = true north (a real heading, not a default) |
| `classification` | `type, confidence` | `classification[0]`; confidence falls back to `detection_confidence` |
| `rid` | `serial, uasId, idType, uaType, macAddress, status, operatorId, operatorIdType, operatorLat, operatorLon, operatorAltM` | `object_info` `rid.*` keys. Operator lat/lon are set ONLY when both parse (half-populated operators suppressed). The display derives the **operator/pilot marker** from these fields |
| `rf` | `rssiDbm, frequencyHz, channel, transport` | `signal[0].amplitude` / `signal[0].centre_frequency` (float32 wire → ~64 Hz quantization at 2.4 GHz); RSSI falls back to `rid.rssiDbm` (CL-R4 BLE channel-0 case); channel/transport from `rid.*` |
| `cot` | `uid, type, how, affiliation, demoProfile` | the consumer's own CoT projection options (`cotadapt.Normalize`); see §3 |
| `agent` | `agentId, sellerEVM, peerID, nodeId, service, protocol, simulated` | the seller's agent-evidence file (`--agent-evidence`, lazily loaded — the seller may start after the consumer). Whole block absent until the evidence loads |
| `feedSource` | top-level string | evidence `feedSource` (live\|replay\|synthetic\|placeholder, 017 FR-R-E02) |
| `wire` | top-level string | always `"BSI Flex 335 v2.0 protobuf"` — the canonical wire label |
| `kind` | top-level string (v1.1.0) | `"adsb"` for JetVision aircraft tracks; **omitted** on rid tracks (legacy ⇒ rid). Drives icon/badge selection |
| `adsb` | `icao24, callsign, registration, typeCode, operator, originIcao, destIcao, country, emitterCategory, squawk, emergency, airGround, source, provenance, signalDbm, baroAltFt, geoAltFt` (v1.1.0) | `object_info` `adsb.*` keys (`neuron.adsb/1` extension from `neuron-jv-bridge`). Present only when `kind="adsb"` |

## §2a Modality routing (v1.1.0)

A DetectionReport whose `object_info` carries any `adsb.*` key is an **ADS-B track**: `version:"1.1.0"`,
`kind:"adsb"`, the `adsb` block present, and the `rid` + `cot` blocks **omitted** (the CoT XML stream is rid-only by contract). Such reports are also **excluded from the legacy
`source="remote-id"` projection** — an aircraft must never render as a drone. A report without `adsb.*`
keys is a rid track and serializes **byte-identically to v1.0.0** (`version:"1.0.0"`, no `kind`/`adsb`
keys). Auto-focus selection prefers rid (drone) tracks; aircraft positions drive focus only when no drone
is on the map.

## §3 Honest-labeling rules (normative for any UI rendering this contract)

1. **FRIENDLY is a demo display choice.** `cot.demoProfile=true` means the affiliation came from the
   operator's `--cot-affiliation friendly` flag — render it labelled as a demo CoT profile, never as an
   assessment. The library default stays unknown (`a-u-A`), never hostile.
2. **SIM means in-memory registry.** `agent.simulated=true` = the EIP-8004 registration was minted on the
   local `MemoryRegistryContract`; render an explicit `EIP-8004 SIM` label (vs `ON-CHAIN` when false).
3. **The canonical wire is SAPIENT protobuf.** This snapshot — like the remote-id projection — is a lossy
   display projection; render the wire label and do not present the JSON as the wire format.

## §4 Compatibility

- The legacy `source="remote-id"` projection to `cmd/fid-display` is emitted unchanged regardless of
  `--sapient-output` — the rich path is additive and the old FID never sees `source="sapient"`.
- `cmd/sapient-fid-display` accepts ONLY `source="sapient"` + `type="track"`; other lines are logged and
  skipped (it is not a generic FID).
- Unknown fields in the frame are ignored on decode (forward-compatible); producers may add fields in
  minor versions. Removing or re-typing a field is a major bump.
