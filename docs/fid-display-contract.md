# FID Display Contract — ADS-B + Remote ID TaggedFrame Display

> **Authored**: 2026-05-08
> **Purpose**: document the buyer → FID display contract (originally ADS-B-only) so the dual-stream extension (ADS-B + Remote ID) is a deliberate additive change, not a discovery.
> **Sibling**: the SAPIENT-rich display (`cmd/sapient-fid-display`, HTTP `:8193`, TCP in `:19194`) consumes its **own** sapient-track stream per `docs/sapient-track-contract.md` — it never shares this contract's TCP stream (`:9090`/`:19191`).

## Current state (ADS-B only)

### Transport

The buyer (`cmd/edge-buyer`) ships an aggregated stream to a configurable sink controlled by `BuyerConfig.OutputSink` (parsed by `internal/edgeapp/output.go` `NewOutputSink`). Four transport options exist today:

| Spec | Sink | Behavior |
|------|------|----------|
| `stdout` (or empty) | `StdoutJSONLSink` | JSONL to `os.Stdout`; one record per line; `\n` terminator. Stdout is line-buffered by default in most environments, so `tail -f` works. |
| `file:/path/x.jsonl` | `FileJSONLSink` (truncate on open) | Same JSONL, written to a file. The file is truncated when the buyer starts. |
| `file+:/path/x.jsonl` | `FileJSONLSink` (append) | Same JSONL, appended to an existing file (or created). |
| `tcp:host:port` | `TCPSink` | Reconnecting TCP socket; one JSONL record per write; `\n` line terminator. |

The React/FID display in production consumes one of these. Most likely deployment: the buyer writes `tcp:127.0.0.1:<port>` and the display process reads, optionally proxying over a websocket to the browser-side React app. The transport between the buyer and the display is **TCP carrying JSONL**, not a websocket directly from the Go process.

The current code does not include any websocket sink; if the React app speaks websocket natively, there is a separate (Node / nginx / custom) bridge between the buyer's TCP sink and the browser.

### Message schema (one JSONL line = one record)

The on-the-wire schema is `internal/edgeapp/aggregated.go` `AggregatedFrame`:

```json
{
  "sellerEVM": "0x1234567890abcdef1234567890abcdef12345678",
  "sellerName": "jv-london",
  "sellerPeerID": "12D3KooWAbc...",
  "frame": {
    "Raw": "BEAST-BYTES-AS-BASE64-OR-HEX",
    "SecondsSinceMidnight": 12345,
    "Nanoseconds": 678900000,
    "Rx": "2026-05-08T14:23:45.6789Z"
  },
  "meta": {
    "DF": 17,
    "ICAO24": "abc123"
  },
  "receivedAt": "2026-05-08T14:23:45.6790Z"
}
```

Field-by-field:

| Field | Type | Source | Units / notes |
|-------|------|--------|---------------|
| `sellerEVM` | string (0x-prefixed 40-hex) | derived from seller PeerID | EIP-55 not enforced; mixed-case acceptable |
| `sellerName` | string | `SellerEntry.DisplayName`, or first 4 hex of EVM | human-readable label |
| `sellerPeerID` | string (`12D3KooW...`) | libp2p multihash | cross-references libp2p logs |
| `frame.Raw` | bytes (Go default JSON encoding: base64 for `[]byte`) | upstream BEAST receiver | the raw Mode-S frame payload (7 or 14 bytes) |
| `frame.SecondsSinceMidnight` | uint64 | upstream BEAST 6-byte GPS timestamp | UTC seconds since midnight, range [0, 86399] |
| `frame.Nanoseconds` | uint64 | upstream BEAST GPS sub-second | nanoseconds within the second |
| `frame.Rx` | RFC3339Nano string | buyer-side dequeue time | when the buyer pulled the frame off the libp2p stream |
| `meta.DF` | int | Mode-S downlink format byte | range [0, 31]; best-effort decode |
| `meta.ICAO24` | string (6 lowercase hex) | extracted from DF 11/17/18 | empty when not extractable |
| `receivedAt` | RFC3339Nano string | buyer-side enqueue time | distinct from `frame.Rx` by goroutine scheduling latency only |

**Rate**: one record per ADS-B detection. A busy JetVision unit emits 50–100 Mode-S detections per second in dense traffic; quiet airspace produces a few per second.

**Ordering**: not guaranteed across sellers (multiple seller goroutines feed the same sink concurrently). Within one seller's stream, ordering matches the order frames arrived on the libp2p stream (FIFO).

### Sink concurrency

`OutputSink` implementations MUST be safe to call from multiple goroutines (the buyer holds one goroutine per seller). All three current implementations serialize via an internal `sync.Mutex`.

## Planned dual-stream extension (Phase 5)

The buyer needs to forward BOTH ADS-B BeastFrames AND Remote ID RemoteIdFrames to the same sink with source tagging. The proposal:

1. Introduce a new envelope type `BuyerOutputMessage` with an explicit `source` discriminator:
   ```json
   {
     "source": "adsb",
     "data": { ...existing AggregatedFrame fields... }
   }
   ```
   ```json
   {
     "source": "remote-id",
     "data": { ...RemoteIdFrame canonical-JSON per 017 FR-R05... }
   }
   ```
2. Keep the existing JSONL-on-TCP transport unchanged — line-delimited JSON; one record per line; reconnecting TCP.
3. Per-stream payload schemas:
   - ADS-B: the existing `AggregatedFrame` shape (unchanged).
   - Remote ID: `{sellerEVM, sellerName, sellerPeerID, frame: RemoteIdFrame, receivedAt}` where the `frame` field carries the canonical-JSON RemoteIdFrame envelope per 017 FR-R05. (The decoded fields `droneId`, `position`, `velocity`, `operator`, etc. are inside `frame`, mirroring how ADS-B's BEAST bytes live inside `frame.Raw`.)
4. Backward compatibility: existing display consumers that parse the legacy shape (without a top-level `source` field) MUST receive ADS-B records in the legacy shape until the React/FID display has been updated to handle the new envelope. The buyer can emit the new envelope behind a config flag (`BuyerConfig.OutputEnvelope = "v1-legacy" | "v2-tagged"`) during the transition window; once the display is updated, switch the flag to `v2-tagged` permanently.


### Standalone display (2026-05-08)

A minimal standalone display ships in-repo so the contract is exercisable without any external frontend.

**Binary**: `cmd/fid-display/` (Go-only, single-file binary).

- **Transport**: TCP listener — `cmd/multistream-buyer --output=tcp:127.0.0.1:9090` dials in; the display accepts the consolidated TaggedFrame JSONL stream.
- **Render path**: embedded HTML page served at `http://127.0.0.1:8080/` using Leaflet over OpenStreetMap tiles; live updates streamed via Server-Sent Events on `GET /events`; current snapshot also available via `GET /state.json` (used by tests and by the operator for non-browser verification).
- **Tagged envelope**: the display consumes the **v2-tagged** shape directly (`{source, type, sellerPeerID, receivedAt, frame: ...}`). Phase 2 streams only `source = "remote-id"`; the same display will accept `source = "adsb"` in Phase 5 with no display-side code change (color-coded marker layer per source already wired).
- **End-to-end run**: `go run ./cmd/fid-display` (terminal 1) + `go run ./cmd/remoteid-seller --synth --synth-fps=2 --synth-drones=3 --advertise-basestation-protocol` (terminal 2) + `go run ./cmd/multistream-buyer --mode=fixture-direct --seller=role=remoteid,multiaddr=<addr>,protocol=/ds240/basestation/1.0.0 --output=tcp:127.0.0.1:9090` (terminal 3) + `open http://127.0.0.1:8080` (browser).
- **Validation**: `cmd/fid-display/main_test.go` covers state.json, SSE event delivery, TCP listener ingest, stale-drone eviction, embedded-HTML serving. Live smoke run captured 19 frames across 3 synthetic drones into a single snapshot.

The fallback is **architecturally equivalent** to the eventual real-FID integration: same wire envelope, same source tagging, same map-marker semantics. When the real FID repo becomes accessible, the integration is mechanical: either (a) port the embedded HTML's marker-update logic into the React app, or (b) point the real FID's Node bridge at the fallback binary's `/events` SSE endpoint and treat the fallback as the dual-stream feed.

The standalone display is the in-repo verification artifact; an external production display can consume the same TaggedFrame stream by implementing this contract.

### Velocity presence flags (2026-05-25)

`/state.json` `normalizedTracks[]` carries additive boolean flags alongside the numeric velocity
fields: `hasGroundSpeed`, `hasHeading`, `hasVerticalRate`. They distinguish **"velocity not decoded"**
(flag `false`) from a **genuine zero** (flag `true`, value `0`). The numeric `groundSpeedMps` /
`headingDeg` / `verticalRateMps` fields are retained for backward compatibility (an absent component
serializes as `0`, so older consumers degrade to the prior — misleading — behaviour; new consumers
MUST gate on the `hasX` flag). The embedded UI renders `—` when a flag is `false` and shows a number
(including `0.0`) only when it is `true`; the aircraft marker stays neutral north-up with no heading
vector when `hasHeading` is `false`. Rationale and full chain:
`docs/tevv/adsb-speed-heading-zero-diagnosis.md`.

## Reference: where in the code

| Concern | File |
|---------|------|
| `OutputSink` interface | `impl/golang/internal/edgeapp/output.go:25` |
| `NewOutputSink` parser | `impl/golang/internal/edgeapp/output.go:36` |
| `jsonlWriter` shared kernel | `impl/golang/internal/edgeapp/output.go:63` |
| `AggregatedFrame` schema | `impl/golang/internal/edgeapp/aggregated.go:35` |
| `StdoutJSONLSink` | `impl/golang/internal/edgeapp/output.go:77` |
| `FileJSONLSink` | `impl/golang/internal/edgeapp/output.go:100`-ish |
| `TCPSink` | `impl/golang/internal/edgeapp/output.go:170`-ish |
| Buyer goroutine pool that emits | `impl/golang/internal/edgeapp/buyer.go` (per-seller goroutines invoke `OutputSink.Emit`) |
| Buyer entry point | `impl/golang/cmd/edge-buyer/main.go` |
| Tagged sink (v2) | `impl/golang/internal/edgeapp/tagged_output.go` — added 2026-05-08 for Phase 2 |
| Multistream buyer | `impl/golang/cmd/multistream-buyer/main.go` — emits consolidated v2-tagged JSONL |
| Fallback FID display | `impl/golang/cmd/fid-display/main.go` + `cmd/fid-display/static/index.html` — minimal standalone fallback display |
