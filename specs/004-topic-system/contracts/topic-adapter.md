# API Contract: TopicAdapter Interface

**Source**: spec.md FR-T04, FR-T22..FR-T29

---

## TopicAdapter Interface

The TopicAdapter is the central abstraction for backend-specific topic operations. All topic interactions go through this interface. Each supported backend (HCS, ERC event logs, Kafka, custom) provides a concrete implementation.

```
TopicAdapter:
    CreateTopic(opts: CreateTopicOpts) → TopicRef or Error
    Publish(ref: TopicRef, msg: TopicMessage, opts: PublishOpts) → PublishResult or Error
    Subscribe(ref: TopicRef, opts: SubscribeOpts) → AsyncStream<MessageDelivery> or Error
    Resolve(ref: TopicRef) → TopicMetadata or Error
    MaxMessageSize() → UnsignedInt64
    EstimatePublishCost(messageSize: UnsignedInt64) → CostEstimate or Error
    SupportedTransport() → BackendKind
```

**Capability contract**:
- All adapters MUST implement `Subscribe()`, `Resolve()`, `MaxMessageSize()`, and `SupportedTransport()`.
- Read-write adapters MUST implement `CreateTopic()` and `Publish()`.
- Read-only adapters (e.g. ERC event log) MUST return `UnsupportedOperation` error from `CreateTopic()` and `Publish()`.
- All adapters SHOULD implement `EstimatePublishCost()`. If not implemented, MUST return `UnsupportedOperation`.

---

## CreateTopic

Creates a new topic on the backend.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `opts` | CreateTopicOpts | Creation options: transport kind, admin key, memo, backend-specific config |

**Output**: Returns TopicRef. Raises Error if creation fails.

**Behavior** (FR-T04):
1. Validate `opts.Transport` matches this adapter's `SupportedTransport()`. Reject with `UnsupportedTransport` if mismatch.
2. Validate `opts.Config` against the transport's config schema (FR-T15). Reject with `InvalidConfig` if malformed.
3. Submit topic creation to the backend (e.g. HCS `ConsensusTopicCreateTransaction`, Kafka `CreateTopics()`).
4. On transient failure, retry with exponential backoff per FR-T26 (base 1s, max 5 retries).
5. On success, return a `TopicRef` with the new topic's transport kind and locator.
6. On permanent failure, return the backend error wrapped in `BackendUnavailable` or `InvalidConfig`.

**Read-only adapters**: Return `UnsupportedOperation` error. MUST NOT partially create or leave state on the backend.

**Backend-specific behavior**:
- **HCS**: Creates via `ConsensusTopicCreateTransaction`. Admin key = `opts.AdminKey` (NeuronPrivateKey's underlying key). Returns `TopicRef{Transport: "hcs", Locator: "<topicId>"}`.
- **Kafka**: Creates via `kafka.Conn.CreateTopics()`. Returns `TopicRef{Transport: "kafka", Locator: "<topicName>"}`. Validates that `opts.Config` includes valid `anchoring` configuration (FR-T16).
- **ERC-log**: Returns `UnsupportedOperation`.

---

## Publish

Publishes a signed TopicMessage to a topic.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `ref` | TopicRef | Target topic reference |
| `msg` | TopicMessage | Signed message envelope |
| `opts` | PublishOpts | Confirmation mode: `FIRE_AND_FORGET` or `WAIT_FOR_CONSENSUS` |

**Output**: Returns PublishResult. Raises Error if publish fails.

**Behavior** (FR-T04, FR-T22, FR-T23):
1. Validate `ref` against this adapter's `SupportedTransport()`. Reject with `UnsupportedTransport` if mismatch.
2. Validate `ref` is well-formed (FR-T12). Reject with `InvalidTopicRef` if invalid.
3. Validate `msg` has a valid signature (FR-T03). Reject with `InvalidSignature` if unsigned or invalid.
4. Validate `msg.SenderAddress` matches the recovered signer from the signature (FR-T10). Reject with `SenderMismatch` if mismatch.
5. Check message size: `len(msg.Payload)` MUST NOT exceed `MaxMessageSize()` (FR-T27). Reject with `MessageTooLarge` BEFORE submitting to the backend.
6. Submit message to the backend.
7. On transient failure, retry with exponential backoff per FR-T26.
8. Return `PublishResult` based on confirmation mode (FR-T23):
   - `FIRE_AND_FORGET`: Return after network acknowledgment. `confirmed = false`, `consensusTimestamp` absent, `sequenceNumber` absent.
   - `WAIT_FOR_CONSENSUS`: Wait for backend finalization. `confirmed = true`, `consensusTimestamp` and `sequenceNumber` populated from backend.
9. On permanent failure, return typed error (FR-T11).

**Read-only adapters**: Return `UnsupportedOperation` error.

**Backend-specific behavior**:
- **HCS**: Serializes TopicMessage to JSON bytes. Submits via `ConsensusMessageSubmitTransaction`. For `WAIT_FOR_CONSENSUS`, calls `GetReceipt()` to obtain consensus timestamp and sequence number. `transactionRef` = Hedera transaction ID string. `consensusTimestamp` = HCS consensus timestamp (nanoseconds). `sequenceNumber` = HCS topic sequence number.
- **Kafka**: Serializes TopicMessage to JSON bytes. Writes via `kafka.Writer`. For `WAIT_FOR_CONSENSUS`, waits for write acknowledgment from all in-sync replicas. `transactionRef` = Kafka partition:offset string. `consensusTimestamp` = absent until anchoring (provisional Kafka timestamp used). `sequenceNumber` = Kafka offset.
- **ERC-log**: Returns `UnsupportedOperation`.

---

## Subscribe

Subscribes to a topic message stream.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `ref` | TopicRef | Topic to subscribe to |
| `opts` | SubscribeOpts | Options including `FromSequence` for resumption |

**Output**: Returns AsyncStream\<MessageDelivery\>. Raises Error if subscription fails.

**Behavior** (FR-T04, FR-T24, FR-T25):
1. Validate `ref` against this adapter's `SupportedTransport()`. Reject with `UnsupportedTransport` if mismatch.
2. Validate `ref` is well-formed (FR-T12). Reject with `InvalidTopicRef` if invalid.
3. If `opts.FromSequence` is set, configure the subscription to start from that backend sequence number (FR-T25). The adapter MUST backfill any messages between `FromSequence` and the current stream position.
4. Open a subscription to the backend and return a channel of `MessageDelivery`.
5. Each `MessageDelivery` contains: the deserialized `TopicMessage`, the authoritative `ConsensusTimestamp` from the backend (FR-T29), and the `BackendSequence` number.
6. On transient subscription failure (disconnect, timeout), automatically reconnect with exponential backoff per FR-T26. Resume from the last delivered `BackendSequence`.
7. Message delivery is at-least-once. Consumers use `BackendSequence` for deduplication (FR-T25).
8. The channel is closed when the subscription is explicitly cancelled or a permanent error occurs.
9. If the topic does not exist, return `TopicNotFound` error.

**Backend-specific behavior**:
- **HCS**: Uses `TopicMessageQuery` via mirror node. `ConsensusTimestamp` = HCS consensus timestamp. `BackendSequence` = HCS topic sequence number. `FromSequence` maps to HCS sequence number.
- **ERC-log**: Uses `ethclient.SubscribeFilterLogs()` for live events and `ethclient.FilterLogs()` for historical backfill. `ConsensusTimestamp` = `block.timestamp` (converted to nanoseconds). `BackendSequence` = `blockNumber * 10000 + logIndex`. `FromSequence` maps to block number.
- **Kafka**: Uses `kafka.Reader` with offset management. `ConsensusTimestamp` = anchor timestamp (from HCS anchoring proof). `BackendSequence` = Kafka partition offset. `FromSequence` maps to Kafka offset.

---

## Resolve

Resolves topic metadata from the backend.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `ref` | TopicRef | Topic to resolve |

**Output**: Returns TopicMetadata. Raises Error if resolution fails.

**Behavior** (FR-T04):
1. Validate `ref` against this adapter's `SupportedTransport()`. Reject with `UnsupportedTransport` if mismatch.
2. Validate `ref` is well-formed (FR-T12). Reject with `InvalidTopicRef` if invalid.
3. Query the backend for topic metadata.
4. On transient failure, retry with exponential backoff per FR-T26.
5. Return `TopicMetadata` with the topic's current state.
6. If the topic does not exist, return `TopicNotFound` error.

**Backend-specific behavior**:
- **HCS**: Uses `TopicInfoQuery`. Returns memo, admin key, sequence number, creation timestamp.
- **ERC-log**: Queries the contract for event count or latest block. Returns contract address, chain ID, latest block.
- **Kafka**: Queries Kafka metadata for partition count, latest offset. Returns topic name, partition info.

---

## MaxMessageSize

Returns the maximum payload size in bytes for the backend.

**Input**: None

**Output**: UnsignedInt64

**Behavior** (FR-T27):
- Returns the maximum number of bytes that can be sent as `TopicMessage.Payload` in a single `Publish()` call.
- `Publish()` MUST check the message size against this limit BEFORE submitting to the backend.
- Exceeding this limit produces `MessageTooLarge` error (SC-T15).

**Backend-specific values**:

| Backend | Max Payload Size | Notes |
|---------|-----------------|-------|
| HCS | 1024 bytes | Single HCS message limit. Chunking support deferred |
| ERC-log | N/A (read-only) | Returns 0; publish is unsupported |
| Kafka | 1,048,576 bytes (1 MB) | Default Kafka `max.message.bytes`. Configurable |

---

## EstimatePublishCost

Estimates the cost of publishing a message of the given size.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `messageSize` | UnsignedInt64 | Size of the message payload in bytes |

**Output**: Returns CostEstimate. Raises Error if unsupported.

**Behavior** (FR-T28):
- Returns a `CostEstimate` with `Amount` (UnsignedInt64) and `Unit` (string) representing the estimated cost.
- SHOULD be implemented for backends with per-message fees.
- If not implemented, MUST return `UnsupportedOperation` error.
- This is an estimate; actual costs may vary.

**Backend-specific behavior**:
- **HCS**: Estimates based on Hedera fee schedule. Unit = `"tinybar"`.
- **ERC-log**: Returns `UnsupportedOperation` (read-only).
- **Kafka**: Returns `UnsupportedOperation` (Kafka does not have per-message fees; infrastructure cost is separate).

---

## Adapter Registration

**RegisterAdapter**

Registers a new TopicAdapter at runtime.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `adapter` | TopicAdapter | The adapter to register |

**Output**: Raises Error if adapter already registered for this transport.

**Behavior** (FR-T05, SC-T05):
1. Retrieve the adapter's `SupportedTransport()`.
2. If an adapter for that transport kind is already registered, return an error (no silent replacement).
3. Store the adapter in the registry.
4. Registering a new adapter MUST NOT require changes to the core API or existing adapters.

**GetAdapter**

Retrieves the registered adapter for a given transport kind.

**Input**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `transport` | BackendKind | The transport kind to look up |

**Output**: Returns TopicAdapter. Raises Error if not found.

**Behavior**:
1. Look up the adapter for the given transport kind.
2. If not found, return `UnsupportedTransport` error.

---

## Retry Contract (FR-T26)

All adapter methods that interact with the backend MUST implement exponential backoff for transient failures:

| Parameter | Value |
|-----------|-------|
| Base delay | 1 second |
| Max retries | 5 |
| Formula | `baseDelay * 2^attempt` |
| Schedule | 1s, 2s, 4s, 8s, 16s (31s total max) |

**Permanent failures** (MUST NOT retry): `TopicNotFound`, `MessageTooLarge`, `UnsupportedOperation`, `InvalidSignature`, `SenderMismatch`, `InvalidTopicRef`, `InvalidConfig`, `ReservedChannelName`, `BrokenTopicRef`, insufficient funds.

**Transient failures** (MUST retry): `BackendUnavailable`, network timeouts, connection refused, temporary server errors.

---

## Message Validation Contract (FR-T10)

Before processing a received TopicMessage, consumers MUST validate integrity:

1. **Recover signer**: `recoveredPubKey = Signature.RecoverPublicKey(Keccak256(timestamp || sequenceNumber || payload))` using Key Library 002.
2. **Derive address**: `recoveredAddress = recoveredPubKey.EVMAddress()`.
3. **Compare**: `recoveredAddress == msg.SenderAddress` (constant-time comparison).
4. **Accept or reject**:
   - If match: message is authentic.
   - If mismatch: reject with `SenderMismatch` error.
   - If recovery fails: reject with `InvalidSignature` error.

This validation is provided as a utility function: `ValidateTopicMessage(msg: TopicMessage) → Error` (FR-T10, SC-T06).
