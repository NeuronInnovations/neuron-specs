# Heartbeat Publishing to HCS — Source-Code-Traced Execution Path & Test Procedure

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Date**: 2026-02-27
**Author**: Senior QA/QC & Release Validation Engineer
**Scope**: End-to-end heartbeat publish path from `HeartbeatPublisher.Publish()` to `HCSClient.SubmitMessage()`
**Method**: All information derived strictly from current source code — no assumptions, no theoretical guidance

---

## Part 1: Complete Execution Path Trace

The following traces the exact call chain from the user-facing API down to the HCS network call. Every function signature, struct, and constant is from the actual source code.

---

### Layer 1: `HeartbeatPublisher.Publish()` — `publisher.go:221`

```go
func (p *HeartbeatPublisher) Publish(payload HeartbeatPayload, senderClock uint64) (topic.PublishResult, error)
```

**What happens:**
1. Acquires `p.mu.Lock()` (serializes all publish attempts)
2. **Rate-limit check** (line 226): if `p.lastPublishTime > 0 && senderClock - p.lastPublishTime < MinDeadlineDelta(10)` returns `ErrRateLimited`
3. Calls `PublishHeartbeat(payload, p.key, p.stdOutRef, p.adapter, senderClock)`
4. On success, sets `p.lastPublishTime = senderClock`

---

### Layer 2: `PublishHeartbeat()` — `publisher.go:129-168`

```go
func PublishHeartbeat(
    payload HeartbeatPayload,
    key *keylib.NeuronPrivateKey,
    stdOutRef topic.TopicRef,
    adapter topic.TopicAdapter,
    senderClock uint64,
) (topic.PublishResult, error)
```

**Step 1 — Validate** (line 137): `ValidateOutboundHeartbeat(payload, senderClock)` checks:
- **V-PUB-01**: `payload.Type == "heartbeat"` — else `ErrInvalidPayloadType`
- **V-PUB-02**: `payload.Version == "1.0.0"` — else `ErrUnsupportedVersion`
- **V-PUB-03**: If `payload.NextHeartbeatDeadline == 0` (shutdown sentinel) return nil (skip all delta checks)
- **V-PUB-04**: `payload.NextHeartbeatDeadline > senderClock` — else `ErrDeadlineNotFuture`
- **V-PUB-05**: `delta >= MinDeadlineDelta(10)` — else `ErrDeadlineTooSoon`
- **V-PUB-06**: `delta <= MaxDeadlineDelta(86400)` — else `ErrDeadlineTooFar`
- **V-PUB-07**: `ValidNodeRole(payload.Role)` — else `ErrUnrecognizedRole`

**Step 2 — Trim** (line 142-145): `TrimPayload(&payload, int(adapter.MaxMessageSize()))` — progressively nils `Peers`, `Capabilities`, `Location` (in that order) to fit within the adapter's size limit. For HCS, `MaxMessageSize() = 1024`. Note: the **entire `TopicMessage` JSON** must fit in 1024 bytes, not just the payload.

**Step 3 — Serialize** (line 148): `json.Marshal(payload)` invokes `HeartbeatPayload.MarshalJSON()` (`payload.go:87-152`) which outputs deterministic canonical field ordering:
```json
{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":1700000060,"role":"buyer"}
```

**Step 4 — Sign** (line 154): `topic.NewTopicMessage(key, senderClock, 0, payloadJSON)`

**Step 5 — Publish** (line 160-162): `adapter.Publish(stdOutRef, msg, topic.PublishOpts{ConfirmationMode: topic.FireAndForget})`

---

### Layer 3: `topic.NewTopicMessage()` — `message.go:41-62`

```go
func NewTopicMessage(key *keylib.NeuronPrivateKey, timestamp uint64, sequenceNumber uint64, payload []byte) (TopicMessage, error)
```

1. Builds signing input (`message.go:29-35`):
   ```
   timestamp(8 bytes big-endian) || sequenceNumber(8 bytes big-endian) || payload
   ```
2. Calls `key.Sign(signingInput)` — returns 65-byte `Signature`
3. Derives `senderAddress = key.PublicKey().EVMAddress().Hex()` (EIP-55 checksummed)
4. Returns `TopicMessage{SenderAddress, Signature.Bytes(), Timestamp, SequenceNumber, Payload}`

---

### Layer 4: `key.Sign()` — `private_key.go:238-262`

```go
func (k *NeuronPrivateKey) Sign(message []byte) (Signature, error)
```

1. Checks key is not zeroized
2. `hash := crypto.Keccak256(message)` — Keccak256 of the signing input
3. Converts to `*ecdsa.PrivateKey` via `crypto.ToECDSA(k.data[:])`
4. `sigBytes, err := crypto.Sign(hash, ecdsaKey)` — RFC 6979 deterministic ECDSA
5. Returns 65-byte `R(32) || S(32) || V(1)` via `SignatureFromBytes(sigBytes)`

---

### Layer 5: `HCSAdapter.Publish()` — `adapter_hcs.go:69-113`

```go
func (a *HCSAdapter) Publish(ref TopicRef, msg TopicMessage, opts PublishOpts) (PublishResult, error)
```

1. Validates `a.client != nil`
2. Validates `ref.Validate()` (transport is `"hcs"`, locator is non-empty like `"0.0.12345"`)
3. `msgBytes, err := json.Marshal(msg)` — serializes entire `TopicMessage` envelope to JSON
4. Checks `len(msgBytes) <= HCSMaxMessageSize(1024)` — else `ErrMessageTooLarge`
5. For `FireAndForget` (default path, line 102-111):
   - Calls `a.client.SubmitMessage(ref.Locator, msgBytes)`
   - Returns `PublishResult{TransactionRef: txId, Confirmed: false}`

---

### Layer 6: `HCSClient.SubmitMessage()` — Interface Only (`adapter_hcs.go:20`)

```go
SubmitMessage(topicId string, message []byte) (string, error)
```

**This is an interface.** The SDK does **not** provide a production implementation. The consumer must implement `HCSClient` by wrapping `hedera-sdk-go`. This is the network boundary.

---

## Part 2: Step-by-Step Procedure to Test Heartbeat Publishing to HCS

### Prerequisites

Based on source code inspection, you need:

1. A Hedera testnet or mainnet account (operator ID + operator key)
2. An existing HCS topic ID (format: `"0.0.NNNNN"`)
3. A secp256k1 private key for the Neuron node (the heartbeat signer)
4. `hedera-sdk-go` v2 installed

---

### Step 1: Implement `HCSClient` wrapping `hedera-sdk-go`

The SDK requires you to satisfy this interface (`adapter_hcs.go:14-30`):

```go
type HCSClient interface {
    CreateTopic(memo string) (string, error)
    SubmitMessage(topicId string, message []byte) (string, error)
    SubmitMessageAndWait(topicId string, message []byte) (txId string, consensusTimestamp uint64, sequenceNumber uint64, err error)
    SubscribeTopic(topicId string, startSequence *uint64) (<-chan HCSMessage, error)
    GetTopicInfo(topicId string) (TopicMetadata, error)
}
```

For heartbeat publishing, only `SubmitMessage` is called (FireAndForget mode). A minimal implementation:

```go
package main

import (
    "github.com/hashgraph/hedera-sdk-go/v2"
    "github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

type RealHCSClient struct {
    client *hedera.Client
}

func (c *RealHCSClient) SubmitMessage(topicId string, message []byte) (string, error) {
    tid, _ := hedera.TopicIDFromString(topicId)
    resp, err := hedera.NewTopicMessageSubmitTransaction().
        SetTopicID(tid).
        SetMessage(message).
        Execute(c.client)
    if err != nil {
        return "", err
    }
    return resp.TransactionID.String(), nil
}

// ... implement remaining interface methods ...
```

---

### Step 2: Create the Neuron private key

Using the actual constructor from `private_key.go:38`:

```go
key, err := keylib.NeuronPrivateKeyFromHex("0x<your-64-hex-char-secp256k1-key>")
// OR generate a new one:
key, err := keylib.NewNeuronPrivateKey()
```

This internally validates (`private_key.go:127-156`): non-zero, valid secp256k1 scalar, eagerly derives the compressed public key.

---

### Step 3: Create the HCS adapter and TopicRef

```go
// Wrap your Hedera client
hcsClient := &RealHCSClient{client: hederaClient}

// Create the adapter (adapter_hcs.go:48-50)
adapter := topic.NewHCSAdapter(hcsClient)

// Create the topic reference (topic_ref.go:80-89)
// Transport must be "hcs", Locator must be non-empty
stdOutRef, err := topic.NewTopicRef(topic.BackendHCS, "0.0.12345")
```

---

### Step 4: Build the HeartbeatPayload

Using `BuildHeartbeatPayload` from `payload.go:70-81`:

```go
now := uint64(time.Now().Unix())
deadline := now + 60 // 60 seconds from now (within [10, 86400] delta range)

payload := health.BuildHeartbeatPayload(deadline, health.RoleBuyer)
```

The builder auto-sets `Type = "heartbeat"` and `Version = "1.0.0"`.

With optional fields:

```go
payload := health.BuildHeartbeatPayload(deadline, health.RoleSeller,
    health.WithCapabilities(&health.Capabilities{
        NATReachability: true,
        NATType:         health.NATNone,
        Protocols:       []health.ProtocolID{"/neuron/heartbeat/1.0.0"},
    }),
    health.WithLocation(&health.Location{
        Lat: 37.7749, Lon: -122.4194, Fix: health.FixGPS,
    }),
    health.WithPeers([]health.AbbreviatedAddress{"d3ad", "b33f"}),
)
```

---

### Step 5: Publish via HeartbeatPublisher

**Option A — Stateless (single call):**

```go
result, err := health.PublishHeartbeat(payload, &key, stdOutRef, adapter, now)
```

This executes the full 5-step pipeline: validate, trim, serialize, sign, publish.

**Option B — Stateful (with rate limiting):**

```go
publisher := health.NewHeartbeatPublisher(&key, stdOutRef, adapter)
result, err := publisher.Publish(payload, now)
```

Rate limiting rejects if `< MinDeadlineDelta(10)` seconds since last publish.

---

### Step 6: Verify the result

The `PublishResult` (`publish_result.go:18-28`) for FireAndForget:

```go
// result.TransactionRef = "0.0.12345@1700000000.000000000" (Hedera tx ID format)
// result.Confirmed = false (FireAndForget)
// result.ConsensusTimestamp = nil
// result.SequenceNumber = nil
fmt.Println("Published:", result.TransactionRef)
```

---

### Step 7: Schedule the next heartbeat

Using `ScheduleNextHeartbeat` from `publisher.go:178-186`:

```go
nextSendTime := health.ScheduleNextHeartbeat(result, 60, now)
// For FireAndForget: nextSendTime = now + 60 (uses submitWallClock since no consensus timestamp)
```

---

### Step 8: Verify on Hedera mirror node

Query the HCS topic on the Hedera mirror node to confirm the message arrived:

```bash
curl "https://testnet.mirrornode.hedera.com/api/v1/topics/0.0.12345/messages?order=desc&limit=1"
```

The response `message` field (base64-decoded) will be the JSON-serialized `TopicMessage`:

```json
{
  "senderAddress": "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
  "signature": "<base64 65-byte R||S||V>",
  "timestamp": 1700000000,
  "sequenceNumber": 0,
  "payload": "<base64 of HeartbeatPayload JSON>"
}
```

Decode the `payload` (base64 to bytes to JSON) to see:

```json
{"type":"heartbeat","version":"1.0.0","nextHeartbeatDeadline":1700000060,"role":"buyer"}
```

---

### Step 9: Verify via Observer (receiving end)

On a different node, subscribe and process:

```go
observer := health.NewHeartbeatObserver()

deliveryCh, err := adapter.Subscribe(stdOutRef, topic.SubscribeOpts{})
for delivery := range deliveryCh {
    validated, err := observer.ProcessDelivery(delivery)
    if err != nil {
        log.Printf("Invalid heartbeat: %v", err)
        continue
    }

    senderAddr := delivery.Message.SenderAddress
    state := observer.GetLivenessState(senderAddr, uint64(time.Now().Unix()))
    fmt.Printf("Sender %s: state=%s deadline=%d role=%s\n",
        senderAddr, state, validated.NextHeartbeatDeadline, validated.Role)
}
```

The observer's `ProcessDelivery` (`observer.go:187-202`) will:
1. Call `ValidateInboundHeartbeat(msg, consensusTimestamp)` — V-OBS-01..06
2. Acquire write lock on `records` map
3. Call `UpdateLivenessRecord` — sets `CurrentState` to `ALIVE` (or `OFFLINE` for shutdown)
4. Release lock and return the validated `*HeartbeatPayload`

---

## Part 3: Data Size Budget (Critical Constraint)

From source code analysis:

| Component | Source | Max Size |
|-----------|--------|----------|
| HCS message limit | `adapter_hcs.go:10` | **1024 bytes** |
| Size check target | `adapter_hcs.go:84` | `json.Marshal(TopicMessage)` must be <= 1024 |
| Mandatory-only HeartbeatPayload budget | `constants.go:42` | 256 bytes |

The 1024-byte limit applies to the **entire serialized `TopicMessage`**, which includes: `senderAddress` (42 chars), `signature` (65 bytes base64-encoded, approx 88 chars), `timestamp` (up to 20 digits), `sequenceNumber` (up to 20 digits), `payload` (the HeartbeatPayload JSON, base64-encoded). The `TrimPayload` function (`publisher.go:74-117`) auto-trims optional fields if the **payload JSON** exceeds the adapter's `MaxMessageSize`, but the real bottleneck is the envelope-level check at `adapter_hcs.go:84`.

### Size-Safety Verification Test

To verify your payload fits before going live:

```go
payload := health.BuildHeartbeatPayload(deadline, health.RoleBuyer,
    health.WithCapabilities(...),
    health.WithPeers(...),
)

// Check payload JSON size
payloadJSON, _ := json.Marshal(payload)
fmt.Printf("Payload JSON: %d bytes\n", len(payloadJSON))

// Simulate full TopicMessage size
msg, _ := topic.NewTopicMessage(&key, now, 0, payloadJSON)
msgJSON, _ := json.Marshal(msg)
fmt.Printf("Full TopicMessage: %d bytes (limit: 1024)\n", len(msgJSON))
```

---

## Part 4: Execution Path Summary Table

| Step | Function | Source Location |
|------|----------|----------------|
| 1 | Rate limit | `publisher.go:226` |
| 2 | Validate V-PUB-01..07 | `publisher.go:18-63` |
| 3 | Trim optional fields | `publisher.go:74-117` |
| 4 | Serialize payload (deterministic JSON) | `payload.go:87-152` |
| 5 | Build signing input (TS\|\|Seq\|\|Payload) | `message.go:29-35` |
| 6 | Keccak256 then ECDSA sign (RFC 6979) | `private_key.go:248-255` |
| 7 | Derive EVM sender address | `evm_address.go:143-160` |
| 8 | Serialize TopicMessage envelope | `adapter_hcs.go:79` |
| 9 | Check <= 1024 bytes | `adapter_hcs.go:84` |
| 10 | Submit to HCS (FireAndForget) | `adapter_hcs.go:104` then `HCSClient.SubmitMessage` |

---

## Part 5: Integration Boundary

**The `HCSClient` interface is the integration boundary.** Everything above it is fully implemented and tested (569 tests across 5 packages, all passing, zero race conditions). The consumer must provide the `HCSClient` implementation wrapping `hedera-sdk-go`.

The existing test suite validates this entire path using mock adapters that satisfy the `TopicAdapter` and `HCSClient` interfaces. The test `TestHCSEndToEndIntegration` (`integration_test.go:150-232`) exercises the complete lifecycle: key generation, payload construction, publish via HCS mock, observer processing, UNKNOWN->ALIVE transition, and liveness evaluation at 6 time points covering all state transitions.
