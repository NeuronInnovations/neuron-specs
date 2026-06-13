# Spec ↔ Code Map

Component-by-component mapping between the specification corpus and the Go/TypeScript implementations, with each component's status in this public source release.

**Status vocabulary**: `source` (SDK source, spec-traced, tested) · `public example` (runnable demo/CLI) · `test utility` (exists to exercise other components).

## Core SDK

| Component | Code | Owning spec | Status | Notes |
| --------- | ---- | ----------- | ------ | ----- |
| Key library | `impl/golang/internal/keylib/`, `impl/typescript/src/keylib/` | 002 | source | FR-traced doc comments; conformance tests against 006 golden vectors (`keylib/conformance_test.go`, `impl/typescript/tests/conformance/`) |
| NeuronAccount identity | `internal/account/`, `src/account/` | 001 | source | Parent/Child/Shared builders, DIDs, ledger attachment |
| Topic system | `internal/topic/`, `src/topic/` | 004 | source | `TopicAdapter` over HCS (`adapter_hcs.go`, `hcs_real_client.go`), memory bus; 1024-byte HCS limit enforced |
| Peer registry | `internal/registry/`, `src/registry/` | 003 | source | EIP-8004 registration, agentURI build/validate, lookup; abigen bindings under `registry/bindings/` |
| Health / heartbeat | `internal/health/`, `src/health/` | 005 | source | Publisher + observer; per-observer liveness state machine |
| Wire format / determinism | `src/wire/`; vectors in `specs/006-protocol-determinism/contracts/test-vectors.md` | 006 | source | Go has no dedicated package; determinism enforced via conformance tests in each package |
| Identity/Reputation/Validation contracts | `contracts/src/` (Foundry), bindings in `internal/{registry,payment}/bindings/` | 007 | source | Deploy scripts read `PRIVATE_KEY` from env; no keys in source |
| Payment / commerce | `internal/payment/` | 008 | source | Nine-message taxonomy, negotiation state machine, `EscrowAdapter` (memory + on-chain), `MemoryEscrow` mock imported by demos |
| P2P data delivery | `internal/delivery/` | 009 | source | libp2p QUIC binding, ECIES multiaddr encryption, file-transfer framing, reverse-connect setup, stream framing |
| Validation framework | `internal/validation/` | 010 | source | EvidenceEnvelope, verdict publisher/observer, validator service |
| Relay | `cmd/relay-node/` | 011 | public example | Circuit Relay v2 reference runtime |
| Browser client profile | `internal/browserprofile/`, `cmd/webtransport-seller/`, browser side in `impl/typescript/` | 012 | source | Buyer-only browser target; Go server bridge + TS client |
| Connectivity profiles | `specs/013-connectivity-profiles/` (profiles A/C/D/E/F) | 013 | spec | Capability vectors + profile descriptors are the deliverable |

## SAPIENT application layer

| Component | Code | Owning spec | Status | Notes |
| --------- | ---- | ----------- | ------ | ----- |
| SAPIENT core (ICD, encode, node-id, catalog, agent card, commerce, heartbeat) | `internal/dapp/sapient/` | 015 | source | Vendored DSTL BSI Flex 335 v2.0 protobuf in `sapientpb/`; LE-framed TCP edge (`edge_framing.go`); registry + EVM funding helpers |
| Audit lane | `internal/dapp/sapient/auditlane/` | 015 (lane binding) | source | Registration/StatusReport/Task/TaskAck onto the 004 auditable topic lane (HCS in testnet mode) |
| Tasking | `internal/dapp/sapient/tasking/` | 015 (FR-S29 STOP/START) | source | Task/TaskAck over the auditable lane |
| CoT adapter | `internal/dapp/sapient/cotadapt/` | 018 (DetectionReport→CoT mapping) | source | CoT XML projection used by `sapient-fid-consumer` |
| FID adapter | `internal/dapp/sapient/fidadapt/` | `docs/sapient-track-contract.md` | source | SAPIENT → SapientTrackSnapshot/TaggedFrame projection |
| Received-API (partner tap) | `internal/dapp/sapient/receivedtap/` | 015 (consumer obligations; informative) | source | Read-only `/sapient/received/{latest,stream,schema,health}` (`?limit=N` on latest), mounted by `cmd/sapient-fid-consumer --sapient-received-http`. Session-scoped pass-through (no modality filter in code; the partner deployment's session is drone-only). Never exposes secrets/env/keys/host paths |
| JetVision ADS-B translator (BaseStation fast path) | `internal/dapp/adsb/` | 016 | source | `NormalizedTrack` over `/jetvision/basestation/1.0.0` per `docs/normalized-track-contract.md`; the SAPIENT-side JV bridge is an external repo (spec 016 is self-sufficient) |
| DroneScout Remote ID translator | `internal/dapp/remoteid/` | 017 | source | `RemoteIdFrame` over `/ds240/raw/1.0.0` (the pre-SAPIENT path); MQTT/JSON decode plus replay and synthetic feed sources in `internal/feeds/remoteid/` |
| Mode-S/BEAST feed decode | `internal/feeds/` | edge demo (out-of-spec) | source | BEAST 0x1A parser, Mode-S DF decoder, ICAO recovery cache; TCP/replay/synthetic sources |
| Edge runtimes (reverse-connect) | `internal/edgeapp/`, `cmd/edge-seller/`, `cmd/edge-buyer/` | 013 Profile E composition | source | Predates the 015 proxy layering; NAT'd seller dials public buyer |

## Demo / runtime CLIs

| CLI | Role | Specs exercised | Status |
| --- | ---- | --------------- | ------ |
| `cmd/buyer-seller-demo` | Canonical end-to-end commerce loop (mock + testnet) | 001–010 | public example |
| `cmd/hcs-heartbeat-test` | Live testnet heartbeat publish + mirror-node verify | 004, 005 | public example |
| `cmd/delivery-demo` | Standalone P2P delivery (two terminals) | 009 | public example |
| `cmd/relay-node` | Circuit Relay v2 node | 011 | public example |
| `cmd/webtransport-seller` | Browser-buyer server bridge | 012 | public example |
| `cmd/ecies-fixture-gen` | Regenerates ECIES golden fixtures | 006/009 | test utility |
| `cmd/adsb-seller`, `cmd/remoteid-seller` | Per-modality sellers (fixture-direct or EIP-8004 modes) | 016/017 + 013 Profile F | public example |
| `cmd/multistream-buyer` | One buyer process, N seller sessions → consolidated TaggedFrame JSONL | `docs/fid-display-contract.md` + 008/009 | public example |
| `cmd/fid-display` | Leaflet map over TaggedFrame TCP input | `docs/fid-display-contract.md` | public example |
| `cmd/edge-validator` | Validator for the reverse-connect demo | 010 | public example |
| `cmd/sapient-jv-seller`, `cmd/sapient-rid-seller` | SAPIENT pushers (dial the buyer, re-stamp `node_id`) | 015 + 016/017 | public example |
| `cmd/sapient-buyer` | Generic Buyer Proxy (listens; forwards to a SAPIENT TCP edge; `GET /sessions` via `--sessions-http`: per-session peerID, nodeId, counts) | 015 | public example |
| `cmd/sapient-feed-replay` | Fixture NDJSON → live LE-framed protobuf feed (bridge stand-in) | 015 FR-S91 edge | test utility |
| `cmd/sapient-task` | HLDMM tasking CLI (STOP/START + TaskAck) | 015 tasking | public example |
| `cmd/sapient-fid-consumer` | SAPIENT → TaggedFrame/CoT projector | 015 + 018 (partial, via `cotadapt`) | public example |
| `cmd/sapient-fid-display` | SAPIENT-rich map display (`/state.json`, `/sources.json`, `/sensors.json`, `/events`; tracks keyed `nodeId\|uid`; consumes buyer `/sessions` via `--sessions-url`) | display composition | public example |
| `cmd/sapient-explorer` | Tactical console (`/agents.json`, `/agents/<id>`, `/tracks.json`, `/sensors.json`, `/events`, `/healthz`; read-only proxy over the live display + agent evidence) | display composition | public example |
| `cmd/sapient-agent-explorer` | Agent-card viewer (evidence files or on-chain lookup) | 003/007 | public example |


