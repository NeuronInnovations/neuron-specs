# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repo Is

Neuron SDK specification corpus with Go and TypeScript reference implementations. Specs define a hierarchical agent identity model with DLT-traceable communication: Parent accounts hold root DIDs and control Child agents, which register as EIP-8004 NFTs. Communication flows through signed, append-only topic channels (HCS primary). Liveness is proven via self-declared deadline heartbeats.

`AGENTS.md` is a parallel quick-reference companion to this file with the same substance — load whichever fits your context budget. `README.md` is user-facing (architecture, spec index, Zero-to-Heartbeat path, build commands).

## Commands

### Go SDK (run from `impl/golang/`)

```bash
# Build
cd impl/golang && go build ./...

# Run all tests + vet
cd impl/golang && go test ./internal/... && go vet ./internal/...

# Run tests for a single module
cd impl/golang && go test ./internal/keylib/...
cd impl/golang && go test ./internal/account/...
cd impl/golang && go test ./internal/topic/...
cd impl/golang && go test ./internal/registry/...
cd impl/golang && go test ./internal/health/...
cd impl/golang && go test ./internal/payment/...
cd impl/golang && go test ./internal/delivery/...
cd impl/golang && go test ./internal/validation/...

# Run a single test function
cd impl/golang && go test ./internal/keylib/... -run TestFunctionName

# Verbose output
cd impl/golang && go test -v ./internal/keylib/...
```

Go module: `github.com/neuron-sdk/neuron-go-sdk` (Go 1.25.7, see `impl/golang/go.mod`).

### Race detection (mandatory after runtime/concurrency changes)

After any runtime, concurrency, scheduling, or shared-state change — buyer/seller flow, topic adapters, payment lifecycle, libp2p/delivery, edge app, FID display, remoteid DApp — run the scoped race check before declaring the change done:

```bash
scripts/validation/race-check.sh
```

The script runs `go test -race ... -count=1 -timeout 180s` across the affected surfaces in three groups (A protocol / B runtime / C cmd) and prints a per-group PASS/FAIL summary. It is **additional to** the standard `go test`, `go vet`, and `go build` gates, not a replacement.

Do not hide race failures. If a group fails, quote the race-detector report verbatim in the change summary and fix it before shipping. This rule exists because a race condition once crashed a deployed buyer in the field (2026-05-13).

### Demo CLIs (run from `impl/golang/`)

```bash
# Buyer-seller JPEG demo — mock mode (instant, no network)
cd impl/golang && go run ./cmd/buyer-seller-demo/ --jpeg cmd/buyer-seller-demo/testdata/photo.jpg --mode=mock

# Buyer-seller JPEG demo — testnet mode (real HCS, ~90s, needs funded operator)
cd impl/golang && go run ./cmd/buyer-seller-demo/ --jpeg cmd/buyer-seller-demo/testdata/photo.jpg --mode=testnet

# Buyer-seller demo with full hash/payload visibility
cd impl/golang && go run ./cmd/buyer-seller-demo/ --jpeg cmd/buyer-seller-demo/testdata/photo.jpg --verbose

# Live HCS heartbeat test (publishes to Hedera testnet, verifies via mirror node)
cd impl/golang && go run ./cmd/hcs-heartbeat-test/

# P2P delivery demo (two terminals: seller + buyer over libp2p QUIC)
cd impl/golang && go run ./cmd/delivery-demo/ --mode seller
cd impl/golang && go run ./cmd/delivery-demo/ --mode buyer --peer <seller-multiaddr>

# Circuit Relay v2 node (spec 011) — runs until Ctrl-C
cd impl/golang && go run ./cmd/relay-node/

# WebTransport seller for browser demos (spec 012, Profile A binding T-WebTransport)
cd impl/golang && go run ./cmd/webtransport-seller/

# Regenerate ECIES golden test fixtures (spec 006 conformance vectors for 009 ECIES profile)
cd impl/golang && go run ./cmd/ecies-fixture-gen/

# Edge demo (Phase B/C — reverse-connect: NAT'd seller dials reachable buyer; BEAST/Mode-S feed)
cd impl/golang && go run ./cmd/edge-seller/ --mode=testnet  # reads BEAST from 127.0.0.1:10003
cd impl/golang && go run ./cmd/edge-buyer/  --mode=testnet  # listens publicly, accepts seller's dial-in
```

### Quick demos via Makefile (run from repo root)

```bash
make demo                    # Canonical zero-friction local demo: mock buyer-seller, <1s, no infra
make demo-help               # List all demo targets with descriptions
make demo-relay              # Start a Circuit Relay v2 node (interactive)
make demo-browser            # Browser <-> Node seller via libp2p WSS (requires Node 20+ and pnpm)
make demo-sapient            # Local SAPIENT Remote ID demo (DS240 sim -> bridge -> seller dials buyer -> FID map)
```

### Solidity contracts + Go bindings (run from repo root, uses `Makefile`)

```bash
# Compile contracts (Foundry)
make forge-build

# Foundry unit tests
make forge-test

# Regenerate abigen Go bindings under impl/golang/internal/{registry,payment}/bindings
make abigen-all

# Combined: forge-build + abigen-all
make contracts

# Deploy to Hedera EVM testnet (https://testnet.hashio.io/api) — requires PRIVATE_KEY env var
make deploy-all          # Identity Registry + Escrow + TestToken
make deploy-registry     # Identity Registry only
make deploy-escrow       # Escrow + TestToken only
```

Prerequisites: Foundry (`forge`) and `abigen` (`go install github.com/ethereum/go-ethereum/cmd/abigen@latest`); on first checkout run `cd contracts && forge install OpenZeppelin/openzeppelin-contracts --no-git`.

### TypeScript SDK (run from `impl/typescript/`)

```bash
# Install dependencies
cd impl/typescript && npm install

# Build
cd impl/typescript && npm run build

# Run all tests
cd impl/typescript && npm test

# Run conformance tests only
cd impl/typescript && npm run test:conformance

# Lint
cd impl/typescript && npm run lint

# Type-check without emitting
cd impl/typescript && npm run typecheck
```

Package: `@neuron-sdk/typescript` (TypeScript 5.7+, Node.js >= 18, vitest for testing).

## Project Structure

```
specs/                              # Language-neutral feature specifications
  NNN-feature-name/
    spec.md                         # Feature specification (primary artifact)
    plan.md                         # Implementation plan
    tasks.md                        # Dependency-ordered task list
    data-model.md                   # Entity/type definitions
    contracts/                      # API contracts
    checklists/                     # Quality checklists
contracts/                          # Solidity sources (Foundry layout: src/, script/, test/, out/)
                                    # Built via Makefile; abigen output lands in impl/golang/.../bindings/
impl/
  golang/                           # Go SDK implementation
    internal/
      keylib/                       # 002 — secp256k1 keys, EVM addresses, PeerID, DID:key, signatures
      account/                      # 001 — Parent/Child/Shared identity, DID, ledger attachment
      topic/                        # 004 — TopicMessage, adapters (HCS/ERC/Kafka), channels
      registry/                     # 003 — EIP-8004 registration, AgentURI, proof-of-control
      health/                       # 005 — HeartbeatPayload, liveness state machine, observer
      payment/                      # 008 — Commerce protocol, negotiation, escrow adapter
      delivery/                     # 009 — P2P data delivery, ECIES encryption, libp2p binding
      validation/                     # 010 — EvidenceEnvelope, verdict model, validator service, publisher/observer
      browserprofile/                 # 012 — buyer-only browser profile: bootstrap writer, control/echo handlers, data sender
      dapp/                           # DApp layer (Principle XII). Subpkgs: sapient/ (015 SAPIENT Interop Profile, BSI Flex 335 v2.0 protobuf, transparent proxy, audit/cot/fid/tasking subpkgs), adsb/ (016 JetVision ADS-B, NormalizedTrack), remoteid/ (017 DroneScout Remote ID, RemoteIdFrame)
      feeds/                          # Edge demo — BEAST/Mode-S decoder, ICAO recovery cache (parity-XOR), TCP/replay/synth sources
      edgeapp/                        # Edge demo — wires feeds + delivery + topic + health into seller/buyer runtimes
    cmd/
      buyer-seller-demo/            # End-to-end demo: negotiation, payment, JPEG delivery, validation
      delivery-demo/                # Standalone libp2p P2P delivery test
      hcs-heartbeat-test/           # Live Hedera testnet heartbeat publishing
      relay-node/                   # Circuit Relay v2 node (spec 011 reference runtime)
      webtransport-seller/          # WebTransport seller bridge for browser-buyer demos (spec 012)
      ecies-fixture-gen/            # Generates ECIES golden test fixtures for spec 006 conformance suite
      edge-seller/                  # Reverse-connect Mode-S seller (NAT'd; reads BEAST :10003, dials reachable buyer)
      edge-buyer/                   # Reverse-connect aggregator (multi-seller; emits JSONL per received frame)
      adsb-seller/                  # 016 — JetVision ADS-B seller (BaseStation fast-path /jetvision/basestation/1.0.0)
      remoteid-seller/              # 017 — DroneScout Remote ID seller (/ds240/raw/1.0.0)
      multistream-buyer/            # fused buyer: one host, N seller sessions, consolidated TaggedFrame JSONL (contract: docs/fid-display-contract.md)
      fid-display/                  # 018 — Flight Information Display (embedded Leaflet map, :8080; TCP TaggedFrame input)
      edge-validator/               # 010 validator for the reverse-connect demo (captures transcript, publishes evidence)
      sapient-jv-seller/            # 015/016 — JetVision SAPIENT pusher (dials buyer on /sapient/detection/2.0.0)
      sapient-rid-seller/           # 015/017 — Remote ID SAPIENT pusher (dials buyer on /sapient/detection/2.0.0)
      sapient-buyer/               # 015 — generic SAPIENT Buyer Proxy (listens, forwards to downstream SAPIENT edge)
      sapient-feed-replay/          # 015 — serves captured SAPIENT NDJSON as live LE-framed protobuf feed (bridge stand-in)
      sapient-task/                 # 015 — HLDMM tasking CLI (Task STOP/START over the auditable lane, awaits TaskAck)
      sapient-fid-consumer/         # 018 — SAPIENT→TaggedFrame/CoT projector (downstream of the Buyer Proxy)
      sapient-fid-display/          # 018 — SAPIENT-rich FID map (drone+operator, classification/confidence, :8193)
      sapient-explorer/             # Tactical situational-awareness console (map proxy + agent registry, :8194)
      sapient-agent-explorer/       # SAPIENT Agent Card viewer (evidence files or on-chain EIP-8004 lookup)
  typescript/                       # TypeScript SDK implementation
    src/
      keylib/                       # 002 implementation
      account/                      # 001 implementation
      topic/                        # 004 implementation
      registry/                     # 003 implementation
      health/                       # 005 implementation
      wire/                         # Wire format utilities
      contracts/                    # Contract bindings
    tests/
      conformance/                  # Spec 006 golden-vector tests (cross-language); pair with impl/golang/internal/keylib/conformance_test.go
docs/                               # Reference materials, architecture docs
.specify/
  memory/constitution.md            # Constitution Principles I-XII (v1.7.0)
  templates/                        # Spec Kit plan + tasks generation templates
```

## Execution Order and Dependencies

Build order is strictly sequential. Each module depends on all modules above it. Do not begin spec N+1 until spec N passes all tests and exported types are stable.

```
002 Key Library            <- Foundation: zero dependencies
 |
001 NeuronAccount          <- Identity built on keys
 |
004 Topic System           <- Communication substrate
 |
003 Peer Registry          <- Registration requires topic schemas
 |
005 Health                 <- Heartbeats flow through topics
 |
006 Protocol Determinism   <- Cross-cutting: wire format, signing algorithms, test vectors
 |
007 Identity Contract      <- On-chain: EVM registries (identity, reputation, validation)
 |
008 Payment                <- Commerce protocol between agents
 |
009 P2P Data Delivery      <- Data plane: libp2p streams, ECIES encryption
 |
010 Validation Framework   <- Evidence model, validator agents, oracle verdicts
 |
011 Relay                  <- Spec-only SDK pkg; reference runtime in cmd/relay-node/
 |
012 Browser Client Profile <- TypeScript browser target (Chromium ≥ 120, Firefox ≥ 115)
 |
013 Connectivity Profiles  <- Spec-only; profile descriptors, contracts/, profiles/
                              v2 (2026-05-08) adds stream-init-direction capability key
 |
 | --- Core SDK / DApp boundary (Constitution Principle XII, v1.7.0) ---
 |
015 SAPIENT Interop Profile <- Shared application profile: SAPIENT (BSI Flex 335 v2.0) sensor
                              envelope over Core lanes; transparent Seller/Buyer Proxy; ASM/HLDMM.
                              (015 slot REPURPOSED from the deprecated Admission Policy spec.)
 |
016 ADS-B DApp             <- JetVision aircraft Mode-S service; vendor SAPIENT translator (neuron.adsb/1)
                              behind 015's proxy + BaseStation fast-path /jetvision/basestation/1.0.0
017 Remote ID DApp         <- DroneScout Open Drone ID service; vendor SAPIENT translator (rid.*)
                              behind 015's proxy
 |
018 CoT Display Consumer   <- Consumer DApp: SAPIENT -> CoT/TAK behind the Buyer Proxy; static
                              affiliation policy; aggregation without association
```

**Why 004 before 003:** Peer Registry's AgentURI must embed `NeuronTopicService` and `NeuronP2PExchangeService` schemas defined by Topic System.

**Specs 011 and 013 have no SDK package yet.** 011 has a reference runtime in `cmd/relay-node/` but no `internal/relay/` package; 013 is purely structural (only `spec.md`, `contracts/`, `profiles/`). Spec 012 has a partial Go-side server bridge in `internal/browserprofile/` and `cmd/webtransport-seller/`; the browser-side TypeScript client lives in `impl/typescript/`.

**Specs 015–018 form the DApp / application layer** (per Constitution Principle XII), implemented under `impl/golang/internal/dapp/`. **015** is the *shared* SAPIENT Sensor Interop Profile (`dapp/sapient/`): it adopts SAPIENT (BSI Flex 335 v2.0 protobuf) as the sensor-data envelope carried over Core lanes, via a **transparent proxy** pair (generic Seller Proxy on the sensor side, generic Buyer Proxy on the consumer side) so a SAPIENT sensor (ASM = seller) and consumer (HLDMM = buyer) talk as if joined by one wire and neither needs to know Neuron exists. **016** (`dapp/adsb/`, JetVision ADS-B) and **017** (`dapp/remoteid/`, DroneScout Remote ID) are *vendor* DApps: Neuron-blind modality→SAPIENT translators that run behind 015's proxy. **018** is the consumer-side CoT Display Consumer (`dapp/sapient/cotadapt/` implements its DetectionReport→CoT mapping); the shipping display chain (the `*fid*`/`multistream-buyer` cmds) follows the `TaggedFrame` contract in `docs/fid-display-contract.md`. The legacy `feeds/` + `edgeapp/` reverse-connect runtime predates this layering and still ships independently.

**The 015 slot was reassigned.** The old core spec 015 (Admission Policy) and 014 (Fan-Out) were deprecated 2026-05-08 (moved to DApp ownership); the 015 directory now holds the SAPIENT Sensor Interop Profile. There is no spec 014.

### Spec-to-Package Mapping

| Spec | Go `internal/` | TS `src/`    | Notes                                                            |
| ---- | -------------- | ------------ | ---------------------------------------------------------------- |
| 002  | `keylib/`      | `keylib/`    | Foundation                                                       |
| 001  | `account/`     | `account/`   | Identity                                                         |
| 004  | `topic/`       | `topic/`     | Communication                                                    |
| 003  | `registry/`    | `registry/`  | Registration                                                     |
| 005  | `health/`      | `health/`    | Liveness                                                         |
| 006  | —              | `wire/`      | Wire format + test vectors; Go has no dedicated package          |
| 007  | —              | `contracts/` | Solidity contracts; SDK bindings in `registry/`                  |
| 008  | `payment/`     | —            | Commerce protocol, negotiation, escrow                           |
| 009  | `delivery/`    | —            | P2P data delivery, libp2p binding                                |
| 010  | `validation/`  | —            | Validation framework, evidence model, verdict publisher/observer |
| 011  | —              | —            | Relay (charter scoped to the relay-only slice). Reference runtime: `cmd/relay-node/` |
| 012  | `browserprofile/` | (browser) | Browser client profile — buyer-only, WSS+libp2p, ephemeral identity, no HCS in browser. Server-side bridge: `cmd/webtransport-seller/` |
| 013  | —              | —            | Connectivity profiles (profile descriptors under `specs/013-connectivity-profiles/profiles/` are the deliverable). v2 (2026-05-08) adds `stream-init-direction` capability key |
| 015  | `dapp/sapient/` | —           | **DApp (shared profile)**: SAPIENT Sensor Interop (BSI Flex 335 v2.0 protobuf over Core lanes). Transparent Seller/Buyer Proxy; `/sapient/detection/2.0.0`. Subpkgs: `auditlane/`, `cotadapt/`, `fidadapt/`, `tasking/`, vendored `sapientpb/`. (Slot reassigned from deprecated Admission Policy.) |
| 016  | `dapp/adsb/`   | —            | **DApp (vendor ASM)**: JetVision ADS-B. `NormalizedTrack`, stream catalog `/jetvision/{raw,basestation,filtered,status,control}/...`. SAPIENT translator (`neuron.adsb/1`) behind 015's proxy |
| 017  | `dapp/remoteid/` | —          | **DApp (vendor ASM)**: DroneScout Remote ID. `RemoteIdFrame`, `/ds240/{raw,basestation,status}/1.0.0`, `rid.*` extension. SAPIENT translator behind 015's proxy; bespoke-frame draft archived |
| 018  | —              | —            | **DApp (consumer leaf)**: CoT Display Consumer — SAPIENT→CoT behind the Buyer Proxy; `dapp/sapient/cotadapt/` implements the DetectionReport→CoT mapping. The TaggedFrame display chain (`cmd/multistream-buyer`, `cmd/fid-display`, `cmd/sapient-fid-*`) follows `docs/fid-display-contract.md` |
| —    | `feeds/`, `edgeapp/` | —      | Edge demo (out-of-spec; reverse-connect topology built atop 002/001/004/005/009 for the JetVision Air!Squitter device). Predates the 015–018 DApp layering |

## Spec Lifecycle (Spec Kit Toolchain)

Each spec follows a strict 6-phase lifecycle via slash commands. Phases are sequential — complete each gate before proceeding.

```
/speckit.specify  ->  /speckit.clarify  ->  /speckit.plan  ->  /speckit.tasks  ->  /speckit.implement  ->  test (manual)
```

| Phase     | Gate to Next                                                 |
| --------- | ------------------------------------------------------------ |
| Specify   | All mandatory sections present (scenarios, FRs, SCs)         |
| Clarify   | Zero `[NEEDS CLARIFICATION]` markers remain                  |
| Plan      | Constitution Check passes 12/12 principles                   |
| Tasks     | FR/SC traceability + test tasks precede implementation tasks |
| Implement | All tasks done, code compiles, tests pass                    |
| Verify    | All tests green, exported types stable, no vet/lint warnings |

Implementation uses TDD Red-Green-Refactor (Constitution IX). Optional quality tools: `/speckit.analyze` (consistency check), `/speckit.checklist` (custom checklist), `/speckit.conform` (cross-language conformance against spec 006 vectors), `/speckit.propagate` (spec-change impact analysis across the dependency graph), `/speckit.taskstoissues` (GitHub Issues).

## AI Context Loading Strategy

When working on a specific spec, load context in this order:

1. **Always**: `CLAUDE.md`, `.specify/memory/constitution.md`, `specs/<target>/{spec,plan,tasks}.md`
2. **Dependencies**: `specs/<dep>/contracts/` (API contracts only) + `<impl-pkg>/doc.go`
3. **Implementation**: `specs/<target>/data-model.md`, `specs/<target>/contracts/`, `<impl-pkg>/`
4. **Skip**: Full spec.md of dependencies, downstream specs, research.md/quickstart.md during implementation

## Hedera Testnet Integration

The Go SDK integrates with Hedera Consensus Service (HCS) for real on-chain message publishing.

- **Operator account**: supplied via `HEDERA_OPERATOR_ID` / `HEDERA_OPERATOR_KEY` env vars (ECDSA secp256k1, funded testnet account; never hardcoded)
- **HCS adapter**: `internal/topic/adapter_hcs.go` — implements `TopicAdapter` over real HCS
- **Reusable client**: `internal/topic/hcs_real_client.go` — `RealHCSClient` wraps the Hiero SDK, implements `HCSClient` interface
- **SDK import**: `hiero "github.com/hiero-ledger/hiero-sdk-go/v2/sdk"`
- **HCS message limit**: 1024 bytes max per message (`HCSMaxMessageSize` constant)
- **Mirror node**: `https://testnet.mirrornode.hedera.com/api/v1/topics/{topicId}/messages`
- **Explorer**: `https://hashscan.io/testnet/topic/{topicId}`

The buyer-seller demo's `--mode=testnet` creates real HCS topics and publishes all protocol messages through Hedera consensus. The `demoBus` interface in the demo abstracts over `memoryTopicBus` (mock) and `hcsTopicBus` (real HCS + local message log for polling).

## Key Dependencies

### Go SDK

- `go-ethereum` — secp256k1, Keccak256, EIP-55, ecrecover
- `go-libp2p` — PeerID derivation, P2P transport (QUIC/WebRTC) for delivery
- `hiero-sdk-go` — Hedera Consensus Service (topic creation, message publishing)
- `go-bip39` / `go-bip32` — Mnemonic/HD key derivation
- `argon2` (golang.org/x/crypto) — Argon2id KDF for key encryption
- `base58` — DID:key base58btc encoding
- `google.golang.org/protobuf` — SAPIENT BSI Flex 335 v2.0 message encoding (DApp layer, `internal/dapp/sapient/sapientpb`)
- `stretchr/testify` — Test assertions

### TypeScript SDK

- `@noble/secp256k1` + `@noble/hashes` — secp256k1, Keccak256
- `@scure/bip32` / `@scure/bip39` — HD key derivation, mnemonics
- `bs58` — Base58 encoding
- `hash-wasm` — Argon2id KDF

## Constitution Principles (v1.7.0)

All implementation must comply with 12 ratified principles. Full text: `.specify/memory/constitution.md`.

| #    | Principle                                            | Key Rule                                                                                        |
| ---- | ---------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| I    | Specification-First                                  | Spec exists before any code                                                                     |
| II   | Testable Stories                                     | Given/When/Then acceptance criteria                                                             |
| III  | Clarification Before Plan                            | No unresolved ambiguities                                                                       |
| IV   | Semantic Types                                       | Domain types, not primitives                                                                    |
| V    | Traceability                                         | Every task traces to FR-_ or SC-_                                                               |
| VI   | Language-Neutral Protocol, Reference Implementations | Specs + Spec 006 vectors are normative; Go is this repo's current reference implementation path |
| VII  | Strict Spec Compliance                               | No silent divergence from MUST/SHOULD                                                           |
| VIII | Hedera Transport Binding                             | HCS is first-class adapter                                                                      |
| IX   | Test-First Development                               | TDD: Red -> Green -> Refactor                                                                   |
| X    | Deterministic Signing                                | RFC 6979 + Keccak256 + R\|\|S\|\|V                                                              |
| XI   | Verifiable Execution                                 | Evidence-based validation; validator autonomy; three-outcome verdicts                           |
| XII  | Layered Architecture (SDK Core vs DApp)              | Core SDK (001–013) defines primitives; DApp specs (016+) define topology, admission, frame formats; 011 relay charter is frozen |

## Cross-Spec Data Flow

```
NeuronPrivateKey (002) ──> NeuronAccount (001) ──> TopicRef (004) ──> AgentURI (003) ──> HeartbeatPayload (005)
      │                          │                      │                   │
      ├── EVMAddress ────────────┤                      │                   ├── register() on-chain (007)
      ├── PeerID ────────────────┼── NeuronP2PExchange ─┘                   │
      ├── DID:key ───────────────┘                                          │
      └── Sign/Verify ─────────────────── TopicMessage envelope ────────────┘
```

- **002 → 001**: Keys create account identities (EVM address, DID)
- **001 → 004**: Account keys sign TopicMessage envelopes
- **004 → 003**: Topic services populate agentURI for registration
- **003 → 007**: SDK registration calls map to on-chain `register()` / `updateAgentURI()` / `revoke()`
- **004 → 005**: Heartbeats are TopicMessages published to stdOut
- **003 → 005**: Observer discovers peer's stdOut via `LookupRegistration` + `ResolveTopics`
- **006**: Wire format and signing algorithms apply across all specs
- **007 → 008**: Payment uses Identity Registry for agent discovery + settlement
- **008 → 009**: Delivery implements the data plane for negotiated service connections (connectionSetup, ECIES encryption)
- **009**: libp2p streams carry service data; multiaddr encrypted via ECIES (secp256k1 ECDH + AES-256-GCM)
- **010**: Validation framework — validators are agents; evidence published to topics, verdicts anchored on-chain via Validation Registry (007)

## Key Integration Patterns

### Publisher-Observer Pattern (health, validation)

Both `health` and `validation` packages follow the same pattern: build payload → serialize to canonical JSON → wrap in signed `TopicMessage` via `topic.NewTopicMessage` → publish to stdOut via `TopicAdapter`. Observer side: validate TopicMessage signature → unmarshal payload → check type discriminator → version compat → field validation.

### Adapter Abstraction (topic, delivery, escrow)

Infrastructure is abstracted behind interfaces: `TopicAdapter` (HCS/Kafka/ERC/memory), `DeliveryAdapter` (libp2p), `EscrowAdapter` (memory/on-chain). The buyer-seller demo introduces `demoBus` which extends `TopicAdapter` with `getMessages()` for polling, implemented by both `memoryTopicBus` and `hcsTopicBus`.

### ConnectionSetup Bridge (008 → 009)

`delivery/negotiate_bridge.go` connects payment negotiation to P2P delivery: `BuildConnectionSetup` (seller: encrypt multiaddrs with buyer's pubkey via ECIES) and `ConnectFromSetup` (buyer: decrypt → `ProcessConnectionSetup` → `adapter.Connect`).

### File Transfer Protocol (delivery)

`delivery/filetransfer.go` wraps the frame protocol: frame 0 = JSON metadata (filename, size, SHA256), frames 1..N = raw data chunks (max 4 MiB each). `SendFile`/`ReceiveFile` operate over `DeliveryAdapter.Send/Receive`.

### Mock Escrow (payment)

`payment/mock_escrow.go` — `MemoryEscrow` implements all 6 `EscrowAdapter` methods with `math/big` arithmetic. Regular `.go` file (not `_test.go`) because demo CLI imports it. Thread-safe via `sync.Mutex`. `SetClock()` for deterministic timeout testing.

### Edge Demo (Phase B/C — reverse-connect Mode-S aggregation)

Out-of-spec runtime built on top of 002/001/004/005/009 to replicate (and extend) the deployed `neuron-sdk` v0.52.101 binary on a JetVision Air!Squitter ADS-B device. **Topology inverts the canonical demo:** the NAT'd `edge-seller` dials a publicly-reachable `edge-buyer`. Built around three new pieces:

- `internal/feeds/` — BEAST 0x1A frame parser, Mode-S DF decoder (`modes.go`), 24-bit Mode-S CRC + parity-XOR ICAO recovery cache for DF 0/4/5/16/20/21 frames (`icao_recovery.go`), three feed sources (TCP `:10003`, replay file, synthetic).
- `internal/edgeapp/` — wires `feeds` + `delivery` + `topic` + `health` into `RunSeller`/`RunBuyer`. Adds reverse-connect via `delivery/reverse_setup.go` (seller decrypts buyer's `ReverseConnectionSetup`, dials in) and stream framing via `delivery/streamframing.go` (length-prefixed frame envelopes with graceful-EOF sentinel). Multi-seller aggregation in the buyer (`buyer.go`) fans in N sellers' streams to one JSONL output sink.
- `cmd/edge-{seller,buyer}/` — entry points; `--mode={mock,testnet}`; `seller-bootstrap.json` is the buyer's pointer to the seller's pubkey + HCS topics.

Heartbeat envelope is **spec-005 v1.0.0** (intentionally not wire-compatible with the legacy deployed binary).

**Connection-manager tuning:** watermarks low=320, high=384, grace=90s, with `Protect()`/`Unprotect()` on active streams (derived from burst-load investigation of the deployed buyer).

### SAPIENT Transparent Proxy (015–018, three-tier DApp)

The DApp layer (`internal/dapp/`) realizes a three-tier model on top of the Core SDK:

1. **Shared profile — 015 (`dapp/sapient/`).** SAPIENT (BSI Flex 335 v2.0, vendored protobuf in `dapp/sapient/sapientpb/`) is the sensor↔consumer **application payload**, carried over Core lanes. A generic **Seller Proxy** (sensor side) and **Buyer Proxy** (consumer side) — one pair for *all* SAPIENT — present a conformant BSI Flex 335 edge to their local peer and route each message onto a Neuron lane purely by SAPIENT message *type*: `DetectionReport`/files → 009 P2P data plane; `Registration`/`StatusReport`/`Task`/`TaskAck` → 004 auditable topic (the "audit lane", `dapp/sapient/auditlane/`). Terms: **ASM** = sensor = seller, **HLDMM** = fusion node = buyer. The proxies own all Neuron identity/signing/lane logic; the sensor stays Neuron-blind. SAPIENT messages flow seller→buyer over `/sapient/detection/2.0.0` (reverse-connect: seller dials the public buyer).

2. **Vendor translators — 016 (`dapp/adsb/`) and 017 (`dapp/remoteid/`).** Each decodes one modality (Mode-S/ADS-B; OpenDroneID/ASTM F3411) into standard SAPIENT and runs *behind* the 015 Seller Proxy. They hold no key/wallet/lane logic — only the modality→SAPIENT mapping plus a vendor `object_info` extension (`neuron.adsb/1`, `rid.*`). 016 also keeps a non-SAPIENT decoded-track fast-path (`NormalizedTrack` over `/jetvision/basestation/1.0.0`); 017's `RemoteIdFrame` raw path rides `/ds240/raw/1.0.0`.

3. **Display composition.** A **fused buyer** (`cmd/multistream-buyer`) holds one 008 agreement per DApp, then forwards heterogeneous frames to a display through the `TaggedFrame` envelope `{source, type, sellerPeerID, receivedAt, frame}` over a sink (`stdout`/`file:`/`file+:`/`tcp:`) — documented in `docs/fid-display-contract.md`; the SAPIENT-side consumer contract is **018 CoT Display Consumer** (`dapp/sapient/cotadapt/` covers the DetectionReport→CoT mapping). Displays are embedded Leaflet map servers: `cmd/fid-display` (:8080), `cmd/sapient-fid-display` (:8193, SAPIENT-rich), `cmd/sapient-explorer` (:8194, tactical console + agent registry). Frontend assets live in each cmd's `static/` dir, embedded via `//go:embed`.

**Demo chain (local):** `make demo-sapient`, or wire the cmds manually — `sapient-feed-replay` (fixture→LE-framed protobuf bridge) → `sapient-rid-seller`/`sapient-jv-seller` (dials buyer) → `sapient-buyer` (Buyer Proxy) → `sapient-fid-consumer` (SAPIENT→TaggedFrame/CoT) → `sapient-fid-display`.

## Code Style

### Go Implementation (`impl/golang/`)

- **Package layout**: `internal/` packages with a `doc.go` per package providing the package-level godoc. Read `doc.go` first when exploring a package.
- **Error handling**: No panics. Structured error type per package (e.g., `KeyError` with `ErrorKind` enum). Errors implement `Is()` and `Unwrap()`. Format: `"pkgname.OperationName: [ErrorKind] descriptive message"`.
- **Factory functions**: `NewTypeName()` for construction, `TypeNameFromX()` for conversion.
- **Immutability**: All types immutable after construction. No setters. Invalid inputs rejected at construction. Valid by construction.
- **Semantic types**: Strong domain types, not primitives (e.g., `NeuronPrivateKey`, `EVMAddress`, `PeerID`, `Signature`).
- **FR/SC tracing**: Doc comments reference spec requirements: `// FR-001: ...`, `// SEC-003: ...`.
- **Security**: Key material MUST NOT appear in error messages or logs (SEC-003, SEC-005).
- **Testing**: `stretchr/testify` assertions. Test files alongside source (`*_test.go`). Integration tests in `integration_test.go`.

### TypeScript Implementation (`impl/typescript/`)

- **Module system**: ESM (`"type": "module"`), build with `tsc -p tsconfig.build.json`.
- **Testing**: vitest. Conformance tests in `tests/conformance/`.
- **Linting**: eslint with `@typescript-eslint`.
- **SDK derivation rule**: TypeScript SDK derives purely from specs, never from Go code.

### Specs (`specs/`)

- Language-neutral — no Go/TS types, runtime details, or framework references
- Solidity `^0.8.20` for informative contract interfaces only
- On-chain EVM state (mappings, arrays); no off-chain database for identity contracts

## Active Technologies

- Go 1.25+ (current reference implementation path per Constitution Principle VI) — `go-ethereum` (Keccak256, secp256k1), `go-libp2p` (P2P transport), `hiero-sdk-go` (Hedera HCS), `testify` (assertions)
- TypeScript 5.7+ / Node.js >= 18 — `@noble/secp256k1`, `@noble/hashes`, vitest
- Solidity ^0.8.20 — EVM contract interfaces (Identity, Reputation, Validation Registries)
- Hedera Consensus Service — real testnet integration for topic creation + message publishing (operator credentials via env vars)
- TypeScript 5.7+ on Node.js 20+ (existing `impl/typescript` package). Browser target: Chromium ≥ 120, Firefox ≥ 115 (mandatory per SC-05). (012-browser-client-profile)

## Recent Changes

- **2026-05-29 → 2026-06-12 SAPIENT sensor-interop layer** (current pass): The DApp layer moved from spec-only to implemented under `impl/golang/internal/dapp/{sapient,adsb,remoteid}/` with a fleet of new `cmd/` entry points (`adsb-seller`, `remoteid-seller`, `multistream-buyer`, `fid-display`, `edge-validator`, and the `sapient-*` family). **Spec 015 was repurposed** from the deprecated Admission Policy slot into the **SAPIENT Sensor Interop Profile** (BSI Flex 335 v2.0 protobuf as the sensor envelope over Core lanes, via a generic transparent Seller/Buyer **Proxy** pair; ASM=seller/HLDMM=buyer; message-type lane routing with an auditable 004 "audit lane" carried to HCS). **016 (JetVision ADS-B)** and **017 (DroneScout Remote ID)** became vendor SAPIENT translators behind 015's proxy (017's bespoke `RemoteIdFrame` is archived). **Spec 018** is now the **CoT Display Consumer** (SAPIENT→CoT behind the Buyer Proxy; the TaggedFrame display chain ships alongside — `multistream-buyer` + the Leaflet `fid-display` / `sapient-fid-display` / `sapient-explorer` map UIs, `static/` assets via `//go:embed`). **Demo protocols renamed** BaseStation → **JetVision** (`/jetvision/*` ADS-B) and **DS240** (`/ds240/*` Remote ID) libp2p protocol IDs. New `make demo-sapient` target. Live SAPIENT runs publish an HCS audit lane and have a recipient-side verification endpoint.
- **2026-05-08 reconciliation**: Constitution bumped to **v1.7.0** with new **Principle XII (Layered Architecture: SDK Core vs DApp)** codifying that core specs (001–013) define primitives + envelopes + signing + transport while DApp specs (016+) define topology, admission, sensor semantics, and frame formats. Core specs **014 (Fan-Out)** and **015 (Admission Policy)** explicitly **DEPRECATED** — both moved to DApp ownership by architecture decision. Core spec amendments: 008 (lifecycle messages StopService/CancelService/RenewService, active-service persistence with configurable cutoff, long-lived service discipline, degraded-mode operation, AdmissionPolicy interface stub, streams[] catalog), 009 (connection-vs-stream-direction split, multi-protocol + wildcard handler, libp2p pubsub primitives exposed without prescribing topology, multiaddr filtering for loopback/Docker/virtual interfaces, degraded mode), 005 (degraded-mode liveness FR-H30–H33), 003 (path-based protocol-ID appendix example), 013 (v1 → v2 major bump adding `stream-init-direction` capability key). New DApp specs **016 (ADS-B)** and **017 (Remote ID)** with stream catalogs, gossipsub fan-out topology, partner-allowlist admission policy implementations, fused-buyer sections. Two new companion docs under `docs/`: `dapp-frame-format-precedent.md` (rules R-FF-01 / R-FF-02 / R-FF-03 for choosing opaque vs canonical-JSON frame formats) and `dapp-admission-anchor-pattern.md` (anchors A1/A2/A3 for AdmissionPolicy backends). Spec 006 wire-format and test-vectors extended to cover all 9 commerce payloads + StreamCatalogEntry + RemoteIdFrame, with `TODO(impl-generated)` placeholders for signatures resolved by the conformance-vector generator at impl time. The reference deployment continues on the existing Hedera testnet EIP-8004 contract — no new contract deployment.
- **013-connectivity-profiles** (current branch, spec-only): Cross-cutting transport-agnostic profile model. Separates protocol semantics from transport choices via a closed-vocabulary capability layer (`control-plane`, `audit-trail`, `identity-lifetime`, `listen-capability`, `nat-traversal`, `settlement`, `max-payload`, `confidentiality`, `ordering`, `reconnect-semantics`, **`stream-init-direction` (added v2, 2026-05-08)**). Three normative profiles live under `specs/013-connectivity-profiles/profiles/`: **A** (Browser → Public Listener, WSS + WebTransport bindings), **C** (Any → NATed Peer via Relay, forward-references 011), **D** (Peer ↔ Peer Direct). **Profile E** (NATed seller → public buyer reverse-connect) under `profiles/E-natd-seller.md` is the JetVision edge-seller shape. Capability vector schema in `contracts/capability-vector.md`; profile descriptor JSON Schema in `contracts/profile-descriptor.schema.json`. Agents publish descriptors at the well-known path `/.well-known/neuron-profile.json`; legacy `bootstrap.json` / `bootstrap-wt.json` paths persist additively for ≥ N+2 releases. v1 descriptors continue to parse against v2 (defaulting `stream-init-direction = seller` for back-compat). New profiles must not require amending core specs.
- **011-relay** (spec-only): Relay-only slice carved out of the frozen "Facilitator Framework" umbrella. Reference runtime: `impl/golang/cmd/relay-node/`.
- **012-browser-client-profile**: Buyer-only browser target. Go-side server bridge in `internal/browserprofile/` and `cmd/webtransport-seller/`. TypeScript 5.7+ on Node.js 20+ (existing `impl/typescript` package). Browser target: Chromium ≥ 120, Firefox ≥ 115 (mandatory per SC-05).
