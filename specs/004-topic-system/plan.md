# Implementation Plan: Topic System

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `004-topic-system` | **Date**: 2026-02-25 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-topic-system/spec.md`

## Summary

Spec 004 defines a unified, technology-agnostic topic system for public, ordered, append-only, DLT-traceable communication channels. The system provides a TopicMessage envelope (signed with NeuronPrivateKey from 002), a TopicAdapter interface abstracting backend-specific implementations (HCS, ERC event logs, Kafka), per-channel EIP-8004 service schemas (`neuron-topic`, `neuron-p2p-exchange`), standard channel roles (stdIn/stdOut/stdErr), custom named channels, and a publish/subscribe execution model with PublishResult and MessageDelivery types. The reference implementation targets Go per Constitution Principle VI, using `hedera-sdk-go` for HCS (Constitution VIII), `go-ethereum` for ERC event log reading and Keccak256 signing, and the `internal/keylib` package (002) for NeuronPrivateKey signing and verification.

## Technical Context

**Language/Version**: Go 1.22+ (Constitution Principle VI: Golang-First SDK)
**Primary Dependencies**:
- `github.com/neuron-sdk/neuron-go-sdk/internal/keylib` (Spec 002 — NeuronPrivateKey, NeuronPublicKey, Signature, EVMAddress, PeerID, Keccak256 signing, R||S||V, verification, recovery)
- `github.com/hashgraph/hedera-sdk-go/v2` (HCS adapter — ConsensusTopicCreateTransaction, ConsensusMessageSubmitTransaction, TopicMessageQuery)
- `github.com/ethereum/go-ethereum` (ERC event log adapter — ethclient, FilterLogs, abi; also Keccak256 via keylib)
- `github.com/segmentio/kafka-go` or `github.com/confluentinc/confluent-kafka-go` (Kafka adapter)
- `encoding/json` (deterministic JSON serialization — FR-T21)

**Storage**: N/A for core topic types — all types are in-memory. TransportConfig and service schemas are JSON-serializable for agentURI integration.
**Testing**: `go test` with `testify` assertions (Constitution Principle IX: Test-First). 15 success criteria (SC-T01..SC-T15). Deterministic signing verified per Principle X. HCS adapter integration tested per Principle VIII.
**Target Platform**: Linux/macOS/Windows (Go cross-compilation).
**Project Type**: Go module — `internal/topic/` package within the neuron-go-sdk module.
**Performance Goals**: TopicMessage construction and signing < 2ms. Publish latency depends on backend (HCS ~3-7s consensus, ERC-log read-only, Kafka < 100ms ack). Service schema serialization/parsing < 1ms.
**Constraints**: HCS is the first and primary adapter (Constitution VIII). ERC event log adapter is read-only (`publish()` returns `UnsupportedOperation`). Kafka requires anchoring config (FR-T16). TopicMessage payload is opaque bytes; this spec does not define payload protocols. Maximum message sizes are backend-specific (HCS ~1024 bytes per chunk).
**Scale/Scope**: Single-topic operations are the primary path. Multi-topic orchestration (e.g. fan-out to all channels) is application-level.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| # | Principle | Gate | Status |
|---|-----------|------|--------|
| I | Specification-First | spec.md exists with mandatory sections (Purpose, User Scenarios, Requirements, Success Criteria) | **PASS** |
| II | Independently Testable Stories | Stories prioritized (P1/P2/P3), Given/When/Then acceptance scenarios, each independently testable | **PASS** -- 7 stories, 19 acceptance scenarios |
| III | Clarification Before Plan | No [NEEDS CLARIFICATION] markers; underspecified areas resolved before plan | **PASS** -- 0 markers. 12 clarification questions resolved in Session 2026-02-10. Out of scope items clearly delineated |
| IV | High-Level Types | Semantic types used (TopicRef, TopicMessage, TopicAdapter, PublishResult, MessageDelivery, NeuronTopicService, NeuronP2PExchangeService, ChannelRole, BackendKind, ConfirmationMode, TopicError) | **PASS** |
| V | Traceability | FRs, SCs, Key Entities present and aligned | **PASS** -- 29 FRs (FR-T01..FR-T29), 15 SCs (SC-T01..SC-T15), 10 key entities, all cross-referenced |
| VI | Golang-First SDK | Plan targets Go 1.22+ with `internal/topic/` layout, `*_test.go` colocated tests, `go test` tooling | **PASS** |
| VII | Strict Spec Compliance | Every task traces to FR-T* or SC-T* requirements; no silent deviations | **PASS** -- 29 FRs + 15 SCs mapped to tasks |
| VIII | Hedera Transport Binding | HCS is the first and primary adapter. HCS adapter uses hedera-sdk-go ConsensusTopicCreateTransaction and ConsensusMessageSubmitTransaction. Constitution VIII is directly applicable | **PASS** -- HCS adapter is first-class, built-in, and the default backend for stdIn/stdErr in the agentURI example |
| IX | Test-First Development | Test tasks (Red phase) precede implementation tasks in each phase; 100% MUST-level coverage | **PASS** -- `*_test.go` tasks before each implementation phase |
| X | Deterministic Signing | TopicMessage signing uses NeuronPrivateKey.Sign() from 002 which provides deterministic signing (Keccak256 + RFC 6979). Signing tests include determinism verification (sign twice, assert byte-equal) | **PASS** -- signing_test.go task included for TopicMessage deterministic signing (FR-T03, FR-T21) |
| -- | Related specs section | Present with internal (001, 002, 003) and external (EIP-8004, HCS) cross-references | **PASS** |
| -- | Mermaid diagrams | Present in spec: erDiagram (entity model), sequenceDiagram (publish/subscribe flow), sequenceDiagram (peer discovery and signaling flow) | **PASS** |
| -- | Blockchain compatibility | HCS (Hedera) is first-class adapter; ERC event logs (Ethereum/EVM) supported; Kafka with ledger anchoring supported. Transport-agnostic design supports future backends | **PASS** |

**Gate result**: All gates PASS. Proceeding to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/004-topic-system/
├── spec.md                  # Feature specification (complete)
├── plan.md                  # This file
├── research.md              # Phase 0 output -- library selection decisions
├── data-model.md            # Phase 1 output -- entity model
├── quickstart.md            # Phase 1 output -- SDK integration guide
├── contracts/               # Phase 1 output -- API contracts
│   ├── topic-adapter.md     # TopicAdapter interface contract
│   └── service-schema.md    # NeuronTopicService + NeuronP2PExchangeService JSON schemas
├── checklists/
│   └── requirements.md      # Requirements traceability checklist
└── 004-topic-system.png     # High-level architecture diagram
```

### Source Code (Go module -- Constitution VI)

```text
internal/topic/
├── topic_ref.go              # TopicRef type (BackendKind + BackendLocator) + Topic URI parsing
├── topic_ref_test.go         # Construction, validation, URI round-trip, invalid inputs
├── message.go                # TopicMessage envelope: senderAddress, signature, timestamp, sequenceNumber, payload
├── message_test.go           # Construction, JSON serialization determinism (FR-T21), field validation
├── adapter.go                # TopicAdapter interface definition + adapter registry
├── adapter_test.go           # Interface compliance tests, registry tests, mock adapter
├── adapter_hcs.go            # HCS adapter: createTopic, publish, subscribe, resolve via hedera-sdk-go
├── adapter_hcs_test.go       # HCS adapter unit tests (mocked Hedera client)
├── adapter_erc_log.go        # ERC event log adapter: subscribe (read-only), resolve via go-ethereum ethclient
├── adapter_erc_log_test.go   # ERC log adapter unit tests, read-only publish rejection
├── adapter_kafka.go          # Kafka adapter: createTopic, publish, subscribe with anchoring (FR-T16)
├── adapter_kafka_test.go     # Kafka adapter unit tests, anchoring validation
├── service.go                # NeuronTopicService + NeuronP2PExchangeService: EIP-8004 service schemas
├── service_test.go           # Service serialization round-trip (SC-T09), cross-ref validation (SC-T10)
├── p2p_exchange.go           # NeuronP2PExchangeService type + topicRef cross-reference validation (FR-T17, FR-T18)
├── p2p_exchange_test.go      # Cross-reference validation, broken ref detection
├── channel.go                # ChannelRole enum + standard/custom channel naming (FR-T07, FR-T08)
├── channel_test.go           # Reserved name rejection, custom namespace validation
├── publish_result.go         # PublishResult type + ConfirmationMode enum (FR-T22, FR-T23)
├── publish_result_test.go    # Confirmation modes, nullable fields, confirmed/unconfirmed states
├── message_delivery.go       # MessageDelivery type + consensus timestamp mapping (FR-T24, FR-T29)
├── message_delivery_test.go  # Timestamp mapping per backend, backendSequence
├── validation.go             # TopicMessage integrity validation: signature verify, sender match (FR-T10)
├── validation_test.go        # Valid/invalid/tampered message tests, sender mismatch (SC-T06)
├── errors.go                 # TopicError structured error types: 11 error kinds (FR-T11)
├── errors_test.go            # Error kind coverage (SC-T07)
├── signing_test.go           # Deterministic TopicMessage signing verification (Constitution X, FR-T03)
├── hcs_test.go               # HCS integration tests: create topic, publish, subscribe, consensus timestamps
└── integration_test.go       # Full lifecycle: create topic -> publish signed message -> subscribe -> verify -> parse agentURI -> discover
```

**Structure Decision**: Go package (`internal/topic/`) within the neuron-go-sdk module. Tests colocated as `*_test.go` per Go conventions. `internal/` ensures the topic package is not importable by external modules -- only the SDK's public API surface in `pkg/` exposes topic functionality. Each entity maps to its own file pair (source + test) for clear separation and parallel development. The adapter pattern uses a Go interface (`TopicAdapter`) with concrete implementations per backend. The HCS adapter is the first built-in adapter per Constitution VIII; ERC event log and Kafka adapters follow.

### Dependencies on Other Specs

| Dependency | Package | Used For | Source FRs |
|------------|---------|----------|------------|
| Spec 002 (Key Library) | `internal/keylib` | NeuronPrivateKey.Sign(), Signature.Verify(), Signature.RecoverPublicKey(), EVMAddress, PeerID, Keccak256 | FR-T02, FR-T03, FR-T10, FR-T17 |
| Spec 003 (Peer Registry) | `internal/registry` (or via agentURI resolution) | agentURI parsing, service discovery, TopicRef extraction | FR-T09, FR-T14, FR-T17 |
| go-ethereum | `github.com/ethereum/go-ethereum` | ERC event log adapter (ethclient, FilterLogs, abi), Keccak256 (via keylib) | FR-T05 (erc-log adapter) |
| hedera-sdk-go | `github.com/hashgraph/hedera-sdk-go/v2` | HCS adapter (ConsensusTopicCreateTransaction, ConsensusMessageSubmitTransaction, TopicMessageQuery) | FR-T05 (hcs adapter), Constitution VIII |
| kafka-go | `github.com/segmentio/kafka-go` | Kafka adapter (Producer, Consumer, topic management) | FR-T05 (kafka adapter), FR-T16 |

## Complexity Tracking

No constitution violations requiring justification.

### Identified Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| HCS message size limit (~1024 bytes) constrains payload | Medium | FR-T27: maxMessageSize() exposed per adapter; publish() rejects oversized before submission. Future: chunking support |
| Kafka anchoring adds latency and complexity | Medium | FR-T16: anchoring config is explicit; interval configurable (batch vs per-message). Non-ledger-native tradeoff documented |
| Deterministic JSON serialization across Go versions | Low | FR-T21: canonical field ordering defined in data-model; tested with round-trip byte equality (SC-T08) |
| ERC event log adapter is read-only | Low | FR-T04: UnsupportedOperation error for publish/createTopic on read-only adapters. Clear in API contract |
