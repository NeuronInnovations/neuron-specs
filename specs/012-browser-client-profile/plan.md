# Implementation Plan: Browser Client Profile (012)

**Branch**: `012-browser-client-profile` | **Date**: 2026-04-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification `specs/012-browser-client-profile/spec.md`; browser-to-server scoping analysis; 010 merge baseline at commit `157bbc6`.

## Summary

Spec 012 defines a thin browser-only buyer profile that connects to a Neuron seller over a single WSS libp2p connection. **This plan scopes v1 as a JS↔JS spike**: Node.js seller + browser client, both using `js-libp2p`. The existing Go seller path (FR-B08) is deferred to v2 as a hardening / interop phase. Phase 1 deliverable is a `pnpm run demo` that serves a page at `http://localhost:PORT`, where clicking "Buy" executes the full `serviceRequest` → `paymentDetails` → `connectionSetup` → `invoiceAck` flow through an in-stream TopicAdapter, receives a 100 KB JPEG over length-prefixed frames, verifies SHA-256 in-browser, and renders the image with a signature-chain ledger. Everything else in the spec (Go seller, TLS with real certs, on-chain escrow, registry lookup, HCS anchoring, validator replay, browser-to-browser) is explicitly deferred.

## Technical Context

**Language/Version**: TypeScript 5.7+ on Node.js 20+ (existing `impl/typescript` package). Browser target: Chromium ≥ 120, Firefox ≥ 115 (mandatory per SC-05).
**New Dependencies**:
- `libp2p` core (v2.x line — pinning resolved in research.md)
- `@libp2p/websockets` (browser + Node client/listener transport)
- `@chainsafe/libp2p-noise` (Noise XX)
- `@chainsafe/libp2p-yamux` (multiplexer)
- `@libp2p/identify` (standard libp2p identify handshake)
- `vite` (browser bundler, dev server)
- `@noble/ciphers` (AES-256-GCM for the ECIES decrypt path of FR-B16)

**Existing Dependencies (reused unchanged)**:
- `@noble/secp256k1`, `@noble/hashes` (already imports in `impl/typescript/src/keylib/`)
- Full `impl/typescript/src/keylib/*` — private key, public key, EVM address, PeerID, DID:key, signatures (12 files, bit-for-bit parity with Go keylib; see spec 002)
- `impl/typescript/src/topic/` — TopicMessage envelope serialization + verification (spec 004)

**Testing**: `vitest` (already configured at `impl/typescript/vitest.config.ts`); browser end-to-end via Playwright scripted against the dev server. No browser-in-CI yet — manual verification drives Phase 1 acceptance.
**Target Platform**: Browser (WSS client) + Node.js (WSS listener). Cross-browser parity tested manually.
**Project Type**: Single TypeScript package extension (`impl/typescript/` monorepo-style with new sub-modules). No new repo.
**Scale/Scope**: 36 functional requirements (FR-B01–B36), 9 success criteria (SC-01–SC-09). Phase 1 implements all identity, transport, control-plane, payment, delivery, bootstrap, lifecycle, verification-UX, and demo FRs end-to-end; defers full multi-browser verification + Go-seller interop to Phase 2.

## Constitution Check

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md v1 draft complete with all mandatory sections | **PASS** |
| II | Independently Testable Stories | 3 prioritised user stories, each with Given/When/Then + Independent Test | **PASS** |
| III | Clarification Before Plan | Session 2026-04-20 encoded 5 Q/As; zero `[NEEDS CLARIFICATION]` | **PASS** |
| IV | High-Level Types | Key Entities defines Browser Session, Bootstrap JSON, In-Stream TopicAdapter, Signature-Chain Ledger, Buyer State Machine — semantic types, not primitives | **PASS** |
| V | Traceability | FR-B01–B36, SC-01–SC-09 aligned; §"FR → Implementation Slice" table below maps each FR to a file | **PASS** |
| VI | Language-Neutral Protocol, Reference Implementations | Spec is language-neutral. Plan names TypeScript/JS as the v1 reference implementation — explicit deviation from the "Go reference path" wording in the template because a browser profile has no Go reference. Recorded in Complexity Tracking. | **PASS with note** |
| VII | Strict Spec Compliance | Each phase task will trace to FR-B## or SC-## (see tasks.md after `/speckit.tasks`) | **PASS** |
| VIII | Hedera Transport Binding | Server retains HCS; browser profile is a client subset per spec Constitution Notes. No HCS adapter test is required for this profile. | **N/A (documented)** |
| IX | Test-First Development | Test tasks precede implementation tasks in the task list; tamper-test and SHA-256-mismatch test are the acceptance gates for P2 story | **PASS** |
| X | Deterministic Signing | Reuse existing keylib signatures (RFC 6979, R‖S‖V) — already tested in 002 | **PASS (inherited)** |
| XI | Verifiable Execution | In-browser envelope verification + SHA-256 verification + per-envelope ledger UX satisfies verifiable-execution framing. No server-side oracle required. | **PASS** |

## Project Structure

### Documentation

```text
specs/012-browser-client-profile/
├── spec.md               # Feature specification (complete)
├── plan.md               # This file
├── research.md           # Phase 0 — transport / libp2p version decisions
├── data-model.md         # Phase 1 — entity definitions
├── contracts/
│   ├── stream-protocols.md   # control + data libp2p stream protocol IDs, framing
│   ├── bootstrap-json.md     # bootstrap JSON schema (Q2 decision)
│   └── in-stream-adapter.md  # in-stream TopicAdapter wire contract
├── quickstart.md         # Manual test steps + expected observable outputs
├── checklists/
│   └── requirements.md
└── tasks.md              # Generated by /speckit.tasks
```

### Source Code (delta on top of the 010-merge baseline)

```text
impl/typescript/
├── package.json              # MODIFIED — add scripts: demo, demo:server, demo:browser
├── src/
│   ├── keylib/               # EXISTING — reused as-is
│   ├── topic/                # EXISTING — TopicMessage envelope reused as-is
│   ├── wire/                 # EXISTING — canonical JSON helpers
│   │   └── frame.ts          # NEW — length-prefix framing, shared by both sides
│   ├── browser-client/       # NEW — browser buyer client (Phase 1)
│   │   ├── index.ts          # Entry point; Buy button wiring
│   │   ├── bootstrap.ts      # Bootstrap JSON loader + schema validator
│   │   ├── session.ts        # Ephemeral session (keypair, state machine)
│   │   ├── transport.ts      # js-libp2p host config (browser WS client)
│   │   ├── topic-adapter.ts  # In-stream TopicAdapter (browser side)
│   │   ├── buyer-flow.ts     # serviceRequest → invoiceAck state machine (4 msg)
│   │   ├── file-receive.ts   # Frame-0 metadata + chunk reassembly + SHA-256
│   │   ├── ecies-decrypt.ts  # ECIES decrypt for connectionSetup (FR-B16)
│   │   ├── errors.ts         # NEURON-BROWSER-NNN taxonomy (FR-B35)
│   │   └── ui/
│   │       ├── ledger.ts     # Signature-chain ledger renderer (FR-B29)
│   │       └── status.ts     # Verified/failure status renderer (FR-B30/31)
│   └── server-demo/          # NEW — Node.js seller (Phase 1)
│       ├── index.ts          # Main; starts libp2p host + HTTP static server
│       ├── transport.ts      # js-libp2p host config (Node WS listener)
│       ├── topic-adapter.ts  # In-stream TopicAdapter (server side — mirror of browser)
│       ├── seller-flow.ts    # Seller half of 4-message buyer flow
│       ├── mock-escrow.ts    # In-memory escrow (seller-only)
│       ├── file-send.ts      # Frame-0 metadata + chunk send + SHA-256
│       ├── ecies-encrypt.ts  # ECIES encrypt for connectionSetup
│       └── assets/
│           └── demo.jpg      # 100 KB sample JPEG
├── examples/
│   └── browser-demo/         # NEW — browser bundle entry
│       ├── index.html        # Demo page
│       ├── main.ts           # Mounts browser-client into the DOM
│       ├── vite.config.ts    # Vite dev server config
│       └── public/
│           └── bootstrap.json  # Generated by seller on startup; see run-demo.sh
└── scripts/
    └── run-demo.ts           # NEW — orchestrator: start seller → write bootstrap.json → start vite
```

**Structure Decision**: Single TypeScript package `impl/typescript`, two new top-level source directories (`browser-client/`, `server-demo/`), one new example app (`examples/browser-demo/`), one new orchestrator script, and one new file in the existing `wire/` package for shared framing. Nothing else in the repo moves. Existing `keylib/` and `topic/` are consumed as library dependencies and MUST NOT be modified by Phase 1.

## Phase 0: Research

Unknowns before this plan, resolved in `research.md`:

- **Which js-libp2p major version?** → v2.x (current as of 2026). Browser WSS transport is stable; Noise XX and yamux are both first-party.
- **Does the spike use `wss://` or `ws://`?** → `ws://` on `localhost:PORT` for the local demo (browsers permit it when the page is served `http://localhost`). Real TLS termination is Phase 2. Documented in quickstart and data-model.
- **Server-side bootstrap-JSON serving?** → The Node.js seller generates `bootstrap.json` at startup (since PeerID is derived from a fresh seller keypair on each run); Vite serves it at the same origin as the browser bundle. FR-B24 (same-origin, no CORS) is preserved.
- **In-stream TopicAdapter framing?** → `[uint32 big-endian length][canonical-JSON TopicMessage]`, identical to FR-B12 / FR-D22. Shared framing utility in `wire/frame.ts`.
- **Mock escrow shape?** → Minimal in-memory object in `server-demo/mock-escrow.ts`; 4 states (proposed / funded / released / refunded), only `proposed` and `released` are exercised by the buyer flow.

## Phase 1: Design & Contracts

**Entities** (full detail in `data-model.md`):

1. **BootstrapJSON** — the static same-origin descriptor: `sellerEVMAddress`, `sellerPeerID`, `sellerWSSMultiaddr`, `controlStreamProtocolID`, `dataStreamProtocolID`, `version`. Unknown top-level keys rejected (Q2 decision).
2. **BrowserSession** — ephemeral in-memory context: `privateKey` (never leaves scope), `evmAddress`, `peerID`, `didKey`, `host` (libp2p handle), `state` (BuyerState), `ledger` (append-only per-envelope log).
3. **BuyerState** — 6-state machine: `idle` → `request-sent` → `quote-received` → `connection-setup-received` → `invoice-ack-sent` → `complete`, with explicit `aborted{reason}` transitions from any state (matches spec.md Key Entities).
4. **InStreamTopicAdapter** — a Spec 004 `TopicAdapter` implementation whose `publish` and `subscribe` operations read/write a single libp2p stream on the control protocol ID. Two concrete implementations (browser side and server side) share the same wire contract.
5. **SignatureChainLedger** — append-only per-session list of `{type, sender, timestamp, sigStatus, payloadHash}` entries; rendered in DOM for the user.

**Contracts** (in `contracts/`):

- `stream-protocols.md` — `/neuron/browser-profile/control/1.0.0` (control), `/neuron/browser-profile/data/1.0.0` (data), framing definition, handshake order.
- `bootstrap-json.md` — JSON schema, field descriptions, unknown-key rejection rule, version compatibility.
- `in-stream-adapter.md` — wire-level behaviour: one stream per role, envelope-at-a-time, backpressure, close semantics.

## Phase 2 (post-v1): Hardening & Interop

Not implemented in v1; surfaced here so `/speckit.tasks` produces Phase 2 tasks that cannot leak into Phase 1:

- **H1 Go-seller interop** — Add WSS listener to `impl/golang/internal/delivery/libp2p_host.go`; verify browser ↔ Go seller handshake on `/neuron/browser-profile/control/1.0.0`. Satisfies FR-B08 as originally written.
- **H2 Real TLS** — Caddy + Let's Encrypt in front of the Node.js seller; browser moves from `ws://` to `wss://`; page served over `https://`.
- **H3 Tamper tests** — Automated vitest + Playwright harness for SC-07 (bit-flip signature → abort) and SHA-256 mismatch (FR-B22); Phase 1 covers these manually.
- **H4 Multi-browser matrix validation** — Scripted runs on Chromium, Firefox, Safari per SC-05.
- **H5 Error-taxonomy alignment** — Map `NEURON-BROWSER-NNN` identifiers to Spec 006 (FR-B35 pre-merge obligation).
- **H6 Read-idle timeout constant** — Implement FR-B34 with a concrete constant (recommended 15 s; 30 s upper bound).
- **H7 Size-cap refusal automated test** — Inject a >1 MiB metadata declaration, verify refusal before first chunk (FR-B21).

## FR → Implementation Slice (traceability map)

| FR | File(s) | Phase |
|---|---|---|
| FR-B01, FR-B03, FR-B04 | `browser-client/session.ts` | 1 |
| FR-B02 | `src/keylib/*` (reused); `browser-client/session.ts` | 1 |
| FR-B05, FR-B07, FR-B09 | `browser-client/transport.ts` | 1 |
| FR-B06 | `browser-client/transport.ts`; `server-demo/transport.ts` | 1 |
| FR-B08 | `server-demo/transport.ts` (JS); Go WSS listener is **H1** | 1 (JS), 2 (Go) |
| FR-B10, FR-B11, FR-B13 | `browser-client/topic-adapter.ts`; `server-demo/topic-adapter.ts` | 1 |
| FR-B12 | `src/wire/frame.ts` (new); reused by both sides | 1 |
| FR-B14 | `browser-client/buyer-flow.ts` (sender-address pinning) | 1 |
| FR-B15 | `browser-client/buyer-flow.ts`; `server-demo/seller-flow.ts` | 1 |
| FR-B16 | `browser-client/ecies-decrypt.ts`; `server-demo/ecies-encrypt.ts` | 1 |
| FR-B17, FR-B22 | `browser-client/file-receive.ts` | 1 |
| FR-B18 | `server-demo/mock-escrow.ts` | 1 |
| FR-B19, FR-B20 | `browser-client/file-receive.ts`; `server-demo/file-send.ts`; `src/wire/frame.ts` | 1 |
| FR-B21 | `browser-client/file-receive.ts` (reject before chunks) | 1 |
| FR-B23, FR-B24 | `browser-client/bootstrap.ts`; `server-demo/index.ts` same-origin serve | 1 |
| FR-B25 | (negative — verified by absence of any EVM RPC client import) | 1 |
| FR-B26, FR-B27 | `browser-client/index.ts` (no SW registration); `browser-client/session.ts` (no persistence) | 1 |
| FR-B28 | `browser-client/buyer-flow.ts` (concurrency guard) | 1 |
| FR-B29, FR-B30, FR-B31 | `browser-client/ui/ledger.ts`, `browser-client/ui/status.ts` | 1 |
| FR-B32, FR-B33 | `examples/browser-demo/main.ts`; `browser-client/index.ts` | 1 |
| FR-B34 | `browser-client/file-receive.ts` (read-idle timeout) — concrete constant **H6** | 1 (wired), 2 (tightened) |
| FR-B35 | `browser-client/errors.ts` (profile-local IDs) — 006 alignment is **H5** | 1 (local), 2 (aligned) |
| FR-B36 | `browser-client/index.ts` (Buy handler rotates session) | 1 |

## Security-sensitive boundaries (even for spike quality)

- **Private key material**: lives in a closure inside `browser-client/session.ts`; never returned from an exported function, never stringified, never logged. The ledger displays only the public `EVMAddress` and `PeerID`.
- **Bootstrap JSON integrity**: the page trusts same-origin delivery. If the page is ever served over plain HTTP in production, the whole trust model collapses. Quickstart explicitly flags this. FR-B24 (no cross-origin fetch) enforced by the bootstrap loader.
- **Server PeerID pinning**: the browser rejects any Noise handshake whose recovered server PeerID does not match `bootstrap.sellerPeerID`. Enforced in `browser-client/transport.ts` before the first byte of application data.
- **Inbound envelope verification**: every TopicMessage is signature-verified and sender-pinned before it reaches the state machine. Failure aborts.
- **WSS for real deployment**: Phase 1 uses `ws://localhost:PORT` for developer convenience. The moment the seller is put on a public IP, it MUST move to `wss://` with a real cert. Phase 2 H2 captures this; Phase 1 quickstart prints a red warning banner if invoked with a non-localhost hostname.

## Risks / unknowns / blockers

| # | Risk | Mitigation |
|---|------|------------|
| R1 | **js-libp2p API drift between minor versions** may make the pinned version stale by Phase 2. | Pin exactly in `package.json` (`libp2p@X.Y.Z`, no `^`). research.md records the chosen version. Exclude this package from automated updates until H1 is ready. |
| R2 | **`ws://` on localhost still fails some corporate-laptop firewalls**. | Quickstart documents the "did my firewall block it?" diagnostic (a curl test). `DEMO_PORT` env override to route around port conflicts. |
| R3 | **js-libp2p Noise handshake + Yamux multiplexer + WebSockets is a lot of moving parts for one spike**. | Phase 1 task 1 is a standalone handshake smoke test before any Neuron-level code: browser opens WS → completes Noise → opens one `/test/echo/1.0.0` stream → round-trips "ping"/"pong". Gate everything else on this test passing. |
| R4 | **Bootstrap JSON schema churn** — adding fields later is easy, removing is hard. | `version` tag enforced from v1; unknown top-level keys rejected at load; browser refuses if `version !== 1`. |
| R5 | **The spike's Node.js seller may diverge from the eventual Go seller's behaviour**. | Wire contracts in `contracts/` are the single source of truth for both sides; the Go server in Phase 2 (H1) MUST pass the same contract tests. |
| R6 | **ECIES decrypt in-browser via `@noble/ciphers`** is unfamiliar territory for the team. | Reuse the same primitives already in `impl/typescript/src/keylib/`; Phase 1 includes a targeted unit test that decrypts a ciphertext produced by `impl/golang/internal/delivery/ecies.go` test vectors — even though the Go seller isn't in the Phase 1 path, its vectors validate the JS decrypt path. |
| R7 | **Page-reload mid-transaction** may surface race conditions (FR-B26 says discard, but WS onclose handlers fire async). | `session.ts` holds a single AbortController fired on `beforeunload`; all WS reads check the token and bail. |
| R8 | **No CI yet for browser tests**. | Phase 1 accepts manual verification per the quickstart. Phase 2 H3/H4 adds Playwright + CI. |

## Manual test approach (Phase 1 acceptance)

Full procedure in `quickstart.md`. High level:

1. `pnpm run demo` — starts Node.js seller on `:8080` (libp2p WS listener) + Vite dev server on `:5173` (page + bootstrap.json) + opens browser to `http://localhost:5173`.
2. Click **Buy** → ledger populates with 4 envelopes (each "✓ verified") → JPEG renders → status: `"verified SHA-256: <hash>"`.
3. DevTools inspection: confirm zero entries in `localStorage`, `sessionStorage`, `IndexedDB`, cookies.
4. Reload → confirm new PeerID in ledger; prior session state cleared.
5. Click Buy again within the same tab → confirm a NEW PeerID (FR-B36 rotation).
6. Tamper test (manual): stop the seller, edit `assets/demo.jpg` to flip one byte, restart. Click Buy. Observe SHA-256 mismatch + red failure banner citing `NEURON-BROWSER-014` (or the equivalent code assigned in `errors.ts`).

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|--------------------------------------|
| Principle VI template text says "Go reference implementation path"; plan targets TypeScript/JS as the v1 reference. | A browser client by definition has no Go reference path. The spec is a client profile; the normative artifact is the TS bundle. Go seller (H1) is Phase 2. | Writing a Go reference for a browser client is a category error — Go does not run in browsers. |
| 1 new file under `impl/typescript/src/wire/` (`frame.ts`). | Spec 009's length-prefix framing exists only in Go today; browser needs the same bytes. `wire/` is the right home — it already holds canonical-JSON encoders/decoders. | Duplicating the framing logic in both `browser-client/` and `server-demo/` would double the test surface and risk drift. |

## Progress Tracking

- [x] Phase 0 — research.md
- [x] Phase 1 — data-model.md, contracts/, quickstart.md
- [ ] Agent context update (`update-agent-context.sh claude`)
- [ ] Phase 2 — `/speckit.tasks`
