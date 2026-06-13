# Getting Started with Neuron

Welcome. This folder is the beginner's guide. If you just cloned the repo and want to see Neuron work, you're in the right place.

Neuron gives any device a verifiable identity, signed communication, provable liveness, and an end-to-end commerce loop on a public ledger. The fastest way to understand it is to run it.

---

## Read this first

```
cd impl/golang && go run ./cmd/buyer-seller-demo --mode=mock
```

That single command spins up three agents (seller, buyer, validator), negotiates a price, funds a mock escrow, opens an ECIES-encrypted libp2p stream, transfers a JPEG, verifies it on the receiver, and produces two `COMPLIANT` validator verdicts — in under a second, with zero infrastructure. You'll see this:

```
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

Lines marked `[REAL]` use real cryptography, real state machines, real libp2p. Only the network substrate (Hedera HCS, on-chain registry, on-chain escrow) is swapped for an in-memory mock so you can run it offline. Promote any single piece to "real" by changing `--mode=mock` to `--mode=testnet`.

---

## What's in this folder

| File                                     | When to read                                                                                         |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| **[quickstart.md](quickstart.md)**       | Annotated walkthrough of the command above (~5 min)                                                  |
| **[architecture.md](architecture.md)**   | Understand the system in 2 minutes                                                                   |
| **[learning-path.md](learning-path.md)** | The recommended 8-step ordered demo sequence                                                         |
| **[demos/](demos/)**                     | One detailed walkthrough per demo (mock, P2P delivery, relay, browser, WebTransport, Hedera testnet) |
| **[sdks/](sdks/)**                       | Use the Go or TypeScript SDK in your own project                                                     |

If you have 5 minutes, read `quickstart.md`. If you have 30, walk through `learning-path.md` end-to-end.

---

## Demo map

Run from repo root unless noted. Difficulty is the friction to get the demo going, not the complexity of what it proves.

| Demo                                                            | Command                                                                                                                                                                                                    | Concept proven                                                                  | Difficulty | Runtime           | Requires external?            |
| --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | ---------- | ----------------- | ----------------------------- |
| **[1. Mock buyer-seller](demos/1-buyer-seller-mock.md)**        | `cd impl/golang && go run ./cmd/buyer-seller-demo --mode=mock`                                                                                                                                             | Full commerce loop offline: setup, register, negotiate, fund, deliver, validate | Easy       | <1s               | No                            |
| **[2. P2P delivery](demos/2-delivery.md)**                      | Terminal A: `go run ./cmd/delivery-demo --mode seller` <br> Terminal B: `go run ./cmd/delivery-demo --mode buyer --peer <addr>`                                                                            | libp2p QUIC + frame protocol across processes                                   | Easy       | ~5s               | No                            |
| **[3. Relayed delivery](demos/3-relay.md)**                     | Terminal A: `go run ./cmd/relay-node` <br> Terminal B: `go run ./cmd/buyer-seller-demo --mode=mock --relay <relay-addr>`                                                                                   | Circuit Relay v2 + AutoNAT v2 NAT traversal                                     | Medium     | runs until Ctrl-C | No                            |
| **[4. Browser demo (WSS)](demos/4-browser-wss.md)**             | `cd impl/typescript && pnpm install && pnpm run demo`                                                                                                                                                      | Browser buyer over libp2p secure WebSockets                                     | Medium     | ~10s              | No                            |
| **[5. Browser WebTransport](demos/5-browser-webtransport.md)**  | Terminal A: `cd impl/golang && go run ./cmd/webtransport-seller --bootstrap-out ../typescript/examples/browser-demo-wt/public/bootstrap-wt.json` <br> Terminal B: `cd impl/typescript && pnpm run demo:wt` | Browser ↔ server over HTTP/3 WebTransport                                       | Medium     | ~10s              | No                            |
| **[6. Hedera heartbeat](demos/6-hedera-heartbeat.md)**          | `cd impl/golang && go run ./cmd/hcs-heartbeat-test`                                                                                                                                                        | Live HCS topic + heartbeat publish + mirror-node verify                         | Hard       | ~30s              | Hedera testnet account + HBAR |
| **[7. Full testnet commerce](demos/7-hedera-full-commerce.md)** | `cd impl/golang && go run ./cmd/buyer-seller-demo --mode=testnet`                                                                                                                                          | The full mock demo, but every message goes through real HCS consensus           | Hard       | ~90s              | Hedera testnet account + HBAR |
| **[8. SAPIENT sensor chain](demos/8-sapient-chain.md)**         | `make demo-sapient`                                                                                                                                                                                        | Sensor → SAPIENT seller → Buyer Proxy → consumer → live map, all local          | Easy       | ~30s + Ctrl-C     | No                            |

Hedera testnet runs need `HEDERA_OPERATOR_ID` and `HEDERA_OPERATOR_KEY` set in the environment. Get them free at [portal.hedera.com](https://portal.hedera.com).

---

## Where to go next

**Run another demo**

- Step-by-step learning sequence → [learning-path.md](learning-path.md)
- Full mock-demo walkthrough → [quickstart.md](quickstart.md)
- Testnet end-to-end (~30 min) → [../zero-to-heartbeat.md](../zero-to-heartbeat.md)

**Use the SDK in your own project**

- Go SDK integrator guide → [sdks/go.md](sdks/go.md)
- TypeScript SDK integrator guide → [sdks/typescript.md](sdks/typescript.md)

**Read a spec**

- All specs → [`../../specs/`](../../specs/)
- Core SDK: 002 Key Library, 001 Account, 004 Topic System, 003 Peer Registry, 005 Health, 006 Protocol Determinism, 007 Identity Contract, 008 Payment, 009 P2P Data Delivery, 010 Validation Framework, 011 Relay, 012 Browser Client Profile, 013 Connectivity Profiles
- Application layer: 015 SAPIENT Interop Profile, 016 JetVision ADS-B, 017 DroneScout Remote ID, 018 CoT Display Consumer (`impl/golang/internal/dapp/`)

**See the code**

- Go SDK packages → [`../../impl/golang/internal/`](../../impl/golang/internal/)
- TypeScript SDK packages → [`../../impl/typescript/src/`](../../impl/typescript/src/)
- Browser demo source → [`../../impl/typescript/examples/browser-demo/`](../../impl/typescript/examples/browser-demo/) (WSS) and [`../../impl/typescript/examples/browser-demo-wt/`](../../impl/typescript/examples/browser-demo-wt/) (WebTransport)
- Demo CLIs → [`../../impl/golang/cmd/`](../../impl/golang/cmd/)

**Understand the rules**

- Constitution (twelve ratified principles) → [`../../.specify/memory/constitution.md`](../../.specify/memory/constitution.md)
- Repository guide → [`../../CLAUDE.md`](../../CLAUDE.md)
