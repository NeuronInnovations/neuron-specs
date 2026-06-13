# Zero to Heartbeat

The minimal path from a clean checkout to a verifiable Neuron liveness proof. Two stages: an **offline mock run** (no accounts, no network, under a second) and an optional **real testnet heartbeat** (one funded Hedera testnet account, a few minutes).

Everything below is grounded in the code in this repository — every command is runnable as written.

## Prerequisites

| Stage | Requirement |
| ----- | ----------- |
| Mock (stage 1) | Go ≥ 1.25 ([go.dev/dl](https://go.dev/dl/)). Nothing else — no accounts, no network beyond `go mod` download. |
| Testnet (stage 2) | A **Hedera testnet account** with test HBAR — free at [portal.hedera.com](https://portal.hedera.com). Use an **ECDSA secp256k1** key when creating it. |

## Build

```bash
cd impl/golang
go build ./...
go test ./internal/keylib/... ./internal/account/... ./internal/health/...
```

A clean build plus green key/account/health tests confirms the toolchain is ready.

## Stage 1 — Offline proof (mock mode)

```bash
cd impl/golang
go run ./cmd/buyer-seller-demo --mode=mock
```

This single command exercises the whole protocol stack in memory:

1. **Identity (specs 002 + 001)** — three real secp256k1 keypairs are generated (seller, buyer, validator); EVM addresses, PeerIDs, and DID:keys are derived. The cryptography is real; only the ledger is mocked.
2. **Channels (spec 004)** — signed `TopicMessage` envelopes flow over an in-memory topic bus.
3. **Registration (spec 003)** — agents register and discover each other via a mock registry.
4. **Commerce, delivery, validation (specs 008/009/010)** — negotiation, mock escrow, a real ECIES-encrypted libp2p transfer, and two `COMPLIANT` validator verdicts.

Phases print as they run; lines marked `[REAL]` use real cryptography and real libp2p. Exit code 0 with two `COMPLIANT` verdicts is the pass condition.

## Stage 2 — Real on-chain heartbeat (testnet)

### 2.1 Configure credentials (placeholders — substitute your own)

```bash
export HEDERA_OPERATOR_ID="0.0.XXXXXXX"                 # your testnet account ID
export HEDERA_OPERATOR_KEY="<ECDSA private key hex>"     # its private key
```

Both variables are required; the SDK fails fast with a clear error naming any missing one (`internal/topic/hcs_real_client.go`, `NewTestnetClientFromEnv`). Credentials are never hardcoded and never logged.

### 2.2 Publish and verify a heartbeat

```bash
cd impl/golang
go run ./cmd/hcs-heartbeat-test/
```

What it does, in order:

1. Creates a real HCS topic on Hedera testnet (`CreateTopic`).
2. Builds a spec-005 `HeartbeatPayload` — role, capability advertisement, and the self-declared `nextHeartbeatDeadline`.
3. Signs it into a spec-004 `TopicMessage` (RFC 6979 deterministic ECDSA over Keccak256) and publishes it.
4. Polls the **public mirror node** until the message appears, then validates the signature and payload round-trip — the observer-side proof that an independent party could verify your liveness from chain data alone.

The output prints the topic ID and a HashScan link:

```
https://hashscan.io/testnet/topic/<topicId>
```

Open it in a browser — your signed heartbeat is publicly visible and independently verifiable. You can also query the mirror node directly:

```bash
curl "https://testnet.mirrornode.hedera.com/api/v1/topics/<topicId>/messages"
```

### 2.3 What "verified" means here

An observer evaluates a heartbeat per spec 005 by checking: the envelope signature recovers to the sender's address; the payload schema and version are valid; and the consensus timestamp respects the previously declared deadline. Liveness states (`ALIVE` / `SUSPECT` / `DEAD` / `OFFLINE`) are evaluated **per observer** from chain data — there is no central liveness oracle.

## Stop / cleanup

- Both demos are run-to-completion CLIs — no background processes to stop.
- Mock mode leaves nothing behind.
- The testnet run leaves the created HCS topic on testnet (topics are cheap and inert; nothing references them afterwards). Unset the credentials when done:

```bash
unset HEDERA_OPERATOR_ID HEDERA_OPERATOR_KEY
```

## Troubleshooting

| Symptom | Cause / fix |
| ------- | ----------- |
| `Hedera operator credentials not set: missing env var(s) [...]` | Export both `HEDERA_OPERATOR_ID` and `HEDERA_OPERATOR_KEY`. |
| `parse HEDERA_OPERATOR_ID=...` error | The account ID must be the `0.0.XXXXXXX` form from the portal, not an EVM address. |
| Key parse failure | Use the **ECDSA** hex key. Ed25519 keys are auto-detected but not used by Neuron. |
| `INSUFFICIENT_PAYER_BALANCE` | Refill test HBAR at the portal faucet. |
| Mirror-node poll is slow | Mirror nodes lag consensus by a few seconds; the test retries. Persistent failure usually means a firewall blocks `testnet.mirrornode.hedera.com:443`. |
| `go: command not found` | Install Go ≥ 1.25 and ensure it is on `PATH`. |

## Where to go next

- **[docs/getting-started/](getting-started/)** — the full learning path: delivery, relay, browser, and full-commerce walkthroughs.
- **`make demo-sapient`** — the local SAPIENT sensor chain: simulated Remote ID sensor → SAPIENT seller → buyer proxy → map display.
- **[specs/005-health/spec.md](../specs/005-health/spec.md)** — the normative heartbeat/liveness model you just exercised.
