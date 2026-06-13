# Quickstart: Peer Registry (EIP-8004 Registration)

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `003-peer-registry` | **Date**: 2026-02-25 | **Phase**: 1

---

## Overview

The Peer Registry module enables Neuron Child accounts to register in EIP-8004 smart contract registries. Registration creates an extended NFT (ERC-721) linked to the Child's EVM address and exposes the Child's communication endpoints as EIP-8004 services: three mandatory `neuron-topic` services (stdIn, stdOut, stdErr) for public communication and one mandatory `neuron-p2p-exchange` service for private multiaddress discovery.

This module bridges identity (Spec 001), cryptographic keys (Spec 002), and the topic system (Spec 004) into an on-chain discoverable profile.

---

## Developer Prerequisites

Before you can use this module, ensure the following are in place:

| # | Prerequisite | Why | How to obtain |
|---|---|---|---|
| 1 | **A NeuronPrivateKey** | Proof-of-control signing for registration transactions | Generate via `keylib.NewNeuronPrivateKey()` (Spec 002) |
| 2 | **A Child NeuronAccount** | The entity being registered as an EIP-8004 NFT | Build via `account.NewChildAccountBuilder()` (Spec 001) |
| 3 | **A funded EVM account with ETH/gas** | Registration calls `register()` on a smart contract, which costs gas | Fund the Child's EVM address on the target chain (e.g. Sepolia faucet for testnet, exchange for mainnet) |
| 4 | **A deployed EIP-8004 Identity Registry contract** | The on-chain registry where the agent NFT is minted | Deploy using the ABI in `internal/registry/contract/`, or use an existing registry address |
| 5 | **An RPC endpoint to the target EVM chain** | `ethclient.Dial()` needs a provider URL | Use Infura, Alchemy, or a self-hosted node (e.g. `https://sepolia.infura.io/v3/YOUR_KEY`) |
| 6 | **Three HCS topics created** (stdIn, stdOut, stdErr) | Mandatory `neuron-topic` services in the agentURI | Create via `topic.NewHCSAdapter()` + `CreateTopic()` (Spec 004) |
| 7 | **Parent DID on allowlist** (permissioned registries only) | Some registries restrict who can mint | Contact the registry operator to add the Parent's DID |

> **Order matters:** You must complete Specs 002 (keys), 001 (account), and 004 (topics) before registering. The agentURI assembles outputs from all three.

### SDK Dependencies

| Dependency | Module | What You Need |
|------------|--------|---------------|
| Spec 001 | Account | Child account with `EVMAddress`, Parent's `DID` (for permissioned registries) |
| Spec 002 | Key Library | `NeuronPrivateKey` for signing registration transactions (proof-of-control) |
| Spec 004 | Topic System | Topic configuration for stdIn/stdOut/stdErr channels; p2p exchange schema |
| go-ethereum | External | `ethclient` for blockchain RPC, `abigen`-generated contract bindings |

---

## Integration Guide

### 1. Create a Child Account and Prepare Keys

```go
import (
    "github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
    "github.com/neuron-sdk/neuron-go-sdk/internal/account"
    "github.com/neuron-sdk/neuron-go-sdk/internal/registry"
)

// Generate Child key
childKey, _ := keylib.NewNeuronPrivateKey()
defer childKey.Zeroize()

childPubKey := childKey.PublicKey()
childAddress := childPubKey.EVMAddress()
childPeerID := childPubKey.PeerID()
childDIDKey := childPubKey.DIDKey()

// Build Child account (from Spec 001)
child, _ := account.NewChildAccountBuilder().
    WithPublicKey(childPubKey).
    WithParentPublicKey(parentPubKey).
    WithCurrency("ETH").
    WithRegistryBinding(account.RegistryBinding{
        RegistryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
        ExternalID:         "",  // assigned after registration
    }).
    Build()
```

### 2. Build the AgentURI with Mandatory Services

```go
// Build neuron-topic services (one per channel — FR-R02, FR-R08)
stdIn := registry.NewNeuronTopicService(
    registry.WithChannel("stdIn"),
    registry.WithTransport("hcs"),
    registry.WithAnchor("hedera-mainnet"),
    registry.WithConfig(map[string]any{
        "network": "hedera-mainnet",
        "topicId": "0.0.4515382",
    }),
    registry.WithEndpoint("hcs://0.0.4515382"),
    registry.WithVersion("1.0.0"),
)

stdOut := registry.NewNeuronTopicService(
    registry.WithChannel("stdOut"),
    registry.WithTransport("kafka"),
    registry.WithAnchor("hedera-mainnet"),
    registry.WithConfig(map[string]any{
        "bootstrapServers": []string{"kafka1.neuron.network:9092"},
        "topicName":        "neuron.agent.alice.stdout",
        "saslMechanism":    "SCRAM-SHA-512",
        "anchoring": map[string]any{
            "method":         "hcs-hash-chain",
            "anchorTopicId":  "0.0.9999999",
            "anchorNetwork":  "hedera-mainnet",
            "interval":       "every-batch",
        },
    }),
    registry.WithEndpoint("kafka+ledger://kafka1.neuron.network:9092/neuron.agent.alice.stdout"),
    registry.WithVersion("1.0.0"),
)

stdErr := registry.NewNeuronTopicService(
    registry.WithChannel("stdErr"),
    registry.WithTransport("hcs"),
    registry.WithAnchor("hedera-mainnet"),
    registry.WithConfig(map[string]any{
        "network": "hedera-mainnet",
        "topicId": "0.0.4515383",
    }),
    registry.WithEndpoint("hcs://0.0.4515383"),
    registry.WithVersion("1.0.0"),
)

// Build neuron-p2p-exchange service (FR-R03)
p2p := registry.NewNeuronP2PExchangeService(
    registry.WithPeerID(childPeerID.String()),
    registry.WithProtocol("/neuron/multiaddr-exchange/1.0.0"),
    registry.WithTopicRef("stdIn"),  // references the stdIn neuron-topic service
    registry.WithVersion("1.0.0"),
)

// Optional: DID service (FR-R13, FR-R14)
did := registry.NewDIDService(childDIDKey)

// Assemble agentURI
agentURI := registry.NewAgentURI(stdIn, stdOut, stdErr, p2p, did)

// Validate completeness before registration
valid, errors := registry.ValidateRegistrationCompleteness(agentURI, childPubKey)
if !valid {
    for _, e := range errors {
        fmt.Println(e)  // e.g. "IncompleteRegistration: missing neuron-topic services"
    }
}
```

**Key constraints**:
- Three `neuron-topic` services are mandatory: stdIn, stdOut, stdErr (FR-R02, FR-R08)
- One `neuron-p2p-exchange` service is mandatory (FR-R03)
- Different channels MAY use different transports (004 FR-T13)
- `topicRef` MUST reference an existing `neuron-topic` service name (004 FR-T18)
- `peerID` MUST match Child's PeerID derived from NeuronPublicKey (FR-R03, 002 FR-006)
- DID service (if present) MUST be `did:key:zQ3s...` from Child's NeuronPublicKey (FR-R14)

### 3. Register the Child in an EIP-8004 Registry

```go
import "github.com/ethereum/go-ethereum/ethclient"

// Connect to the chain
client, _ := ethclient.Dial("https://mainnet.infura.io/v3/YOUR_KEY")

// Register — mints an extended NFT owned by the Child (FR-R06, FR-R10)
result, err := registry.Register(
    childKey,
    registryAddress,
    chainId,
    agentURI,
    client,
)
if err != nil {
    // err is typed: IncompleteRegistration, DuplicateRegistration,
    // AdmissionDenied, RegistryUnavailable, etc.
    log.Fatal(err)
}

fmt.Println("Token ID:", result.TokenId)
fmt.Println("TX Hash:", result.TransactionHash)
fmt.Println("Owner:", result.ChildAddress.Hex())
```

**Key constraints**:
- The Child signs the transaction (proof-of-control, FR-R06)
- The Child's EVMAddress becomes the NFT owner (FR-R10)
- One registration per Child per registry (FR-R05)
- Permissioned registries may reject if Parent DID is not on allowlist (FR-R09, FR-R12)

### 4. Look Up a Registration

```go
// Lookup by Child EVM address (SC-R01)
reg, err := registry.LookupRegistration(
    registryAddress,
    chainId,
    registry.ByEVMAddress(childAddress),
    client,
)
if err != nil {
    // err is typed: RegistrationNotFound, RegistryUnavailable
    log.Fatal(err)
}

fmt.Println("Token ID:", reg.TokenId)
fmt.Println("Services:", len(reg.AgentURI.Services))

// Lookup by registry binding (alternative)
reg2, _ := registry.LookupRegistration(
    registryAddress,
    chainId,
    registry.ByExternalID("42"),
    client,
)
```

### 5. Resolve Topics from a Registration

```go
// Extract the three mandatory topic services (SC-R02)
stdIn, stdOut, stdErr, err := registry.ResolveTopics(reg)
if err != nil {
    log.Fatal(err) // IncompleteRegistration if any channel is missing
}

// Each channel is independently backed (004 FR-T13)
fmt.Println("stdIn transport:", stdIn.Transport)    // "hcs"
fmt.Println("stdOut transport:", stdOut.Transport)   // "kafka"
fmt.Println("stdErr transport:", stdErr.Transport)   // "hcs"

// Use the appropriate TopicAdapter for each transport (Spec 004)
// hcsAdapter := topic.NewHCSAdapter(hcsConfig)
// kafkaAdapter := topic.NewKafkaAdapter(kafkaConfig)
```

### 6. Resolve P2P Exchange and Discover Multiaddress

```go
// Extract the p2p exchange service (SC-R03)
p2pService, err := registry.ResolveP2PExchange(reg)
if err != nil {
    log.Fatal(err) // IncompleteRegistration or BrokenTopicRef
}

fmt.Println("Peer ID:", p2pService.PeerID)        // "12D3KooW..."
fmt.Println("Protocol:", p2pService.Protocol)      // "/neuron/multiaddr-exchange/1.0.0"
fmt.Println("Topic Ref:", p2pService.TopicRef)     // "stdIn"

// Step 1: Resolve the referenced topic (topicRef → neuron-topic service)
// The topicRef "stdIn" points to the stdIn neuron-topic service
// Step 2: Connect to that topic using the appropriate adapter
// Step 3: Initiate the exchange protocol to get the peer's multiaddress
// Step 4: Dial the peer using the received multiaddress
//
// The actual multiaddress is NOT stored in the registry (FR-R03).
// It is exchanged at runtime over the referenced topic channel.
```

---

## Module Structure

```text
internal/registry/
├── registration.go        # Register(), UpdateRegistration(), RevokeRegistration()
├── service.go             # NeuronTopicService, NeuronP2PExchangeService, DIDService
├── agent_uri.go           # AgentURI type + JSON serialization
├── validation.go          # ValidateRegistrationCompleteness (V-REG-01..12)
├── resolver.go            # LookupRegistration(), ResolveTopics(), ResolveP2PExchange()
├── errors.go              # Error types (RegistryUnavailable, NotFound, etc.)
└── contract/
    └── identity_registry.go   # abigen-generated EIP-8004 contract bindings
```

---

## API Reference

| Function | Role | Contract |
|----------|------|----------|
| `Register` | Write | [registration.md](contracts/registration.md) |
| `UpdateRegistration` | Write | [registration.md](contracts/registration.md) |
| `RevokeRegistration` | Write | [registration.md](contracts/registration.md) |
| `LookupRegistration` | Read | [registration.md](contracts/registration.md) |
| `ResolveTopics` | Read | [registration.md](contracts/registration.md) |
| `ResolveP2PExchange` | Read | [registration.md](contracts/registration.md) |
| `ValidateRegistrationCompleteness` | Validation | [registration.md](contracts/registration.md) |

---

## Cross-Spec Dependencies

```text
Spec 001 (Account)
  ├─ Child EVMAddress → registering entity and NFT owner
  ├─ Parent DID → admission policy trust anchor
  └─ RegistryBinding → alternative lookup key

Spec 002 (Key Library)
  ├─ NeuronPrivateKey.Sign() → proof-of-control for registration transactions
  ├─ NeuronPublicKey.EVMAddress() → Child identity in registry
  ├─ NeuronPublicKey.PeerID() → peerID in neuron-p2p-exchange service
  └─ NeuronPublicKey.DIDKey() → DID service value

Spec 004 (Topic System)
  ├─ neuron-topic schema (FR-T14) → stdIn/stdOut/stdErr service objects
  ├─ neuron-p2p-exchange schema (FR-T17) → peerID/protocol/topicRef
  ├─ topicRef validation (FR-T18) → cross-reference integrity
  └─ TopicAdapter → runtime interaction with topic channels

EIP-8004 (External)
  ├─ Identity Registry contract → register(), agentURI(), ownerOf()
  └─ ERC-721 → NFT ownership, approval, transfer
```

---

## Running Tests

```bash
cd internal/registry
go test ./... -v

# Deterministic signing verification (Constitution X)
go test -run TestDeterministicRegistrationSigning -v

# Integration test: full lifecycle
go test -run TestRegistrationLifecycle -v
```
