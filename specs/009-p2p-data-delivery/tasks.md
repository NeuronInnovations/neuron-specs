# Tasks: P2P Data Delivery (Spec 009)

**Input**: Design documents from `specs/009-p2p-data-delivery/`
**Prerequisites**: plan.md (complete), spec.md (complete), data-model.md (complete), contracts/ (complete)

**Tests**: Included per Constitution Principle IX (Test-First Development). Test tasks precede implementation tasks.

**Organization**: Tasks grouped by user story. Each story is independently testable after its phase completes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Exact file paths included in descriptions

## Path Conventions

- Go SDK layout per Constitution VI: `internal/delivery/`
- Tests colocated as `*_test.go`
- All paths relative to `impl/golang/`

## Constitution Compliance Notes

- **Principle VII**: Every task references FR-* or SC-* it satisfies
- **Principle IX**: Test tasks (Red) precede implementation tasks (Green) in each phase
- **Principle XI**: Integration tests include validator-perspective scenarios

---

## Phase 1: Setup

**Purpose**: Create delivery package structure and shared infrastructure

- [x] T001 Create `internal/delivery/doc.go` with package-level godoc documenting Spec 009 scope, dependency chain (keylib→payment→delivery), architecture decisions DD-D01–D05, and the DeliveryAdapter/TopicAdapter analogy
- [x] T002 [P] Create `internal/delivery/errors.go` with `DeliveryError` structured error type, `DeliveryErrorKind` enum (10 kinds per FR-D29: DialFailed, StreamError, RelayError, PeerIDMismatch, NoCompatibleTransport, InvalidMultiaddr, ChannelClosed, FrameTooLarge, BackoffExhausted, ConnectionSetupEncryptionFailed), and `NEURON-DELIVERY-*` error code constants. FR-D29

**Checkpoint**: Package structure exists, error types defined.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and pure-logic modules that ALL user stories depend on. No libp2p dependency yet.

### Tests (Red)

- [x] T003 [P] Write tests for `ConnectionState` enum and state machine in `internal/delivery/connection_test.go`: all 12 valid transitions from FR-D08, reject invalid transitions, IDLE initial state, **verify state-change callback fires on each transition with new state and reason (FR-D10)**. FR-D07, FR-D08, FR-D10, SC-D03
- [x] T004 [P] Write tests for `BackoffConfig` and exponential backoff logic in `internal/delivery/backoff_test.go`: default config (5s/2x/10min/1hr), delay computation per attempt, max delay cap, max duration exceeded. FR-D09
- [x] T005 [P] Write tests for `FrameWriter`/`FrameReader` in `internal/delivery/framing_test.go`: write frame + read frame round-trip, 4-byte BE length prefix, 4 MiB max enforcement (`FrameTooLarge` error), zero-length keep-alive consumed silently, empty reader returns EOF, **binary payload integrity (arbitrary bytes including nulls and high bytes survive round-trip unmodified per FR-D23)**. FR-D22, FR-D23, FR-D24, SC-D06
- [x] T006 [P] Write tests for `DeliveryAdapter` interface types (`DeliveryChannel`, `ChannelStatus`, `DataFrame`, `SendResult`) in `internal/delivery/adapter_test.go`: construction and field access. FR-D01–D06

### Implementation (Green)

- [x] T007 [P] Implement `ConnectionState` enum (6 values) and `ConnectionStateMachine` with `Transition(event)` method in `internal/delivery/connection.go`. All 12 transitions from FR-D08. Return `DeliveryError` on invalid transition. Include state change callback support (FR-D10). FR-D07, FR-D08, FR-D10
- [x] T008 [P] Implement `BackoffConfig` struct with spec defaults and `NextDelay(attempt)` / `IsExhausted(elapsed)` methods in `internal/delivery/backoff.go`. Initial 5s, factor 2, max delay 10min, max duration 1hr. FR-D09
- [x] T009 [P] Implement `FrameWriter` and `FrameReader` in `internal/delivery/framing.go`. `FrameWriter.WriteFrame(data []byte) error` writes 4-byte BE length + payload; rejects > 4 MiB. `FrameReader.ReadFrame() ([]byte, error)` reads length + payload; silently skips zero-length keep-alive. `WriteKeepAlive()` sends zero-length sentinel. FR-D22, FR-D23, FR-D24
- [x] T010 [P] Implement `DeliveryAdapter` interface, `DeliveryChannel`, `ChannelStatus`, `DataFrame`, `SendResult` types in `internal/delivery/adapter.go`. FR-D01–D06

**Checkpoint**: Foundation types exist, state machine works, framing tested. Ready for user story phases.

---

## Phase 3: User Story 2 — Encrypt and Decrypt connectionSetup Multiaddrs (Priority: P1)

**Goal**: ECIES encrypt/decrypt satisfying 008 FR-P34. No libp2p dependency — pure crypto.

**Independent Test**: Encrypt multiaddrs with test key, decrypt with matching key, verify wrong-key rejection.

### Tests (Red)

- [x] T011 [P] [US2] Write ECIES encrypt/decrypt tests in `internal/delivery/ecies_test.go`: encrypt + decrypt round-trip recovers original multiaddrs, wrong key → `ConnectionSetupEncryptionFailed` error, two encryptions of same data produce different ciphertexts (randomized FR-D13), ciphertext format is valid base64. FR-D11, FR-D12, FR-D13, FR-D14, SC-D02
- [x] T012 [P] [US2] Write ECIES edge case tests in `internal/delivery/ecies_test.go`: empty multiaddr array, single multiaddr, large multiaddr list, malformed base64 input to decrypt, truncated ciphertext. FR-D14

### Implementation

- [x] T013 [US2] Implement `EncryptMultiaddrs(multiaddrs []string, recipientPubKey *ecdsa.PublicKey) (string, error)` in `internal/delivery/ecies.go`. Per contracts/ecies-profile.md: generate ephemeral keypair, ECDH shared secret, HKDF-SHA256 (salt=empty, info="neuron-multiaddr-v1"), AES-256-GCM encrypt, output = ephemeral_pub(33) || nonce(12) || ciphertext || tag(16), base64 encode. FR-D11, FR-D12, FR-D13
- [x] T014 [US2] Implement `DecryptMultiaddrs(encryptedBase64 string, recipientPrivKey *ecdsa.PrivateKey) ([]string, error)` in `internal/delivery/ecies.go`. Base64 decode, parse ephemeral_pub + nonce + ciphertext + tag, ECDH, HKDF, AES-256-GCM decrypt, verify tag, parse JSON array. FR-D11, FR-D14

**Checkpoint**: ECIES encrypt/decrypt works standalone. 008 FR-P34 satisfied.

---

## Phase 4: User Story 1 — Establish a P2P Delivery Channel After Agreement (Priority: P1)

**Goal**: Full connect→send/receive→disconnect cycle using libp2p QUIC transport.

**Independent Test**: Two in-process libp2p hosts, one dials the other, data frames exchanged, disconnect cleanly.

### Tests (Red)

- [x] T015 [P] [US1] Write connectionSetup processing tests in `internal/delivery/setup_test.go`: decrypt multiaddrs → validate format → initiate connect; invalid multiaddrs produce `InvalidMultiaddr` error; protocol field matching. FR-D15, FR-D16, FR-D17
- [x] T016 [P] [US1] Write libp2p adapter unit tests in `internal/delivery/libp2p_adapter_test.go`: connect two in-process hosts via QUIC, send data frame, receive data frame in order, disconnect cleanly, verify PeerID matches (FR-D28). FR-D02, FR-D03, FR-D04, FR-D05, FR-D28, SC-D01
- [x] T017 [P] [US1] Write PeerID verification test in `internal/delivery/libp2p_adapter_test.go`: connect with mismatched PeerID → `PeerIDMismatch` error. FR-D28, SC-D07

### Implementation

- [x] T018 [US1] Implement `ProcessConnectionSetup(setup payment.ConnectionSetup, privKey *ecdsa.PrivateKey) (peerID, []multiaddr, protocol, error)` in `internal/delivery/setup.go`. Steps: (1) decrypt encryptedMultiaddrs via ecies.go, (2) validate multiaddr format, (3) return parsed fields for connect(). FR-D15, FR-D16
- [x] T019 [US1] Add `github.com/libp2p/go-libp2p` and `github.com/multiformats/go-multiaddr` to `go.mod`. Configure libp2p host with QUIC transport, secp256k1 identity from NeuronPrivateKey. FR-D25, FR-D27
- [x] T020 [US1] Implement `Libp2pAdapter` struct implementing `DeliveryAdapter` interface in `internal/delivery/libp2p_adapter.go`. `connect()`: dial peer via libp2p host, open stream with protocol ID, wrap with FrameWriter/FrameReader, verify PeerID (FR-D28). `send()`: write frame. `receive()`: read frames as channel/iterator. `disconnect()`: close stream + connection. `getStatus()`: return current ConnectionState. FR-D01–D06, FR-D25–D28
- [x] T021 [US1] Implement retry `connectionSetup` logic in `internal/delivery/setup.go`: if connect fails (DISCONNECTED), caller MAY retry up to 3 times with updated multiaddrs. FR-D17

**Checkpoint**: Two agents can establish P2P channel, exchange data, disconnect. SC-D01 verified.

---

## Phase 5: User Story 3 — NAT Traversal with Automatic Fallback (Priority: P2)

**Goal**: Direct dial fails → relay fallback → DCUtR upgrade attempt.

**Independent Test**: Two agents, one with non-routable address, verify relay connectivity and DCUtR attempt.

### Tests (Red)

- [x] T022 [P] [US3] Write NAT traversal tests in `internal/delivery/libp2p_adapter_test.go`: direct dial fails → state transitions CONNECTING→RELAYING, relay connection delivers data, DCUtR upgrade attempt (mock). FR-D18, FR-D20, FR-D21, SC-D04
- [x] T023 [P] [US3] Write AutoNAT tests in `internal/delivery/libp2p_adapter_test.go`: natStatus reflects reachability probe result ("public"/"private"/"unknown"). FR-D19

### Implementation

- [x] T024 [US3] Configure libp2p host with Circuit Relay v2 client and DCUtR in `internal/delivery/libp2p_adapter.go`. Add relay discovery (DHT or static config). FR-D18, FR-D20, FR-D21
- [x] T025 [US3] Configure libp2p host with AutoNAT in `internal/delivery/libp2p_adapter.go`. Expose `NATStatus() string` method reflecting latest probe. FR-D19
- [x] T026 [US3] Update `connect()` in libp2p adapter to implement fallback: direct dial first → if fail + natStatus private/unknown → relay dial → if relay succeeds → DCUtR upgrade attempt → state transitions per FR-D08. FR-D18, FR-D20, FR-D21

**Checkpoint**: NAT traversal works with relay fallback and DCUtR upgrade.

---

## Phase 6: User Story 4 — Reconnection After Connection Loss (Priority: P2)

**Goal**: Transport drop → automatic reconnection with exponential backoff.

**Independent Test**: Establish channel, simulate drop, verify reconnection attempts and eventual recovery.

### Tests (Red)

- [x] T027 [P] [US4] Write reconnection tests in `internal/delivery/libp2p_adapter_test.go`: connection drop → RECONNECTING state, backoff delays match 5s/10s/20s/..., reconnection succeeds → CONNECTED, max duration exceeded → DISCONNECTED + notify application. FR-D09, FR-D10, SC-D05
- [x] T028 [P] [US4] Write state machine reconnection transitions in `internal/delivery/connection_test.go`: CONNECTED→RECONNECTING, RECONNECTING→CONNECTED, RECONNECTING→RELAYING, RECONNECTING→DISCONNECTED. FR-D08

### Implementation

- [x] T029 [US4] Implement reconnection goroutine in `internal/delivery/libp2p_adapter.go`: on stream/connection drop, transition to RECONNECTING, attempt re-dial with BackoffConfig, fire state change events (FR-D10), transition to CONNECTED/RELAYING/DISCONNECTED based on outcome. FR-D08, FR-D09, FR-D10
- [x] T030 [US4] Wire backoff config into adapter construction: `NewLibp2pAdapter(host, opts...)` with `WithBackoffConfig(config)` option. Default uses spec parameters. FR-D09

**Checkpoint**: Reconnection works with exponential backoff and state notifications.

---

## Phase 7: User Story 5 — Multi-Transport Delivery (Priority: P3)

**Goal**: QUIC + WebRTC support, automatic transport selection from multiaddr.

**Independent Test**: Seller connects to QUIC buyer and WebRTC buyer; both receive data.

### Tests (Red)

- [x] T031 [P] [US5] Write transport selection tests in `internal/delivery/libp2p_adapter_test.go`: QUIC multiaddr → QUIC transport, WebRTC multiaddr → WebRTC transport, multiple transports → QUIC preferred. FR-D25, FR-D26

### Implementation

- [x] T032 [US5] Add WebRTC transport to libp2p host configuration in `internal/delivery/libp2p_adapter.go`. FR-D25
- [x] T033 [US5] Implement automatic transport selection in `connect()`: parse multiaddr protocols, attempt in preference order (QUIC > WebTransport > WebRTC > TCP), populate `ChannelStatus.transport` with selected transport ID. FR-D26
- [x] T034 [US5] Verify transport-layer encryption enforcement: reject unencrypted connections. FR-D27

**Checkpoint**: Multi-transport delivery works with automatic selection.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Integration tests, interop verification, validator perspective

- [x] T035 [P] Write full end-to-end integration test in `internal/delivery/integration_test.go`: US1 complete flow (encrypt multiaddrs → connectionSetup → connect → send frames → receive frames → disconnect) with real libp2p hosts. SC-D01
- [x] T036 [P] Write ConnectionState determinism test in `internal/delivery/integration_test.go`: same transport event sequence on two independent state machines produces same final state. SC-D03
- [x] T037 [P] Write framing interop test in `internal/delivery/framing_test.go`: generate a frame with known data, verify wire format matches 4-byte BE prefix + payload bytes exactly — cross-SDK interop baseline. SC-D06
- [x] T038 [P] Write validator-perspective test in `internal/delivery/integration_test.go`: simulate third-party observing `connectionSetup` on stdIn, verify VR-DEL-01 (PeerID format), VR-DEL-02 (requestId consistency). VR-DEL-01, VR-DEL-02
- [x] T039 Run `go vet ./internal/delivery/...` — zero warnings
- [x] T040 Verify all existing tests still pass: `go test ./internal/...`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1. BLOCKS all user stories.
- **Phase 3 (US2: ECIES)**: Depends on Phase 2. No libp2p dependency — pure crypto.
- **Phase 4 (US1: P2P Channel)**: Depends on Phase 2 + Phase 3 (needs ECIES for connectionSetup processing). Requires adding libp2p to go.mod.
- **Phase 5 (US3: NAT)**: Depends on Phase 4 (extends libp2p adapter)
- **Phase 6 (US4: Reconnection)**: Depends on Phase 4 (extends libp2p adapter)
- **Phase 7 (US5: Multi-Transport)**: Depends on Phase 4 (extends libp2p adapter)
- **Phase 8 (Polish)**: Depends on Phases 3–7

### User Story Dependencies

```
Phase 2 (Foundational) ──> Phase 3 (US2: ECIES) ──> Phase 4 (US1: P2P Channel)
                                                       │
                                                       ├──> Phase 5 (US3: NAT)
                                                       ├──> Phase 6 (US4: Reconnection)
                                                       └──> Phase 7 (US5: Multi-Transport)
```

Phases 5, 6, 7 can run in parallel after Phase 4 completes.

### Parallel Opportunities

Within Phase 2: T003–T006 (all tests) in parallel, then T007–T010 (all implementations) in parallel.
Phase 5 (US3), Phase 6 (US4), Phase 7 (US5) can run in parallel.

---

## Implementation Strategy

### MVP First (User Stories 2 + 1 Only)

1. Complete Phase 1 (Setup) + Phase 2 (Foundational)
2. Complete Phase 3 (US2: ECIES) — standalone encryption works
3. Complete Phase 4 (US1: P2P Channel) — full QUIC delivery works
4. **STOP and VALIDATE**: Two agents can exchange encrypted connectionSetup, establish P2P channel, stream data frames, disconnect cleanly
5. Deploy/demo: end-to-end data delivery operational

### Incremental Delivery

1. Setup + Foundational → Types, state machine, framing ready
2. US2 → ECIES encrypt/decrypt (008 FR-P34 satisfied)
3. US1 → QUIC P2P channel (core data plane operational)
4. US3 → NAT traversal (production-ready connectivity)
5. US4 → Reconnection (production-ready reliability)
6. US5 → WebRTC multi-transport (browser agent support)

---

## Summary

| Metric | Value |
|--------|-------|
| Total tasks | 40 |
| Phase 1 (Setup) | 2 tasks |
| Phase 2 (Foundational) | 8 tasks |
| Phase 3 (US2: ECIES) | 4 tasks |
| Phase 4 (US1: P2P Channel) | 7 tasks |
| Phase 5 (US3: NAT) | 5 tasks |
| Phase 6 (US4: Reconnection) | 4 tasks |
| Phase 7 (US5: Multi-Transport) | 4 tasks |
| Phase 8 (Polish) | 6 tasks |
| Parallel opportunities | 24 tasks marked [P] |
| MVP scope | Phases 1–4 (US2 + US1): 21 tasks |

---

## Phase 9 — 2026-05-08 Amendment

### Multiaddr filtering (FR-D11a)

- **T-NA-101 [test]** — Unit tests for `filterMultiaddrs` in `internal/delivery/multiaddr_filter_test.go` covering: loopback exclusion, RFC1918 inclusion, public inclusion, link-local exclusion, Docker bridge exclusion (mocked interface map), `veth*`/`utun*` exclusion. Trace: SC-D10.
- **T-NA-102** — Implement `filterMultiaddrs` in `internal/delivery/multiaddr_filter.go` with OS-specific interface-name lookup. Trace: FR-D11a.
- **T-NA-103** — Wire `filterMultiaddrs` into `BuildConnectionSetup` (`internal/delivery/negotiate_bridge.go`) before ECIES encryption, and into `ConnectFromSetup` after decryption. Trace: FR-D11a, FR-D15.
- **T-NA-104** — Update `cmd/edge-seller/main.go` to listen on `/ip4/0.0.0.0/...` so `host.Addrs()` enumerates the full set; rely on FR-D11a filtering for advertisement hygiene. Trace: FR-D11a.

### Stream direction split (FR-D-stream-direction)

- **T-NA-110 [test]** — Test all four combinations of (connection direction × stream-init direction) in `internal/delivery/stream_direction_test.go`. Trace: SC-D08.
- **T-NA-111** — Add `StreamDirection` type and `RegisterStream(protocolID, direction, handler)` in `internal/delivery/libp2p_adapter.go`. Reject wrong-direction `OpenStream` with `StreamDirectionViolation`. Trace: FR-D-stream-direction, FR-D29.
- **T-NA-112 [test]** — Back-compat test: legacy single-string `Protocol` defaults to `seller-initiates`. Trace: FR-D-stream-direction.

### Multi-protocol + wildcard handler (FR-D-multi-protocol, FR-D-wildcard-handler)

- **T-NA-120 [test]** — Multi-protocol-ID registration test: register three concurrent literal protocol IDs on one host; open each from the counterparty; verify each lands in the correct handler. Trace: FR-D-multi-protocol.
- **T-NA-121** — Add `RegisterStream(protocolID, ...)` for literal protocol IDs (delegates to `Host.SetStreamHandler`). Trace: FR-D-multi-protocol.
- **T-NA-122 [test]** — Wildcard registration test: register `/test/wildcard/*`; open `/test/wildcard/123` and `/test/wildcard/456`; verify handler receives the parameter `"123"` and `"456"` respectively. Open `/test/wildcard/` (empty parameter); verify rejection. Trace: FR-D-wildcard-handler.
- **T-NA-123** — Add `RegisterStreamPattern(prefix, handler)` for wildcard registration (delegates to `Host.SetStreamHandlerMatch`). Trace: FR-D-wildcard-handler.
- **T-NA-124 [test]** — UnknownProtocol error: open a protocol ID that matches no literal and no wildcard; verify rejection with `UnknownProtocol`. Trace: FR-D29.

### Pubsub primitive exposure (FR-D-pubsub-primitives)

- **T-NA-130 [test]** — Smoke test: create a GossipSub instance via `internal/delivery/pubsub.go`, join a test topic, publish a message, verify it arrives at a subscriber on a second host. No mesh parameters are configured — verify libp2p's defaults apply. Trace: SC-D11.
- **T-NA-131** — Re-export `pubsub.PubSub`, `gossipsub.GossipSub`, `floodsub.FloodSub` (and the `gossipsub.WithMeshSize` / equivalent option functions) from `internal/delivery/pubsub.go`. Trace: FR-D-pubsub-primitives.
- **T-NA-132 [test]** — Constitution Principle XII check: assert that `internal/delivery/pubsub.go` does NOT define any topic naming convention, mesh parameter default, or fan-out topology rule. Trace: FR-D-pubsub-primitives, Principle XII.

### Degraded mode (FR-D-degraded)

- **T-NA-140 [test]** — Topic-backend-down integration test: with an active delivery channel, force the topic adapter into a fault state for 60 seconds; assert no transition to DISCONNECTED, frames continue to flow. Trace: SC-D12.
- **T-NA-141** — Audit and refactor `internal/delivery/libp2p_adapter.go` to ensure no control-plane dependency in the data-plane critical path. Trace: FR-D-degraded.

### Phase 9 totals

- Test tasks: 10
- Implementation tasks: 8
- Total Phase 9: 18 tasks
- Cumulative project total: 21 (MVP) + 18 (Phase 9) = 39 tasks within the active scope
