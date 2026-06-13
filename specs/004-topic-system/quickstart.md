# Quickstart: Topic System

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `004-topic-system` | **Date**: 2026-02-25

---

## Developer Prerequisites

Before you can use this module, ensure the following are in place:

| # | Prerequisite | Why | How to obtain |
|---|---|---|---|
| 1 | **A NeuronPrivateKey** | Signing TopicMessages (Constitution X: deterministic signing) | Generate via `keylib.NewNeuronPrivateKey()` (Spec 002) |
| 2 | **A funded Hedera account** (for HCS adapter) | Creating topics and submitting messages costs HBAR | Create at [portal.hedera.com](https://portal.hedera.com) (testnet) or fund via exchange (mainnet). You need both an **Account ID** (e.g. `0.0.12345`) and the account's **private key** |
| 3 | **Network access to Hedera nodes** | HCS topic operations require gRPC connectivity to consensus and mirror nodes | Testnet: automatic via SDK. Mainnet: configure endpoints or use a managed provider |
| 4 | **Kafka cluster** (only if using Kafka adapter) | Alternative transport backend for high-throughput channels | Provide bootstrap servers, topic name, and SASL credentials |

> **Hedera "auto-account" note:** On Hedera, sending HBAR to any public key auto-creates an "alias account" — you do not need to explicitly create the account first. However, you DO need the account to be funded before submitting HCS transactions.

### SDK Dependencies

- Go 1.22+
- `internal/keylib` package (Spec 002 — NeuronPrivateKey, Signature, EVMAddress, PeerID)
- `hedera-sdk-go` module (HCS adapter)
- `go-ethereum` module (ERC event log adapter, Keccak256)

## Basic Usage

### 1. Create a topic on HCS

```go
import (
    "github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
    "github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// Load or generate a NeuronPrivateKey (from Spec 002)
privKey, err := keylib.NewNeuronPrivateKey()
if err != nil {
    log.Fatal(err)
}
defer privKey.Zeroize()

// Register the HCS adapter (Constitution VIII -- HCS is first-class)
hcsAdapter := topic.NewHCSAdapter(hederaClient)
topic.RegisterAdapter(hcsAdapter)

// Create a new HCS topic (FR-T04)
ref, err := hcsAdapter.CreateTopic(topic.CreateTopicOpts{
    Transport: topic.BackendHCS,
    AdminKey:  privKey,
    Memo:      "agent-alice stdIn channel",
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(ref.URI()) // "hcs://0.0.4515382"
```

### 2. Publish a signed message

```go
// Build and sign a TopicMessage (FR-T02, FR-T03)
// The constructor signs automatically: Keccak256(timestamp || sequenceNumber || payload)
msg, err := topic.NewTopicMessage(
    privKey,                         // Signs with NeuronPrivateKey (Constitution X: deterministic)
    uint64(time.Now().UnixNano()),   // Timestamp (nanoseconds)
    1,                               // SequenceNumber (per-sender monotonic, FR-T06)
    []byte(`{"type":"heartbeat"}`),  // Payload (opaque bytes, FR-T20)
)
if err != nil {
    log.Fatal(err)
}

// Publish with WAIT_FOR_CONSENSUS (FR-T22, FR-T23)
result, err := hcsAdapter.Publish(ref, msg, topic.PublishOpts{
    ConfirmationMode: topic.WaitForConsensus,
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Confirmed)           // true
fmt.Println(result.ConsensusTimestamp)   // 1708000000000000000 (HCS consensus, FR-T29)
fmt.Println(result.SequenceNumber)       // 1 (HCS topic sequence)
fmt.Println(result.TransactionRef)       // "0.0.1234@1708000000.000000000"
```

### 3. Subscribe and consume verified messages

```go
// Subscribe to the topic (FR-T04, FR-T24)
deliveries, err := hcsAdapter.Subscribe(ref, topic.SubscribeOpts{})
if err != nil {
    log.Fatal(err)
}

// Consume messages
for delivery := range deliveries {
    // Validate message integrity (FR-T10, SC-T06)
    if err := topic.ValidateTopicMessage(delivery.Message); err != nil {
        log.Printf("Invalid message: %v", err)
        continue
    }

    fmt.Printf("From: %s\n", delivery.Message.SenderAddress.Hex())
    fmt.Printf("Payload: %s\n", delivery.Message.Payload)
    fmt.Printf("Consensus time: %d\n", delivery.ConsensusTimestamp) // Backend authoritative clock (FR-T29)
    fmt.Printf("Backend seq: %d\n", delivery.BackendSequence)
}
```

### 4. Resume subscription from a sequence number

```go
// Resume from a specific backend sequence (FR-T25)
// Useful after disconnect -- backfills any missed messages
lastSeq := uint64(42)
deliveries, err := hcsAdapter.Subscribe(ref, topic.SubscribeOpts{
    FromSequence: &lastSeq,
})
if err != nil {
    log.Fatal(err)
}
// Messages from sequence 42 onward are delivered (at-least-once)
```

### 5. Check message size limits

```go
// Check max payload size before publishing (FR-T27, SC-T15)
maxSize := hcsAdapter.MaxMessageSize()
fmt.Printf("HCS max payload: %d bytes\n", maxSize) // 1024

payload := make([]byte, 2048) // Too large for HCS
msg, _ := topic.NewTopicMessage(privKey, ts, seq, payload)
_, err = hcsAdapter.Publish(ref, msg, topic.PublishOpts{})
// err.Kind == topic.MessageTooLarge (rejected BEFORE network submission)
```

### 6. Estimate publish cost

```go
// Estimate cost for backends with per-message fees (FR-T28)
cost, err := hcsAdapter.EstimatePublishCost(512) // 512 byte message
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Estimated cost: %d %s\n", cost.Amount, cost.Unit) // e.g. "1000 tinybar"
```

## EIP-8004 Service Integration

### 7. Build neuron-topic services for agentURI

```go
// Represent topics as EIP-8004 services (FR-T14)
stdInService := topic.NeuronTopicService{
    Type:      "neuron-topic",
    Name:      "stdIn",
    Endpoint:  ref.URI(), // "hcs://0.0.4515382"
    Version:   "1.0.0",
    Channel:   "stdIn",
    Transport: "hcs",
    Anchor:    "hedera-mainnet",
    Config: topic.HCSConfig{
        Network: "hedera-mainnet",
        TopicId: "0.0.4515382",
    },
}

// Serialize to JSON (SC-T09: round-trip guarantee)
jsonBytes, err := topic.SerializeTopicService(stdInService)
if err != nil {
    log.Fatal(err)
}
```

### 8. Build neuron-p2p-exchange service

```go
// Represent multiaddress discovery method (FR-T17)
peerID := privKey.PublicKey().PeerID()

p2pService := topic.NeuronP2PExchangeService{
    Type:     "neuron-p2p-exchange",
    Name:     "p2p",
    Version:  "1.0.0",
    PeerID:   peerID.String(),
    Protocol: "/neuron/multiaddr-exchange/1.0.0",
    TopicRef: "stdIn", // Cross-reference to stdIn topic service (FR-T18)
}
```

### 9. Parse a peer's agentURI to discover topics

```go
// Resolve peer's agentURI (via Spec 003 registry) and parse services (FR-T09)
agentURIJSON := resolveAgentURI(peerEVMAddress) // From Spec 003

// Parse all Neuron services
topics, p2pServices, err := topic.ParseAgentURIServices(agentURIJSON)
if err != nil {
    log.Fatal(err)
}

// Validate cross-references (FR-T18, SC-T10)
if err := topic.ValidateCrossReferences(topics, p2pServices); err != nil {
    log.Fatal(err) // BrokenTopicRef if invalid
}

// Extract TopicRefs for each channel
for _, svc := range topics {
    ref, err := topic.ExtractTopicRef(svc)
    if err != nil {
        log.Printf("Invalid service %s: %v", svc.Name, err)
        continue
    }
    fmt.Printf("Channel: %s -> %s\n", svc.Channel, ref.URI())
}
// Output:
// Channel: stdIn -> hcs://0.0.4515382
// Channel: stdOut -> kafka+ledger://kafka1.neuron.network:9092/neuron.agent.alice.stdout
// Channel: stdErr -> hcs://0.0.4515383
```

### 10. Discover and connect to a peer's channels

```go
// Full discovery flow: registry -> agentURI -> topics -> subscribe (US4, SC-T04)

// Step 1: Resolve peer's agentURI
agentURIJSON := resolveAgentURI(peerAddress)

// Step 2: Parse services
topics, p2pServices, _ := topic.ParseAgentURIServices(agentURIJSON)

// Step 3: Find stdOut to read peer's output
var stdOutRef topic.TopicRef
for _, svc := range topics {
    if svc.Channel == "stdOut" {
        stdOutRef, _ = topic.ExtractTopicRef(svc)
        break
    }
}

// Step 4: Get the appropriate adapter for this topic's transport
adapter, err := topic.GetAdapter(stdOutRef.Transport)
if err != nil {
    log.Fatal(err) // UnsupportedTransport if adapter not registered
}

// Step 5: Subscribe to peer's stdOut
deliveries, err := adapter.Subscribe(stdOutRef, topic.SubscribeOpts{})
if err != nil {
    log.Fatal(err)
}

// Step 6: Consume and verify
for delivery := range deliveries {
    if err := topic.ValidateTopicMessage(delivery.Message); err != nil {
        continue // Reject invalid messages (SC-T06)
    }
    fmt.Printf("Peer output: %s\n", delivery.Message.Payload)
}
```

## Custom Channels and Adapters

### 11. Define a custom named channel

```go
// Custom channel with required namespace prefix (FR-T08)
metricsRef, err := hcsAdapter.CreateTopic(topic.CreateTopicOpts{
    Transport: topic.BackendHCS,
    AdminKey:  privKey,
    Memo:      "custom metrics channel",
})

// Represent as neuron-topic service with custom channel name
metricsSvc := topic.NeuronTopicService{
    Type:      "neuron-topic",
    Name:      "custom:metrics",
    Endpoint:  metricsRef.URI(),
    Version:   "1.0.0",
    Channel:   "custom:metrics",
    Transport: "hcs",
    Anchor:    "hedera-mainnet",
    Config: topic.HCSConfig{
        Network: "hedera-mainnet",
        TopicId: metricsRef.Locator,
    },
}

// Attempting a reserved name fails (FR-T07, FR-T11)
_, err = topic.ChannelRoleFromString("stdIn") // OK -- standard role
_, err = topic.ChannelRoleFromString("custom:stdIn") // OK -- custom with custom: prefix
// But creating a custom channel named "stdIn" (without prefix) would fail:
// err.Kind == topic.ReservedChannelName
```

### 12. Register a custom adapter

```go
// Implement a custom adapter (US6, SC-T05)
type InMemoryAdapter struct{}

func (a *InMemoryAdapter) SupportedTransport() topic.BackendKind { return "custom:in-memory" }
func (a *InMemoryAdapter) CreateTopic(opts topic.CreateTopicOpts) (topic.TopicRef, error) { /* ... */ }
func (a *InMemoryAdapter) Publish(ref topic.TopicRef, msg topic.TopicMessage, opts topic.PublishOpts) (topic.PublishResult, error) { /* ... */ }
func (a *InMemoryAdapter) Subscribe(ref topic.TopicRef, opts topic.SubscribeOpts) (<-chan topic.MessageDelivery, error) { /* ... */ }
func (a *InMemoryAdapter) Resolve(ref topic.TopicRef) (topic.TopicMetadata, error) { /* ... */ }
func (a *InMemoryAdapter) MaxMessageSize() uint64 { return 1048576 }
func (a *InMemoryAdapter) EstimatePublishCost(size uint64) (topic.CostEstimate, error) {
    return topic.CostEstimate{}, topic.NewTopicError(topic.UnsupportedOperation, "no fees")
}

// Register -- does NOT require changes to core API (SC-T05)
topic.RegisterAdapter(&InMemoryAdapter{})

// Use identically to built-in adapters
ref, _ := topic.GetAdapter("custom:in-memory")
```

## Running Tests

```bash
cd internal/topic
go test ./... -v

# Deterministic TopicMessage signing verification (Constitution X)
go test -run TestDeterministicTopicMessageSigning -v

# HCS integration tests (Constitution VIII -- requires Hedera testnet)
go test -run TestHCS -v -tags=integration

# Full lifecycle integration test
go test -run TestTopicLifecycle -v

# All error kinds coverage (SC-T07)
go test -run TestTopicErrors -v

# Service schema round-trip (SC-T09)
go test -run TestServiceRoundTrip -v
```

## Error Handling

All topic operations return typed errors with specific error kinds (FR-T11, SC-T07):

```go
result, err := adapter.Publish(ref, msg, opts)
if err != nil {
    var topicErr *topic.TopicError
    if errors.As(err, &topicErr) {
        switch topicErr.Kind {
        case topic.BackendUnavailable:
            // Backend is down -- retry or alert
        case topic.MessageTooLarge:
            // Reduce payload size
        case topic.InvalidSignature:
            // Message was not properly signed
        case topic.SenderMismatch:
            // Recovered signer != envelope senderAddress
        case topic.UnsupportedOperation:
            // Adapter is read-only
        case topic.TopicNotFound:
            // Topic does not exist on backend
        }
    }
}
```
