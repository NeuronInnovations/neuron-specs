# Research: Topic System

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `004-topic-system` | **Date**: 2026-02-25 | **Source**: spec.md

---

## R1: HCS Adapter -- Hedera Consensus Service (Constitution VIII)

**Decision**: Use `github.com/hashgraph/hedera-sdk-go/v2` for the HCS topic adapter.

**Rationale**: Constitution Principle VIII (Hedera Transport Binding) mandates HCS as the first and primary adapter. The Hedera Go SDK provides production-grade access to the Hedera Consensus Service via:
- `ConsensusTopicCreateTransaction` -- creates a new HCS topic, returning a TopicID that maps to the TopicRef locator (FR-T01, FR-T04 `createTopic()`).
- `ConsensusMessageSubmitTransaction` -- publishes a message to an HCS topic. Accepts `[]byte` payload (the serialized TopicMessage envelope). Returns a `TransactionReceipt` with consensus timestamp and topic sequence number, mapping directly to PublishResult (FR-T22).
- `TopicMessageQuery` -- subscribes to an HCS topic via mirror node. Delivers messages with `ConsensusTimestamp`, `SequenceNumber`, and `Contents`, mapping to MessageDelivery (FR-T24, FR-T29).
- `TopicInfoQuery` -- resolves topic metadata (admin key, memo, sequence number), mapping to `resolve()` (FR-T04).

HCS inherently satisfies all four transactional invariants (FR-T19): messages are sequenced (per-topic sequence number), immutable (append-only consensus log), verifiable (Hedera ledger), and signed (this spec adds NeuronPrivateKey signing on top of Hedera's native transaction signing).

**Max message size**: HCS supports ~1024 bytes per single message; the SDK supports chunked messages for larger payloads. For v1, the adapter exposes `maxMessageSize()` returning 1024 (FR-T27) and rejects oversized messages before submission. Chunking support deferred.

**Consensus timestamps**: HCS consensus timestamps are nanosecond-precision Unix timestamps assigned by the Hedera network. These map to `MessageDelivery.consensusTimestamp` (FR-T29) -- the adapter MUST use this, not the sender's self-reported timestamp.

**Cost estimation**: HCS has per-message fees. `estimatePublishCost()` (FR-T28) uses the Hedera fee schedule to estimate cost based on message size.

**Confirmation modes** (FR-T23):
- `FIRE_AND_FORGET`: submit transaction, return after network acknowledgment (TransactionResponse). `confirmed = false`, `consensusTimestamp = null`.
- `WAIT_FOR_CONSENSUS`: submit transaction, call `GetReceipt()` to wait for consensus. `confirmed = true`, `consensusTimestamp` and `sequenceNumber` populated from receipt.

**Retry strategy**: Hedera SDK has built-in retry for transient network errors. The adapter adds exponential backoff per FR-T26 for mirror node subscription reconnects.

**Alternatives considered**:
- Direct Hedera REST API: Lower-level, no built-in transaction management, no subscription support. Would require significant glue code.
- Third-party Hedera wrapper: No mature alternatives in Go ecosystem.

---

## R2: ERC Event Log Adapter -- Ethereum/EVM Event Logs

**Decision**: Use `github.com/ethereum/go-ethereum/ethclient` and `github.com/ethereum/go-ethereum/core/types` for the ERC event log adapter.

**Rationale**: The ERC event log adapter provides **read-only** access to on-chain topic events emitted by smart contracts on Ethereum or EVM-compatible chains. The `go-ethereum` package provides:
- `ethclient.Client.FilterLogs()` -- queries historical event logs matching a filter (contract address + event signature). Maps to `subscribe()` with historical replay.
- `ethclient.Client.SubscribeFilterLogs()` -- WebSocket subscription for new event logs. Maps to `subscribe()` with live streaming.
- `abi.Event.Inputs.Unpack()` -- decodes event log data into TopicMessage fields.

The adapter maps event log metadata to MessageDelivery (FR-T24):
- `consensusTimestamp` = `block.timestamp` of the block containing the log (FR-T29).
- `backendSequence` = `logIndex` within the block (or a composite of `blockNumber + logIndex`).

**Read-only behavior** (FR-T04): ERC event logs are emitted by smart contracts during on-chain transactions. External observers cannot directly emit events. Therefore:
- `createTopic()` returns `UnsupportedOperation` (the contract must already be deployed).
- `publish()` returns `UnsupportedOperation` (publishing requires an on-chain transaction via the contract, which is out of scope for this adapter).
- `subscribe()` and `resolve()` are fully supported.

**Transactional invariants** (FR-T19): Inherently satisfied on-chain -- events are sequenced (block number + log index), immutable (chain finality), verifiable (Ethereum state root), and signed (transaction signature + Neuron signature in event data).

**Config schema** (FR-T15): `chainId` (uint64), `contractAddress` (EVM address), `eventSignature` (Solidity event signature string, e.g. `"TopicMessage(address,uint256,bytes)"`).

**Max message size**: Theoretically limited by block gas limit. The adapter returns a large value (e.g. 128KB) from `maxMessageSize()` with a documentation note that practical limits depend on gas costs.

**Alternatives considered**:
- `github.com/umbracle/ethgo`: Lighter-weight Ethereum client, but less ecosystem support and fewer features than go-ethereum.
- Direct JSON-RPC via HTTP: Possible but requires manual ABI decoding, WebSocket management, and reconnection logic. go-ethereum handles all of this.

---

## R3: Kafka Adapter with HCS Anchoring

**Decision**: Use `github.com/segmentio/kafka-go` for the Kafka adapter with periodic anchoring to HCS (or another ledger) per FR-T16.

**Rationale**: Kafka provides high-throughput, low-latency messaging suitable for environments requiring more bandwidth than HCS or on-chain events can provide. However, Kafka is NOT natively DLT-traceable, so FR-T16 mandates an `anchoring` configuration.

**Kafka operations**:
- `kafka.Writer` -- produces messages to a Kafka topic. Maps to `publish()` (FR-T04).
- `kafka.Reader` -- consumes messages from a Kafka topic with offset management. Maps to `subscribe()` (FR-T04). Supports `fromSequence` via Kafka offset seeking (FR-T25).
- `kafka.Conn.CreateTopics()` -- creates a Kafka topic. Maps to `createTopic()`.
- `kafka.ReaderConfig.MaxBytes` -- exposes maximum message size. Maps to `maxMessageSize()` (FR-T27). Default Kafka max message size is 1MB.

**Anchoring** (FR-T16): The adapter implements a periodic anchoring mechanism:
1. Collect published messages into batches (configurable batch size or time window).
2. Compute a Merkle root (or hash chain) over the batch.
3. Submit the Merkle root to the anchor ledger (e.g. HCS topic via hedera-sdk-go).
4. The anchoring record includes: batch sequence range, Merkle root, Kafka topic name, anchor timestamp.

Anchoring config schema: `method` (e.g. `"hcs-hash-chain"`), `anchorTopicId` (HCS topic ID for anchor proofs), `anchorNetwork` (e.g. `"hedera-mainnet"`), `interval` (e.g. `"every-batch"`, `"every-100-messages"`, `"every-60-seconds"`).

**Consensus timestamps** (FR-T29): For Kafka, the `consensusTimestamp` in MessageDelivery is mapped to the **anchor timestamp** -- the HCS consensus timestamp of the anchoring transaction that covers the message's batch. For messages not yet anchored, the adapter uses the Kafka broker timestamp as a provisional value and updates upon anchor confirmation.

**SASL authentication**: `kafka-go` supports `SCRAM-SHA-512` and other SASL mechanisms. Config includes `saslMechanism` field.

**Alternatives considered**:
- `github.com/confluentinc/confluent-kafka-go`: C-based wrapper (librdkafka), better performance but requires CGO and complicates cross-compilation. `kafka-go` is pure Go.
- `github.com/IBM/sarama`: Mature pure-Go client but more complex API. `kafka-go` has a simpler, more idiomatic Go interface.

---

## R4: Deterministic JSON Serialization (FR-T21)

**Decision**: Implement deterministic JSON serialization for TopicMessage using Go's `encoding/json` with a **canonical field order** enforced by struct field declaration order and explicit JSON tags.

**Rationale**: FR-T21 requires deterministic JSON serialization for signature purposes. The signature covers `Keccak256(timestamp + sequenceNumber + payload)` (FR-T03), but the full envelope JSON must also be deterministic for SC-T08 (round-trip byte equality).

**Approach**:
1. Go's `encoding/json.Marshal` outputs struct fields in declaration order (this is guaranteed by the Go specification for `json.Marshal` of struct types).
2. Define the TopicMessage struct with explicit `json:"fieldName"` tags in canonical order: `senderAddress`, `signature`, `timestamp`, `sequenceNumber`, `payload`.
3. Numeric types use fixed-width representations (uint64 for timestamps and sequence numbers).
4. Byte arrays (payload, signature) use base64 encoding (standard Go JSON behavior for `[]byte`).
5. No optional or nullable fields in the core envelope -- all fields are required.
6. No map types in the envelope (maps have non-deterministic iteration order in Go).

**Verification**: SC-T08 testing -- construct a TopicMessage, serialize to JSON, deserialize, re-serialize, assert byte-equal output. Repeat across multiple runs.

**Alternatives considered**:
- JSON Canonical Form (JCS / RFC 8785): Full spec compliance is overkill; TopicMessage has a fixed schema with no nested dynamic objects. Struct field ordering provides sufficient determinism.
- Protobuf: Deterministic by design but FR-T02 mandates JSON as the canonical serialization. Protobuf MAY be supported as an additional format.
- CBOR: Similar to Protobuf -- deterministic but not the canonical format. MAY be supported later.

---

## R5: Topic Discovery via agentURI Parsing

**Decision**: Implement agentURI parsing as a standalone function in `internal/topic/service.go` that extracts `neuron-topic` and `neuron-p2p-exchange` services from an EIP-8004 agentURI JSON document.

**Rationale**: FR-T09 requires topic discovery via the Peer Registry (003). The discovery flow is:
1. Resolve a peer's agentURI by EVM address using the registry (003).
2. Parse the agentURI JSON document.
3. Filter the `services` array for `type: "neuron-topic"` entries.
4. For each `neuron-topic` service, extract a TopicRef from `transport` + `config` fields.
5. Optionally, extract `neuron-p2p-exchange` services and validate `topicRef` cross-references (FR-T18).

**Implementation**:
- `ParseAgentURIServices(jsonBytes) -> ([]NeuronTopicService, []NeuronP2PExchangeService, error)` -- parses the agentURI JSON and returns typed service objects.
- `ExtractTopicRef(service NeuronTopicService) -> (TopicRef, error)` -- extracts a TopicRef from a neuron-topic service's transport and config fields.
- `ValidateCrossReferences(topics []NeuronTopicService, p2p []NeuronP2PExchangeService) -> error` -- validates that all `topicRef` in p2p services resolve to existing topic service names (FR-T18). Returns `BrokenTopicRef` error for invalid references.

The agentURI JSON structure is defined by EIP-8004. The parser handles:
- Top-level fields: `type`, `name`, `description`, `services`, `registrations`, etc.
- Service discrimination: `type` field determines the service kind.
- Unknown service types are ignored (forward-compatible).

**Alternatives considered**:
- Full EIP-8004 agentURI SDK: Out of scope -- this spec only needs to parse the `services` array for topic-related types. A full agentURI SDK may exist in Spec 003.
- GraphQL/REST wrapper: Over-engineered for JSON parsing. Direct `encoding/json` is sufficient.

---

## R6: Retry Strategy -- Exponential Backoff (FR-T26)

**Decision**: Implement exponential backoff with the parameters specified in FR-T26: `baseDelay = 1s`, `maxRetries = 5`, formula `baseDelay * 2^attempt`.

**Rationale**: FR-T26 specifies: adapters MUST retry transient failures with exponential backoff: `baseDelay * 2^attempt` where `baseDelay = 1 second` and `maxRetries = 5`. Permanent failures MUST NOT be retried.

**Backoff schedule**:

| Attempt | Delay |
|---------|-------|
| 0 | 1s |
| 1 | 2s |
| 2 | 4s |
| 3 | 8s |
| 4 | 16s |
| Total | 31s max |

**Transient vs permanent classification**:

| Error Kind | Transient? | Retry? |
|------------|------------|--------|
| `BackendUnavailable` | Yes | Yes -- backend may recover |
| Network timeout | Yes | Yes -- network may recover |
| Connection refused | Yes | Yes -- service may restart |
| `TopicNotFound` | No | No -- topic does not exist |
| `MessageTooLarge` | No | No -- message must be resized |
| `UnsupportedOperation` | No | No -- adapter does not support operation |
| `InvalidSignature` | No | No -- message is malformed |
| `SenderMismatch` | No | No -- message is malformed |
| `InvalidTopicRef` | No | No -- reference is invalid |
| `InvalidConfig` | No | No -- configuration is wrong |
| Insufficient funds (HCS) | No | No -- requires external action |

**Implementation**: A shared `retryWithBackoff(fn, classify)` utility in `internal/topic/adapter.go` that:
1. Calls `fn()`.
2. If `fn()` returns an error, calls `classify(err)` to determine transient vs permanent.
3. If transient and `attempt < maxRetries`, sleeps for `baseDelay * 2^attempt` and retries.
4. If permanent or max retries exceeded, returns the error.

Each adapter wraps backend calls with `retryWithBackoff` using a backend-specific classifier.

**Jitter**: The spec does not mandate jitter, but implementations SHOULD add random jitter (0-50% of delay) to prevent thundering herd in multi-peer scenarios. This is a SHOULD recommendation, not a MUST.

**Alternatives considered**:
- Fixed delay retry: Does not adapt to varying recovery times. Exponential backoff is standard practice and spec-mandated.
- Circuit breaker pattern: More complex; suitable for long-running services but overkill for individual publish/subscribe operations. May be added in a future version.
- External retry library (e.g. `cenkalti/backoff`): Adds a dependency for ~20 lines of code. Inline implementation is preferred to minimize dependencies.
