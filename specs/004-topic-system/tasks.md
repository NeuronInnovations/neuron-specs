# Tasks: Topic System (Unified Topics)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Input**: Design documents from `/specs/004-topic-system/`
**Prerequisites**: plan.md (required), spec.md (required), data-model.md, contracts/topic-adapter.md, contracts/service-schema.md
**Branch**: `004-topic-system`

**Tests**: Constitution IX mandates test-first development. Within each phase, test tasks (`*_test.go`) appear BEFORE implementation tasks. Constitution X mandates deterministic signing verification. Constitution VIII mandates HCS as the first adapter implemented and tested.

**Organization**: Tasks are grouped by phase aligned to user stories. Each task traces to FR-T* or SC-T* requirements per Constitution VII. Every task includes the exact Go file path within `internal/topic/`.

## Format: `- [x] T### [P?] [US#?] Description`

- `- [x]` checkbox is MANDATORY
- **T###**: Sequential task IDs (T001, T002, ...)
- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[US#]**: User story label (US1 through US7) for story-phase tasks
- Every task references FR-T* or SC-T* it satisfies (Constitution VII)
- Every task includes exact Go file path in `internal/topic/`

## Dependencies on Other Specs

| Dependency | Package | Used For | Source FRs |
|------------|---------|----------|------------|
| Spec 002 (Key Library) | `internal/keylib` | NeuronPrivateKey.Sign(), Signature.Verify(), Signature.RecoverPublicKey(), EVMAddress, PeerID, Keccak256 | FR-T02, FR-T03, FR-T10, FR-T17 |
| Spec 003 (Peer Registry) | `internal/registry` | agentURI parsing, service discovery, TopicRef extraction | FR-T09, FR-T14, FR-T17 |
| go-ethereum | `github.com/ethereum/go-ethereum` | ERC event log adapter (ethclient, FilterLogs, abi), Keccak256 (via keylib) | FR-T05 |
| hedera-sdk-go | `github.com/hashgraph/hedera-sdk-go/v2` | HCS adapter (ConsensusTopicCreateTransaction, ConsensusMessageSubmitTransaction, TopicMessageQuery) | FR-T05, Constitution VIII |
| kafka-go | `github.com/segmentio/kafka-go` | Kafka adapter (Producer, Consumer, topic management) | FR-T05, FR-T16 |

---

## Phase 1: Setup (Go Module Init, Dependencies, Project Structure)

**Purpose**: Initialize the Go module, declare dependencies, and create the `internal/topic/` package skeleton with all planned files.

- [x] T001 Create `internal/topic/` package directory and initialize all planned Go source files per plan.md project structure: `topic_ref.go`, `message.go`, `adapter.go`, `adapter_hcs.go`, `adapter_erc_log.go`, `adapter_kafka.go`, `service.go`, `p2p_exchange.go`, `channel.go`, `publish_result.go`, `message_delivery.go`, `validation.go`, `errors.go` with package declarations (FR-T01..FR-T29)
- [x] T002 [P] Add external Go module dependencies to `go.mod`: `github.com/hashgraph/hedera-sdk-go/v2` (Constitution VIII), `github.com/ethereum/go-ethereum`, `github.com/segmentio/kafka-go`, `github.com/stretchr/testify` (FR-T05)
- [x] T003 [P] Create all planned test files in `internal/topic/`: `topic_ref_test.go`, `message_test.go`, `adapter_test.go`, `adapter_hcs_test.go`, `adapter_erc_log_test.go`, `adapter_kafka_test.go`, `service_test.go`, `p2p_exchange_test.go`, `channel_test.go`, `publish_result_test.go`, `message_delivery_test.go`, `validation_test.go`, `errors_test.go`, `signing_test.go`, `hcs_test.go`, `integration_test.go` with package declarations (Constitution IX)

---

## Phase 2: Foundational (Shared Types, Enums, Error Types)

**Purpose**: Implement all shared types, enums, and error types that every user story depends on. MUST complete before any user story phase.

**CRITICAL**: No user story work can begin until this phase is complete.

### Tests First (Constitution IX)

- [x] T004 Write tests for `BackendKind` enum in `internal/topic/topic_ref_test.go`: validate known kinds (`hcs`, `erc-log`, `kafka`, `custom:<type>`), reject empty/unknown kinds, test string round-trip (FR-T01, FR-T05)
- [x] T005 [P] Write tests for `TopicRef` in `internal/topic/topic_ref_test.go`: construction with `NewTopicRef`, validation (non-empty locator, known transport), URI round-trip via `URI()` and `TopicRefFromURI()` for all schemes (hcs, erc-log, kafka), reject invalid inputs with `InvalidTopicRef` error (FR-T01, FR-T12)
- [x] T006 [P] Write tests for `ChannelRole` enum in `internal/topic/channel_test.go`: parse standard roles (`stdIn`, `stdOut`, `stdErr`), parse custom roles (`custom:metrics`, `custom:heartbeat`), reject reserved name collision, reject missing `custom:` prefix, reject empty custom name (FR-T07, FR-T08, SC-T07)
- [x] T007 [P] Write tests for `TopicError` structured errors in `internal/topic/errors_test.go`: construct all 11 error kinds (`BackendUnavailable`, `TopicNotFound`, `MessageTooLarge`, `UnsupportedOperation`, `InvalidSignature`, `SenderMismatch`, `UnsupportedTransport`, `InvalidTopicRef`, `ReservedChannelName`, `InvalidConfig`, `BrokenTopicRef`), verify Kind/Message/BackendError fields, verify error string output (FR-T11, SC-T07)
- [x] T008 [P] Write tests for `ConfirmationMode` enum in `internal/topic/publish_result_test.go`: validate `FIRE_AND_FORGET` and `WAIT_FOR_CONSENSUS` values (FR-T23)

### Implementation

- [x] T009 Implement `BackendKind` type and constants (`HCS`, `ERCLog`, `Kafka`, custom prefix detection) in `internal/topic/topic_ref.go` (FR-T01, FR-T05)
- [x] T010 Implement `TopicRef` struct with `Transport` (BackendKind) and `Locator` (string) fields, `NewTopicRef()` constructor with validation, `Validate()` method, `URI()` serialization, `TopicRefFromURI()` parser in `internal/topic/topic_ref.go` (FR-T01, FR-T12)
- [x] T011 [P] Implement `ChannelRole` type with `ChannelRoleFromString()` parser, `StandardChannelRoles()` helper, reserved name validation, `custom:` namespace enforcement in `internal/topic/channel.go` (FR-T07, FR-T08, FR-T13)
- [x] T012 [P] Implement `TopicError` struct with `Kind` (TopicErrorKind), `Message` (string), `BackendError` (error) fields, `Error()` method, all 11 error kind constants in `internal/topic/errors.go` (FR-T11)
- [x] T013 [P] Implement `ConfirmationMode` enum (`FIRE_AND_FORGET`, `WAIT_FOR_CONSENSUS`) in `internal/topic/publish_result.go` (FR-T23)
- [x] T014 [P] Implement `PublishResult` struct with `TransactionRef`, `ConsensusTimestamp` (*uint64), `SequenceNumber` (*uint64), `Confirmed` (bool) fields in `internal/topic/publish_result.go` (FR-T22)
- [x] T015 [P] Implement `MessageDelivery` struct with `Message` (TopicMessage), `ConsensusTimestamp` (uint64), `BackendSequence` (uint64) fields in `internal/topic/message_delivery.go` (FR-T24, FR-T29)
- [x] T016 [P] Implement supporting types in `internal/topic/adapter.go`: `CreateTopicOpts`, `PublishOpts`, `SubscribeOpts`, `TopicMetadata`, `CostEstimate` structs (FR-T04, FR-T23, FR-T25, FR-T27, FR-T28)

**Checkpoint**: Foundation ready -- all shared types, enums, and error types are implemented and tested. User story phases can begin.

---

## Phase 3: US1 -- Create and Publish Signed Messages (Priority: P1)

**Goal**: A developer can create a topic on a supported backend, construct a signed TopicMessage, and publish it through the unified TopicAdapter interface, receiving a PublishResult.

**Independent Test**: Create topic via HCS adapter, publish a signed TopicMessage, verify message persisted with correct envelope fields. Covers SC-T01, SC-T02, SC-T08, SC-T11, SC-T12, SC-T15.

### Tests First (Constitution IX)

- [x] T017 [US1] Write tests for `TopicMessage` construction and field validation in `internal/topic/message_test.go`: construct via `NewTopicMessage(key, timestamp, seqNum, payload)`, verify senderAddress derived from key, verify all mandatory fields populated, reject nil key, reject nil payload (FR-T02)
- [x] T018 [US1] Write deterministic JSON serialization tests in `internal/topic/message_test.go`: serialize TopicMessage to JSON, verify canonical field order (`senderAddress`, `signature`, `timestamp`, `sequenceNumber`, `payload`), round-trip serialize -> deserialize -> re-serialize produces identical bytes (FR-T21, SC-T08)
- [x] T019 [US1] Write deterministic signing test in `internal/topic/signing_test.go`: sign the same TopicMessage (identical timestamp, sequenceNumber, payload) twice with the same NeuronPrivateKey, assert byte-equal signatures; verify signing covers `Keccak256(timestamp_8bytes_BE || sequenceNumber_8bytes_BE || payload)` (FR-T03, Constitution X)
- [x] T020 [US1] Write tests for `TopicAdapter` interface compliance in `internal/topic/adapter_test.go`: define mock adapter implementing TopicAdapter, verify all 7 interface methods are callable, verify adapter registry (`RegisterAdapter`, `GetAdapter`) works, verify duplicate registration rejected, verify `GetAdapter` for unknown transport returns `UnsupportedTransport` error (FR-T04, FR-T05, SC-T05)
- [x] T021 [US1] Write tests for `PublishResult` states in `internal/topic/publish_result_test.go`: verify `FIRE_AND_FORGET` state (confirmed=false, timestamps nil), verify `WAIT_FOR_CONSENSUS` state (confirmed=true, timestamps populated), verify nullable fields (FR-T22, FR-T23, SC-T12)
- [x] T022 [US1] Write HCS adapter unit tests in `internal/topic/adapter_hcs_test.go`: mock Hedera client, test `CreateTopic` returns valid TopicRef, test `Publish` with both confirmation modes, test `MaxMessageSize` returns 1024, test `SupportedTransport` returns `hcs`, test oversized message rejected with `MessageTooLarge` before submission (FR-T04, FR-T05, FR-T27, SC-T15, Constitution VIII)

### Implementation

- [x] T023 [US1] Implement `TopicMessage` struct with `SenderAddress`, `Signature`, `Timestamp`, `SequenceNumber`, `Payload` fields in `internal/topic/message.go` (FR-T02)
- [x] T024 [US1] Implement `NewTopicMessage(key NeuronPrivateKey, timestamp uint64, sequenceNumber uint64, payload []byte)` constructor in `internal/topic/message.go`: derive senderAddress from key, compute signing hash as `Keccak256(timestamp_8bytes_BE || sequenceNumber_8bytes_BE || payload)`, sign with `key.Sign(hash)`, populate all fields (FR-T02, FR-T03)
- [x] T025 [US1] Implement deterministic JSON serialization in `internal/topic/message.go`: `ToJSON()` and `TopicMessageFromJSON()` with canonical field order (`senderAddress`, `signature`, `timestamp`, `sequenceNumber`, `payload`), base64 encoding for Signature and Payload (FR-T21, SC-T08)
- [x] T026 [US1] Implement `TopicAdapter` interface definition in `internal/topic/adapter.go`: all 7 methods (`CreateTopic`, `Publish`, `Subscribe`, `Resolve`, `MaxMessageSize`, `EstimatePublishCost`, `SupportedTransport`) (FR-T04)
- [x] T027 [US1] Implement adapter registry in `internal/topic/adapter.go`: `RegisterAdapter(adapter TopicAdapter) error`, `GetAdapter(transport BackendKind) (TopicAdapter, error)`, global registry map `BackendKind -> TopicAdapter`, reject duplicate registration (FR-T05, SC-T05)
- [x] T028 [US1] Implement HCS adapter in `internal/topic/adapter_hcs.go`: `CreateTopic` via `ConsensusTopicCreateTransaction`, `Publish` via `ConsensusMessageSubmitTransaction` with both confirmation modes, `Subscribe` via `TopicMessageQuery` (mirror node), `Resolve` via `TopicInfoQuery`, `MaxMessageSize` returns 1024, `EstimatePublishCost` with tinybar estimate, `SupportedTransport` returns `hcs`, exponential backoff retry for transient failures (FR-T04, FR-T05, FR-T22, FR-T23, FR-T26, FR-T27, FR-T28, Constitution VIII)
- [x] T029 [US1] Implement message size pre-check in HCS adapter `Publish()` in `internal/topic/adapter_hcs.go`: validate `len(msg.Payload) <= MaxMessageSize()` BEFORE submitting to backend, return `MessageTooLarge` error on violation (FR-T27, SC-T15)

**Checkpoint**: US1 complete -- topics can be created on HCS, signed messages can be published, PublishResult returned with correct confirmation modes. SC-T01 (partial), SC-T02 (partial), SC-T08, SC-T11 (partial), SC-T12, SC-T15 (HCS) are verifiable.

---

## Phase 4: US2 -- Subscribe and Consume Verified Messages (Priority: P1)

**Goal**: A developer can subscribe to an existing topic via any adapter, receive messages as `MessageDelivery` objects with authoritative consensus timestamps, validate message integrity (signature + sender match), and resume from a given sequence number.

**Independent Test**: Subscribe to a topic with known messages, consume them, verify per-sender ordering, envelope integrity, signature validity, and resumption from sequence. Covers SC-T02, SC-T03, SC-T06, SC-T13, SC-T14.

### Tests First (Constitution IX)

- [x] T030 [US2] Write tests for `MessageDelivery` timestamp mapping in `internal/topic/message_delivery_test.go`: verify HCS consensus timestamp (nanoseconds), verify ERC-log `blockNumber * 10000 + logIndex` composite, verify Kafka anchor timestamp, verify `backendSequence` populated per backend (FR-T24, FR-T29, SC-T13)
- [x] T031 [US2] Write tests for `ValidateTopicMessage` in `internal/topic/validation_test.go`: valid message passes, tampered payload rejected with `InvalidSignature`, altered senderAddress rejected with `SenderMismatch`, unsigned message rejected, verify constant-time address comparison (FR-T10, SC-T06)
- [x] T032 [US2] Write subscription resumption test in `internal/topic/adapter_hcs_test.go`: subscribe with `FromSequence`, verify backfill from that sequence to current position, verify at-least-once delivery semantics, verify channel closure on cancel (FR-T25, SC-T14)

### Implementation

- [x] T033 [US2] Implement `ValidateTopicMessage(msg TopicMessage) error` in `internal/topic/validation.go`: recover public key from signature via `Signature.RecoverPublicKey(Keccak256(timestamp || sequenceNumber || payload))`, derive recovered address via `recoveredPubKey.EVMAddress()`, constant-time compare with `msg.SenderAddress`, return `InvalidSignature` or `SenderMismatch` on failure (FR-T10, SC-T06)
- [x] T034 [US2] Implement HCS adapter `Subscribe()` in `internal/topic/adapter_hcs.go`: open subscription via `TopicMessageQuery` on mirror node, map HCS consensus timestamp to `MessageDelivery.ConsensusTimestamp` (nanoseconds), map HCS sequence number to `BackendSequence`, support `FromSequence` resumption, automatic reconnect with exponential backoff, return `TopicNotFound` for non-existent topics (FR-T04, FR-T24, FR-T25, FR-T26, FR-T29, SC-T13, SC-T14)
- [x] T035 [US2] Implement message integrity validation in subscription pipeline in `internal/topic/adapter_hcs.go`: each received message is deserialized via `TopicMessageFromJSON()`, validated via `ValidateTopicMessage()`, wrapped in `MessageDelivery` before delivery to consumer channel (FR-T10, SC-T06)
- [x] T036 [US2] Implement per-sender ordering guarantee enforcement in `internal/topic/validation.go`: expose `sequenceNumber` on `MessageDelivery` for consumer-side per-sender ordering verification; document that cross-sender ordering is backend-dependent (FR-T06, SC-T03)

**Checkpoint**: US1 + US2 complete -- full publish -> subscribe -> verify lifecycle works on HCS. SC-T02, SC-T03, SC-T06, SC-T13, SC-T14 are verifiable.

---

## Phase 5: US3 -- Represent Topics as EIP-8004 Services (Priority: P2)

**Goal**: A developer can serialize topic channels as `neuron-topic` services in an EIP-8004 agentURI JSON document, with transport-specific config schemas for HCS, ERC-log, and Kafka.

**Independent Test**: Create three topics, form NeuronTopicService objects, assemble agentURI JSON, parse back, extract TopicRefs, verify round-trip. Covers SC-T04, SC-T09.

### Tests First (Constitution IX)

- [x] T037 [US3] Write tests for `NeuronTopicService` in `internal/topic/service_test.go`: construct with all required fields (`type`, `name`, `version`, `channel`, `transport`, `anchor`, `config`), verify `type` is always `"neuron-topic"`, verify JSON serialization round-trip for HCS config, ERC-log config, and Kafka config produces identical structures (FR-T14, FR-T15, SC-T09)
- [x] T038 [US3] Write tests for `TransportConfig` validation in `internal/topic/service_test.go`: validate HCS config (`network` + `topicId`), validate ERC-log config (`chainId` + `contractAddress` + `eventSignature`), validate Kafka config (`bootstrapServers` + `topicName` + `anchoring`), reject missing required fields with `InvalidConfig` error, reject Kafka config without anchoring (FR-T15, FR-T16, SC-T07)
- [x] T039 [US3] Write tests for `ParseAgentURIServices` and `ExtractTopicRef` in `internal/topic/service_test.go`: parse complete agentURI JSON with 3 neuron-topic services and 1 neuron-p2p-exchange service, extract TopicRefs for each, verify forward-compatible (ignore unknown service types) (FR-T09, SC-T04)

### Implementation

- [x] T040 [US3] Implement `NeuronTopicService` struct with all required fields in `internal/topic/service.go`: `Type`, `Name`, `Endpoint`, `Version`, `Channel`, `Transport`, `Anchor`, `Config` (TransportConfig) (FR-T14)
- [x] T041 [US3] Implement `TransportConfig` discriminated union types in `internal/topic/service.go`: `HCSConfig` (network, topicId), `ERCLogConfig` (chainId, contractAddress, eventSignature), `KafkaConfig` (bootstrapServers, topicName, saslMechanism, anchoring), `AnchoringConfig` (method, anchorTopicId, anchorNetwork, interval) with validation per schema (FR-T15, FR-T16)
- [x] T042 [US3] Implement `SerializeTopicService(svc NeuronTopicService) ([]byte, error)` in `internal/topic/service.go`: validate all required fields, serialize to JSON with canonical field order (`type`, `name`, `endpoint`, `version`, `channel`, `transport`, `anchor`, `config`) (FR-T14, SC-T09)
- [x] T043 [US3] Implement `ParseAgentURIServices(jsonBytes []byte) ([]NeuronTopicService, []NeuronP2PExchangeService, error)` in `internal/topic/service.go`: parse services array, discriminate by `type` field, deserialize and validate, ignore unknown types for forward-compatibility (FR-T09, SC-T04)
- [x] T044 [US3] Implement `ExtractTopicRef(svc NeuronTopicService) (TopicRef, error)` in `internal/topic/service.go`: read transport as BackendKind, extract locator from config (HCS: topicId, ERC-log: chainId+contractAddress, Kafka: topicName), construct and validate TopicRef (FR-T09, FR-T12)
- [x] T045 [US3] Implement `TopicRefFromService(svc NeuronTopicService) (TopicRef, error)` as alias/wrapper for `ExtractTopicRef` in `internal/topic/topic_ref.go` for constructor symmetry (FR-T09)

**Checkpoint**: US3 complete -- topics representable as EIP-8004 services, serialized/parsed with round-trip fidelity. SC-T04, SC-T09 are verifiable.

---

## Phase 6: US4 -- Discover Peer Channels (Priority: P2)

**Goal**: A developer can resolve a peer's agentURI (via Spec 003), parse `neuron-topic` services, extract TopicRefs for standard channels, and subscribe to them.

**Independent Test**: Resolve peer's agentURI, parse services, extract stdOut TopicRef, subscribe, receive messages. Covers SC-T04 (end-to-end).

### Tests First (Constitution IX)

- [x] T046 [US4] Write tests for end-to-end peer discovery in `internal/topic/service_test.go`: given a complete agentURI JSON, parse all neuron-topic services, extract TopicRefs for stdIn/stdOut/stdErr, verify each TopicRef has correct transport and locator, verify services with mixed backends (HCS + Kafka) parsed correctly (FR-T09, FR-T13, SC-T04)

### Implementation

- [x] T047 [US4] Implement `DiscoverPeerChannels(agentURIJson []byte) (map[ChannelRole]TopicRef, error)` in `internal/topic/service.go`: parse agentURI, extract neuron-topic services, map each to ChannelRole -> TopicRef, verify standard channels present, support mixed backends (FR-T09, FR-T13, SC-T04)
- [x] T048 [US4] Implement convenience functions in `internal/topic/service.go`: `FindStdIn(services []NeuronTopicService) (*NeuronTopicService, error)`, `FindStdOut(...)`, `FindStdErr(...)` for quick standard channel lookup from parsed services (FR-T07, FR-T09)

**Checkpoint**: US4 complete -- peer discovery workflow functional from agentURI parse through TopicRef extraction. SC-T04 is fully verifiable end-to-end.

---

## Phase 7: US5 -- Multiaddress Discovery in AgentURI (Priority: P2)

**Goal**: A developer can construct a `neuron-p2p-exchange` service with a valid topicRef cross-reference to a neuron-topic service, and validate cross-references within an agentURI document.

**Independent Test**: Construct p2p exchange service with valid topicRef, validate cross-reference succeeds; use broken topicRef, validate cross-reference fails with `BrokenTopicRef`. Covers SC-T10.

### Tests First (Constitution IX)

- [x] T049 [US5] Write tests for `NeuronP2PExchangeService` in `internal/topic/p2p_exchange_test.go`: construct with all required fields (`type`, `name`, `version`, `peerID`, `protocol`, `topicRef`), verify `type` is always `"neuron-p2p-exchange"`, verify peerID format, verify protocol starts with `/`, verify topicRef is valid channel role name (FR-T17)
- [x] T050 [US5] Write tests for cross-reference validation in `internal/topic/p2p_exchange_test.go`: valid topicRef resolves to existing neuron-topic service name, broken topicRef produces `BrokenTopicRef` error, multiple p2p services with mixed valid/broken refs (FR-T18, SC-T10)

### Implementation

- [x] T051 [US5] Implement `NeuronP2PExchangeService` struct with all required fields in `internal/topic/p2p_exchange.go`: `Type`, `Name`, `Version`, `PeerID`, `Protocol`, `TopicRef`, field-level validation (FR-T17)
- [x] T052 [US5] Implement `ValidateCrossReferences(topics []NeuronTopicService, p2p []NeuronP2PExchangeService) error` in `internal/topic/p2p_exchange.go`: build name set from topic services, check each p2p service's topicRef against the set, return `BrokenTopicRef` error for unresolved references (FR-T18, SC-T10)
- [x] T053 [US5] Integrate cross-reference validation into `ParseAgentURIServices` in `internal/topic/service.go`: after parsing both service types, automatically call `ValidateCrossReferences` and return error if any broken refs found (FR-T18)

**Checkpoint**: US5 complete -- p2p exchange services fully validated including cross-references. SC-T10 is verifiable.

---

## Phase 8: US6 -- Custom Backend via Adapter (Priority: P3)

**Goal**: A developer can implement a custom `TopicAdapter`, register it at runtime, and use the unified API identically to built-in adapters. Includes ERC-log adapter (read-only) and Kafka adapter (read-write with anchoring).

**Independent Test**: Register mock/in-memory adapter, create topic, publish, subscribe, verify unified API works. Register ERC-log adapter, verify publish returns `UnsupportedOperation`. Covers SC-T05, SC-T07 (UnsupportedOperation).

### Tests First (Constitution IX)

- [x] T054 [US6] Write HCS integration tests in `internal/topic/hcs_test.go`: full lifecycle test with real or emulated HCS client -- create topic, publish signed message with `WAIT_FOR_CONSENSUS`, subscribe from sequence 0, verify received message matches published, verify consensus timestamp is HCS-authoritative (not sender-reported), verify backendSequence matches HCS sequence number (FR-T04, FR-T22, FR-T23, FR-T24, FR-T29, SC-T01, SC-T02, SC-T11, SC-T12, SC-T13, Constitution VIII)
- [x] T055 [US6] Write ERC-log adapter unit tests in `internal/topic/adapter_erc_log_test.go`: mock ethclient, test `Subscribe` with `SubscribeFilterLogs` and `FilterLogs` for backfill, test `Resolve` returns contract metadata, test `CreateTopic` returns `UnsupportedOperation`, test `Publish` returns `UnsupportedOperation`, test `MaxMessageSize` returns 0, test consensus timestamp mapping (`block.timestamp` -> nanoseconds), test backendSequence mapping (`blockNumber * 10000 + logIndex`) (FR-T04, FR-T05, FR-T29, SC-T07, SC-T13)
- [x] T056 [US6] Write Kafka adapter unit tests in `internal/topic/adapter_kafka_test.go`: mock kafka.Writer/Reader, test `CreateTopic`, test `Publish` with both confirmation modes, test `Subscribe` with offset resumption, test `MaxMessageSize` returns 1048576, test anchoring config validation (reject missing anchoring), test backendSequence mapping (Kafka partition offset) (FR-T04, FR-T05, FR-T16, FR-T27, SC-T01, SC-T15)
- [x] T057 [US6] Write custom adapter registration test in `internal/topic/adapter_test.go`: implement minimal in-memory TopicAdapter with `custom:inmemory` transport, register, create topic, publish, subscribe, verify unified API works identically to built-in adapters, verify no core API changes required (FR-T05, SC-T05)

### Implementation

- [x] T058 [US6] Implement ERC-log adapter in `internal/topic/adapter_erc_log.go`: `Subscribe` via `ethclient.SubscribeFilterLogs()` (live) + `ethclient.FilterLogs()` (backfill), `Resolve` via contract query, `CreateTopic` returns `UnsupportedOperation`, `Publish` returns `UnsupportedOperation`, `MaxMessageSize` returns 0, `EstimatePublishCost` returns `UnsupportedOperation`, `SupportedTransport` returns `erc-log`, consensus timestamp = `block.timestamp` (seconds -> nanoseconds), backendSequence = `blockNumber * 10000 + logIndex`, exponential backoff for transient failures (FR-T04, FR-T05, FR-T26, FR-T29)
- [x] T059 [US6] Implement Kafka adapter in `internal/topic/adapter_kafka.go`: `CreateTopic` via `kafka.Conn.CreateTopics()`, `Publish` via `kafka.Writer`, `Subscribe` via `kafka.Reader` with offset management, `Resolve` via metadata query, `MaxMessageSize` returns 1048576 (configurable), `SupportedTransport` returns `kafka`, validate anchoring config on construction (FR-T16), consensus timestamp from anchor proof, backendSequence = Kafka offset, exponential backoff for transient failures (FR-T04, FR-T05, FR-T16, FR-T26, FR-T29)
- [x] T060 [US6] Implement exponential backoff retry utility in `internal/topic/adapter.go`: shared retry function with `baseDelay = 1s`, `maxRetries = 5`, formula `baseDelay * 2^attempt` (1s, 2s, 4s, 8s, 16s), classify permanent vs transient errors, used by all adapters (FR-T26)

**Checkpoint**: US6 complete -- all three built-in adapters (HCS, ERC-log, Kafka) implemented, custom adapter registration proven. SC-T01, SC-T05 are fully verifiable.

---

## Phase 9: US7 -- Custom Named Channels (Priority: P3)

**Goal**: A developer can define custom-named channels beyond stdIn/stdOut/stdErr using the `custom:<name>` namespace, represent them as neuron-topic services, and parse them alongside standard channels.

**Independent Test**: Create custom channel `custom:metrics`, serialize as neuron-topic service in agentURI, parse alongside standard channels, verify both present. Covers FR-T08, SC-T04 (extended).

### Tests First (Constitution IX)

- [x] T061 [US7] Write tests for custom channel creation in `internal/topic/channel_test.go`: create `custom:metrics`, `custom:heartbeat`, verify valid ChannelRole, verify `custom:` prefix enforced, reject `metrics` without prefix, reject `custom:` with empty name, reject `custom:stdIn` (reserved name after prefix should still be valid since the full string is `custom:stdIn` which is a custom channel, not reserved) (FR-T08)
- [x] T062 [US7] Write tests for custom channels in agentURI in `internal/topic/service_test.go`: serialize neuron-topic service with `name: "custom:metrics"` and `channel: "custom:metrics"`, parse agentURI with mix of standard and custom channels, verify all returned (FR-T08, FR-T14, SC-T04)

### Implementation

- [x] T063 [US7] Verify custom channel name validation in `internal/topic/channel.go`: `ChannelRoleFromString` already supports `custom:<name>` from T011 -- ensure `<name>` portion is non-empty and consists of alphanumeric characters, hyphens, and underscores only (FR-T08)
- [x] T064 [US7] Verify custom channel representation in `internal/topic/service.go`: `NeuronTopicService` with `channel: "custom:<name>"` serializes and parses correctly, `DiscoverPeerChannels` returns custom channels alongside standard channels (FR-T08, FR-T14)

**Checkpoint**: US7 complete -- custom named channels work alongside standard channels. FR-T08 is fully verifiable.

---

## Phase 10: Polish (Integration Tests, Determinism, Cross-Chain, Transactional Invariants)

**Purpose**: Final integration tests, deterministic signing verification, cross-chain compatibility, and transactional invariant validation.

### HCS Integration Test (Constitution VIII -- FIRST Adapter)

- [x] T065 Write HCS integration test in `internal/topic/hcs_test.go`: end-to-end lifecycle with HCS -- create topic via HCS adapter, publish signed TopicMessage with `WAIT_FOR_CONSENSUS`, subscribe from sequence 0, receive MessageDelivery, verify consensus timestamp is HCS-authoritative, verify message integrity via `ValidateTopicMessage`, verify per-sender ordering preserved (FR-T04, FR-T06, FR-T10, FR-T22, FR-T24, FR-T29, SC-T01, SC-T02, SC-T03, SC-T11, SC-T12, SC-T13, Constitution VIII)

### Deterministic Signing Test (Constitution X)

- [x] T066 Write comprehensive deterministic signing test in `internal/topic/signing_test.go`: construct identical TopicMessage parameters (same timestamp, sequenceNumber, payload), sign with same NeuronPrivateKey twice in separate calls, assert `signature1 == signature2` byte-for-byte; additionally verify the signing hash is `Keccak256(timestamp_8bytes_BE || sequenceNumber_8bytes_BE || payload)` by manually computing and comparing (FR-T03, FR-T21, Constitution X)

### Full Lifecycle Integration Test

- [x] T067 Write full lifecycle integration test in `internal/topic/integration_test.go`: create topic (HCS) -> publish signed message -> subscribe -> receive MessageDelivery -> validate message integrity -> serialize as neuron-topic service in agentURI -> parse agentURI -> extract TopicRef -> subscribe via extracted TopicRef -> verify messages received (FR-T01, FR-T02, FR-T03, FR-T04, FR-T09, FR-T10, FR-T14, SC-T01, SC-T02, SC-T04)

### Cross-Chain Compatibility Test

- [x] T068 Write cross-chain compatibility test in `internal/topic/integration_test.go`: same TopicMessage published to HCS and Kafka (two different adapters), verify both return valid PublishResult, verify subscribers on both receive identical TopicMessage content, verify consensus timestamps use backend-authoritative sources (not sender timestamp) (FR-T05, FR-T29, SC-T01, SC-T13)

### Transactional Invariant Verification

- [x] T069 Write transactional invariant test in `internal/topic/integration_test.go`: verify (1) Sequenced -- messages have monotonically increasing sequence numbers per sender, (2) Immutable -- published messages are not modifiable, (3) Verifiable -- signatures verifiable against sender, (4) Signed -- every message has valid signature; verify per backend (HCS, Kafka) (FR-T19, SC-T11)

### Error Kind Coverage

- [x] T070 Write comprehensive error kind coverage test in `internal/topic/errors_test.go`: trigger all 11 error kinds through actual API calls -- `BackendUnavailable` (unavailable backend), `TopicNotFound` (non-existent topic), `MessageTooLarge` (oversized payload), `UnsupportedOperation` (publish on ERC-log), `InvalidSignature` (tampered message), `SenderMismatch` (wrong sender), `UnsupportedTransport` (unknown backend), `InvalidTopicRef` (malformed ref), `ReservedChannelName` (reserved name as custom), `InvalidConfig` (missing Kafka anchoring), `BrokenTopicRef` (broken p2p cross-ref) (FR-T11, SC-T07)

### MaxMessageSize Enforcement

- [x] T071 Write maxMessageSize enforcement test in `internal/topic/integration_test.go`: for each adapter (HCS, Kafka), verify `MaxMessageSize()` returns documented value, publish message exactly at limit (succeeds), publish message one byte over limit (fails with `MessageTooLarge` before network submission) (FR-T27, SC-T15)

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 1: Setup             -- No dependencies; start immediately
Phase 2: Foundational      -- Depends on Phase 1; BLOCKS all user stories
Phase 3: US1 (P1)          -- Depends on Phase 2; no other story dependencies
Phase 4: US2 (P1)          -- Depends on Phase 2; can run PARALLEL with US1
Phase 5: US3 (P2)          -- Depends on Phase 2; can run PARALLEL with US1/US2
Phase 6: US4 (P2)          -- Depends on US3 (needs ParseAgentURIServices)
Phase 7: US5 (P2)          -- Depends on US3 (needs NeuronTopicService)
Phase 8: US6 (P3)          -- Depends on US1 + US2 (needs adapter interface + validation)
Phase 9: US7 (P3)          -- Depends on Phase 2 (channel.go) + US3 (service.go)
Phase 10: Polish           -- Depends on all user stories complete
```

### Cross-Spec Blocking Dependencies

| Dependency | Status | Blocking? |
|------------|--------|-----------|
| Spec 002 `internal/keylib` (NeuronPrivateKey, Signature, EVMAddress, PeerID, Keccak256) | Required before Phase 3 | **YES** -- US1 signing requires keylib |
| Spec 003 `internal/registry` (agentURI resolution) | Required for US4 end-to-end | Soft -- can mock agentURI JSON for testing |
| hedera-sdk-go | Required for HCS adapter (Phase 3+) | **YES** -- Constitution VIII |
| go-ethereum | Required for ERC-log adapter (Phase 8) | No -- Phase 8 is P3 |
| kafka-go | Required for Kafka adapter (Phase 8) | No -- Phase 8 is P3 |

### User Story Dependencies

```text
US1 (P1): Phase 2 complete -> can start immediately
US2 (P1): Phase 2 complete -> can start PARALLEL with US1
US3 (P2): Phase 2 complete -> can start PARALLEL with US1/US2
US4 (P2): US3 complete (needs ParseAgentURIServices, ExtractTopicRef)
US5 (P2): US3 complete (needs NeuronTopicService for cross-ref validation)
US6 (P3): US1 + US2 complete (needs working adapter interface + validation)
US7 (P3): Phase 2 + US3 complete (needs channel.go + service.go)
```

---

## Parallel Opportunities

### Phase 2 (Foundational)

- T004, T005, T006, T007, T008 -- all test files, different files, no dependencies
- T009, T010 (TopicRef) sequential; T011, T012, T013, T014, T015, T016 all [P] -- different files

### US1 + US2 + US3 (Phases 3, 4, 5) -- THREE-WAY PARALLEL

After Phase 2 completes, three developers can work simultaneously:

```text
Developer A: US1 (Phase 3) -- message.go, adapter.go, adapter_hcs.go, signing_test.go
Developer B: US2 (Phase 4) -- validation.go, message_delivery.go (subscribe path)
Developer C: US3 (Phase 5) -- service.go, p2p_exchange.go (service schemas)
```

### US4 + US5 (Phases 6, 7) -- TWO-WAY PARALLEL

After US3 completes:

```text
Developer A: US4 (Phase 6) -- service.go discovery functions
Developer B: US5 (Phase 7) -- p2p_exchange.go cross-reference validation
```

### US6 + US7 (Phases 8, 9) -- TWO-WAY PARALLEL

```text
Developer A: US6 (Phase 8) -- adapter_erc_log.go, adapter_kafka.go
Developer B: US7 (Phase 9) -- channel.go, service.go (custom channel paths)
```

---

## Implementation Strategy

### MVP First (US1 + HCS Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T016)
3. Complete Phase 3: US1 -- Create and Publish (T017-T029)
4. **STOP and VALIDATE**: Create HCS topic, publish signed TopicMessage, verify PublishResult. SC-T01 (HCS), SC-T08, SC-T12 pass.

### Core Complete (US1 + US2)

5. Complete Phase 4: US2 -- Subscribe and Verify (T030-T036)
6. **STOP and VALIDATE**: Full publish -> subscribe -> verify lifecycle on HCS. SC-T02, SC-T03, SC-T06, SC-T13, SC-T14 pass.

### EIP-8004 Integration (US3 + US4 + US5)

7. Complete Phase 5: US3 -- Service Schemas (T037-T045)
8. Complete Phase 6: US4 -- Peer Discovery (T046-T048)
9. Complete Phase 7: US5 -- P2P Exchange (T049-T053)
10. **STOP and VALIDATE**: End-to-end agentURI round-trip with cross-reference validation. SC-T04, SC-T09, SC-T10 pass.

### Multi-Backend (US6 + US7)

11. Complete Phase 8: US6 -- Custom Backend (T054-T060)
12. Complete Phase 9: US7 -- Custom Channels (T061-T064)
13. **STOP and VALIDATE**: All three adapters working, custom channels functional. SC-T01 (full), SC-T05, SC-T15 (full) pass.

### Production Ready (Polish)

14. Complete Phase 10: Polish (T065-T071)
15. **FINAL VALIDATION**: All 15 success criteria (SC-T01..SC-T15) pass. Constitution VIII (HCS first), IX (test-first), X (deterministic signing) verified.

### Parallel Team Strategy (3 Developers)

| Week | Dev A | Dev B | Dev C |
|------|-------|-------|-------|
| 1 | Phase 1 + 2 (collab) | Phase 1 + 2 (collab) | Phase 1 + 2 (collab) |
| 2 | US1 (message + adapter + HCS) | US2 (validation + subscribe) | US3 (service schemas) |
| 3 | US4 (peer discovery) | US5 (p2p exchange) | US6 (ERC-log + Kafka) |
| 4 | US7 (custom channels) | Phase 10 (integration) | Phase 10 (polish) |

---

## Task-to-Requirement Traceability Summary

| FR/SC | Tasks |
|-------|-------|
| FR-T01 | T004, T005, T009, T010, T067 |
| FR-T02 | T017, T023, T024, T067 |
| FR-T03 | T019, T024, T066, T067 |
| FR-T04 | T020, T026, T028, T034, T054, T055, T056, T065, T067 |
| FR-T05 | T002, T009, T020, T027, T055, T056, T057, T058, T059, T068 |
| FR-T06 | T036, T065, T069 |
| FR-T07 | T006, T011, T048 |
| FR-T08 | T006, T011, T061, T062, T063, T064 |
| FR-T09 | T039, T043, T044, T046, T047, T048, T067 |
| FR-T10 | T031, T033, T035, T065, T067 |
| FR-T11 | T007, T012, T038, T070 |
| FR-T12 | T005, T010, T044 |
| FR-T13 | T046, T047 |
| FR-T14 | T037, T040, T042, T062, T064, T067 |
| FR-T15 | T038, T041 |
| FR-T16 | T038, T041, T056, T059 |
| FR-T17 | T049, T051 |
| FR-T18 | T050, T052, T053 |
| FR-T19 | T069 |
| FR-T20 | T017, T023 |
| FR-T21 | T018, T025, T066 |
| FR-T22 | T021, T028, T054, T065 |
| FR-T23 | T008, T013, T021, T028, T054 |
| FR-T24 | T030, T034, T054, T065 |
| FR-T25 | T032, T034 |
| FR-T26 | T028, T034, T058, T059, T060 |
| FR-T27 | T022, T029, T056, T071 |
| FR-T28 | T016, T028 |
| FR-T29 | T015, T030, T034, T054, T055, T058, T059, T065, T068 |
| SC-T01 | T054, T056, T065, T067, T068 |
| SC-T02 | T054, T065, T067 |
| SC-T03 | T036, T065 |
| SC-T04 | T039, T046, T047, T062, T067 |
| SC-T05 | T020, T027, T057 |
| SC-T06 | T031, T033, T035 |
| SC-T07 | T006, T007, T038, T055, T070 |
| SC-T08 | T018, T025 |
| SC-T09 | T037, T042 |
| SC-T10 | T050, T052 |
| SC-T11 | T054, T065, T069 |
| SC-T12 | T021, T054, T065 |
| SC-T13 | T030, T054, T055, T065, T068 |
| SC-T14 | T032, T034 |
| SC-T15 | T022, T029, T056, T071 |

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [US#] label maps task to specific user story for traceability
- Each user story is independently completable and testable after its dependencies
- Constitution VIII: HCS adapter is FIRST implemented (Phase 3, T028) and FIRST integration-tested (Phase 10, T065)
- Constitution IX: Test tasks (`*_test.go`) appear BEFORE implementation tasks in every phase
- Constitution X: Deterministic signing verified in T019 (Phase 3) and T066 (Phase 10)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
