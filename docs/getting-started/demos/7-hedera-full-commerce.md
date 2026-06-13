# 7. Full Testnet Commerce Demo

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

The full mock buyer-seller commerce loop from [Step 1](1-buyer-seller-mock.md), but every protocol message goes through real Hedera Consensus Service. The same code, the same nine phases — only the substrate changes.

## What this demo proves

- The Go SDK runs unchanged against real HCS — the same buyer, seller, and validator code that handled in-memory topics in step 1 handles real consensus topics here
- The full commerce loop (negotiate → fund → deliver → settle → validate) survives the latency and ordering guarantees of real HCS
- Every protocol envelope is publicly auditable via Hedera mirror node and HashScan
- Wire format determinism (spec [006](../../../specs/006-protocol-determinism/spec.md)) holds in production: the same canonical JSON that signs locally also verifies through HCS

## When to run it

Run this **after** [Step 6](6-hedera-heartbeat.md). Step 6 promoted the heartbeat layer to real HCS; this demo promotes the entire commerce loop. By the time you reach here, you've understood every protocol piece — you're now just watching the substrate change.

## Prerequisites

| What | How to check |
|------|--------------|
| Go 1.22+ | `go version` |
| Hedera testnet account (Account ID + ECDSA private key) | [portal.hedera.com](https://portal.hedera.com) |
| ~30 HBAR of testnet balance | this demo creates 6+ topics |
| Internet access | needed to reach Hedera testnet nodes |
| Step 6 already ran cleanly | verifies your credentials work |

## Setup

Same env vars as [Step 6](6-hedera-heartbeat.md):

```bash
export HEDERA_OPERATOR_ID=0.0.<your-account-id>
export HEDERA_OPERATOR_KEY=<your-ecdsa-private-key-hex>
```

## Run it

```bash
cd impl/golang
go run ./cmd/buyer-seller-demo --mode=testnet \
  --jpeg cmd/buyer-seller-demo/testdata/photo.jpg
```

Optionally add `--verbose` for full hash and DID:key visibility.

There's also `--mode=evm-testnet`, which adds **on-chain Solidity escrow** on top of HCS topics. That mode needs a deployed escrow contract address; see the [Solidity contracts section in `CLAUDE.md`](../../../CLAUDE.md) for the `make deploy-escrow` workflow.

## Expected output

The output structure is identical to [Step 1](1-buyer-seller-mock.md). The key difference is the `[REAL]` / `[MOCK]` tags — most of the `[MOCK]` entries from step 1 become `[REAL]` here, and each phase prints a HashScan link.

Excerpt:

```
================================================================
  Neuron SDK -- Buyer-Seller JPEG Demo
  Mode: testnet (real HCS topics, real HCS messages, mock escrow)
================================================================

── 1: SETUP ─────────────────────────────────────────
  [REAL] secp256k1 identities created
  [REAL] HCS topic created for Seller.stdOut
         TopicId:   0.0.<id>
         HashScan:  https://hashscan.io/testnet/topic/0.0.<id>
  [REAL] HCS topic created for Seller.stdIn  (0.0.<id>)
  [REAL] HCS topic created for Buyer.stdOut  (0.0.<id>)
  [REAL] HCS topic created for Buyer.stdIn   (0.0.<id>)
  [REAL] HCS topic created for Validator.stdOut (0.0.<id>)
  [REAL] HCS topic created for Validator.stdIn  (0.0.<id>)

── 4: NEGOTIATE ─────────────────────────────────────────
  [REAL] Buyer -> Seller: serviceRequest published to HCS
         TxId:           0.0.<account>@<consensus-time>
         SequenceNumber: 1
         HashScan:       https://hashscan.io/testnet/transaction/<tx-id>
  [REAL] Seller -> Buyer: serviceResponse published to HCS
         SequenceNumber: 1
  [REAL] Agreement state machine: IDLE -> REQUESTED -> AGREED

── 7: DELIVER ─────────────────────────────────────────
  [REAL] libp2p QUIC stream established (ECIES encrypted)
  [REAL] photo.jpg 50282 bytes delivered, SHA256 match: YES

── 9: VALIDATE ─────────────────────────────────────────
  [REAL] Validator observed 6 messages (read from HCS mirror)
  [REAL] All 6 signatures VALID
  [REAL] Verdict #1: 008-payment  -> COMPLIANT
         HashScan:  https://hashscan.io/testnet/topic/0.0.<validator-stdout>
  [REAL] Verdict #2: 009-delivery -> COMPLIANT

================================================================
  DEMO SUMMARY
================================================================

  Mode:       testnet (real HCS topic bus, mock escrow)
  Total HCS messages: 8 (3 buyer -> seller, 3 seller -> buyer, 2 validator -> stdOut)
  Total topics:       6 (3 agents x 2 topics each)
  Wall time:          ~90s

  Explorer links:
    Seller stdOut:     https://hashscan.io/testnet/topic/0.0.<id>
    Buyer stdIn:       https://hashscan.io/testnet/topic/0.0.<id>
    Validator stdOut:  https://hashscan.io/testnet/topic/0.0.<id>
```

> Behaviour-level output. Specific topic IDs, sequence numbers, and timestamps will differ on each run.

## How to verify success

Pass criteria:
- Demo prints `9 VALIDATE OK` and exits cleanly
- Both validator verdicts read `COMPLIANT`
- Open any HashScan link from the output and confirm the message is publicly visible

The HashScan link is the proof a third party can independently verify the entire exchange happened — every signed envelope is on the public record.

## Cost

Approximately **$0.005 USD** in testnet HBAR per run (6 topic creates + ~8 messages). On testnet this is free; the same workload on mainnet would cost ~$0.05 with current HBAR pricing.

If your testnet balance is low, refill at the [portal faucet](https://portal.hedera.com).

## What this maps to

Same component map as [Step 1](1-buyer-seller-mock.md), with two substrate swaps:

| Component | Spec | Step 1 (mock) | Step 7 (testnet) |
|-----------|------|---------------|------------------|
| Topic bus | [004](../../../specs/004-topic-system/spec.md) | `memoryTopicBus` | [`internal/topic/adapter_hcs.go`](../../../impl/golang/internal/topic/) |
| Registry | [003](../../../specs/003-peer-registry/spec.md) | in-memory | in-memory (testnet mode keeps the registry mocked; full on-chain registry requires `--mode=evm-testnet` + deployed contracts) |
| Escrow | [008](../../../specs/008-payment/spec.md) | `MemoryEscrow` | `MemoryEscrow` (use `--mode=evm-testnet` for the Solidity escrow contract) |

The demo source ([`cmd/buyer-seller-demo/main.go`](../../../impl/golang/cmd/buyer-seller-demo/main.go)) wraps both modes behind a `demoBus` interface — the protocol code above the bus is identical.

## EVM testnet mode (advanced)

To run with **on-chain Solidity escrow** in addition to real HCS topics:

1. Deploy the escrow contract: `make deploy-escrow` (requires `PRIVATE_KEY` env var with funded EVM balance — see [`../../../CLAUDE.md`](../../../CLAUDE.md))
2. Note the deployed address and pass it via additional flags (see `cmd/buyer-seller-demo --help`)
3. Run with `--mode=evm-testnet`

EVM testnet mode is the closest thing to mainnet behaviour available without spending real money.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `INSUFFICIENT_PAYER_BALANCE` | Refill testnet HBAR at the portal faucet |
| Demo hangs in phase 1 (SETUP) | Topic creation can take 10–30s in busy testnet conditions; wait |
| `INVALID_SIGNATURE` | Wrong key format — must be ECDSA hex, not Ed25519 base32 (same as Step 6) |
| Mirror node returns 404 for a freshly created topic | Mirror lag — wait 5–10 seconds and retry |
| Demo runs in mock mode despite `--mode=testnet` | Env vars not set when the binary launched. Confirm `echo $HEDERA_OPERATOR_ID` returns your account ID |
| Browser-style errors about RPC endpoint | You ran `--mode=evm-testnet` without setting up RPC. Stick with `--mode=testnet` first |

## You did it

You just ran the full Neuron commerce protocol over a public ledger. From here:

- Browse the topic on HashScan to see your signed envelopes
- Replay the validator independently — anyone with the topic IDs can rebuild the same evidence and verdicts
- Read the [Constitution](../../../.specify/memory/constitution.md) — you have now seen all eleven principles applied
- Build something. The [Go SDK guide](../sdks/go.md) and [TypeScript SDK guide](../sdks/typescript.md) are your next stops

## Where to go next

- **Build with the Go SDK** → [sdks/go.md](../sdks/go.md)
- **Build with the TypeScript SDK** → [sdks/typescript.md](../sdks/typescript.md)
- **Read the formal contracts** → [`../../../specs/`](../../../specs/)
- **Deploy a public seller for browser clients** → run `cmd/webtransport-seller` on a public host with TLS
