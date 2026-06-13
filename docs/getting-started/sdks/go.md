# Go SDK — Integrator Guide

← Back to [Getting Started](../README.md)

This guide is for engineers who want to **embed the Neuron SDK in their own Go application**. If you want to run the bundled demos instead, see the [demo map](../README.md#demo-map).

## Install

The Go SDK lives at module path `github.com/neuron-sdk/neuron-go-sdk`.

```bash
go get github.com/neuron-sdk/neuron-go-sdk@latest
```

Requires Go 1.22 or newer (1.25.7 recommended).

> The packages live under `internal/`, which means external imports are restricted. To embed, vendor the repository or work from a local checkout.

## Package map

| Spec                                                     | Package                                                                     | Purpose                                                         |
| -------------------------------------------------------- | --------------------------------------------------------------------------- | --------------------------------------------------------------- |
| [002](../../../specs/002-key-library/spec.md)            | [`internal/keylib/`](../../../impl/golang/internal/keylib/)                 | secp256k1 keys, EVM addresses, PeerIDs, DID:keys, signatures    |
| [001](../../../specs/001-neuron-account-module/spec.md)  | [`internal/account/`](../../../impl/golang/internal/account/)               | Parent / Child / Shared identity, DIDs, ledger attachment       |
| [004](../../../specs/004-topic-system/spec.md)           | [`internal/topic/`](../../../impl/golang/internal/topic/)                   | `TopicMessage`, adapters (HCS / ERC / Kafka / memory), channels |
| [003](../../../specs/003-peer-registry/spec.md)          | [`internal/registry/`](../../../impl/golang/internal/registry/)             | EIP-8004 NFT registration, `agentURI`, proof-of-control         |
| [005](../../../specs/005-health/spec.md)                 | [`internal/health/`](../../../impl/golang/internal/health/)                 | `HeartbeatPayload`, liveness state machine, observer            |
| [008](../../../specs/008-payment/spec.md)                | [`internal/payment/`](../../../impl/golang/internal/payment/)               | Commerce protocol, negotiation state machine, escrow adapter    |
| [009](../../../specs/009-p2p-data-delivery/spec.md)      | [`internal/delivery/`](../../../impl/golang/internal/delivery/)             | P2P data delivery, ECIES encryption, libp2p binding             |
| [010](../../../specs/010-validation-framework/spec.md)   | [`internal/validation/`](../../../impl/golang/internal/validation/)         | Evidence model, validator service, verdict publisher            |
| [012](../../../specs/012-browser-client-profile/spec.md) | [`internal/browserprofile/`](../../../impl/golang/internal/browserprofile/) | Browser client profile (server-side)                            |

Read each package's `doc.go` first — it gives the package-level overview.

## Hello, Neuron — minimal example

The smallest meaningful program: generate a key, derive identities, sign and verify a message.

```go
package main

import (
    "fmt"
    "github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
)

func main() {
    // Generate a fresh secp256k1 key
    key, err := keylib.NewNeuronPrivateKey()
    if err != nil {
        panic(err)
    }

    pub := key.PublicKey()

    // Derive all four identities from the single key
    evm := pub.DeriveEVMAddress()                    // 0x... (EIP-55 checksummed)
    peerID := pub.DerivePeerID()                     // 16Uiu2HAm...
    did := pub.DeriveDIDKey()                        // did:key:zQ3sh...

    fmt.Println("EVM:    ", evm)
    fmt.Println("PeerID: ", peerID)
    fmt.Println("DID:    ", did)

    // Sign and verify a message (RFC 6979 deterministic ECDSA + Keccak256)
    sig, err := key.Sign([]byte("hello, neuron"))
    if err != nil {
        panic(err)
    }

    ok := keylib.Verify([]byte("hello, neuron"), sig, pub)
    fmt.Println("Verified:", ok)
}
```

API names match the patterns documented in spec [002](../../../specs/002-key-library/spec.md). Read the package source for the full surface.

## Patterns by use case

### "I want to publish heartbeats from my IoT device"

1. Generate a key with `keylib.NewNeuronPrivateKey()` (or restore from BIP-39 mnemonic with `NeuronPrivateKeyFromMnemonic(...)`)
2. Create a `topic.TopicAdapter` — start with the in-memory mock, swap to HCS adapter for production
3. Build a `health.HeartbeatPayload` and publish it via `topic.NewTopicMessage(...).Publish(...)`
4. Run [Step 1](../demos/1-buyer-seller-mock.md) of the learning path to see this pattern in code

### "I want to validate other agents' behaviour"

1. Subscribe to peer `stdOut` topics through your `TopicAdapter`
2. Verify each `TopicMessage` signature with the publisher's public key (resolved via the [Identity Registry](../../../specs/003-peer-registry/spec.md))
3. Build `EvidenceEnvelope` records via [`internal/validation/`](../../../impl/golang/internal/validation/) and publish verdicts to your own `stdOut`
4. Optionally anchor the verdict hash on-chain via the Validation Registry

### "I want to sell data over libp2p"

1. Implement an offer + listing on the control plane (use `payment.Negotiation`)
2. Use [`internal/delivery/negotiate_bridge.go`](../../../impl/golang/internal/delivery/) to bridge from negotiation to `connectionSetup` (ECIES-encrypted multiaddr)
3. Implement a libp2p stream handler that wraps `delivery.SendFile`
4. Reference: [Step 1](../demos/1-buyer-seller-mock.md) (in-process) and [Step 2](../demos/2-delivery.md) (across processes)

### "I want to deploy on Hedera testnet / mainnet"

1. Set `HEDERA_OPERATOR_ID` and `HEDERA_OPERATOR_KEY` (ECDSA hex)
2. Use [`internal/topic/hcs_real_client.go`](../../../impl/golang/internal/topic/hcs_real_client.go) as your `HCSClient`
3. Wire it into a `topic.TopicAdapter` via `adapter_hcs.go`
4. Reference: [Step 6](../demos/6-hedera-heartbeat.md) and [Step 7](../demos/7-hedera-full-commerce.md)

## Build & test

```bash
# from repo root
cd impl/golang
go build ./...                          # build everything
go test ./internal/...                  # run all tests
go test ./internal/keylib/... -run TestSign   # one test
go vet ./internal/...                   # static analysis
```

The full command list lives in [`../../../CLAUDE.md`](../../../CLAUDE.md).

## Conventions

- **No panics in production code paths.** Each package has a typed error (e.g. `KeyError`) with `Is()` and `Unwrap()` support.
- **Immutable types.** Construction validates; no setters. `NewTypeName()` for fresh values, `TypeNameFromX()` for conversions.
- **Doc comments reference spec FRs/SCs.** Look for `// FR-001:` or `// SEC-003:` to trace any function back to the requirement that motivated it.
- **Key material never appears in errors or logs.** This is enforced (see SEC-003, SEC-005 in spec [002](../../../specs/002-key-library/spec.md)).

## Where to read more

- The eleven Constitution principles → [`../../../.specify/memory/constitution.md`](../../../.specify/memory/constitution.md)
- Repository architecture and build order → [`../../../CLAUDE.md`](../../../CLAUDE.md)
- Each package's `doc.go` is the canonical package overview — start there before diving into individual files
- Specs are the source of truth: [`../../../specs/`](../../../specs/) — read the relevant `spec.md` before extending any module

## Deployment considerations

- **Import surface** — packages live under `internal/`; embed by vendoring the repository or working from a local checkout.
- **Versioning** — the SDK is at `v0.x`; spec [006](../../../specs/006-protocol-determinism/spec.md) golden test vectors are the cross-language conformance gate.
- **Key custody** — `keylib` keeps key material in process memory; use process isolation and the OS keyring for production deployments.
- **Observability** — structured logging is in place across negotiation and delivery flows.
