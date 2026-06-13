# 1. Mock Buyer-Seller Demo

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

The shortest path to see Neuron work end-to-end. Three agents, one process, no infrastructure, sub-second runtime. This is **Step 1** of the [learning path](../learning-path.md).

## What this demo proves

- secp256k1 key derivation produces consistent EVM addresses, PeerIDs, and DID:keys (spec [002](../../../specs/002-key-library/spec.md))
- `TopicMessage` envelopes carry deterministic signatures that survive serialisation (specs [004](../../../specs/004-topic-system/spec.md), [006](../../../specs/006-protocol-determinism/spec.md))
- The negotiation state machine moves through `IDLE → REQUESTED → AGREED → FUNDED → ACTIVE → INVOICED → COMPLETED` correctly under signed messages (spec [008](../../../specs/008-payment/spec.md))
- ECIES (secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM) encrypts a multiaddr to the buyer's pubkey (spec [009](../../../specs/009-p2p-data-delivery/spec.md))
- The libp2p frame protocol delivers a 50 KB JPEG with end-to-end SHA-256 integrity (spec [009](../../../specs/009-p2p-data-delivery/spec.md))
- An independent validator can verify every signature and emit a hash that would anchor on-chain (spec [010](../../../specs/010-validation-framework/spec.md))

## When to run it

Run this **first**, before any other demo. It sets the mental model — you'll see all nine phases of the commerce loop in one place. Every later demo deepens one phase from this run.

## Prerequisites

| What | How to check |
|------|--------------|
| Go 1.22+ (1.25.7 recommended) | `go version` |
| Repo cloned | `ls impl/golang/cmd/buyer-seller-demo/` |

You do **not** need: Hedera account, testnet HBAR, EVM RPC, env vars, second terminal, internet at runtime, secrets.

## Run it

```bash
cd impl/golang
go run ./cmd/buyer-seller-demo --mode=mock
```

For full hashes and DID:keys:

```bash
go run ./cmd/buyer-seller-demo --mode=mock --verbose
```

To use your own image:

```bash
go run ./cmd/buyer-seller-demo --mode=mock --jpeg path/to/your.jpg
```

## Expected output

```
================================================================
  Neuron SDK -- Buyer-Seller JPEG Demo
  Mode: mock (in-memory topic bus, registry, escrow)
================================================================

── 1: SETUP ─────────────────────────────────────────         # spec 002 + 001
  [REAL] secp256k1 identity created: Seller
         EVM     = 0xD97BFcB7d199e7362aadc26CD8DB480419a03A3b
         PeerID  = 16Uiu2HAkwaRbWY21gru1xEraknXsioLCTUShYKXBQudB3YDoVGJg
  [REAL] secp256k1 identity created: Buyer
  [REAL] secp256k1 identity created: Validator
  [MOCK] Topic bus: in-memory (9 topics: 3 agents x stdIn/stdOut/stdErr)

── 4: NEGOTIATE ─────────────────────────────────────────      # spec 008 + 004
  [REAL] Buyer -> Seller: serviceRequest (signed envelope on Seller.stdIn)
  [REAL] Agreement state machine: IDLE -> REQUESTED -> AGREED
  [REAL] agreementHash = 0x6e1e41375100acca... (keccak256 of canonical serviceResponse)

── 7: DELIVER ─────────────────────────────────────────        # spec 009
  [REAL] Seller sent file via libp2p framing protocol (FR-D22)
         file = photo.jpg, size = 50282 bytes, SHA256 = 2b78cbe32b150f7ef8...
  [REAL] Buyer received and independently verified file (SHA256)
         SHA256 match = YES

── 9: VALIDATE ─────────────────────────────────────────       # spec 010
  [REAL] Validator observed 6 protocol messages
  [REAL] Verifying ECDSA signatures (ecrecover + sender match)
         result: all 6 signatures VALID
  [REAL] Verdict #1: 008-payment  -> COMPLIANT
  [REAL] Verdict #2: 009-delivery -> COMPLIANT

================================================================
  DEMO SUMMARY
================================================================

  Phase Results:
    1  SETUP       OK   3 identities created             [REAL crypto]
    2  REGISTER    OK   3 agents registered              [MOCK registry]
    3  DISCOVER    OK   Seller found via lookup          [MOCK registry]
    4  NEGOTIATE   OK   IDLE -> AGREED (2 msgs)          [REAL state machine + MOCK bus]
    5  FUND        OK   500000000 tinybar deposited      [MOCK escrow]
    6  CONNECT     OK   quic-v1, ECIES encrypted         [REAL libp2p + ECIES + MOCK bus]
    7  DELIVER     OK   photo.jpg 50282 bytes            [REAL libp2p + SHA256]
    8  SETTLE      OK   500000000 tinybar released       [MOCK escrow + MOCK bus]
    9  VALIDATE    OK   6 sigs, 2 COMPLIANT verdicts     [REAL validation + MOCK bus]
```

> Captured 2026-04-29. Specific addresses, ports, and hashes will differ on each run; the structure is stable. See [quickstart.md](../quickstart.md) for the full unabridged output.

## How to verify success

Pass criteria: every line in the **Phase Results** table reads `OK`, and the **Validation** section emits `COMPLIANT` for both verdicts.

Quick grep checks:
```bash
go run ./cmd/buyer-seller-demo --mode=mock | grep -E "SHA256 match = YES|COMPLIANT|10  DONE"
```

## What this maps to

| Phase in output | Spec | Source |
|-----------------|------|--------|
| 1 SETUP | [002](../../../specs/002-key-library/spec.md), [001](../../../specs/001-neuron-account-module/spec.md) | [`internal/keylib/`](../../../impl/golang/internal/keylib/), [`internal/account/`](../../../impl/golang/internal/account/) |
| 2 REGISTER, 3 DISCOVER | [003](../../../specs/003-peer-registry/spec.md) | [`internal/registry/`](../../../impl/golang/internal/registry/) |
| 4 NEGOTIATE | [008](../../../specs/008-payment/spec.md), [004](../../../specs/004-topic-system/spec.md) | [`internal/payment/`](../../../impl/golang/internal/payment/), [`internal/topic/`](../../../impl/golang/internal/topic/) |
| 5 FUND, 8 SETTLE | [008](../../../specs/008-payment/spec.md) | [`internal/payment/mock_escrow.go`](../../../impl/golang/internal/payment/mock_escrow.go) |
| 6 CONNECT, 7 DELIVER | [009](../../../specs/009-p2p-data-delivery/spec.md) | [`internal/delivery/`](../../../impl/golang/internal/delivery/) |
| 9 VALIDATE | [010](../../../specs/010-validation-framework/spec.md) | [`internal/validation/`](../../../impl/golang/internal/validation/) |

Source: [`impl/golang/cmd/buyer-seller-demo/main.go`](../../../impl/golang/cmd/buyer-seller-demo/main.go).

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `command not found: go` | Install Go 1.22+ from [go.dev](https://go.dev/dl/) |
| `go: cannot find main module` | Run from `impl/golang/`, not the repo root |
| `go mod download` slow or failing | Check `GOPROXY`; corporate networks may need a private proxy |
| Hedera / HBAR / operator errors | You're not in mock mode. `--mode=mock` is the default; confirm you didn't override it |
| Demo hangs in phase 6 (CONNECT) | Some networks block ephemeral UDP. Pass `--listen=/ip4/127.0.0.1/udp/0/quic-v1` |
| `permission denied` on `testdata/photo.jpg` | Fix file mode, or pass your own image with `--jpeg path/to/image.jpg` |

## Next demo

→ **[Step 2: P2P delivery](2-delivery.md)** — see the same libp2p data plane run across two separate processes.
