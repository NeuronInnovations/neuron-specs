# 5. Browser Demo — WebTransport

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

The same browser buyer as [Step 4](4-browser-wss.md), but the data plane runs over **WebTransport** (HTTP/3) instead of secure WebSockets. The seller is the Go binary `webtransport-seller`; the browser uses the TypeScript SDK's WebTransport binding.

## What this demo proves

- WebTransport is a working browser data-plane binding for Neuron (spec [012](../../../specs/012-browser-client-profile/spec.md))
- The TypeScript SDK's transport abstraction holds — swapping WSS for WebTransport changes one binding and nothing else in the protocol layer
- HTTP/3 streams carry the same libp2p frame protocol that QUIC does in [Step 2](2-delivery.md)
- A Go seller serves both Tier A (echo, simple) and Tier B (full buy flow) for browser clients

## When to run it

Run this **after** [Step 4](4-browser-wss.md). Step 4 proved the buyer-side SDK works in the browser; this demo proves the SDK's transport layer is genuinely pluggable.

## Prerequisites

| What                                          | How to check     |
| --------------------------------------------- | ---------------- |
| Node.js 20.12+                                | `node --version` |
| `pnpm` (or `npm`)                             | `pnpm --version` |
| Go 1.22+                                      | `go version`     |
| Chromium ≥ 120 (WebTransport browser support) | —                |
| Two terminals                                 | —                |

You do **not** need: Hedera, env vars, certificates (a local self-signed cert is auto-generated).

## Run it

This demo needs **two terminals**. Unlike step 4, the orchestrator does **not** auto-spawn the Go seller — you start it yourself.

**Terminal A — start the Go WebTransport seller:**

```bash
cd impl/golang
go run ./cmd/webtransport-seller \
  --listen 4443 \
  --public-ip 127.0.0.1 \
  --bootstrap-out ../typescript/examples/browser-demo-wt/public/bootstrap-wt.json
```

The seller writes a `bootstrap-wt.json` file that tells the browser how to dial it. Leave this terminal running.

**Terminal B — start the browser orchestrator:**

```bash
cd impl/typescript
pnpm install            # first time only
pnpm run demo:wt
```

This spawns Vite serving [`examples/browser-demo-wt/`](../../../impl/typescript/examples/browser-demo-wt/). Open the URL Vite prints (typically `http://127.0.0.1:5174/`) in Chromium and click **Buy**.

## Expected output

**Terminal A (Go seller):**

```
=== WEBTRANSPORT SELLER READY ===
PeerID:     12D3KooW...
Listen:     udp:4443 (HTTP/3 + WebTransport)
Cert:       self-signed (sha256: ...)
Bootstrap:  written to ../typescript/examples/browser-demo-wt/public/bootstrap-wt.json
Tiers:      A (echo), B (buy flow with bundled JPEG)

[Tier-A connect] from 16Uiu2HAm...
[Tier-A echo] 4 bytes <- "ping"
[Tier-A echo] 4 bytes -> "pong"
[Tier-B connect] from 16Uiu2HAm...
[Tier-B negotiate] offer accepted, agreement hash 0x...
[Tier-B deliver] 50282 bytes streamed
```

**Browser tab:**

The browser-demo-wt page has two buttons:

- **Ping** (Tier A) — round-trips a single message and shows the latency
- **Buy** (Tier B) — runs the full nine-phase flow exactly like [Step 4](4-browser-wss.md), but over WebTransport

After clicking **Buy**, you'll see:

```
Connecting via WebTransport to 127.0.0.1:4443… connected.
Negotiating offer… AGREED.
Funding mock escrow… deposited.
Reading connectionSetup… decrypted.
Receiving JPEG… 50282 bytes. SHA256 match: YES.
Done.
```

The received JPEG renders in the page.

> Behaviour-level output. The exact wording depends on the browser-demo-wt UI.

## How to verify success

Pass criteria:

- Tier A: `Ping` button produces a sub-100ms round trip
- Tier B: the nine-phase flow completes and the JPEG renders in the page
- Terminal A logs the corresponding `[Tier-A]` or `[Tier-B]` lines

In Chromium DevTools (Network tab), filter by `http3` to see the WebTransport session.

## What this maps to

| Component                             | Spec                                                                                                              | Source                                                                                                |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| WebTransport seller (HTTP/3 + libp2p) | [009](../../../specs/009-p2p-data-delivery/spec.md), [012](../../../specs/012-browser-client-profile/spec.md)     | [`impl/golang/cmd/webtransport-seller/main.go`](../../../impl/golang/cmd/webtransport-seller/main.go) |
| Browser WebTransport client           | [012](../../../specs/012-browser-client-profile/spec.md), [013](../../../specs/013-connectivity-profiles/spec.md) | [`impl/typescript/examples/browser-demo-wt/`](../../../impl/typescript/examples/browser-demo-wt/)     |
| Bootstrap descriptor flow             | [013](../../../specs/013-connectivity-profiles/spec.md)                                                           | [`impl/golang/cmd/webtransport-seller/main.go`](../../../impl/golang/cmd/webtransport-seller/main.go) |

Orchestrator: [`impl/typescript/scripts/run-demo-wt.ts`](../../../impl/typescript/scripts/run-demo-wt.ts).

## Useful flags

**Go seller:**

| Flag              | Default               | Purpose                                                                           |
| ----------------- | --------------------- | --------------------------------------------------------------------------------- |
| `--listen`        | `4443`                | UDP port for the WebTransport listener                                            |
| `--public-ip`     | `127.0.0.1`           | IP advertised in the bootstrap multiaddr                                          |
| `--bootstrap-out` | `./bootstrap-wt.json` | Path to write the bootstrap JSON (point this at the browser demo's public folder) |
| `--identity`      | (ephemeral)           | Optional persistent secp256k1 key file                                            |
| `--jpeg`          | (bundled)             | Optional asset path for Tier B; Tier A echo is registered unconditionally         |

**TS orchestrator:**

| Command                       | Purpose                                                            |
| ----------------------------- | ------------------------------------------------------------------ |
| `pnpm run demo:wt`            | Local mode (Go seller on the same machine)                         |
| `pnpm run demo:wt -- --fetch` | Pull `bootstrap-wt.json` from a VPS via scp (see VPS recipe below) |

## Production deployment — browser ↔ VPS

To deploy the WebTransport seller on a public host so a browser can dial it across the open internet, the flow is:

1. Run `webtransport-seller` on the VPS with `--public-ip <vps-ip>`
2. `scp` the resulting `bootstrap-wt.json` to your laptop (or use `pnpm run demo:wt -- --fetch`)
3. Open the browser demo locally; it dials your VPS over real internet HTTP/3

Use placeholders, not real IPs, when sharing this configuration.

## Troubleshooting

| Symptom                                             | Fix                                                                                                                                                          |
| --------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Browser fails with `WebTransport not supported`     | Use Chromium ≥ 120 or Firefox ≥ 115. WebTransport is not yet in Safari                                                                                       |
| Browser tab spins forever on connect                | Self-signed cert rejected. Open `chrome://flags`, enable "Treat insecure origins as secure", add `https://127.0.0.1:4443`                                    |
| `bootstrap-wt.json: not found` (in browser console) | The Go seller didn't run yet, or `--bootstrap-out` pointed to the wrong path. Confirm the file exists at `examples/browser-demo-wt/public/bootstrap-wt.json` |
| Port 4443 already in use                            | Pass `--listen 4445` to the Go seller; the bootstrap file picks up the new port automatically                                                                |
| Vite port collision (5174)                          | Set `WT_VITE_PORT=5175 pnpm run demo:wt`                                                                                                                     |

## Next demo

→ **[Step 6: Hedera heartbeat](6-hedera-heartbeat.md)** — first time on a real public ledger.
