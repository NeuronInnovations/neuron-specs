# Demo 8 — SAPIENT Sensor Chain (local, no hardware)

← Back to [Getting Started](../README.md) · [Architecture](../architecture.md)

**Goal:** run the full SAPIENT sensor-to-map chain on your laptop: a simulated Remote ID sensor feed → SAPIENT seller (dials the buyer) → generic Buyer Proxy → consumer → live map.

**Specs exercised:** 015 (SAPIENT Interop Profile: proxies, lanes, tasking), 017 (DroneScout Remote ID translator semantics), 009 (P2P data plane), 005 (heartbeat, in memory/HCS modes).

**Everything below is local-only**: loopback addresses, in-memory or file-backed backends, ephemeral keys, no network beyond `localhost`, no hardware, no testnet unless explicitly marked.

---

## One-shot run

```bash
make demo-sapient
```

This builds the binaries into a temp dir, starts the chain (simulator → seller → buyer → consumer → map), opens the map at `http://127.0.0.1:8080`, and cleans up on Ctrl-C. Add the rich SAPIENT display variant with:

```bash
scripts/demo/sapient-rid-demo.sh --sapient-display   # rich UI at http://127.0.0.1:8193
```

## The chain, process by process

The topology (seller **dials** the buyer — reverse-connect):

```
fixture ─▶ sapient-feed-replay ─▶ sapient-rid-seller ──(dials)──▶ sapient-buyer ─▶ sapient-fid-consumer ─▶ displays
           (bridge stand-in)       (Seller Proxy +                 (Buyer Proxy)     (SAPIENT→tracks/CoT)
                                    RID translator side)
```

Run it manually, one terminal per process (all from `impl/golang`):

```bash
# 1. Bridge stand-in: serve a captured SAPIENT fixture as a live LE-framed protobuf feed
go run ./cmd/sapient-feed-replay --listen 127.0.0.1:9999 \
  --fixture internal/dapp/sapient/testdata/bridge-sample.ndjson --loop

# 2. Buyer Proxy: listen for the seller, serve the SAPIENT edge + sessions endpoint
go run ./cmd/sapient-buyer --listen /ip4/127.0.0.1/udp/19192/quic-v1 \
  --sapient-edge 127.0.0.1:19193 --sessions-http 127.0.0.1:19201
# note the printed multiaddr: /ip4/127.0.0.1/udp/19192/quic-v1/p2p/<PEER_ID>

# 3. Seller: pull the bridge feed, re-stamp node_id with its Neuron identity, dial the buyer
go run ./cmd/sapient-rid-seller --bridge-addr 127.0.0.1:9999 \
  --buyer /ip4/127.0.0.1/udp/19192/quic-v1/p2p/<PEER_ID> \
  --feed-source replay

# 4. Consumer: SAPIENT edge → TaggedFrame for the map(s) + optional received-API
go run ./cmd/sapient-fid-consumer --edge 127.0.0.1:19193 \
  --output tcp:127.0.0.1:19191 \
  --sapient-output tcp:127.0.0.1:19194 \
  --sapient-received-http 127.0.0.1:19200

# 5a. Map (compatibility display)
go run ./cmd/fid-display --tcp 127.0.0.1:19191 --http 127.0.0.1:8080

# 5b. Rich SAPIENT display (classification, confidence, agent identity)
go run ./cmd/sapient-fid-display --tcp 127.0.0.1:19194 --http 127.0.0.1:8193 \
  --sessions-url http://127.0.0.1:19201/sessions

# 5c. Optional: tactical console (proxies 5b read-only + agent registry)
go run ./cmd/sapient-explorer --http 127.0.0.1:8194 --fid-url http://127.0.0.1:8193
```

### Tasking (STOP/START over the auditable lane)

With the seller started with `--control-lane file:/tmp/control.ndjson`, a consumer can issue the mandatory per-session STOP/START Task and wait for the TaskAck:

```bash
go run ./cmd/sapient-task --lane file:/tmp/control.ndjson \
  --asm-node-id <SELLER_NODE_ID> --control stop --wait 3s
```

The default audit-lane backend is a local file; `--auditlane-backend=hcs` on the seller publishes the lane to real Hedera topics instead (testnet — requires `HEDERA_OPERATOR_ID` / `HEDERA_OPERATOR_KEY`).

## HTTP surfaces

All bind loopback by default. None of them expose secrets, environment values, keys, or host paths.

| Process | Default address | Routes |
| ------- | --------------- | ------ |
| `fid-display` | `127.0.0.1:8080` (TCP in `:9090`, demo uses `:19191`) | `/` map UI, `/config.json`, `/state.json`, `/events` (SSE) |
| `sapient-fid-display` | `127.0.0.1:8193` (TCP in `:19194`) | `/` map UI, `/config.json`, `/state.json`, **`/sources.json`** (per-source seller cards + session health), **`/sensors.json`** (sensor-location layer, from `--sensors` file), `/events` (SSE) |
| `sapient-explorer` | `127.0.0.1:8194` | `/` console UI, `/config.json`, `/agents.json`, `/agents/<id>`, `/tracks.json`, `/sensors.json`, `/events`, `/healthz` — read-only proxy over the live display + local agent evidence |
| `sapient-buyer` | `--sessions-http` (off unless set; demo uses `:19201`) | `GET /sessions` — open seller sessions: `id`, `peerID`, `remoteAddr`, `nodeId`, `connectedAt`, `lastSeen`, `messageCount`, plus cumulative totals |
| `sapient-fid-consumer` | `--sapient-received-http` (off unless set; demo uses `:19200`) | **received-API**: `/sapient/received/latest` (`?limit=N`, default 50), `/sapient/received/stream` (NDJSON), `/sapient/received/schema`, `/sapient/received/health` |

### The received-API in one paragraph

`/sapient/received/*` is a read-only **verification tap**: a lossy JSON projection of every SAPIENT message as decoded at the consumer's receive boundary, *before* any map projection. It exists so an external partner can confirm their sensor's payload arrives and decodes correctly. It is **session-scoped**: it carries exactly what the consumer's buyer sessions deliver. In the partner-facing deployment the buyer session carries only the drone (`rid`) service, so that API surface is **drone-only**; the internal displays above are multi-source.

## Modality rules (how to read the data)

- **Tracks are keyed `nodeId|uid`** — never by SAPIENT `object_id` alone, because two independent sensors may emit colliding IDs. One physical aircraft seen by two sensors appears as two tracks (aggregation without association).
- **Aircraft are not drones.** ADS-B tracks carry `kind:"adsb"` with `callsign`, `icao24`, etc. (the `neuron.adsb/1` extension); Remote ID tracks carry `rid.*` fields (serial, operator — the `neuron.rid/1` extension). An aircraft's identity field is its **callsign**, never a Remote ID.
- **Source status is runtime-verified.** Whether a seller is live comes from its open session (`/sessions`), its heartbeat (spec 005), and its `feedSource` advertisement (`live` / `replay` / `synthetic` / `placeholder`) — this demo runs `--feed-source replay` and the displays disclose it.

## Going further (testnet, optional)

- `sapient-rid-seller --register --registry-backend evm --registry-address <addr>` registers a real EIP-8004 AgentCard (needs a funded key and an RPC endpoint via env).
- `sapient-buyer --commerce-mode full --seller-evm <addr> …` runs an escrow-funded, admission-gated session against one seller and exits after settlement.
- All credentials come from environment variables (`HEDERA_OPERATOR_ID`, `HEDERA_OPERATOR_KEY`, contract addresses) — nothing is baked in.

## Troubleshooting

| Symptom | Fix |
| ------- | --- |
| Seller exits with "dial" errors | The `--buyer` multiaddr must include the `/p2p/<PEER_ID>` suffix printed by `sapient-buyer` at startup. |
| Map shows nothing | Check the chain order: replay → buyer → seller → consumer → display. The consumer logs each received message count. |
| `/sessions` connection refused | `--sessions-http` is off by default — pass an address. |
| Fixture exhausted | Add `--loop` to `sapient-feed-replay`. |
