# Data Model: Topic System

**Branch**: `004-topic-system` | **Date**: 2026-02-25 | **Source**: spec.md FR-T01..FR-T29, Key Entities

---

## Entities

### TopicRef

The globally unique reference to a topic. Every topic operation (publish, subscribe, resolve) requires a TopicRef.

| Field / Method | Type                    | Description                                                                            | Source FR             |
| -------------- | ----------------------- | -------------------------------------------------------------------------------------- | --------------------- |
| Transport      | BackendKind             | Transport backend kind (`hcs`, `erc-log`, `kafka`, `custom:<type>`)                    | FR-T01                |
| Locator        | BackendLocator (string) | Backend-specific topic address (e.g. HCS topic ID, contract address, Kafka topic name) | FR-T01                |
| `URI()`        | string                  | Compact Topic URI (e.g. `hcs://0.0.4515382`)                                           | spec Topic URI Scheme |
| `Validate()`   | Error                   | Validates transport is registered and locator is non-empty and well-formed             | FR-T12                |

**Construction**:

- `NewTopicRef(transport: BackendKind, locator: string)` -- creates a TopicRef with validation (FR-T12)
- `TopicRefFromURI(uri: string)` -- parses a compact Topic URI into a TopicRef
- `TopicRefFromService(svc: NeuronTopicService)` -- extracts a TopicRef from a neuron-topic service object

**Uniqueness**: The pair `(transport, locator)` MUST be globally unique within the system (FR-T01).

**Invariants**: Immutable after construction. Valid by construction -- `NewTopicRef` rejects invalid inputs with `InvalidTopicRef` error.

---

### TopicMessage

A signed, sequenced message published to a topic. This is the core message envelope.

| Field          | Type       | JSON Key           | Description                                                | Source FR      |
| -------------- | ---------- | ------------------ | ---------------------------------------------------------- | -------------- |
| SenderAddress  | EVMAddress | `"senderAddress"`  | EVM address of the sender (from 001/002)                   | FR-T02         |
| Signature      | Signature  | `"signature"`      | R\|\|S\|\|V per Key Library 002 (65 bytes, base64 in JSON) | FR-T02, FR-T03 |
| Timestamp      | UnsignedInt64 | `"timestamp"`      | Unix timestamp in nanoseconds (sender-reported)            | FR-T02         |
| SequenceNumber | UnsignedInt64 | `"sequenceNumber"` | Monotonically increasing per sender per topic              | FR-T02, FR-T06 |
| Payload        | ByteArray     | `"payload"`        | Opaque application data (base64 in JSON)                   | FR-T02, FR-T20 |

**Construction**:

- `NewTopicMessage(key: NeuronPrivateKey, timestamp: UnsignedInt64, sequenceNumber: UnsignedInt64, payload: ByteArray)` -- constructs and signs in one step. `senderAddress` derived from key. Signature covers `Keccak256(timestamp || sequenceNumber || payload)` per FR-T03.
- `TopicMessageFromJSON(jsonBytes: ByteArray)` -- deserializes from canonical JSON.

**Signing** (FR-T03): The signature covers `Keccak256(timestamp + sequenceNumber + payload)` where `+` is byte concatenation:

1. Encode timestamp as 8-byte big-endian unsigned 64-bit integer.
2. Encode sequenceNumber as 8-byte big-endian unsigned 64-bit integer.
3. Concatenate: `[8 bytes timestamp][8 bytes sequenceNumber][N bytes payload]`.
4. Compute `hash = Keccak256(concatenated)`.
5. `signature = NeuronPrivateKey.Sign(hash)` -- returns R\|\|S\|\|V (65 bytes).

**JSON Serialization** (FR-T21): Canonical field order is the struct declaration order: `senderAddress`, `signature`, `timestamp`, `sequenceNumber`, `payload`. This order MUST be used for deterministic serialization (SC-T08).

**Invariants**: All fields are required. Unsigned messages are invalid. SequenceNumber MUST be monotonically increasing per sender per topic (FR-T06). Payload is opaque -- the envelope does not interpret it (FR-T20).

---

### TopicAdapter (Interface)

The interface abstracting backend-specific topic operations. One adapter per BackendKind.

| Method                | Signature                                                                                      | Description                                                                              | Source FR              |
| --------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------- |
| `CreateTopic`         | `(opts: CreateTopicOpts) → TopicRef or Error`                                                  | Creates a new topic on the backend. Returns `UnsupportedOperation` on read-only backends | FR-T04                 |
| `Publish`             | `(ref: TopicRef, msg: TopicMessage, opts: PublishOpts) → PublishResult or Error`               | Publishes a signed message. Returns `UnsupportedOperation` on read-only backends         | FR-T04, FR-T22, FR-T23 |
| `Subscribe`           | `(ref: TopicRef, opts: SubscribeOpts) → AsyncStream<MessageDelivery> or Error`                 | Subscribes to a topic message stream. REQUIRED for all adapters                          | FR-T04, FR-T24, FR-T25 |
| `Resolve`             | `(ref: TopicRef) → TopicMetadata or Error`                                                     | Resolves topic metadata. REQUIRED for all adapters                                       | FR-T04                 |
| `MaxMessageSize`      | `() → UnsignedInt64`                                                                           | Returns maximum payload size in bytes for the backend                                    | FR-T27                 |
| `EstimatePublishCost` | `(messageSize: UnsignedInt64) → CostEstimate or Error`                                         | Estimates per-message cost. SHOULD be implemented for backends with fees                 | FR-T28                 |
| `SupportedTransport`  | `() → BackendKind`                                                                             | Returns the transport kind this adapter handles                                          | FR-T01                 |

**Capabilities**: Each adapter has a capability set: `ReadOnly` (subscribe + resolve only) or `ReadWrite` (all operations). Read-only adapters (e.g. ERC event log) MUST return `UnsupportedOperation` for `CreateTopic()` and `Publish()`.

**Adapter Registry**: Adapters are registered at runtime via `RegisterAdapter(adapter: TopicAdapter)`. The registry maps `BackendKind → TopicAdapter`. Registering a new adapter MUST NOT require changes to the core API (FR-T05, SC-T05).

**Retry**: All adapters MUST implement exponential backoff for transient failures per FR-T26 (see research.md R6).

---

### PublishResult

Result returned by `Publish()` on a TopicAdapter.

| Field              | Type     | JSON Key               | Description                                                         | Source FR      |
| ------------------ | -------- | ---------------------- | ------------------------------------------------------------------- | -------------- |
| TransactionRef     | string                       | `"transactionRef"`     | Backend-specific transaction/receipt identifier                     | FR-T22         |
| ConsensusTimestamp | UnsignedInt64 (optional)     | `"consensusTimestamp"` | Authoritative backend clock at finalization (absent if not confirmed) | FR-T22, FR-T29 |
| SequenceNumber     | UnsignedInt64 (optional)     | `"sequenceNumber"`     | Backend-assigned sequence number (absent if not confirmed)            | FR-T22         |
| Confirmed          | Boolean                      | `"confirmed"`          | true if backend finalized; false if fire-and-forget                 | FR-T22, FR-T23 |

**Confirmation Modes** (FR-T23):

- `FIRE_AND_FORGET`: `Confirmed = false`, `ConsensusTimestamp` absent, `SequenceNumber` absent. Returns after network acknowledgment.
- `WAIT_FOR_CONSENSUS`: `Confirmed = true`, `ConsensusTimestamp` and `SequenceNumber` populated. Returns after backend finalization.

The mode is selected per-call via `PublishOpts.ConfirmationMode`.

---

### MessageDelivery

Wrapper returned by `Subscribe()` for each received message.

| Field              | Type         | JSON Key               | Description                                                | Source FR      |
| ------------------ | ------------ | ---------------------- | ---------------------------------------------------------- | -------------- |
| Message            | TopicMessage  | `"message"`            | The deserialized TopicMessage envelope                     | FR-T24         |
| ConsensusTimestamp | UnsignedInt64 | `"consensusTimestamp"` | Authoritative backend clock when the message was finalized | FR-T24, FR-T29 |
| BackendSequence    | UnsignedInt64 | `"backendSequence"`    | Backend-native ordering number                             | FR-T24         |

**Consensus Timestamp Mapping** (FR-T29):

| Backend | ConsensusTimestamp Source                             | BackendSequence Source                       |
| ------- | ----------------------------------------------------- | -------------------------------------------- |
| HCS     | Hedera consensus timestamp (nanoseconds)              | HCS topic sequence number                    |
| ERC-log | `block.timestamp` (seconds, converted to nanoseconds) | `blockNumber * 10000 + logIndex` (composite) |
| Kafka   | Anchor timestamp from HCS anchoring proof             | Kafka partition offset                       |

The adapter MUST NOT use the sender's self-reported timestamp as `consensusTimestamp` (FR-T29).

**Resumption** (FR-T25): `Subscribe()` accepts a `fromSequence` option. On reconnection, the adapter MUST backfill any messages between the last delivered `backendSequence` and the current stream position. Delivery is at-least-once; consumers use `backendSequence` for deduplication.

---

### NeuronTopicService

EIP-8004 service object with `type: "neuron-topic"`. Represents a single topic channel in the agentURI.

| Field     | Type            | JSON Key      | Required | Description                                                   | Source FR      |
| --------- | --------------- | ------------- | -------- | ------------------------------------------------------------- | -------------- |
| Type      | string          | `"type"`      | MUST     | Always `"neuron-topic"`                                       | FR-T14         |
| Name      | string          | `"name"`      | MUST     | Channel role: `stdIn`, `stdOut`, `stdErr`, or `custom:<name>` | FR-T14         |
| Endpoint  | string          | `"endpoint"`  | SHOULD   | Compact Topic URI for backward-compatible EIP-8004 consumers  | FR-T14         |
| Version   | string          | `"version"`   | MUST     | Topic protocol version (semver, e.g. `"1.0.0"`)               | FR-T14         |
| Channel   | string          | `"channel"`   | MUST     | Neuron channel role (same as `name` for standard channels)    | FR-T14         |
| Transport | string          | `"transport"` | MUST     | Backend kind: `hcs`, `erc-log`, `kafka`, `custom:<type>`      | FR-T14         |
| Anchor    | string          | `"anchor"`    | MUST     | Ledger that anchors the topic                                 | FR-T14         |
| Config    | TransportConfig | `"config"`    | MUST     | Transport-specific configuration object                       | FR-T14, FR-T15 |

**Validation** (FR-T14, FR-T15): All MUST fields are required. `Transport` must match a known BackendKind. `Config` must conform to the transport's config schema. `Channel` must be a valid ChannelRole name.

**Round-trip** (SC-T09): Serialize to JSON and parse back to identical NeuronTopicService -- verified for all registered transports.

---

### NeuronP2PExchangeService

EIP-8004 service object with `type: "neuron-p2p-exchange"`. Defines the method for multiaddress discovery.

| Field    | Type   | JSON Key     | Required | Description                                                                       | Source FR      |
| -------- | ------ | ------------ | -------- | --------------------------------------------------------------------------------- | -------------- |
| Type     | string | `"type"`     | MUST     | Always `"neuron-p2p-exchange"`                                                    | FR-T17         |
| Name     | string | `"name"`     | MUST     | Service name (e.g. `"p2p"`)                                                       | FR-T17         |
| Version  | string | `"version"`  | MUST     | Exchange protocol version (semver)                                                | FR-T17         |
| PeerID   | string | `"peerID"`   | MUST     | Libp2p PeerID from Key Library 002                                                | FR-T17         |
| Protocol | string | `"protocol"` | MUST     | Protocol ID for multiaddress exchange (e.g. `"/neuron/multiaddr-exchange/1.0.0"`) | FR-T17         |
| TopicRef | string | `"topicRef"` | MUST     | Cross-reference to a `neuron-topic` service `name` in the same agentURI           | FR-T17, FR-T18 |

**Cross-reference Validation** (FR-T18): `TopicRef` must resolve to an existing `neuron-topic` service name in the same agentURI document. Invalid references produce `BrokenTopicRef` error (SC-T10).

---

### ChannelRole (Enum)

Named role of a topic channel.

| Value           | Description                                                              | Reserved | Source FR |
| --------------- | ------------------------------------------------------------------------ | -------- | --------- |
| `stdIn`         | Inbound public channel -- other peers publish messages TO this peer      | Yes      | FR-T07    |
| `stdOut`        | Outbound public channel -- this peer publishes its own output            | Yes      | FR-T07    |
| `stdErr`        | Error/diagnostic public channel -- this peer publishes errors and status | Yes      | FR-T07    |
| `custom:<name>` | Custom named channel (namespace prefix required)                         | No       | FR-T08    |

**Validation**:

- Standard roles (`stdIn`, `stdOut`, `stdErr`) are reserved names (FR-T07). Attempting to create a custom channel with a reserved name MUST be rejected with `ReservedChannelName` error (FR-T11).
- Custom channel names MUST use the `custom:` namespace prefix (FR-T08). Names without the prefix (that are not standard roles) MUST be rejected.
- The `<name>` portion of custom channels MUST be non-empty and consist of alphanumeric characters, hyphens, and underscores.

**Construction**:

- `ChannelRoleFromString(s: string) → ChannelRole or Error` -- parses and validates a channel role string.
- `StandardChannelRoles() → Array<ChannelRole>` -- returns the three mandatory roles.

---

### BackendKind (Enum)

Transport backend kind identifier.

| Value           | Description                           | Capabilities    | Source FR                 |
| --------------- | ------------------------------------- | --------------- | ------------------------- |
| `hcs`           | Hedera Consensus Service              | ReadWrite       | FR-T05, Constitution VIII |
| `erc-log`       | ERC event logs on Ethereum/EVM chains | ReadOnly        | FR-T05                    |
| `kafka`         | Kafka with ledger anchoring           | ReadWrite       | FR-T05, FR-T16            |
| `custom:<type>` | Custom transport (runtime-registered) | Adapter-defined | FR-T05                    |

**Validation**: `BackendKind` must match a registered adapter (FR-T12). Unknown backend kinds produce `UnsupportedTransport` error.

---

### TransportConfig (Discriminated Union)

Transport-specific configuration carried in the `config` field of NeuronTopicService.

#### HCSConfig (`transport: "hcs"`)

| Field   | Type   | JSON Key    | Required | Description                                                             | Source FR |
| ------- | ------ | ----------- | -------- | ----------------------------------------------------------------------- | --------- |
| Network | string | `"network"` | MUST     | Hedera network identifier (e.g. `"hedera-mainnet"`, `"hedera-testnet"`) | FR-T15    |
| TopicId | string | `"topicId"` | MUST     | HCS topic ID (e.g. `"0.0.4515382"`)                                     | FR-T15    |

#### ERCLogConfig (`transport: "erc-log"`)

| Field           | Type   | JSON Key            | Required | Description                                                             | Source FR |
| --------------- | ------ | ------------------- | -------- | ----------------------------------------------------------------------- | --------- |
| ChainId         | UnsignedInt64 | `"chainId"`  | MUST     | EVM chain ID (e.g. `1` for Ethereum mainnet)                            | FR-T15    |
| ContractAddress | string | `"contractAddress"` | MUST     | EVM contract address (EIP-55 checksummed)                               | FR-T15    |
| EventSignature  | string | `"eventSignature"`  | MUST     | Solidity event signature (e.g. `"TopicMessage(address,uint256,bytes)"`) | FR-T15    |

#### KafkaConfig (`transport: "kafka"`)

| Field            | Type            | JSON Key             | Required | Description                                                     | Source FR |
| ---------------- | --------------- | -------------------- | -------- | --------------------------------------------------------------- | --------- |
| BootstrapServers | StringArray     | `"bootstrapServers"` | MUST     | Kafka broker addresses                                          | FR-T15    |
| TopicName        | string          | `"topicName"`        | MUST     | Kafka topic name                                                | FR-T15    |
| SASLMechanism    | string          | `"saslMechanism"`    | SHOULD   | SASL authentication mechanism (e.g. `"SCRAM-SHA-512"`)          | FR-T15    |
| Anchoring        | AnchoringConfig | `"anchoring"`        | MUST     | Ledger anchoring configuration (required for non-ledger-native) | FR-T16    |

#### AnchoringConfig (nested in KafkaConfig)

| Field         | Type   | JSON Key          | Required | Description                                                        | Source FR |
| ------------- | ------ | ----------------- | -------- | ------------------------------------------------------------------ | --------- |
| Method        | string | `"method"`        | MUST     | Anchoring method (e.g. `"hcs-hash-chain"`)                         | FR-T16    |
| AnchorTopicId | string | `"anchorTopicId"` | MUST     | Topic ID on the anchor ledger for proof submission                 | FR-T16    |
| AnchorNetwork | string | `"anchorNetwork"` | MUST     | Network identifier of the anchor ledger                            | FR-T16    |
| Interval      | string | `"interval"`      | MUST     | Anchoring frequency (e.g. `"every-batch"`, `"every-100-messages"`) | FR-T16    |

---

### ConfirmationMode (Enum)

Controls `Publish()` behavior regarding confirmation.

| Value                | Description                         | PublishResult State                      | Source FR |
| -------------------- | ----------------------------------- | ---------------------------------------- | --------- |
| `FIRE_AND_FORGET`    | Return after network acknowledgment | `confirmed = false`, timestamps null     | FR-T23    |
| `WAIT_FOR_CONSENSUS` | Return after backend finalization   | `confirmed = true`, timestamps populated | FR-T23    |

Selectable per-call via `PublishOpts.ConfirmationMode`.

---

### TopicError

Structured error types for topic operations.

| Error Kind             | Description                                                               | Source FR      |
| ---------------------- | ------------------------------------------------------------------------- | -------------- |
| `BackendUnavailable`   | Backend is unreachable or unavailable                                     | FR-T11         |
| `TopicNotFound`        | Topic does not exist on the backend                                       | FR-T11         |
| `MessageTooLarge`      | Message exceeds backend's maximum payload size                            | FR-T11, FR-T27 |
| `UnsupportedOperation` | Operation not supported (e.g. publish on read-only adapter)               | FR-T11, FR-T04 |
| `InvalidSignature`     | Signature verification failed                                             | FR-T11, FR-T10 |
| `SenderMismatch`       | Recovered signer does not match envelope senderAddress                    | FR-T11, FR-T10 |
| `UnsupportedTransport` | No adapter registered for the given BackendKind                           | FR-T11         |
| `InvalidTopicRef`      | TopicRef has invalid transport or locator                                 | FR-T11, FR-T12 |
| `ReservedChannelName`  | Attempted to use a reserved channel name for a custom channel             | FR-T11, FR-T07 |
| `InvalidConfig`        | Transport config is missing required fields or malformed                  | FR-T11, FR-T15 |
| `BrokenTopicRef`       | `topicRef` in neuron-p2p-exchange does not resolve to an existing service | FR-T11, FR-T18 |

**Structure**:

| Field        | Type             | Description                                                         |
| ------------ | ---------------- | ------------------------------------------------------------------- |
| Kind         | TopicErrorKind   | One of the 11 error kinds above                                     |
| Message      | string           | Human-readable error message                                        |
| BackendError | Error (optional) | Wrapped backend-specific error (e.g. Hedera SDK error, Kafka error) |

**Error kind coverage**: SC-T07 requires 100% coverage of all 11 error kinds in tests.

---

### Supporting Types

#### CreateTopicOpts

| Field     | Type                   | Description                            | Source FR |
| --------- | ---------------------- | -------------------------------------- | --------- |
| Transport | BackendKind            | Which backend to create the topic on   | FR-T04    |
| AdminKey  | NeuronPrivateKey       | Key that controls the topic (from 002) | FR-T04    |
| Memo      | string                 | Optional topic description             | --        |
| Config    | JSONObject             | Backend-specific creation options      | FR-T15    |

#### PublishOpts

| Field            | Type             | Description                               | Source FR |
| ---------------- | ---------------- | ----------------------------------------- | --------- |
| ConfirmationMode | ConfirmationMode | `FIRE_AND_FORGET` or `WAIT_FOR_CONSENSUS` | FR-T23    |

#### SubscribeOpts

| Field        | Type     | Description                                             | Source FR |
| ------------ | -------- | ------------------------------------------------------- | --------- |
| FromSequence | UnsignedInt64 (optional) | Resume from this backend sequence number (absent = latest) | FR-T25    |

#### TopicMetadata

| Field          | Type              | Description                                   | Source FR |
| -------------- | ----------------- | --------------------------------------------- | --------- |
| TopicRef       | TopicRef          | The topic reference                           | FR-T04    |
| SequenceNumber | UnsignedInt64     | Current latest sequence number on the backend | FR-T04    |
| CreatedAt      | UnsignedInt64     | Topic creation timestamp                      | --        |
| AdminKey       | \*NeuronPublicKey | Topic admin public key (if available)         | --        |
| Memo           | string            | Topic memo/description                        | --        |

#### CostEstimate

| Field  | Type   | Description                                          | Source FR |
| ------ | ------ | ---------------------------------------------------- | --------- |
| Amount | UnsignedInt64 | Estimated cost amount                          | FR-T28    |
| Unit   | string | Cost unit (e.g. `"tinybar"`, `"wei"`, `"USD-cents"`) | FR-T28    |

---

## Relationships

```
TopicRef ──[handled by]──► TopicAdapter (FR-T01, FR-T04)
TopicMessage ──[published to]──► TopicRef (via TopicAdapter.Publish) (FR-T02)
TopicMessage ──[signed by]──► NeuronPrivateKey (002) (FR-T03)
TopicMessage ──[verified by]──► NeuronPublicKey (002) / Signature (FR-T10)
TopicAdapter ──[returns on publish]──► PublishResult (FR-T22)
TopicAdapter ──[returns on subscribe]──► MessageDelivery (FR-T24)
MessageDelivery ──[wraps]──► TopicMessage (FR-T24)
ChannelRole ──[backed by]──► TopicRef (FR-T07, FR-T08)
NeuronTopicService ──[represents]──► ChannelRole (FR-T14)
NeuronTopicService ──[carries]──► TransportConfig (FR-T14, FR-T15)
NeuronP2PExchangeService ──[topicRef references]──► NeuronTopicService (FR-T17, FR-T18)
AgentURI ──[contains 3+]──► NeuronTopicService (FR-T14)
AgentURI ──[contains 0..1]──► NeuronP2PExchangeService (FR-T17)
ChildRegistration (003) ──[resolves to]──► AgentURI (FR-T09)
PublishOpts ──[carries]──► ConfirmationMode (FR-T23)
SubscribeOpts ──[carries]──► FromSequence (FR-T25)
TopicError ──[wraps]──► backend-specific error (FR-T11)
```
