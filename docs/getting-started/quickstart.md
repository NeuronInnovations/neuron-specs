# Quickstart: Your First Neuron Demo

**Goal:** see Neuron's full commerce loop run end-to-end on your laptop in under one minute, with zero infrastructure.

**Specs exercised:** 002 (keys), 001 (account), 004 (topics), 003 (registry), 008 (payment), 009 (delivery), 010 (validation).

← Back to [Getting Started](README.md)

---

## Prerequisites

| What                                   | Why                              | How                                                              |
| -------------------------------------- | -------------------------------- | ---------------------------------------------------------------- |
| Go 1.22 or newer (1.25.7 recommended)  | The reference SDK is Go          | `go version` to check; install from [go.dev](https://go.dev/dl/) |
| A clone of this repo                   | The demo lives here              | `git clone <repo-url>`                                           |
| ~30 MB of disk for the Go module cache | First run downloads dependencies | Anywhere with write access                                       |

You do **not** need a Hedera account, testnet HBAR, an EVM RPC endpoint, environment variables, a second terminal, or internet at runtime (after the initial `go mod download`).

---

## Run it

```bash
cd impl/golang
go run ./cmd/buyer-seller-demo --mode=mock
```

For full hash and DID:key visibility:

```bash
go run ./cmd/buyer-seller-demo --mode=mock --verbose
```

To swap in your own JPEG instead of the bundled `testdata/photo.jpg`:

```bash
go run ./cmd/buyer-seller-demo --mode=mock --jpeg path/to/your.jpg
```

---

## Expected output

You'll see nine numbered phases. Annotated excerpt:

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

── 2: REGISTER ─────────────────────────────────────────       # spec 003
  [MOCK] Seller registered in-memory registry  agentId=1
         services = neuron-commerce (jpeg-delivery, p2p, 500000000 tinybar)
  [MOCK] Validator registered  agentId=2
  [MOCK] Buyer registered      agentId=3

── 3: DISCOVER ─────────────────────────────────────────       # spec 003
  [MOCK] Buyer looked up Seller via in-memory registry
         offer: JPEG delivery, 500000000 tinybar, p2p-stream

── 4: NEGOTIATE ─────────────────────────────────────────      # spec 008 + 004
  [REAL] Buyer -> Seller: serviceRequest (signed envelope on Seller.stdIn)
  [REAL] Agreement state machine: IDLE -> REQUESTED -> AGREED
  [REAL] agreementHash = 0x6e1e41375100acca... (keccak256 of canonical serviceResponse)

── 5: FUND ─────────────────────────────────────────           # spec 008
  [MOCK] Escrow created  ref=mem-escrow-1
  [MOCK] Buyer deposited 500000000 tinybar
  [REAL] state machine: AGREED -> FUNDED

── 6: CONNECT ─────────────────────────────────────────        # spec 009
  [REAL] Seller libp2p host started
         listening: /ip4/127.0.0.1/udp/64910/quic-v1/p2p/16Uiu2HAk...
  [REAL] Seller -> Buyer: connectionSetup
         ECIES encryption: secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM
  [REAL] Buyer decrypted connectionSetup and connected (transport = quic-v1)

── 7: DELIVER ─────────────────────────────────────────        # spec 009
  [REAL] Seller sent file via libp2p framing protocol (FR-D22)
         file = photo.jpg, size = 50282 bytes, SHA256 = 2b78cbe32b150f7ef8...
  [REAL] Buyer received and independently verified file (SHA256)
         SHA256 match = YES

── 8: SETTLE ─────────────────────────────────────────         # spec 008
  [REAL] Seller -> Buyer: invoice  (evidenceHash = 0x786ebd61ca23c421...)
  [REAL] Buyer -> Seller: invoiceAck (action=approved)
  [MOCK] Escrow released 500000000 tinybar

── 9: VALIDATE ─────────────────────────────────────────       # spec 010
  [REAL] Validator observed 6 protocol messages
  [REAL] Verifying ECDSA signatures (ecrecover + sender match)
         result: all 6 signatures VALID
  [REAL] Verdict #1: 008-payment  -> COMPLIANT
  [REAL] Verdict #2: 009-delivery -> COMPLIANT
  [REAL] Evidence hash chain verification: YES (both)

================================================================
  DEMO SUMMARY
================================================================

  Phase Results:
    1  SETUP       OK   3 identities created             [REAL crypto]
    2  REGISTER    OK   3 agents registered              [MOCK registry]
    ...
    9  VALIDATE    OK   6 sigs, 2 COMPLIANT verdicts     [REAL validation + MOCK bus]
```

Tags `[REAL]` and `[MOCK]` mark which pieces use real cryptography and which are swapped for in-memory stand-ins. Every signature, state machine transition, libp2p stream, ECIES handshake, SHA-256 hash, and validator verdict is real. Only the network substrate (HCS, on-chain registry, on-chain escrow) is mocked.

> Captured 2026-04-29 from `buyer-seller-demo --mode=mock`. Specific addresses, ports, and hashes will differ on each run; the structure is stable.

---

## What this demo proves

- secp256k1 key derivation produces consistent EVM addresses, PeerIDs, and DID:keys (spec 002)
- TopicMessage envelopes carry deterministic signatures that survive serialisation (spec 004 + 006)
- The negotiation state machine moves through `IDLE → REQUESTED → AGREED → FUNDED → ACTIVE → INVOICED → COMPLETED` correctly under signed messages (spec 008)
- The `EscrowAdapter` interface accepts a mock implementation that produces the same observable behaviour as the on-chain Solidity contract (spec 008)
- ECIES (secp256k1 ECDH + HKDF-SHA256 + AES-256-GCM) successfully encrypts a multiaddr to the buyer's pubkey, and the buyer decrypts and connects (spec 009)
- The libp2p frame protocol delivers a 50 KB JPEG with end-to-end SHA-256 integrity (spec 009)
- An independent validator can verify every signature, produce two `EvidenceEnvelope` records, and emit a hash that would anchor on-chain in the Validation Registry (spec 010)

---

## What it does NOT require

- No Hedera account, no testnet HBAR
- No EVM RPC endpoint, no funded EVM account
- No deployed Identity Registry, Reputation Registry, or Validation Registry
- No environment variables
- No second terminal
- No internet access at runtime (after `go mod download`)
- No external secrets, no `.env` files, no credentials

---

## Troubleshooting

| Symptom                                            | Fix                                                                                                           |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `command not found: go`                            | Install Go 1.22 or newer from [go.dev](https://go.dev/dl/)                                                    |
| `go: cannot find main module`                      | Run from `impl/golang/`, not from the repo root                                                               |
| `go mod download` is slow or failing               | Check `GOPROXY` (default `https://proxy.golang.org`); retry once; corporate networks may need a private proxy |
| `permission denied` reading `testdata/photo.jpg`   | Fix the file mode, or pass your own image with `--jpeg path/to/image.jpg`                                     |
| Errors about Hedera, HBAR, or operator credentials | You're not in mock mode. Confirm `--mode=mock` (it is the default) and that you didn't override it            |
| Demo hangs in phase 6 (CONNECT)                    | Some networks block ephemeral UDP. Set `--listen=/ip4/127.0.0.1/udp/0/quic-v1` to force loopback              |

---

## Next step

- Continue the [learning path](learning-path.md) — step 2 introduces the libp2p data plane across separate processes.
- Promote this same flow to real Hedera HCS by switching one flag: `--mode=testnet` (needs a Hedera testnet account; see [`../zero-to-heartbeat.md`](../zero-to-heartbeat.md) for setup).
- Skip ahead to the [browser demo](demos/4-browser-wss.md) to see a browser tab as the buyer.
