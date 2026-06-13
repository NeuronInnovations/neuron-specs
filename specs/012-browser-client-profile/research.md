# Phase 0 Research: Browser Client Profile (012)

**Purpose**: Resolve every technical unknown before design (Phase 1). Each entry below has a **Decision**, **Rationale**, **Alternatives considered**, and (where material) a **Constraint** that downstream artifacts must respect.

---

## R0.1 — Seller runtime

**Decision**: Node.js 20+ LTS, TypeScript. The Phase 1 spike ships a JS seller; the Go seller is a Phase 2 hardening task (H1).

**Rationale**: User direction + spec §6's "smallest credible step" criterion. JS↔JS removes all go-libp2p ↔ js-libp2p interop risk from v1. No changes to the Go tree are required for Phase 1 success. The existing `impl/typescript/` package is the natural home.

**Alternatives considered**:
- *Go seller (spec.md FR-B08 as-written)*: larger v1 scope, real interop value, carries real risk of handshake / yamux / Noise version skew. Moved to H1.
- *Browser ↔ browser via WebRTC signalling*: out of scope for 012 (it's 013/014).
- *Plain-HTTP REST seller, no libp2p*: rejected. Loses PeerID auth, loses the "browser is a real Neuron peer" claim, and forks the wire format — Option 5.4 in the scoping analysis.

**Constraint**: contracts in `contracts/` are the single source of truth. When H1 lands, the Go seller MUST pass the same contract tests.

---

## R0.2 — js-libp2p version + dependency pinning

**Decision (resolved at T001 pin time, 2026-04-20)**: `libp2p@3.2.2` line — newer major than the v2 line this entry originally proposed; v3 shipped a redesigned stream API (event-based with `addEventListener('message', …)` + `stream.send()` instead of async-iterator source/sink) but keeps the same three-transport primitives. Matching pins from first install:
- `libp2p@3.2.2`
- `@libp2p/websockets@10.1.10` (no `/filters` subpath in v10 — loopback gating is done via `connectionGater.denyDialMultiaddr` override instead)
- `@chainsafe/libp2p-noise@17.0.0`
- `@chainsafe/libp2p-yamux@8.0.1`
- `@libp2p/identify@4.1.2`
- `@multiformats/multiaddr@13.x`

**Rationale**: v3 was the current stable line at spike time and is the only one with first-party browser WebSocket support for Noise XX + yamux. The upgrader uses `Promise.withResolvers()`, which requires Node ≥ 22 / 21.12 / 20.12 — a one-line polyfill covers older runtimes (see `tests/smoke/fixture-server.ts`). Exact pinning (no `^`) should land in `package.json` during Phase 1G polish; currently the pins ARE recorded in `pnpm-lock.yaml` but ranges remain in `package.json`.

**Alternatives considered**:
- *js-libp2p v1.x / v2.x*: superseded by v3. v2's async-iterator stream API was the target of this research entry originally, but by install time v3 was shipping and adopting it avoided a migration later.
- *Hand-rolling libp2p on top of raw `WebSocket`*: Option 5.4 style fork — rejected for the same reasons as R0.1.

**Constraints**:
- `package.json` pins exact versions for the 6 libp2p packages during T037 polish (currently on `^` ranges).
- Node ≥ 20.12 / 21.12 / 22.0 for the `Promise.withResolvers` API; older runtimes polyfilled in `fixture-server.ts`. H6 removes the polyfill when `package.json` engines bumps to ≥ 22.
- Browser default `connectionGater` denies loopback + insecure `/ws`; both Phase 1 clients MUST override `denyDialMultiaddr` until H2 (wss://) lands.

---

## R0.3 — Local transport: `ws://` vs `wss://`

**Decision**: Phase 1 uses `ws://localhost:8080` for the demo. Phase 2 H2 introduces `wss://` via Caddy + Let's Encrypt on a dedicated subdomain.

**Rationale**: Browsers permit `ws://` from pages served over `http://localhost` (mixed-content rules exempt loopback). Real TLS certs for localhost require mkcert, device trust-store trickery, or Caddy internal CA — all of which add a full day of setup for zero demo value. Deferring WSS to H2 is honest and documented.

**Alternatives considered**:
- *mkcert + self-signed wss://*: works but adds "trust this certificate" friction to every reviewer. Phase 2 pays this cost once, properly.
- *WebTransport*: rejected per spec — WSS only in v1.
- *HTTPS page + WSS from day 1*: forces the whole TLS-cert question upfront; Phase 1 scope-creep trap.

**Constraint**: `browser-client/transport.ts` MUST reject any bootstrap multiaddr whose scheme is not `/ws` (localhost loopback) or `/wss` (production), and MUST emit a clearly-categorised configuration error per FR-B31.

---

## R0.4 — Bootstrap JSON delivery

**Decision**: The Node.js seller generates `bootstrap.json` at startup (derived from its freshly-generated keypair and the port it binds) and writes it to `examples/browser-demo/public/bootstrap.json`. Vite's dev server serves it at the same origin as the browser bundle. FR-B24 (same-origin, no CORS) is preserved.

**Rationale**: PeerID is derived from the seller's keypair; the keypair is fresh on every demo run (mirroring the browser's ephemeral-identity stance); therefore the PeerID cannot be baked in at repo commit time. The orchestrator script `scripts/run-demo.ts` starts the seller, waits for it to print its multiaddr, writes the bootstrap file, then starts Vite.

**Alternatives considered**:
- *Seller serves `/bootstrap.json` from its own Node HTTP handler*: wastes a port, complicates CORS story. Rejected.
- *Static `bootstrap.json` committed to repo*: would require stable committed seller keypair, which contradicts the demo's "fresh keypair each run" principle. Rejected.
- *Embedded in `index.html`*: couples page rebuild to seller start — clumsy. Rejected.

**Constraint**: `scripts/run-demo.ts` owns the startup ordering. If the seller crashes before writing `bootstrap.json`, the orchestrator must fail loudly, not leave a stale file.

---

## R0.5 — In-stream TopicAdapter framing

**Decision**: `[uint32 big-endian length][canonical-JSON TopicMessage]`, identical to Spec 009's FR-D22 / Spec 012's FR-B12. Shared implementation at `impl/typescript/src/wire/frame.ts`.

**Rationale**: Using identical framing for control-plane envelopes and data-plane chunks minimises code surface and removes the "two framings to debug" class of bug. `wire/` is the right home because it already holds canonical-JSON helpers that the frame reader/writer depend on.

**Alternatives considered**:
- *MsgPack / CBOR for control envelopes*: smaller wire, but diverges from Spec 004's canonical-JSON commitment.
- *Varint length prefix (libp2p-style msg-io)*: would tie us to an external library version and diverge from Go's `framing.go`. Rejected.

**Constraint**: the new `wire/frame.ts` must produce byte-for-byte identical output to `impl/golang/internal/delivery/framing.go` for the same input. A targeted test vector (fixture file) gates this.

---

## R0.6 — Mock escrow shape

**Decision**: Seller-only in-memory escrow with 4 states (`proposed`, `funded`, `released`, `refunded`). Phase 1 exercises `proposed → released` on successful delivery. The browser never inspects escrow state; it only sees the seller's `paymentDetails` and `invoiceAck` envelopes.

**Rationale**: FR-B18 explicitly locks Phase 1 to the existing mock pattern (`payment.MemoryEscrow` equivalent). Buyer visibility into escrow is deferred to H1+ when on-chain escrow lands.

**Alternatives considered**:
- *Port `impl/golang/internal/payment/mock_escrow.go` to TypeScript 1:1*: overkill; most states are never visited by Phase 1's flow. A minimal mock is honest.

**Constraint**: `server-demo/mock-escrow.ts` tests cover the two transitions the flow actually uses; full escrow state-machine fidelity is a Go-seller concern (H1).

---

## R0.7 — Buyer state machine subset (Clarify Q1)

**Decision**: 4-message minimal subset per Clarify Q1: `serviceRequest` → `paymentDetails` → `connectionSetup` → `invoiceAck`. Buyer state machine = 6 states matching spec.md Key Entities.

**Rationale**: already finalised in spec.md. Captured here for research-phase completeness.

**Alternatives considered**: full escrow observation — deferred to v2 (H1 gating).

---

## R0.8 — ECIES in-browser (FR-B16)

**Decision**: Reuse `@noble/secp256k1` (already imported by `keylib`) for ECDH, `@noble/hashes/hkdf` for key derivation, `@noble/ciphers/aes/gcm` for AES-256-GCM. Info string `"neuron-multiaddr-v1"` (matches Go `internal/delivery/ecies.go`).

**Rationale**: bit-compatible with the Go implementation. All three libraries are actively maintained, browser-friendly, and already half-present in this repo's TS package.

**Alternatives considered**:
- *WebCrypto `crypto.subtle`*: covers AES-GCM and HKDF but not secp256k1 ECDH. Mixing WebCrypto + noble adds complexity with no benefit. Rejected.
- *Port Go's ECIES directly*: already covered by noble primitives. Unnecessary re-implementation.

**Constraint**: Phase 1 ships a test vector in `tests/` that decrypts a ciphertext fixture produced by Go's `internal/delivery/ecies.go`. This gates FR-B16 compliance and pre-validates the Go interop path (H1).

---

## R0.9 — Error taxonomy (Clarify Q4 / FR-B35)

**Decision**: Profile-local identifiers of the form `NEURON-BROWSER-NNN` in v1. Phase 2 H5 aligns them with Spec 006's `NEURON-{DOMAIN}-{NNN}` scheme before merge to main.

**Rationale**: already finalised in spec.md. Listed here so the code-level `errors.ts` file has a clear research-phase anchor.

**Proposed numbering scheme** (informative, locked in `browser-client/errors.ts`):

| Range | Category |
|-------|----------|
| 001–019 | Configuration (bootstrap, unsupported browser) |
| 020–039 | Transport (TLS, WS dial, port blocked) |
| 040–059 | Handshake (Noise, PeerID mismatch) |
| 060–079 | Signature (envelope verification) |
| 080–099 | Hash mismatch / data integrity |
| 100–119 | Size cap / framing |
| 120–139 | Timeout / read-idle |
| 140–159 | Buyer-flow state-machine errors |

---

## R0.10 — Read-idle timeout (Clarify Q5 / FR-B34)

**Decision**: Hard upper bound 30 s in the FR. Implementation constant 15 s in Phase 1 (recorded in `tasks.md`).

**Rationale**: spec clause says 30 s is the ceiling; 15 s gives the demo responsiveness without risking false timeouts on slow mobile hotspot conditions.

**Constraint**: constant is named (`READ_IDLE_MS`) and centralised in `browser-client/file-receive.ts`. Phase 2 H6 tightens if real measurements recommend.

---

## R0.11 — UI framework for the demo page

**Decision**: **No framework.** Plain DOM + TypeScript. The browser-client's `ui/ledger.ts` and `ui/status.ts` render into pre-existing `<section>` elements in `index.html` via `textContent` / `appendChild`.

**Rationale**: This is a spike. React / Vue / Svelte add build-time complexity and a large dependency surface for a page that has ~5 DOM elements. A no-framework page is reviewable in one sitting and has a smaller attack surface for the XSS-sensitive signature ledger.

**Alternatives considered**:
- *React + vite-plugin-react*: 30–60 seconds of dev-server startup added, 150 KB of JS bundle for a page that needs <10 KB of app logic. Overkill.
- *Lit / web components*: plausible, but another dependency to review. Defer until Phase 2 if UI complexity grows.

**Constraint**: all DOM writes from `ui/*` go through `textContent` or equivalent — never `innerHTML` — so a malicious signed-envelope sender-address cannot inject HTML into the ledger.

---

## R0.12 — Node.js WS listener binding

**Decision**: bind to `127.0.0.1:8080` (loopback only) in Phase 1. `0.0.0.0` binding is never used in v1.

**Rationale**: defence-in-depth. A developer running the demo on a corporate laptop with a shared Wi-Fi has no business accepting inbound connections from the LAN. Binding to loopback means only the local browser can connect.

**Constraint**: `server-demo/index.ts` hard-codes `127.0.0.1` unless an explicit `--bind` flag is passed (which Phase 1 doesn't use). `--bind 0.0.0.0` is reserved for Phase 2 H2 deployment.

---

## Summary table

| Unknown | Decision | Gate |
|---|---|---|
| Seller runtime | Node.js (Phase 1); Go in H1 | spec.md FR-B08 (Go path) moved to H1 |
| js-libp2p version | v2.x, exact-pinned | `package.json` records pin |
| Local transport | `ws://localhost` | H2 promotes to wss:// |
| Bootstrap delivery | Seller generates at startup, Vite serves same-origin | `scripts/run-demo.ts` owns startup ordering |
| Framing | `[uint32 BE len][canonical-JSON]`, shared in `wire/frame.ts` | Byte-for-byte fixture test vs Go |
| Mock escrow | `proposed → released` only in v1 | Full lifecycle is H1 |
| Buyer state machine | 4-message subset | spec.md Clarify Q1 |
| ECIES in browser | `@noble/*` primitives | Go-vector decrypt test |
| Error IDs | `NEURON-BROWSER-NNN` profile-local | H5 aligns to 006 |
| Read-idle | 15 s actual / 30 s cap | H6 tightens if measured |
| UI framework | None | No dep added |
| Node listener bind | `127.0.0.1:8080` | H2 opens to public |

All Phase 0 unknowns resolved. No `[NEEDS CLARIFICATION]` markers remain. Proceed to Phase 1.
