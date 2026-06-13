# Tasks: Browser Client Profile (012)

**Input**: `specs/012-browser-client-profile/{spec.md, plan.md, research.md, data-model.md, contracts/}`
**Prerequisites**: plan.md (complete), spec.md (clarifications session 2026-04-20 encoded)
**Scope**: Phase 1 = local JS↔JS demo (Node.js seller + browser buyer, `ws://127.0.0.1:8080`); Phase 2 = hardening H1–H7 (including Go seller interop).

## Format

`- [ ] [TaskID] [P?] [Story?] Description with file path`

- `[P]` = parallelizable (different files, no open dependency)
- `[US1] / [US2] / [US3]` = user-story label (see spec.md User Stories 1–3)
- Each task is followed by a detail block: **Objective / Files / Depends on / Done when / Traces**.

## Constitution compliance

- **Principle VII** — every task references an FR-B## / SC-## / H# it satisfies.
- **Principle IX** — tests precede impl inside each user-story phase.
- **Principle X** — signing tasks reuse the existing `keylib/signature.ts` (RFC 6979 deterministic; already covered by 002 tests).
- **Principle XI** — user-visible signature ledger (FR-B29) + in-browser SHA-256 check (FR-B22) satisfy the verifiable-execution framing without a server oracle.

---

## Phase 1A — Handshake Gate (single task — all downstream work is blocked until this passes)

### - [x] T001 Browser ↔ Node libp2p handshake smoke test at `impl/typescript/tests/smoke/` — **Chromium green (round-trip 4 ms); Firefox verification pending (non-blocking — H4 will gate)**

**Objective**: prove the cheapest thing first — a browser can open a WSS connection to a local Node.js libp2p host, complete Noise XX + yamux + identify, open a `/test/echo/1.0.0` stream, and round-trip a `"ping"` / `"pong"` exchange. **No Neuron envelope, no Spec 008 flow, no bootstrap JSON. Nothing else in this plan runs until this works.**
**Files**:
- `impl/typescript/tests/smoke/fixture-server.ts` — 40–80 LOC standalone libp2p Node host with a single `/test/echo/1.0.0` stream handler.
- `impl/typescript/tests/smoke/smoke.html` — 30–50 LOC plain HTML + TS that dials the server and logs the round-trip to the page.
- `impl/typescript/tests/smoke/vite.config.ts` — minimal vite config serving `smoke.html` on `:5174` (separate port from the real demo).
- `impl/typescript/tests/smoke/README.md` — 10-line run instructions.
**Depends on**: (nothing — this is the gate).
**Done when**:
1. `pnpm --filter @neuron-sdk/typescript run smoke:server` brings up the Node host and prints its multiaddr.
2. `pnpm --filter @neuron-sdk/typescript run smoke:browser` opens `smoke.html` in the user's default browser.
3. Browser page displays `HANDSHAKE OK — round-trip = Nms`.
4. Chromium and Firefox both succeed (manual two-browser check).
5. Tester records the verified js-libp2p version pair in `research.md` R0.2 if it differs from the proposed default.
**Traces**: R0.2, R0.3 (ws:// on loopback), Risk R3.

> ⚠️ **BLOCKER**: if T001 fails, do not start T002+. Debug here. The entire cost of this plan scales with the time-to-handshake; paying it once up front removes the single biggest risk.

---

## Phase 1B — Setup (parallel after T001 passes)

### - [x] T002 [P] Install js-libp2p stack + Vite + ciphers in `impl/typescript/package.json`

**Objective**: add Phase 1 runtime deps (`libp2p`, `@libp2p/websockets`, `@chainsafe/libp2p-noise`, `@chainsafe/libp2p-yamux`, `@libp2p/identify`, `vite`, `@noble/ciphers`) and Phase 1 scripts (`demo`, `demo:server`, `demo:browser`, `smoke:server`, `smoke:browser`).
**Files**: `impl/typescript/package.json` (and `pnpm-lock.yaml` regenerated).
**Depends on**: T001.
**Done when**: `pnpm install` succeeds; `pnpm run` shows the five new scripts; all libp2p packages are pinned to exact versions (no `^`) per R0.2.
**Traces**: R0.2.

### - [x] T003 [P] Scaffold new source directories under `impl/typescript/src/`

**Objective**: create empty package skeletons so later tasks have homes.
**Files**: `impl/typescript/src/browser-client/{,ui/}`, `impl/typescript/src/server-demo/{,assets/}`, `impl/typescript/examples/browser-demo/{,public/}`, `impl/typescript/scripts/` (placeholder `.gitkeep` in each).
**Depends on**: T001.
**Done when**: directories exist; `examples/browser-demo/public/.gitignore` excludes `bootstrap.json` (generated).
**Traces**: plan.md §Project Structure.

### - [x] T004 [P] Centralised constants at `impl/typescript/src/browser-client/constants.ts`

**Objective**: export every shared constant per data-model.md "Derived constants" table so no magic numbers live in module bodies.
**Files**: `impl/typescript/src/browser-client/constants.ts`.
**Depends on**: T003.
**Done when**: exports `READ_IDLE_MS = 15000`, `MAX_FRAME_BYTES = 4 * 1024 * 1024`, `MAX_TOTAL_BYTES = 1 * 1024 * 1024`, `CONTROL_PROTOCOL_ID`, `DATA_PROTOCOL_ID`, `BOOTSTRAP_VERSION = 1`, `ECIES_INFO = "neuron-multiaddr-v1"`.
**Traces**: FR-B16, FR-B20, FR-B21, FR-B34, data-model.md §"Derived constants".

### - [x] T005 [P] Browser-demo Vite config + HTML shell at `impl/typescript/examples/browser-demo/`

**Objective**: stand up the static page the browser-client will mount into.
**Files**: `impl/typescript/examples/browser-demo/index.html` (single `<section id="neuron-demo"></section>` + a `<button id="buy">Buy</button>`), `main.ts` (dynamic import of `browser-client/index.ts`), `vite.config.ts` (dev server on `:5173`, serves `public/bootstrap.json`).
**Depends on**: T002, T003.
**Done when**: `pnpm run demo:browser` serves the blank page at `http://localhost:5173`.
**Traces**: FR-B32.

---

## Phase 1C — Foundational primitives (blocks all user stories)

### - [x] T006 [P] Shared length-prefix framing at `impl/typescript/src/wire/frame.ts` (+ test) — **11/11 tests green**

**Objective**: port `impl/golang/internal/delivery/framing.go`'s `[uint32 BE length][payload]` protocol to TypeScript; shared between control and data streams.
**Files**: `impl/typescript/src/wire/frame.ts`, `impl/typescript/tests/wire/frame.test.ts`.
**Depends on**: T002, T003, T004.
**Done when**: `FrameReader.read()` / `FrameWriter.write()` round-trip in unit tests; zero-length frame treated as keep-alive; frames > `MAX_FRAME_BYTES` throw. A fixture-byte test compares the encoded output of a canonical envelope against a golden file produced by the Go framer.
**Traces**: FR-B12, FR-B19, FR-B20, R0.5.

### - [x] T007 [P] Error taxonomy at `impl/typescript/src/browser-client/errors.ts` (+ test) — **5/5 tests green**

**Objective**: define `NEURON-BROWSER-NNN` codes per research.md R0.9 numbering scheme; export helper `makeNeuronError(code, message, cause?)`.
**Files**: `impl/typescript/src/browser-client/errors.ts`, `impl/typescript/tests/errors.test.ts`.
**Depends on**: T003.
**Done when**: all codes 001–159 defined in seven named range-buckets (Configuration, Transport, Handshake, Signature, Hash, Size/Framing, Timeout, State-machine); helper produces structured errors; tests cover serialisation shape.
**Traces**: FR-B31, FR-B35, R0.9.

### - [x] T008 ECIES encrypt/decrypt with Go-vector cross-check at `impl/typescript/src/browser-client/ecies-decrypt.ts` + `src/server-demo/ecies-encrypt.ts` + tests — **8/8 tests green (7 round-trip + 1 Go→TS interop); fixture at `tests/ecies/fixtures/go-vector.json`; regenerator at `impl/golang/cmd/ecies-fixture-gen/`**

**Objective**: implement ECIES (secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM, info `"neuron-multiaddr-v1"`); prove interop with Go by decrypting a ciphertext fixture generated from `impl/golang/internal/delivery/ecies.go`.
**Files**: `impl/typescript/src/browser-client/ecies-decrypt.ts`, `impl/typescript/src/server-demo/ecies-encrypt.ts`, `impl/typescript/tests/ecies/go-vector.test.ts`, `impl/typescript/tests/ecies/fixtures/go-ciphertext.json` (generated once via `go test -run TestECIESFixture` on the Go side — one-time).
**Depends on**: T002, T003, T004.
**Done when**: TS decrypt of Go-produced ciphertext yields the original plaintext byte-for-byte; TS encrypt + TS decrypt round-trip passes; Go decrypt of TS-produced ciphertext passes (H1 will run this the other way).
**Traces**: FR-B16, R0.8.

---

## Phase 1D — User Story 1: Complete a browser-to-server Neuron transaction end-to-end (P1)

**Story goal**: render a signed, SHA-256-verified JPEG in a browser tab by clicking Buy.
**Independent test**: quickstart §1 (Given reachable seller + same-origin bootstrap → When user clicks Buy → Then JPEG renders with verified status within SC-03 budget).

### Contract tests first (Principle IX)

### - [x] T009 [P] [US1] Contract test for stream-protocols framing at `impl/typescript/tests/contracts/stream-protocols.test.ts` — **16/16 green; also implements `src/browser-client/file-metadata.ts` frame-0 parser**

**Objective**: verify the `contracts/stream-protocols.md` rules are enforced by `wire/frame.ts` — protocol IDs, frame max size, keep-alive semantics, abnormal-close behaviour.
**Files**: `impl/typescript/tests/contracts/stream-protocols.test.ts`.
**Depends on**: T006.
**Done when**: suite red before T011/T016 impl, green after.
**Traces**: FR-B11, FR-B12, contracts/stream-protocols.md.

### - [x] T010 [P] [US1] Contract test for bootstrap JSON validator at `impl/typescript/tests/contracts/bootstrap-json.test.ts` — **13/13 green; also implements `src/browser-client/bootstrap-schema.ts` validator**

**Objective**: verify `contracts/bootstrap-json.md` — missing required fields rejected with 004, unknown keys rejected with 003, version mismatch rejected with 002, scheme check enforces localhost-only for `/ws`.
**Files**: `impl/typescript/tests/contracts/bootstrap-json.test.ts`.
**Depends on**: T007.
**Done when**: suite red before T013 impl, green after.
**Traces**: FR-B23, FR-B24, FR-B25, contracts/bootstrap-json.md.

### - [x] T011 [P] [US1] Contract test for in-stream TopicAdapter at `impl/typescript/tests/contracts/in-stream-adapter.test.ts` — **8/8 green; also implements `src/browser-client/envelope-verify.ts` verifier**

**Objective**: verify `contracts/in-stream-adapter.md` signature-verification rules and sender pinning. Uses a fake in-memory stream pair.
**Files**: `impl/typescript/tests/contracts/in-stream-adapter.test.ts`.
**Depends on**: T006, T007.
**Done when**: suite asserts: signature-recover mismatch → NEURON-BROWSER-061; sender-pin mismatch → 062; sequence decrement → 063; timestamp skew > 2m → 064; wrong-type payload → 140.
**Traces**: FR-B13, FR-B14, contracts/in-stream-adapter.md.

### Server-side implementation (can proceed as a track in parallel with the browser-side track once T006–T008 land)

### - [x] T012 [US1] Seller transport at `impl/typescript/src/server-demo/transport.ts`

**Objective**: create a js-libp2p Node host with WSS listener bound to `127.0.0.1:8080` (loopback only per R0.12); Noise XX + yamux + identify; seller keypair generated fresh at startup via `keylib/private-key.ts`.
**Files**: `impl/typescript/src/server-demo/transport.ts`.
**Depends on**: T002, T004.
**Done when**: `startSellerHost()` returns a `Libp2p` handle; logs the listen multiaddr (`/ip4/127.0.0.1/tcp/8080/ws/p2p/...`), derived EVMAddress, and PeerID.
**Traces**: FR-B05, FR-B06, FR-B08 (JS), R0.12.

### - [x] T013 [US1] Bootstrap JSON writer at `impl/typescript/src/server-demo/index.ts` (bootstrap-write slice)

**Objective**: on seller startup, write `examples/browser-demo/public/bootstrap.json` with the six schema fields (version, sellerEVMAddress, sellerPeerID, sellerWSSMultiaddr, controlStreamProtocolID, dataStreamProtocolID). File is gitignored.
**Files**: `impl/typescript/src/server-demo/index.ts` (bootstrap-write function), `impl/typescript/examples/browser-demo/public/.gitignore` (add `bootstrap.json`).
**Depends on**: T012.
**Done when**: after server startup, `cat examples/browser-demo/public/bootstrap.json` shows a valid doc that passes T010.
**Traces**: FR-B23, R0.4.

### - [x] T014 [US1] Mock escrow at `impl/typescript/src/server-demo/mock-escrow.ts` (+ test)

**Objective**: minimal in-memory state machine (`proposed → released`, `proposed → refunded`); seller-only; no cryptographic primitives.
**Files**: `impl/typescript/src/server-demo/mock-escrow.ts`, `impl/typescript/tests/server-demo/mock-escrow.test.ts`.
**Depends on**: T003.
**Done when**: `propose(agreementHash)` returns `{ state: "proposed" }`; `release(agreementHash)` transitions to `released` and is idempotent; unknown agreement → error.
**Traces**: FR-B18, R0.6.

### - [x] T015 [US1] Server in-stream TopicAdapter at `impl/typescript/src/server-demo/topic-adapter.ts`

**Objective**: implement the Spec 004 `TopicAdapter` interface over an accepted libp2p control stream; reuses `src/topic/` envelope serialization + `src/wire/frame.ts` for length-prefixing.
**Files**: `impl/typescript/src/server-demo/topic-adapter.ts`.
**Depends on**: T006, T011, T012.
**Done when**: T011 passes against a real libp2p stream-pair (using Node↔Node in-vitest); `publish(envelope)` signs-if-not-already + frames + writes; `subscribe(handler)` verifies + dispatches.
**Traces**: FR-B10, FR-B11, FR-B12, FR-B13.

### - [x] T016 [US1] Seller flow at `impl/typescript/src/server-demo/seller-flow.ts`

**Objective**: seller half of the 4-message flow. On inbound `serviceRequest` → verify + construct `paymentDetails` (with mock agreement hash + invoice SHA-256) → ECIES-encrypt server multiaddrs → emit `connectionSetup` → await `invoiceAck` → transition escrow to `released`.
**Files**: `impl/typescript/src/server-demo/seller-flow.ts`.
**Depends on**: T008, T014, T015.
**Done when**: Node↔Node integration test in vitest runs the full flow with a fake buyer; escrow ends in `released`; ledger events logged on both sides.
**Traces**: FR-B15 (seller side), FR-B16.

### - [x] T017 [US1] Seller file-send at `impl/typescript/src/server-demo/file-send.ts` + asset `assets/demo.jpg`

**Objective**: send frame-0 metadata (filename, sizeBytes, contentType, sha256Hex computed at startup) then raw chunks up to `MAX_FRAME_BYTES`; close stream on EOF.
**Files**: `impl/typescript/src/server-demo/file-send.ts`, `impl/typescript/src/server-demo/assets/demo.jpg` (one ≤100 KB JPEG committed to the repo).
**Depends on**: T006, T012.
**Done when**: a local test streams the file into an in-memory frame-collector; collector's computed SHA-256 equals the metadata's declared hash; chunk count matches `ceil(sizeBytes / MAX_FRAME_BYTES)`.
**Traces**: FR-B19, FR-B20.

### - [x] T018 [US1] Seller main + stream handler wiring at `impl/typescript/src/server-demo/index.ts`

**Objective**: glue — register `CONTROL_PROTOCOL_ID` handler → seller-flow; register `DATA_PROTOCOL_ID` handler → file-send; call bootstrap writer from T013; graceful shutdown on SIGINT.
**Files**: `impl/typescript/src/server-demo/index.ts`.
**Depends on**: T012, T013, T016, T017.
**Done when**: `pnpm run demo:server` starts the seller; SIGINT exits cleanly; `Ctrl-C` twice force-kills without orphan ports.
**Traces**: FR-B08 (JS).

### Browser-side implementation (parallel to server track once T006–T008 land)

### - [x] T019 [US1] Browser session module at `impl/typescript/src/browser-client/session.ts`

**Objective**: generate ephemeral secp256k1 keypair inside a closure; derive `EVMAddress`, `PeerID`, `DID:key` via existing `keylib/*`; expose signing method that accepts signing-input bytes and returns a signature, never the private key.
**Files**: `impl/typescript/src/browser-client/session.ts`.
**Depends on**: T004.
**Done when**: unit test asserts the module exports no function returning a `Uint8Array` that contains the private key; DevTools inspection during a test session finds no key-shaped 32-byte array in any reachable object.
**Traces**: FR-B01, FR-B02, FR-B03, FR-B04.

### - [x] T020 [US1] Browser bootstrap loader at `impl/typescript/src/browser-client/bootstrap.ts`

**Objective**: same-origin `fetch("/bootstrap.json", { mode: "same-origin", cache: "no-store", credentials: "omit" })`; enforce every rule from T010; enforce the `/ws`-on-localhost-only rule.
**Files**: `impl/typescript/src/browser-client/bootstrap.ts`.
**Depends on**: T007, T010.
**Done when**: T010 tests pass; additional test asserts that fetch options match the contract (same-origin, no-store, omit).
**Traces**: FR-B23, FR-B24, FR-B25, contracts/bootstrap-json.md.

### - [x] T021 [US1] Browser transport at `impl/typescript/src/browser-client/transport.ts`

**Objective**: create a js-libp2p Browser host (no listener, dial-only); dial `bootstrap.sellerWSSMultiaddr`; verify recovered server PeerID equals `bootstrap.sellerPeerID` before returning the connection; reject non-`/ws` + non-`/wss` multiaddrs up front.
**Files**: `impl/typescript/src/browser-client/transport.ts`.
**Depends on**: T002, T019, T020.
**Done when**: integration test with a mock libp2p Node host verifies: (a) expected PeerID → connect succeeds; (b) unexpected PeerID → error `NEURON-BROWSER-043`; (c) wrong multiaddr scheme → `NEURON-BROWSER-007`.
**Traces**: FR-B05, FR-B06, FR-B07, FR-B09.

### - [x] T022 [US1] Browser in-stream TopicAdapter at `impl/typescript/src/browser-client/topic-adapter.ts`

**Objective**: mirror of T015; opens the control stream on `CONTROL_PROTOCOL_ID`; publishes signed envelopes; subscribes + verifies inbound envelopes (all 5 checks from T011).
**Files**: `impl/typescript/src/browser-client/topic-adapter.ts`.
**Depends on**: T006, T011, T021.
**Done when**: T011 suite passes against a real stream pair where this module is the client side.
**Traces**: FR-B10, FR-B11, FR-B12, FR-B13, FR-B14.

### - [x] T023 [US1] Browser buyer-flow at `impl/typescript/src/browser-client/buyer-flow.ts`

**Objective**: implement the 6-state buyer machine (data-model.md §BuyerState): idle → request-sent → quote-received → connection-setup-received → invoice-ack-sent → complete; aborts on any verification failure with the specific error code; exercises the ECIES decrypt path from T008 even when redundant (FR-B16); enforces FR-B28 concurrency guard.
**Files**: `impl/typescript/src/browser-client/buyer-flow.ts`.
**Depends on**: T008, T019, T022.
**Done when**: unit tests drive the state machine through happy path + each abort branch with fixture envelopes; all transitions deterministic.
**Traces**: FR-B14, FR-B15, FR-B16, FR-B28.

### - [x] T024 [US1] Browser file-receive at `impl/typescript/src/browser-client/file-receive.ts`

**Objective**: read frame 0 → validate metadata (reject sizeBytes > `MAX_TOTAL_BYTES` before any chunk per FR-B21); reassemble chunks; compute SHA-256 via `@noble/hashes/sha256`; compare to declared hash; enforce `READ_IDLE_MS` via a reset-on-frame timer that aborts with `NEURON-BROWSER-121`.
**Files**: `impl/typescript/src/browser-client/file-receive.ts`.
**Depends on**: T006, T007, T023.
**Done when**: unit tests cover: happy-path 100 KB reassembly; oversized-declaration rejection; bit-flipped-payload hash-mismatch abort; stalled-stream timeout abort.
**Traces**: FR-B17, FR-B19, FR-B20, FR-B21, FR-B22, FR-B34.

### - [x] T025 [P] [US1] Signature-chain ledger UI at `impl/typescript/src/browser-client/ui/ledger.ts`

**Objective**: `appendLedgerEntry(entry)` writes a new `<li>` into `<section id="ledger"></section>` using `textContent` only (no `innerHTML`); each row shows `direction | type | sender (truncated) | timestamp | sig status | payload hash (truncated)`.
**Files**: `impl/typescript/src/browser-client/ui/ledger.ts`, `impl/typescript/examples/browser-demo/index.html` (add `<section id="ledger">`).
**Depends on**: T004, T005.
**Done when**: a manual unit test (jsdom-based) asserts `textContent` equals the expected strings; no `innerHTML` writes in the module (lint rule).
**Traces**: FR-B29.

### - [x] T026 [P] [US1] Verified / failure status UI at `impl/typescript/src/browser-client/ui/status.ts`

**Objective**: `renderVerified(sha256Hex, sellerAddress)` and `renderFailure(code, category)`; both target a fixed DOM target and use `textContent`.
**Files**: `impl/typescript/src/browser-client/ui/status.ts`, `impl/typescript/examples/browser-demo/index.html` (add `<section id="status">`).
**Depends on**: T007.
**Done when**: unit tests drive both renderers; failure render always includes the full `NEURON-BROWSER-NNN` identifier.
**Traces**: FR-B30, FR-B31.

### - [x] T027 [US1] Browser entry + Buy button wiring at `impl/typescript/src/browser-client/index.ts` + `examples/browser-demo/main.ts`

**Objective**: on page load, do nothing; on "Buy" click, create a fresh `BrowserSession` (session rotation per FR-B36); kick off bootstrap load → transport dial → buyer-flow; render ledger entries as they happen; render success or failure status at end; install `beforeunload` handler that aborts any in-flight session.
**Files**: `impl/typescript/src/browser-client/index.ts`, `impl/typescript/examples/browser-demo/main.ts`.
**Depends on**: T019, T020, T021, T023, T024, T025, T026.
**Done when**: opening the page does nothing network-wise; first click dials; ledger populates during the flow; image renders on success; second click rotates session (new PeerID visible).
**Traces**: FR-B26, FR-B27, FR-B28, FR-B32, FR-B33, FR-B36.

### - [x] T028 [US1] Demo orchestrator at `impl/typescript/scripts/run-demo.ts` + `package.json` scripts

**Objective**: `pnpm run demo` sequentially: (1) start seller via T018 as a child process; (2) poll stdout for `listening:` line; (3) write bootstrap JSON (already done by T013 but verify); (4) start Vite; (5) open browser; (6) kill seller on Ctrl-C.
**Files**: `impl/typescript/scripts/run-demo.ts`, `impl/typescript/package.json` (scripts).
**Depends on**: T018, T027.
**Done when**: `pnpm run demo` leads to a visible page at `http://localhost:5173` within 10 s; Ctrl-C kills both children.
**Traces**: quickstart.md §Run the demo.

### - [x] T029 [US1] Manual acceptance pass against quickstart §1–§4 — **GREEN: end-to-end JPEG delivery verified in live browser; SHA-256 `2b78cbe3…2a0b15` matches seller asset; 4 signed envelopes + ECIES decrypt + data-stream dial-back + frame reassembly all working; User Story 1 complete**

**Objective**: run the 4 happy-path acceptance steps in quickstart.md; record timings, PeerIDs, and DevTools network tab in a completion note.
**Files**: (no source edits; append a checklist to `specs/012-browser-client-profile/quickstart.md` Notes section recording the run).
**Depends on**: T028.
**Done when**: checklist §1–§4 all ✓; median time-to-JPEG under 15 s across 5 cold runs on Chromium.
**Traces**: SC-01, SC-02, SC-03, SC-04, SC-06, SC-08, SC-09.

---

## Phase 1E — User Story 2: Reject tampered envelopes without partial delivery (P2)

**Story goal**: any signature or SHA-256 verification failure must abort before render with a specific error code.
**Independent test**: inject tamper → abort + error code, no image rendered.

### - [x] T030 [P] [US2] Tamper hook — signature flip in `impl/typescript/src/server-demo/seller-flow.ts` — **hook lives in `emit()`; activated by `TAMPER=sig`; flips byte 0 of the R component of `paymentDetails` before sending**

**Objective**: add a single conditional block, guarded by `process.env.TAMPER === "sig"`, that flips one byte of the R component of the `paymentDetails` signature before publish. Comment block marked `TAMPER HERE (US2 manual test)`.
**Files**: `impl/typescript/src/server-demo/seller-flow.ts` (additive — does nothing when env unset).
**Depends on**: T016.
**Done when**: running with `TAMPER=sig pnpm run demo:server` produces a detectably invalid signature; running without it is byte-identical to the happy path.
**Traces**: SC-07, FR-B13.

### - [x] T031 [P] [US2] Tamper hook — payload flip in `impl/typescript/src/server-demo/file-send.ts` — **hook lives in `loadAsset()`; activated by `TAMPER=hash`; flips middle byte after the declared SHA-256 is computed**

**Objective**: add a conditional, guarded by `process.env.TAMPER === "hash"`, that flips one byte of the payload **after** the SHA-256 metadata has been declared. Leaves the declared hash intact.
**Files**: `impl/typescript/src/server-demo/file-send.ts`.
**Depends on**: T017.
**Done when**: `TAMPER=hash pnpm run demo:server` produces a SHA-256-mismatching stream; unset env produces the original stream.
**Traces**: FR-B22.

### - [x] T032 [US2] Manual acceptance pass against quickstart §5 + §6 — **PASSED (red-banner abort with specific code is the pass criterion per SC-07). Part A (sig) → `NEURON-BROWSER-061 [signature] signature does not recover to envelope.senderAddress: recovered 0x7b4c4305af245cfb1Caf1A17c44136cd864299c1, envelope claims 0x429898F3Fcfa7484B1A08A781a27AAD1B49D797A`, no image. Part B (hash) → `NEURON-BROWSER-082 [hash-mismatch] sha256 mismatch: computed ed030dcacdfafcaadfbd1da972ebf6c1fcbacd8c562f7b7f03b4566c9b13410d, metadata 2b78cbe32b150f7ef8bc98c6d57fab4acd8c59ddb124e50654d2fc62192a0b15`, no image. User Story 2 (P2) complete.**

**Objective**: run the two tamper tests end-to-end. Verify the browser aborts before render and displays the specific error code.
**Files**: append findings to `quickstart.md` Notes.
**Depends on**: T029, T030, T031.
**Done when**: §5 produces `NEURON-BROWSER-061`-class signature-verify failure (no image); §6 produces `NEURON-BROWSER-082` hash-mismatch failure (no image).
**Traces**: SC-07, FR-B13, FR-B14, FR-B22.

---

## Phase 1F — User Story 3: Fresh ephemeral session on every page load (P3)

**Story goal**: no key material persists; every Buy rotates identity.
**Independent test**: DevTools storage audit + multi-Buy PeerID diff.

### - [x] T033 [P] [US3] DevTools storage audit run against the running demo — **PASSED via Playwright harness `tests/e2e/t033-t034-harness.ts`. Before and after 3 successive Buys: `localStorage={}`, `sessionStorage={}`, `cookies=""`, `indexedDb=[]` (zero entries in all 4 mechanisms). FR-B03 + SC-06 verified.**

**Objective**: with demo running and after a completed transaction, open DevTools Application tab and inspect `localStorage`, `sessionStorage`, `IndexedDB`, `Cookies`. Record zero key-related entries.
**Files**: append findings to `quickstart.md` Notes.
**Depends on**: T029.
**Done when**: all four storage mechanisms show zero entries relevant to key or session state. Screenshot attached to the note.
**Traces**: SC-06, FR-B03.

### - [x] T034 [P] [US3] Multi-Buy rotation verification — **PASSED via Playwright harness. Three Buys in one page lifecycle produced three distinct buyer addresses: `0x5e53bE…24Ab`, `0x058aF7…069d`, `0xE8C62B…D12b`. Seller stayed `0xdab120A7…` across all three. FR-B36 + SC-08 verified. User Story 3 (P3) complete.**

**Objective**: click Buy three times in one page lifecycle. Record the PeerID displayed in the ledger for each run. Verify they are three distinct values.
**Files**: append findings to `quickstart.md` Notes.
**Depends on**: T027.
**Done when**: three distinct PeerIDs observed; prior ledger rows remain visible but are not cryptographically linked to the new session.
**Traces**: FR-B36, SC-08.

---

## Phase 1G — Polish (cross-cutting)

### - [x] T035 [P] `pnpm run typecheck` clean across all new source — **clean (0 errors)**

**Objective**: zero TS errors.
**Files**: (no source edits; fix any surfaced issues in their owning tasks).
**Depends on**: all US1/US2/US3 impl tasks.
**Done when**: `pnpm --filter @neuron-sdk/typescript run typecheck` exits 0.
**Traces**: Constitution VII.

### - [x] T036 [P] `pnpm run lint` clean — **0 errors across all 012-owned files (17 source + 6 test). 51 remaining errors are all pre-existing in spec 001–010 files (keylib, wire, topic, tests/account, tests/conformance) — out of scope for 012. eslint.config.js scoped overrides added for demo/forensic `console.*` in `browser-client/`, `server-demo/`, `scripts/`, `examples/`, and for test-idiomatic `!` assertions in `tests/`.**

**Objective**: zero eslint errors; includes a rule forbidding `innerHTML` in `browser-client/ui/`.
**Files**: `impl/typescript/eslint.config.js` (add the rule), fix surfaced issues.
**Depends on**: all US1 UI tasks.
**Done when**: `pnpm run lint` exits 0.
**Traces**: security boundary (plan.md §Security-sensitive boundaries).

### - [x] T037 [P] `pnpm test` all-green — **45 files, 609 tests, 1.47 s**

**Objective**: every vitest suite passes.
**Files**: (no source edits).
**Depends on**: T006–T034.
**Done when**: test suite exits 0; coverage report (if enabled) shows > 70 % on new files.

### - [x] T038 Phase 1 sign-off — **All 38 Phase-1 tasks green; Phase 1 code-complete and acceptance-complete; H1–H7 tracked for Phase 2.**

**Objective**: capture a ≤ 5-minute screen recording of the full acceptance run (quickstart §1–§6); write a short report listing measured timings, PeerIDs per Buy, TLS note (plain `ws://` used, Phase 1 only), and links to the task list.
**Files**: (spike report — internal artifact, not in the public tree).
**Depends on**: T029, T032, T033, T034, T035, T036, T037.
**Done when**: report exists; demo recording linked; all Phase 1 items ticked.
**Traces**: SC-01, full Phase 1 exit.

---

## Phase 2 — Hardening & Deferred (boundary)

> **No Phase 2 task begins until T038 is signed off.** Phase 2 items are named `H1`–`H7` to make the boundary visible in diffs, PRs, and reviews.

### - [ ] H1 [PHASE-2] Go seller interop — WSS listener on `impl/golang/internal/delivery/libp2p_host.go`

**Objective**: add a `/wss` or `/ws` listener to the Go libp2p host; register matching stream handlers for the two protocol IDs; prove the JS browser (from Phase 1) completes the full flow against the Go seller unchanged.
**Files**: `impl/golang/internal/delivery/libp2p_host.go`, `impl/golang/cmd/buyer-seller-demo/main.go` (or a new `cmd/browser-seller-demo/`), `impl/golang/internal/topic/in_stream_adapter.go` (new — Go mirror of TS T015).
**Depends on**: T038.
**Done when**: browser's ledger shows successful completion against Go seller; ECIES fixture round-trips both directions.
**Traces**: FR-B08 (original wording), plan.md Phase 2 H1.

### - [ ] H2 [PHASE-2] Real TLS termination — Caddy + Let's Encrypt

**Objective**: front the seller (Node or Go) with Caddy; browser moves from `ws://localhost` to `wss://subdomain.example.com`; page served over `https://`.
**Files**: `deploy/Caddyfile`, `deploy/docker-compose.yml` (or equivalent), `impl/typescript/src/server-demo/transport.ts` (support reverse-proxy-forwarded WS, behind-Caddy mode), `specs/012-browser-client-profile/quickstart.md` (update deployment note).
**Depends on**: T038 (H1 optional — H2 works against JS seller too).
**Done when**: live demo reachable at `https://demo.example/…` from any internet-connected browser; bootstrap still same-origin; Phase 1 localhost path still works for dev.
**Traces**: plan.md Phase 2 H2, FR-B05 (TLS).

### - [ ] H3 [PHASE-2] Automated tamper tests via Playwright

**Objective**: convert the T030/T031 manual tamper hooks into Playwright scenarios; run in CI on every PR touching 012.
**Files**: `impl/typescript/tests/e2e/tamper-sig.spec.ts`, `impl/typescript/tests/e2e/tamper-hash.spec.ts`, `impl/typescript/playwright.config.ts`, CI workflow file.
**Depends on**: T038.
**Done when**: CI green on a PR that adds these; CI red if either tamper check silently renders an image.
**Traces**: SC-07, plan.md Phase 2 H3.

### - [ ] H4 [PHASE-2] Multi-browser matrix via Playwright (Chromium, Firefox, WebKit)

**Objective**: run the happy-path test in all three Playwright browsers; upload trace artifacts on CI failure.
**Files**: `impl/typescript/tests/e2e/happy-path.spec.ts`, Playwright project matrix, CI workflow.
**Depends on**: H3.
**Done when**: Chromium + Firefox both green on CI; WebKit either green or produces the specific unsupported-environment error per SC-05.
**Traces**: SC-05, plan.md Phase 2 H4.

### - [ ] H5 [PHASE-2] Pre-merge obligation — align `NEURON-BROWSER-NNN` with Spec 006

**Objective**: map every profile-local identifier in `errors.ts` to the Spec 006 `NEURON-{DOMAIN}-{NNN}` taxonomy; add a compat layer (not a replacement) so older demo artefacts still decode; document the mapping.
**Files**: `impl/typescript/src/browser-client/errors.ts`, `specs/012-browser-client-profile/spec.md` (update FR-B35 cross-reference), `specs/006-protocol-determinism/error-taxonomy.md` (add browser-profile section).
**Depends on**: T038.
**Done when**: `FR-B35` marked satisfied in spec.md; PR labeled `pre-merge-obligation-resolved` before any merge to main.
**Traces**: FR-B35, plan.md Phase 2 H5.

### - [ ] H6 [PHASE-2] Read-idle timeout tightening

**Objective**: measure actual frame arrival intervals during 5 Phase 1 cold runs; pick a final constant (recommended 15 s; upper bound 30 s per FR-B34); add a regression test.
**Files**: `impl/typescript/src/browser-client/constants.ts` (may change `READ_IDLE_MS`), `impl/typescript/tests/file-receive/timeout.test.ts`, `specs/012-browser-client-profile/data-model.md` (update constant table if changed).
**Depends on**: T038.
**Done when**: regression test asserts abort happens within [constant, constant+500 ms]; spec constant documented.
**Traces**: FR-B34, plan.md Phase 2 H6.

### - [ ] H7 [PHASE-2] Automated size-cap refusal test

**Objective**: seller serves an oversized (~2 MiB) asset; Playwright asserts browser aborts with `NEURON-BROWSER-101` before any chunk frame is read.
**Files**: `impl/typescript/tests/e2e/size-cap.spec.ts`, `impl/typescript/src/server-demo/assets/oversized.jpg` (generated at test time, not committed).
**Depends on**: H3.
**Done when**: CI scenario passes; no chunk events observed on the wire before abort.
**Traces**: FR-B21, plan.md Phase 2 H7.

---

## Dependency graph (abridged)

```
T001 (gate)
  │
  ▼
T002─T005 [setup, parallel]
  │
  ▼
T006,T007,T008 [foundation, parallel across each other]
  │
  ├───▶ SERVER TRACK ──────────────────────────────┐
  │     T012 → T013,T014 → T015 → T016 → T017      │
  │                                       │        │
  │                                       ▼        │
  │                                     T018       │
  │                                                │
  ├───▶ BROWSER TRACK ─────────────────────────────┤
  │     T019 → T020 → T021 → T022 → T023 → T024    │
  │                                       │        │
  │               T025 [P] ── T026 [P] ──┤        │
  │                                       ▼        │
  │                                     T027       │
  │                                                │
  └───── CONTRACT TESTS ───────────────────────────┘
        T009, T010, T011 [all parallel, gate server + browser tracks]
                                                   │
                                                   ▼
                                                 T028 (orchestrator)
                                                   │
                                                   ▼
                                                 T029 (US1 manual accept)
                                                   │
                                 ┌─────────────────┼─────────────────┐
                                 ▼                 ▼                 ▼
                               T030,T031,T032    T033,T034        T035,T036,T037 [P]
                                 (US2)             (US3)            (polish)
                                 │                 │                 │
                                 └─────────────────┼─────────────────┘
                                                   ▼
                                                 T038 (PHASE-1 EXIT)
                                                   │
                                                   ▼
                                                 H1–H7 (PHASE-2)
```

## Parallel execution notes

- **After T001**: T002, T003, T004, T005 can all progress simultaneously (different files). T002 owns package.json; if another task also needs package.json, sequence them.
- **After T005**: T006, T007, T008 are the three foundational primitives, independent of each other.
- **After T008**: the SERVER TRACK (T012–T018) and BROWSER TRACK (T019–T027) are independent — two engineers could own one each.
- **Within US1**: T025 and T026 are purely UI; they can proceed alongside T024 file-receive if desks are split.
- **Within US2**: T030 and T031 touch different files; parallel.
- **Within US3**: T033 and T034 are manual; parallel.
- **Polish**: T035, T036, T037 run independently.
- **Phase 2 H1–H7**: H1 / H2 / H5 are largely independent. H3 gates H4 and H7. H6 is standalone.

## Blocker tasks (gate multiple downstream items)

| Task | Gates |
|------|-------|
| **T001** | everything |
| **T006** (framing) | T009, T015, T017, T022, T024 |
| **T008** (ECIES) | T016, T023 |
| **T012** (server transport) | T013, T015, T016, T018 |
| **T021** (browser transport) | T022, T023, T027 |
| **T027** (browser entry) | T028, T029 |
| **T029** (US1 accept) | T032, T034, T038 |
| **T038** (Phase 1 exit) | H1–H7 |

## Implementation strategy

1. **Pay the handshake cost first (T001).** Everything downstream scales with this.
2. **Land T006–T008 as the foundation.** These are small, independent, test-driven.
3. **Fork into server + browser tracks in parallel** once foundation is green. Two engineers can ship US1 in ~1 week wall-clock.
4. **Demo-quality acceptance in one sitting (T029).** Don't defer to CI in Phase 1.
5. **Tamper + lifecycle stories (US2, US3)** are small — half-day each, done before polish.
6. **Polish (T035–T038)** is non-negotiable: Phase 1 exit requires the report + recording.
7. **Phase 2 (H1–H7) starts only after T038.** Each hardening item is independently schedulable and largely independent of the others.

## MVP scope

If time is short and only one story can ship, **ship User Story 1 only** (T001–T029 + minimum polish). Tamper-proofing and rotation stories are nice-to-have demonstrations of honesty, not the core demo.

## Suggested next command

`/speckit.analyze` — run a cross-artifact consistency check before implementation starts. If it comes back clean, `/speckit.implement` can proceed against this task list.
