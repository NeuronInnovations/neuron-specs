# Quickstart: NeuronAccount Module

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `001-neuron-account-module` | **Date**: 2026-02-25

---

## Developer Prerequisites

Before you can use this module, ensure the following are in place:

| # | Prerequisite | Why | How to obtain |
|---|---|---|---|
| 1 | **A NeuronPrivateKey** | Parent and Child accounts are built around secp256k1 key pairs | Generate via `keylib.NewNeuronPrivateKey()` or restore from mnemonic (Spec 002) |
| 2 | **A Parent key pair** (for Child accounts) | Child accounts reference their Parent's public key | The Parent account must be created first |

> **No DLT account required.** NeuronAccount objects are in-memory identity structures. A ledger account is only needed when you attach to a chain (Step 3 below) or register in a registry (Spec 003).

### SDK Dependencies

- Go 1.22+
- `internal/keylib` package (Spec 002 Key Library)

## Basic Usage

### 1. Create a Parent account

```go
import (
    "github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
    "github.com/neuron-sdk/neuron-go-sdk/internal/account"
)

// Generate a key
privKey, _ := keylib.NewNeuronPrivateKey()
pubKey := privKey.PublicKey()

// Build Parent account
parent, err := account.NewParentAccountBuilder().
    WithPublicKey(pubKey).
    WithDID(account.GenerateDID(pubKey)). // uses pubKey.DIDKey()
    WithCurrency("ETH").
    Build()
if err != nil {
    log.Fatal(err)
}

// Payment address (= Parent's EVMAddress)
addr, _ := parent.PaymentAddress()
fmt.Println(addr.Hex()) // "0x5aAe..."
```

### 2. Create a Child account

```go
childPrivKey, _ := keylib.NewNeuronPrivateKey()
childPubKey := childPrivKey.PublicKey()

child, err := account.NewChildAccountBuilder().
    WithPublicKey(childPubKey).
    WithParentPublicKey(pubKey). // references Parent
    WithCurrency("ETH").
    WithRegistryBinding(account.RegistryBinding{
        RegistryIdentifier: "eip155:1:0x742d35Cc6634C0532925a3b844Bc9e7595f2bD18",
        ExternalID:         "42",
    }).
    BuildComplete() // validates all required fields including registry binding
if err != nil {
    log.Fatal(err)
}

// Child's PeerID (derived from same key as EVMAddress)
fmt.Println(child.PeerID().String()) // "12D3KooW..."

// Child's payment address resolves to Parent's
childPayAddr, _ := child.PaymentAddress()
fmt.Println(childPayAddr.Hex() == addr.Hex()) // true
```

### 3. Attach to a ledger

```go
attachment := account.LedgerAttachment{
    LedgerIdentifier: "ethereum-mainnet",
    AttachedAddress:  pubKey.EVMAddress(),
    State:            account.Attached,
}
parent.SetLedgerAttachment(attachment)
```

### 4. Validate accounts

```go
errors := parent.Validate()
if len(errors) > 0 {
    for _, e := range errors {
        fmt.Println(e) // "missing DID for Parent account"
    }
}
```

### 5. JSON serialization

```go
jsonBytes, _ := json.Marshal(parent)
var restored account.NeuronAccount
json.Unmarshal(jsonBytes, &restored)
```

## Running Tests

```bash
cd internal/account
go test ./... -v
```
