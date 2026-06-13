# 4. Browser Demo — WSS

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

The buyer is now a browser tab. The TypeScript SDK derives purely from the specs — the protocol semantics in your browser are byte-for-byte identical to what step 1 ran in Go.

## What this demo proves

- The TypeScript SDK ([`impl/typescript/src/`](../../../impl/typescript/src/)) implements specs 001–005 correctly enough to negotiate, fund, and complete a sale with a Go seller
- libp2p over secure WebSockets (WSS) is a working browser data-plane binding (spec [012](../../../specs/012-browser-client-profile/spec.md))
- Ephemeral browser identity — a fresh secp256k1 key per tab — produces a valid Neuron agent without external setup
- Cross-language wire compatibility: a TypeScript buyer talks to a Go seller without either side knowing about the other's runtime

## When to run it

Run this **after** [Step 3](3-relay.md). You've seen the protocol work between Go processes; this demo proves the same protocol works across languages and across runtime environments (browser ↔ Node).

## Prerequisites

| What | How to check |
|------|--------------|
| Node.js 20.12+ | `node --version` |
| `pnpm` (or `npm`) | `pnpm --version` |
| A modern browser (Chromium ≥ 120 or Firefox ≥ 115) | — |
| Repo cloned | `ls impl/typescript/package.json` |

You do **not** need: Go (the Node.js seller in this demo replaces the Go seller from step 1), Hedera, env vars, or external infrastructure.

## Run it

```bash
cd impl/typescript
pnpm install            # first time only
pnpm run demo
```

This runs the orchestrator in [`scripts/run-demo.ts`](../../../impl/typescript/scripts/run-demo.ts):

1. Spawns the Node.js seller from [`src/server-demo/index.ts`](../../../impl/typescript/src/server-demo/) — listens on a random port via libp2p WSS
2. Waits for `[seller] ready` on stdout
3. Spawns Vite to serve the browser demo from [`examples/browser-demo/`](../../../impl/typescript/examples/browser-demo/)
4. Opens the browser page automatically (or prints the URL for you to open)

In the browser, click **Buy** to run the full flow.

## Expected output

**Terminal:**

```
[seller] starting libp2p host on /ip4/127.0.0.1/tcp/<PORT>/wss
[seller] ready
[seller] PeerID: 12D3KooWBuyerAndSellerListening...
[vite] dev server running at http://127.0.0.1:5173
[vite] open in browser ↑
```

**Browser tab (after clicking Buy):**

```
[1/9] Generating ephemeral key... done. PeerID 12D3Koo...
[2/9] Connecting to seller via WSS... connected.
[3/9] Discovering offer... received (jpeg, 500000000 tinybar).
[4/9] Negotiating... AGREED. agreementHash 0x...
[5/9] Funding mock escrow... 500000000 tinybar deposited.
[6/9] Reading connectionSetup... ECIES decrypted.
[7/9] Receiving JPEG... 50282 bytes received. SHA256 match: YES.
[8/9] Settling... invoice approved, escrow released.
[9/9] Done. Browser buyer completed the full Neuron flow.
```

The browser also displays the JPEG it received in the page so you can visually confirm the delivery.

> Behaviour-level output. The exact wording depends on the browser-demo UI; the nine-phase progression is stable.

## How to verify success

Pass criteria: every phase reads `done` / `success`, and the browser displays the received JPEG.

If you open the browser developer tools (Network tab), you should see a single WSS connection to the seller with libp2p frames flowing in both directions.

## What this maps to

| Component | Spec | Source |
|-----------|------|--------|
| Browser key generation | [002](../../../specs/002-key-library/spec.md), [012](../../../specs/012-browser-client-profile/spec.md) | [`impl/typescript/src/keylib/`](../../../impl/typescript/src/keylib/) |
| TopicMessage signing in browser | [004](../../../specs/004-topic-system/spec.md), [006](../../../specs/006-protocol-determinism/spec.md) | [`impl/typescript/src/topic/`](../../../impl/typescript/src/topic/) |
| WSS data plane | [009](../../../specs/009-p2p-data-delivery/spec.md), [012](../../../specs/012-browser-client-profile/spec.md) | [`impl/typescript/examples/browser-demo/`](../../../impl/typescript/examples/browser-demo/) |
| Node.js seller | — | [`impl/typescript/src/server-demo/`](../../../impl/typescript/src/server-demo/) |

Demo orchestrator: [`impl/typescript/scripts/run-demo.ts`](../../../impl/typescript/scripts/run-demo.ts).

## Useful scripts

| Command | Purpose |
|---------|---------|
| `pnpm run demo` | Full orchestrator: seller + browser |
| `pnpm run demo:server` | Node seller only — useful when running the browser dev server separately |
| `pnpm run demo:browser` | Browser dev server only — assumes a seller is already running and bootstrap is in place |

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `pnpm: command not found` | Install pnpm: `npm install -g pnpm` (or use `npm install` and `npm run demo`) |
| `Node.js version too old` | Use Node 20.12+. Check with `node --version`; install via [nvm](https://github.com/nvm-sh/nvm) |
| Browser tab never connects | Check the Vite console for WSS upgrade errors; the seller might be listening on `127.0.0.1` only — try opening Vite at `http://127.0.0.1:5173` rather than `localhost` |
| `WebSocket connection failed` | Some browser configurations block WSS to localhost without certs. Try Chromium with `--ignore-certificate-errors` for local testing only |
| Seller exits immediately | Port collision; kill any other Node processes on that port and retry |

## Next demo

→ **[Step 5: Browser WebTransport](5-browser-webtransport.md)** — same browser flow, HTTP/3 transport instead of WSS.
