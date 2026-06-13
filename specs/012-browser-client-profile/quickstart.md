# Quickstart: Browser Client Profile (012) — Phase 1 Demo

**Goal**: run the end-to-end JS↔JS browser-to-server Neuron transaction on your laptop in under 5 minutes, and manually verify the Phase 1 acceptance criteria.

## Developer prerequisites

- macOS, Linux, or WSL2. (Windows-native works but is untested in v1.)
- Node.js ≥ 20 (LTS). Verify: `node --version`.
- `pnpm` ≥ 9. Install via corepack: `corepack enable && corepack prepare pnpm@latest --activate`.
- A modern browser: Chromium ≥ 120 **or** Firefox ≥ 115 (SC-05 mandatory set).
- An open TCP port (default `8080` for the seller, `5173` for Vite). Override via `SELLER_PORT` / `DEMO_PORT` if in use.
- No Hedera credentials, no ETH node, no relay, no TLS certs. This is a pure-local demo.

## One-time setup

```bash
cd impl/typescript
pnpm install
```

Installs `libp2p`, `@libp2p/websockets`, `@chainsafe/libp2p-noise`, `@chainsafe/libp2p-yamux`, `@libp2p/identify`, `vite`, and `@noble/ciphers` in addition to the existing TS keylib dependencies.

## Run the demo

```bash
cd impl/typescript
pnpm run demo
```

`pnpm run demo` executes `scripts/run-demo.ts`, which:

1. Starts the Node.js seller on `ws://127.0.0.1:8080` (libp2p WS listener, loopback only — R0.12).
2. Waits for the seller to print its freshly-derived PeerID + multiaddr.
3. Writes `examples/browser-demo/public/bootstrap.json` with the seller's `EVMAddress`, `PeerID`, `WSS multiaddr`, and the two stream protocol IDs.
4. Starts Vite dev server on `http://localhost:5173`.
5. Opens your default browser to `http://localhost:5173` (or prints the URL if headless).

Expected console output:

```
[seller]  listening: /ip4/127.0.0.1/tcp/8080/ws/p2p/12D3KooW...
[seller]  EVMAddress: 0xA0B1c2D3e4F5a6B7c8D9e0F1a2B3c4D5e6F7a8B9
[seller]  escrow: mock in-memory (FR-B18)
[orchestrator]  wrote bootstrap.json
[vite]    dev server running at http://localhost:5173
[orchestrator]  opened browser
```

## Phase 1 acceptance checklist (manual)

Do all of these in one sitting. Each step maps to a specific FR or SC.

### 1. End-to-end happy path (SC-01, SC-02, SC-03, FR-B32, FR-B33)

- Open DevTools **before** clicking Buy. You want the network + console tabs visible.
- Click the single **Buy** button on the page.
- Within 15 seconds (SC-03), the 100 KB sample JPEG should render on the page.
- A `"verified SHA-256: <64-hex>"` status should appear beneath it.
- The ledger should list exactly 4 envelopes, each marked `✓ verified`:
  1. outbound `serviceRequest` (self-signed)
  2. inbound `paymentDetails`
  3. inbound `connectionSetup`
  4. outbound `invoiceAck` (self-signed)

**Pass criterion**: image rendered + SHA-256 displayed + ledger complete.

### 2. Ephemeral identity (FR-B01, FR-B03, SC-06, SC-08)

- In DevTools, **Application** tab → inspect `localStorage`, `sessionStorage`, `IndexedDB`, `Cookies`. **Zero entries** related to keys or session state should be visible at any point during or after the transaction.
- Note the PeerID shown in the ledger. Call it `P1`.
- Reload the page.
- Click Buy again. Note the new PeerID `P2`.
- `P1 ≠ P2`. (Reload produces a new keypair.)

**Pass criterion**: no persistent storage, `P1 ≠ P2`.

### 3. Per-Buy rotation within a single page (FR-B36)

- Without reloading, click Buy a third time.
- Note the new PeerID `P3`.
- `P3 ≠ P2` (each Buy rotates, even in the same page lifecycle).

**Pass criterion**: `P3 ≠ P2`.

### 4. Network surface (SC-09)

- In DevTools **Network** tab (with "Preserve log" on), clear before loading the page, then reload.
- After DOMContentLoaded, the only non-page-load network activity should be:
  - **1 fetch**: `GET /bootstrap.json` (same origin, `application/json`).
  - **1 WebSocket**: `ws://127.0.0.1:8080/...` (opened on Buy).
- No analytics, no CDNs, no favicons beyond the static page, no third-party resources.

**Pass criterion**: exactly 1 bootstrap fetch + 1 WS connection.

### 5. Signature tamper (SC-07, FR-B13, FR-B31) — MANUAL

- Stop the demo (`Ctrl-C`).
- Edit `impl/typescript/src/server-demo/seller-flow.ts` and temporarily replace one byte of the `paymentDetails` signature before sending (e.g., flip the last `R` byte). Search for the "TAMPER HERE" comment marker added during Phase 1 development.
- Re-run `pnpm run demo`.
- Click Buy.
- Expected: the ledger shows `paymentDetails` with `✗ failed`, a red failure banner appears citing a `NEURON-BROWSER-061` error (signature recovery mismatch), and the JPEG is **not** rendered.

**Pass criterion**: abort before render + specific error code visible.

- Revert the tampering edit before committing anything.

### 6. SHA-256 mismatch (FR-B22) — MANUAL

- Stop the demo.
- Open `impl/typescript/src/server-demo/assets/demo.jpg` in a hex editor and flip a single byte somewhere in the middle.
- Re-run the demo. The seller's frame-0 metadata will still declare the **original** SHA-256 (because `file-send.ts` hashes at startup); the browser will compute a different hash on reassembly.
- Click Buy.
- Expected: all 4 envelopes verify ✓, the JPEG bytes arrive, the reassembly completes, but SHA-256 verification fails. Red banner cites `NEURON-BROWSER-082`. Image does **not** render.

**Pass criterion**: abort at SHA-256 check + error code visible.

- Restore `demo.jpg` from git before committing.

### 7. Size cap (FR-B21) — OPTIONAL

- Swap `demo.jpg` for a 2 MiB file.
- Click Buy.
- Expected: the seller's frame-0 metadata declares `sizeBytes > 1_048_576`; the browser aborts **before reading any chunk** with `NEURON-BROWSER-101`.

**Pass criterion**: abort before chunks + error code visible.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `EADDRINUSE: 127.0.0.1:8080` on seller startup | Port already bound | `SELLER_PORT=8090 pnpm run demo` |
| Vite fails to open browser | Running headless or in WSL without display | Open `http://localhost:5173` manually |
| `bootstrap.json` loads but WS connection hangs | Corporate laptop firewall blocking loopback WS | `curl -v http://127.0.0.1:8080/` to isolate; add loopback to firewall allowlist |
| Browser console: `Noise handshake failed: peer id mismatch` | Stale `bootstrap.json` from a previous run | Stop demo, `rm examples/browser-demo/public/bootstrap.json`, re-run |
| `Unknown field 'extraField'` error on page | A hand-edited `bootstrap.json` has a typo | Don't hand-edit; let the orchestrator regenerate it |
| `NEURON-BROWSER-008` on page load | You're running with `NODE_ENV=production` on a non-loopback URL | v1 only supports `http://localhost`; use H2 path for public deployment |

## Security note for non-localhost deployment

> ⚠️ This demo binds to `127.0.0.1` and uses plain `ws://` for developer convenience. If you deploy the Node.js seller to any host reachable from the public internet, you MUST:
>
> 1. Front it with TLS termination (Caddy + Let's Encrypt recommended).
> 2. Update `sellerWSSMultiaddr` in the bootstrap to `/dns4/.../tcp/443/wss/p2p/...`.
> 3. Serve the browser page over `https://` (mixed-content rules will otherwise block `wss://`).
> 4. Unbind from loopback-only (`server-demo/index.ts` has a `--bind` flag reserved for this).
>
> Phase 1 deliberately does not ship any of this. It is Phase 2 H2's job.

## Next steps after Phase 1 acceptance

- `/speckit.tasks` — generate the dependency-ordered task list from this plan.
- `/speckit.checklist` — generate a quality checklist for Phase 2 hardening (H1–H7).
- `/speckit.implement` — only after tasks.md is reviewed.
