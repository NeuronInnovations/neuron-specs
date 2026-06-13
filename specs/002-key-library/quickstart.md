# Quickstart: Key Library

> **Implementation Note**: This file describes the Go SDK implementation. It is not required reading for implementing the protocol in other languages. For the language-neutral protocol definition, see `spec.md`, `data-model.md`, and `contracts/`.

**Branch**: `002-key-library` | **Date**: 2026-02-25

---

## Developer Prerequisites

Before you can use this module, ensure the following are in place:

| # | Prerequisite | Why | How to obtain |
|---|---|---|---|
| 1 | **A secp256k1-compatible runtime** | All Neuron keys are secp256k1 ECDSA | Included via `go-ethereum` dependency — no external setup needed |

> **No DLT account required.** This module is pure cryptography — key generation, signing, and verification are entirely offline operations. A ledger account is only needed when you begin using keys with on-chain modules (Spec 004+).

### SDK Dependencies

- Go 1.22+
- `go-ethereum` module (provides secp256k1, Keccak256, EIP-55)
- `go-libp2p` module (provides PeerID derivation)

## Basic Usage

### 1. Generate a new key

```go
import "github.com/neuron-sdk/neuron-go-sdk/internal/keylib"

// Generate a new NeuronPrivateKey
privKey, err := keylib.NewNeuronPrivateKey()
if err != nil {
    log.Fatal(err)
}
defer privKey.Zeroize() // Always clean up
```

### 2. Derive addresses and identifiers

```go
// Public key
pubKey := privKey.PublicKey()

// EVM address (EIP-55 checksummed)
address := pubKey.EVMAddress()
fmt.Println(address.Hex()) // "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"

// libp2p PeerID
peerID := pubKey.PeerID()
fmt.Println(peerID.String()) // "12D3KooW..."

// DID:key
didKey := pubKey.DIDKey()
fmt.Println(didKey) // "did:key:zQ3s..."
```

### 3. Sign and verify messages

```go
message := []byte("hello neuron")

// Sign (Keccak256 + ECDSA R||S||V, deterministic via RFC 6979)
sig := privKey.Sign(message)

// Verify against known public key
valid := sig.Verify(message, pubKey)

// Recover public key from signature
recovered, err := sig.RecoverPublicKey(message)
if err != nil {
    log.Fatal(err)
}
fmt.Println(pubKey.Matches(recovered)) // true
```

### 4. Restore from mnemonic

```go
mnemonic := "abandon abandon abandon ... about"
privKey, err := keylib.NeuronPrivateKeyFromMnemonic(mnemonic, "") // default path m/44'/60'/0'/0/0
```

### 5. Encrypt / decrypt

```go
// Encrypt with password
encrypted, err := keylib.Encrypt(privKey, "my-password", nil) // nil = default v1 options
if err != nil {
    log.Fatal(err)
}

// Serialize to JSON for storage
jsonBytes, _ := json.Marshal(encrypted)

// Later: decrypt
var loaded keylib.EncryptedPrivateKey
json.Unmarshal(jsonBytes, &loaded)
restored, err := keylib.Decrypt(loaded, "my-password")
```

### 6. Type-safe matching

```go
// Verify key relationships
fmt.Println(privKey.Matches(pubKey))    // true
fmt.Println(privKey.Matches(address))   // true
fmt.Println(pubKey.Matches(peerID))     // true
```

## Integration with Spec 004 (Topic System)

```go
// Sign a TopicMessage payload
payload := buildHeartbeatJSON()
sig := privKey.Sign(payload) // Keccak256 + ECDSA → R||S||V (65 bytes)

// Observer recovers signer
recovered, _ := sig.RecoverPublicKey(payload)
senderAddr := recovered.EVMAddress()
// Compare senderAddr with TopicMessage.senderAddress
```

## Running Tests

```bash
cd internal/keylib
go test ./... -v

# Deterministic signing verification (Constitution X)
go test -run TestDeterministicSigning -v
```
