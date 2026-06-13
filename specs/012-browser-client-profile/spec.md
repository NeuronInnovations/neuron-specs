# Feature Specification: Browser Client Profile

**Feature Branch**: `012-browser-client-profile`
**Created**: 2026-04-20
**Status**: Draft

## Related Specs

**In-repo:**

- **001 NeuronAccount**: Child agent identity, ephemeral account profile, DID derivation.
- **002 Key Library**: secp256k1 keypair generation (FR-003), RFC 6979 deterministic signatures (FR-014/017), Keccak-256, libp2p PeerID derivation (FR-006), DID:key encoding, ECIES for multiaddr encryption.
- **004 Topic System**: TopicMessage envelope (FR-T02/T03), canonical JSON serialization, `TopicAdapter` interface (FR-T10), payload extensibility (FR-T20).
- **008 Payment**: Buyer-side negotiation messages `serviceRequest`, `paymentDetails`, `connectionSetup` (FR-P33–P35), `invoiceAck`, mock escrow adapter, SHA-256 invoice hash (FR-P20).
- **009 P2P Data Delivery**: Length-prefixed frame protocol (FR-D22), file-transfer metadata frame, libp2p stream protocol negotiation, `ConnectionSetup` bridge.
- **006 Protocol Determinism**: Canonical JSON (FR-W01–W10), signature format `R‖S‖V` (FR-W07).

**Does NOT amend:**

- **009 FR-D25 (transports)**: Spec 009 requires QUIC + WebRTC as the minimum delivery-node transport set with WebTransport recommended. This spec defines a *client profile* that uses WSS. It does not modify 009's obligations on delivery nodes.

**External:**

- **libp2p WebSocket transport**: `/wss` multiaddr, TLS 1.3 underlying, Noise XX handshake on top.
- **js-libp2p** ecosystem (browser reference): `@libp2p/websockets`, `@chainsafe/libp2p-noise`, `@chainsafe/libp2p-yamux`. Specific version pinning is a plan-level concern.
- **IETF RFC 6455**: The WebSocket Protocol.
- **IETF RFC 6066**: TLS Server Name Indication (for multi-tenant deployments).

## Constitution Notes

- **Principle VIII (Hedera Transport Binding)**: HCS remains a first-class adapter at the protocol level and is retained by the server in this profile. The browser profile is a deliberate subset that omits HCS — it does not remove HCS from Neuron. The on-chain audit gap for browser-originated messages is accepted in v1 and tracked as Deferred v2b (below).
- **Principle XI (Verifiable Execution)**: Verifiability is preserved in-browser via per-envelope signature recovery and payload SHA-256 comparison against the invoice hash. No server-side oracle is required to validate the client's view of the transaction.
- **Principle VI (Language-Neutral Protocol)**: The spec is language-neutral; concrete js-libp2p and go-libp2p bindings are reference implementations named in `plan.md`, not normative requirements in this spec.

## Clarifications

### Session 2026-04-20

- Q: What buyer-side state machine shape should the browser implement — the minimal 4-message subset or full escrow-lifecycle observation? → A: Minimal 4-message subset (`serviceRequest` → `paymentDetails` → `connectionSetup` → `invoiceAck`). Full escrow-lifecycle observation is deferred to v2, gated on real escrow landing.
- Q: Does the bootstrap JSON need a formal schema declared in this spec, or can the shape live in plan-level artifacts? → A: plan-level. `data-model.md` declares a minimal schema with exactly the fields FR-B23 enumerates plus a top-level `version` tag; unknown top-level keys are rejected.
- Q: Within a single page lifecycle, does a second "Buy" click reuse the ephemeral identity or rotate to a fresh one? → A: Rotate. Each "Buy" action generates a fresh secp256k1 keypair and derived identifiers. Prior ledger entries remain visible in-page but are not cryptographically linked to the new identity. See FR-B36.
- Q: Should the browser's failure categories map onto Spec 006's `NEURON-{DOMAIN}-{NNN}` error taxonomy now, or stay profile-local for v1? → A: Profile-local in v1 (`NEURON-BROWSER-NNN`). Alignment with Spec 006 is a follow-up and MUST be resolved before this branch merges to main. See FR-B35.
- Q: Is the data-plane read-idle timeout a spec-level FR or a plan/task-level decision? → A: Spec-level upper bound (≤ 30 s) via FR-B34; the implementation constant is picked in `plan.md` and bounded by that FR.

## Out of Scope (v1)

- **HCS from the browser.** No reads or writes to Hedera Consensus Service from the browser client. The browser does not publish heartbeats, control-plane envelopes, or any other data to HCS. **Consequence**: browser-originated envelopes carry no on-chain audit receipt in v1. Server-originated envelopes retain their normal HCS audit trail because the server is unchanged. See Deferred v2b.
- **Hedera mirror-node polling from the browser.** No browser-originated fetches to `testnet.mirrornode.hedera.com` or equivalent.
- **EIP-8004 / Identity Registry on-chain lookup from the browser.** Seller identity is loaded from a static bootstrap JSON served alongside the page.
- **Real escrow / on-chain settlement.** The seller retains the existing mock escrow (`payment.MemoryEscrow`) for this profile.
- **Heartbeat publisher.** Spec 005's liveness machinery does not run in the browser.
- **Validator observer.** Spec 010's evidence loops do not run in the browser.
- **Browser-to-browser delivery.** Only browser-to-server is in scope.
- **WebTransport, WebRTC, or any non-WSS browser transport.** A single browser transport (WSS) in v1.
- **Persistent identity.** No wallet UX, mnemonic recovery, key export, or long-lived identity. Every page load produces a fresh key.
- **Service Workers, Background Sync, Push, PWA installation, multi-tab coordination.**
- **Stream content encryption.** libp2p Noise covers transport security; application-layer content encryption is a dApp-level concern per Spec 009.

## Deferred to Later Versions

- **v2a — WebTransport profile.** Same client shape, HTTP/3 / QUIC-based browser transport. Depends on `js-libp2p` WebTransport maturity.
- **v2b — HCS-anchored browser audit trail.** v1 ships with a known audit gap: the browser produces signed envelopes, but none of its outbound messages land on HCS and therefore none receive an on-chain receipt. The server's own publishes retain their HCS audit trail unchanged. v2 will close this gap via one or more of: (a) browser-to-mirror reads via CORS-enabled endpoints for independent verification; (b) a server-side publish proxy that relays browser-produced envelopes to HCS under the browser's signature; (c) both. Until then, this profile explicitly accepts that browser-originated control-plane messages are ephemeral and un-anchored, and the demo report MUST surface this fact visibly.
- **v2c — Persistent-identity wallet profile.** Encrypted IndexedDB key store, mnemonic import/export, account recovery flow.
- **v3 — Browser-to-browser.** WebRTC DataChannel over libp2p with a signaling path (likely the 011 relay or a dedicated signaler).
- **v4 — On-chain registry resolution.** Browser consults the Identity Registry (Spec 007) to resolve seller identity without a bootstrap JSON.

These items are tracked so that they do not leak into v1 scope; each warrants its own spec (013, 014, …) when prioritised.

## Assumptions

- The Neuron seller is reachable on the public internet at a valid TLS subdomain terminating to a libp2p WebSocket listener. Certificate issuance and renewal happen out of band (e.g., via a reverse proxy such as Caddy with Let's Encrypt). How TLS is terminated is an operations concern, not a spec-layer concern.
- Browser clock is within ±2 minutes of wall-clock time, sufficient for TopicMessage timestamp and TLS certificate validity checks.
- The Spec 008 negotiation for the spike is single-offer, single-delivery (no multi-round bargaining, no partial fulfilment).
- The bootstrap JSON is *static-and-stale* in v1: it is served as a regular HTTP resource alongside the page and is updated out of band. No signing, versioning, or freshness protocol is required.
- The demo payload is a small JPEG (≤ 100 KB). The 009 framing permits much larger payloads; v1 success criteria target the 100 KB case only.
- js-libp2p and go-libp2p versions selected for the profile are interop-tested as a paired set; version pinning is a plan/task artifact.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Complete a Browser-to-Server Neuron Transaction End-to-End (Priority: P1)

A consumer opens a URL in a browser. Behind the page, a thin Neuron client connects to a public Neuron seller, negotiates the exchange of a 100 KB JPEG, receives the file over a single WSS libp2p stream, verifies the file's SHA-256 in-browser, and renders the image.

**Why this priority**: This is the entire reason the profile exists. Without P1, there is no browser surface. Every other story is a refinement of this one.

**Independent Test**: Point a fresh Chromium profile at the demo URL, click a single "Buy" button, and observe the image render with a "verified" indicator within the budget in SC-03. No backend proxy, no native app, no CLI.

**Acceptance Scenarios**:

1. **Given** a reachable Neuron seller with a valid TLS certificate and a bootstrap JSON served at the same origin, **When** the user opens the page and clicks "Buy", **Then** within the SC-03 budget the JPEG renders in the page and a "verified" indicator is visible.
2. **Given** the session is underway, **When** the server completes delivery, **Then** the in-page ledger shows at least the `serviceRequest`, `paymentDetails`, `connectionSetup`, and `invoiceAck` envelopes with each signature marked as verified by the browser.
3. **Given** a successful session, **When** the transaction completes, **Then** the browser's reported SHA-256 of the received payload matches the hash declared in the seller's invoice byte-for-byte.

---

### User Story 2 — Reject Tampered or Mis-Signed Envelopes Without Partial Delivery (Priority: P2)

If any inbound envelope in the negotiation flow fails signature verification, or if the payload's SHA-256 does not match the invoice hash, the browser aborts the transaction and surfaces a specific, user-visible failure. No partial image renders. No silent fallback.

**Why this priority**: The browser's job is not just to "show the image". It is to demonstrate that a browser can verify Neuron's cryptographic claims independently of the server it is talking to. A profile that renders tampered payloads would be worse than useless.

**Independent Test**: Replay a recorded transcript with one envelope's signature bit-flipped, then re-run the session against a test harness that feeds the tampered stream into the browser. The browser must abort before rendering the image and must display a verification-failure message that identifies the specific envelope that failed.

**Acceptance Scenarios**:

1. **Given** a transaction is in flight, **When** the browser receives a `paymentDetails` envelope whose signature does not recover to the expected seller identity, **Then** the browser immediately aborts the transaction, renders no image, and displays a verification-failure indicator naming the failing envelope type.
2. **Given** the file transfer has completed, **When** the browser's computed SHA-256 of the reassembled payload differs from the hash declared in the `paymentDetails` invoice, **Then** the browser discards the payload without rendering and displays a SHA-256-mismatch error.
3. **Given** the server's Noise-established PeerID does not match the PeerID declared in the bootstrap JSON, **When** the handshake completes, **Then** the browser closes the connection before any application-level messages are exchanged.

---

### User Story 3 — Start a Fresh Ephemeral Session on Every Page Load (Priority: P3)

Every visit to the page produces a new, in-memory-only secp256k1 identity. No state from a prior session leaks into a new one. Closing the tab destroys all session state. The browser never writes key material to persistent storage.

**Why this priority**: This is the v1 contract with the user and with Security. It is what lets us defer the entire "browser wallet" question to a later spec without hand-waving about XSS or key exfiltration today.

**Independent Test**: Open the page in one tab, complete a transaction, note the PeerID shown in the ledger. Close the tab. Clear all in-memory state (or open a new private window). Re-open the page. Confirm the new PeerID is different and that inspecting `localStorage`, `sessionStorage`, `IndexedDB`, and cookies via browser DevTools shows no key-related entries at any point.

**Acceptance Scenarios**:

1. **Given** a fresh page load, **When** the browser client initialises, **Then** a newly generated secp256k1 keypair exists in memory and no key-related entries exist in `localStorage`, `sessionStorage`, `IndexedDB`, or cookies.
2. **Given** a transaction has completed, **When** the user reloads the page, **Then** the new session has a different PeerID, EVMAddress, and DID:key from the prior session.
3. **Given** a transaction is in flight, **When** the user closes the tab, **Then** no artefact of the session remains inspectable via browser storage or cookies when the browser is later re-opened.

---

### Edge Cases

- **Bootstrap JSON missing, malformed, or cross-origin**: browser must display a specific configuration error and not attempt any network dial to an unrelated origin.
- **WSS connection failure (TLS error, DNS failure, server unreachable, port blocked)**: browser must distinguish these from signature-verification failures and report accordingly.
- **Noise handshake yields an unexpected server PeerID**: browser must refuse to proceed, even if the TLS layer accepted the certificate.
- **User clicks "Buy" multiple times in rapid succession**: only one transaction is initiated; further clicks during an active session are no-ops or explicitly declined.
- **Tab closes, crashes, or is suspended mid-transaction**: no resumption is attempted on reopen. A fresh session starts from scratch.
- **Seller-declared payload size exceeds a sane per-session cap** (e.g., > 10 MiB in v1): browser declines delivery and aborts.
- **Seller delivers fewer chunks than declared in metadata**: browser times out after a bounded read-idle period and aborts.
- **Browser lacks required crypto primitives** (extremely old browsers): browser displays an "unsupported browser" error before attempting the transaction.
- **Bootstrap JSON specifies a non-WSS multiaddr** (e.g., QUIC, WebRTC, TCP): browser refuses to dial; this profile is WSS-only.
- **TopicMessage signature verifies but sender is not the expected seller**: browser treats this as a tampering indicator and aborts.

## Requirements *(mandatory)*

### Functional Requirements

**Identity**

- **FR-B01**: Browser MUST generate a fresh secp256k1 keypair on every page load; no prior identity is reused.
- **FR-B02**: Browser MUST derive `EVMAddress`, libp2p `PeerID`, and `DID:key` from the generated keypair consistent with Spec 002 (FR-003, FR-006).
- **FR-B03**: Browser MUST NOT persist private-key material, derived identifiers, or session state to `localStorage`, `sessionStorage`, `IndexedDB`, cookies, `window.name`, or any other browser storage mechanism.
- **FR-B04**: Browser MUST NOT expose private-key material to any error message, log surface, analytics pipeline, or third-party resource.

**Transport**

- **FR-B05**: Browser client MUST connect to the Neuron seller exclusively over a TLS-secured WebSocket (WSS) transport bound by libp2p, using the `/wss` multiaddr scheme.
- **FR-B06**: Browser MUST perform mutual authentication via the Noise XX handshake and MUST verify that the server-advertised PeerID matches the PeerID declared in the bootstrap JSON before sending any application-level message.
- **FR-B07**: Browser MUST abort the connection if TLS certificate validation fails or if the Noise handshake yields a server PeerID that does not match the bootstrap JSON's expected value.
- **FR-B08**: The Neuron seller MUST expose a WSS listen multiaddr alongside its existing transports, and MUST NOT remove or downgrade those existing transports to meet this profile.
- **FR-B09**: Browser MUST reject any bootstrap JSON that advertises a non-WSS scheme; this profile does not fall back to QUIC, WebTransport, WebRTC, or raw TCP from the browser.

**Control plane (in-stream TopicAdapter)**

- **FR-B10**: Browser and server MUST exchange signed TopicMessage envelopes conforming to Spec 004 over an in-stream TopicAdapter — a libp2p stream whose contents are Spec 004 envelopes.
- **FR-B11**: The in-stream TopicAdapter MUST use a libp2p stream protocol identifier distinct from the data-plane file-transfer stream protocol identifier.
- **FR-B12**: In-stream TopicAdapter envelopes MUST use the same length-prefixed framing used by Spec 009 file frames: `[uint32 big-endian length][canonical-JSON payload]`.
- **FR-B13**: Browser MUST verify the signature on every inbound TopicMessage envelope before dispatching it to the buyer state machine.
- **FR-B14**: Browser MUST reject any inbound TopicMessage whose recovered sender address does not equal the expected seller's `EVMAddress` pinned in the bootstrap JSON.

**Payment (buyer-side subset of Spec 008)**

- **FR-B15**: Browser MUST implement the buyer half of the Spec 008 negotiation flow in a single-offer single-delivery form: publish `serviceRequest`, consume `paymentDetails` and `connectionSetup`, publish `invoiceAck`.
- **FR-B16**: Browser MUST decrypt the `connectionSetup.encryptedMultiaddrs` field via the ECIES scheme declared by Spec 009 (secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM, info string `"neuron-multiaddr-v1"`). When the resulting multiaddr is identical to the bootstrap WSS endpoint already in use, the browser MUST still exercise the decrypt path as protocol conformance.
- **FR-B17**: Browser MUST compute and verify the payload SHA-256 against the invoice-declared hash per Spec 008 (FR-P20); mismatch MUST abort the transaction without rendering.
- **FR-B18**: Server-side escrow in this profile MUST be the existing mock escrow; no on-chain settlement is performed in v1.

**Delivery (per Spec 009)**

- **FR-B19**: File transfer MUST use the Spec 009 frame protocol: frame 0 = JSON metadata (filename, size in bytes, content-type, SHA-256 hex), frames 1..N = raw payload chunks, each prefixed with `[uint32 big-endian length][payload]`.
- **FR-B20**: Browser MUST enforce a maximum single-frame payload size consistent with Spec 009 (4 MiB).
- **FR-B21**: Browser MUST enforce a maximum total-payload size for this profile. The v1 cap is 1 MiB; a payload declaring a larger size MUST be refused before any chunk is read. Per-frame cap remains 4 MiB per FR-B20; the total-payload cap is the binding v1 limit for SC-02 / SC-03 measurement.
- **FR-B22**: Browser MUST surface a user-visible error if the reassembled payload's SHA-256 differs from the metadata-declared hash.

**Bootstrap**

- **FR-B23**: Browser MUST load the seller's identity (`EVMAddress`, `PeerID`), WSS multiaddr, control-plane stream protocol ID, and data-plane stream protocol ID from a static bootstrap JSON served same-origin with the browser bundle.
- **FR-B24**: Browser MUST NOT fetch bootstrap or configuration data cross-origin in v1. No CORS-dependent resource is introduced by this profile.
- **FR-B25**: Browser MUST NOT perform any on-chain registry lookup (EIP-8004 Identity Registry or any other) in v1.

**Lifecycle**

- **FR-B26**: A browser-client session MUST be scoped to a single page lifecycle. On reload, close, or top-level navigation, all session state MUST be discarded.
- **FR-B27**: Browser MUST NOT register a Service Worker, MUST NOT register for Background Sync or Push, and MUST NOT install as a PWA.
- **FR-B28**: Browser MUST reject a second concurrent "Buy" invocation while a prior transaction is active in the same page lifecycle.
- **FR-B36**: Each user-invoked "Buy" action within a single page lifecycle MUST generate a fresh secp256k1 keypair and derived identifiers. Prior transaction ledger entries remain visible in the page but MUST NOT be cryptographically linked to the new identity. The session-bound buyer state machine MUST reset to idle before the new keypair is used.

**Verification UX**

- **FR-B29**: Browser MUST surface an in-page signature-chain ledger showing, for every exchanged TopicMessage envelope: message type, sender address, timestamp, signature status (OK/FAIL), and signed-payload hash.
- **FR-B30**: Browser MUST display, upon successful completion, a verified status that references the payload SHA-256 and the seller's pinned identity.
- **FR-B31**: Browser MUST display, upon any failure, a specific failure category (transport / handshake / signature / hash-mismatch / size-cap / timeout / configuration) and MUST NOT present a generic "something went wrong" without categorisation.

**Demo**

- **FR-B32**: Browser client MUST present a single user-invoked action ("Buy") that triggers the full flow end-to-end; no configuration screens, no multi-step wizards in v1.
- **FR-B33**: Upon successful completion, the browser MUST render the received JPEG in the page and MUST display the verified status from FR-B30.

**Resilience / Timeouts**

- **FR-B34**: Browser MUST abort a data-plane receive session if no bytes arrive for ≥ 30 seconds after the last data-frame activity. The abort MUST surface a `timeout` failure category per FR-B31. The exact timeout constant used in implementation is a plan-level decision bounded by this upper limit.

**Error Taxonomy**

- **FR-B35**: Failure categories produced by FR-B31 MUST be encoded as profile-local identifiers of the form `NEURON-BROWSER-NNN` in v1. Alignment with the Spec 006 error taxonomy (`NEURON-{DOMAIN}-{NNN}`) is tracked as a follow-up and MUST be resolved before this branch merges to main.

### Key Entities

- **Browser Session**: An in-memory ephemeral context spanning a single page lifecycle. Holds the generated secp256k1 keypair, derived identifiers, the active libp2p host, and the running buyer state machine. Discarded on reload/close.
- **Bootstrap JSON**: A static same-origin resource enumerating the seller's identity and endpoints. Contains at least: seller `EVMAddress`, seller `PeerID`, seller WSS multiaddr, control-plane stream protocol ID, data-plane stream protocol ID. Out-of-band updated.
- **In-Stream TopicAdapter**: A Spec 004 `TopicAdapter` implementation that uses an open libp2p stream as its transport. Semantically equivalent to other TopicAdapter implementations (HCS, Kafka, ERC-log) from the envelope's perspective; differs only in the transport layer.
- **Signature-Chain Ledger**: The browser's in-page record of every exchanged TopicMessage envelope with verification status, used by the user / reviewer to confirm end-to-end cryptographic integrity.
- **Buyer State Machine**: The buyer-side Spec 008 state machine subset needed for single-offer single-delivery: idle → request-sent → quote-received → connection-setup-received → invoice-ack-sent → complete, with explicit transitions to aborted on any verification failure.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-01**: A non-technical reviewer can complete the end-to-end browser demo from a fresh browser profile following only the written instructions, in under 2 minutes, with no terminal access.
- **SC-02**: Across 5 successive cold runs on Chromium, the median time from page load to first received data-plane byte is ≤ 8 seconds.
- **SC-03**: Across 5 successive cold runs on Chromium, the median time from page load to rendered-and-verified image is ≤ 15 seconds.
- **SC-04**: Every envelope entry in the in-page signature-chain ledger is independently verifiable against the pinned seller identity without contacting the server or any third party.
- **SC-05**: The browser client functions end-to-end on Chromium ≥ 120 and Firefox ≥ 115 (mandatory). On Safari ≥ 17 (best-effort) the client either completes the flow or displays a specific unsupported-environment error. Versions older than the mandatory minimums are out of scope; attempting the demo on them MUST produce the same unsupported-environment error, not a silent failure.
- **SC-06**: Inspection of browser storage (DevTools) at any point in the session shows zero key-related entries in `localStorage`, `sessionStorage`, `IndexedDB`, or cookies.
- **SC-07**: Injecting a single bit-flipped signature into any inbound envelope causes the browser to abort the transaction before any image bytes are rendered, with a failure category shown per FR-B31.
- **SC-08**: Closing the tab mid-transaction and re-opening the page produces a session with a different PeerID, EVMAddress, and DID:key from the prior session.
- **SC-09**: After DOMContentLoaded, the client's external network surface consists of exactly one same-origin bootstrap JSON fetch and one WSS connection to the seller. Page-load resources (HTML, JS bundle, CSS, fonts, favicons) are explicitly excluded from this count. Any additional network activity after DOMContentLoaded is a spec violation.
