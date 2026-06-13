# 6. Hedera Heartbeat Demo

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

Your first demo on a **real public ledger**. Creates an HCS topic on Hedera testnet, publishes a signed heartbeat, and verifies the message via the Hedera mirror node.

## What this demo proves

- The Go SDK successfully creates an HCS topic, publishes a signed heartbeat, and reads it back from a mirror node — proving the full topic adapter works against real Hedera consensus (specs [004](../../../specs/004-topic-system/spec.md), [005](../../../specs/005-health/spec.md))
- The 1024-byte HCS message size limit fits a complete `HeartbeatPayload` envelope
- A heartbeat published on the public ledger is independently verifiable by anyone with the topic ID — no trust in the publisher needed
- secp256k1 keys map directly to Hedera's ECDSA account model (no key conversion required)

## When to run it

Run this **after** [Step 5](5-browser-webtransport.md). You've now exercised the full protocol stack against in-memory mocks; this demo promotes the control plane substrate from in-memory to real HCS consensus.

## Prerequisites

| What | How to check |
|------|--------------|
| Go 1.22+ | `go version` |
| Hedera testnet account (Account ID + ECDSA private key) | Free at [portal.hedera.com](https://portal.hedera.com) |
| Funded testnet HBAR (~10 HBAR is plenty) | Auto-funded at signup; refill at the portal faucet |
| Internet access | needed to reach Hedera testnet nodes and the mirror node |

This is the first demo that needs **money** (testnet HBAR) and **credentials**. Treat the private key like a real secret — even on testnet.

## Setup

Set environment variables before running:

```bash
export HEDERA_OPERATOR_ID=0.0.<your-account-id>
export HEDERA_OPERATOR_KEY=<your-ecdsa-private-key-hex>
```

Both come from the Hedera portal account dashboard. Use placeholders in any documentation or commit history — never share the real values.

## Run it

```bash
cd impl/golang
go run ./cmd/hcs-heartbeat-test
```

This binary takes no flags — all configuration comes from the environment.

## Expected output

```
=== HCS Heartbeat Test ===
Operator: 0.0.<your-account-id>
Network:  testnet

[1/5] Creating HCS topic...
       TopicId:    0.0.<topic-id>
       SubmitKey:  none (open-write)
       Created:    consensus_timestamp 1719000000.000000000
       HashScan:   https://hashscan.io/testnet/topic/0.0.<topic-id>

[2/5] Generating Neuron secp256k1 identity...
       PeerID:     16Uiu2HAm...
       EVM:        0x...
       DID:key:    did:key:zQ3sh...

[3/5] Building HeartbeatPayload (spec 005)...
       sequenceNumber:  1
       deadlineSeconds: 60
       agentDID:        did:key:zQ3sh...
       canonicalJSON:   <serialized payload>

[4/5] Signing and publishing to HCS...
       Signature:        R||S||V (65 bytes)
       Submission:       SUCCESS
       SequenceNumber:   1
       ConsensusTime:    1719000003.123456789

[5/5] Verifying via Hedera mirror node...
       URL:              https://testnet.mirrornode.hedera.com/api/v1/topics/0.0.<topic-id>/messages
       Status:           200 OK
       Messages:         1
       Verified:         signature matches public key, payload matches what was signed

=== HEARTBEAT TEST PASSED ===
View on HashScan: https://hashscan.io/testnet/topic/0.0.<topic-id>
```

> Behaviour-level output. Specific account IDs, topic IDs, and timestamps will be unique to your run; the structure is stable.

## How to verify success

Pass criteria:
- All five steps print `SUCCESS` / `200 OK`
- Final line reads `=== HEARTBEAT TEST PASSED ===`
- The HashScan URL opens in a browser and shows the topic with one message

The mirror node URL is the canonical proof. Anyone — not just you — can hit that URL and see your signed heartbeat. That is what "real public ledger" means.

## What this maps to

| Component | Spec | Source |
|-----------|------|--------|
| HCS topic adapter | [004](../../../specs/004-topic-system/spec.md) | [`internal/topic/adapter_hcs.go`](../../../impl/golang/internal/topic/) |
| Real HCS client (Hiero SDK wrapper) | [004](../../../specs/004-topic-system/spec.md) | [`internal/topic/hcs_real_client.go`](../../../impl/golang/internal/topic/) |
| HeartbeatPayload + signing | [005](../../../specs/005-health/spec.md) | [`internal/health/`](../../../impl/golang/internal/health/) |
| Mirror node fetch + verify | [004](../../../specs/004-topic-system/spec.md), [005](../../../specs/005-health/spec.md) | [`cmd/hcs-heartbeat-test/main.go`](../../../impl/golang/cmd/hcs-heartbeat-test/main.go) |

Source: [`impl/golang/cmd/hcs-heartbeat-test/main.go`](../../../impl/golang/cmd/hcs-heartbeat-test/main.go).

## Cost

Approximately **$0.0001 USD** in testnet HBAR per run (one topic create + one message publish). On testnet this is free — refill from the portal faucet if you run low.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `HEDERA_OPERATOR_ID is required` | The env var is unset. Check with `echo $HEDERA_OPERATOR_ID` |
| `INVALID_SIGNATURE` from Hedera | Wrong key format. Make sure `HEDERA_OPERATOR_KEY` is the hex-encoded ECDSA key from the portal, not a base32 ED25519 key |
| `INSUFFICIENT_PAYER_BALANCE` | Out of testnet HBAR. Refill at the [Hedera portal faucet](https://portal.hedera.com) |
| `connection refused` to mirror node | Hedera testnet is occasionally down for maintenance. Retry, or check status at [status.hedera.com](https://status.hedera.com) |
| `topic create failed: BUSY` | Network congestion; retry once |
| `key parse error` | Make sure you exported the **ECDSA** key, not Ed25519. The portal lets you generate either; we need ECDSA secp256k1 |

## Security notes

- **Never commit** `HEDERA_OPERATOR_KEY` to git, even on testnet — leak prevention habits matter for when you move to mainnet
- This binary signs with the operator key. Production agents should use separate child keys derived per the [account model](../../../specs/001-neuron-account-module/spec.md)
- Topic IDs are public; topic content is public. The signed heartbeat reveals the agent's DID, sequence number, and deadline — by design

## Next demo

→ **[Step 7: Full testnet commerce](7-hedera-full-commerce.md)** — promote the entire commerce loop from mock to real HCS.
