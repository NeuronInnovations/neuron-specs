# Learning Path

← Back to [Getting Started](README.md)

A new technical reader can run every demo in this repo in the order below. Each step adds **one** new concept on top of the previous step. Skip steps at your peril — context compounds.

Annotations: `[L]` = local-only, `[T]` = needs Hedera testnet keys.

| #   | Stage            | Demo                                                     | New concept introduced                                                       | Time              | Reqs                                |
| --- | ---------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------- | ----------------- | ----------------------------------- |
| 1   | Hello            | [Mock buyer-seller](demos/1-buyer-seller-mock.md)        | The full commerce loop in one process                                        | <1s               | `[L]` Go 1.22+                      |
| 2   | Across processes | [P2P delivery](demos/2-delivery.md)                      | The data plane is real libp2p across separate OS processes                   | ~5s               | `[L]` Go 1.22+                      |
| 3   | NAT topology     | [Relayed delivery](demos/3-relay.md)                     | Circuit Relay v2 + AutoNAT v2 — how NATed peers reach each other             | runs until Ctrl-C | `[L]` Go 1.22+                      |
| 4   | Browser buyer    | [Browser demo (WSS)](demos/4-browser-wss.md)             | The buyer is a browser tab, talking via libp2p over secure WebSockets        | ~10s              | `[L]` Node 20+, modern browser      |
| 5   | Browser direct   | [Browser WebTransport](demos/5-browser-webtransport.md)  | Same browser flow, HTTP/3 transport instead of WebSockets                    | ~10s              | `[L]` Node 20+, Chromium ≥ 120      |
| 6   | Liveness layer   | [Hedera heartbeat](demos/6-hedera-heartbeat.md)          | First time on a real public ledger — heartbeats + mirror-node verification   | ~30s              | `[T]` Hedera testnet account        |
| 7   | Full real path   | [Full testnet commerce](demos/7-hedera-full-commerce.md) | Steps 1 + 6 combined: every protocol message goes through real HCS consensus | ~90s              | `[T]` Hedera testnet account + HBAR |
| 8   | Application layer | [SAPIENT sensor chain](demos/8-sapient-chain.md)        | A real sensor-data product on top of the stack: SAPIENT proxies, live map    | ~30s + Ctrl-C     | `[L]` Go 1.22+                      |

## Why this order

**Step 1 first** because it is genuinely zero-friction (sub-second runtime, no env vars, no network). One command and you have the whole story. Everything else is exploring how each piece scales beyond that.

**Step 2 immediately after** because it answers the question every step-1 reader has: "is the libp2p stuff real, or just simulated like the topic bus?" Yes — watch two processes talk over QUIC. The frame protocol, multiaddrs, and PeerIDs are the same code paths step 1 exercised in-process.

**Step 3 (relay)** before browser because relays are the obvious next topology question and stay entirely in Go. By this point you've seen the data plane work directly between two peers; relayed delivery shows what happens when neither peer can dial the other directly.

**Step 4 introduces a new language** only after the Go demos have built intuition. The TypeScript SDK derives purely from specs — the protocol semantics in your browser are byte-for-byte identical to what step 1 ran. The transport binding changes (WSS instead of QUIC); everything else is the same.

**Step 5 is a transport swap** — same browser code, same negotiation, different libp2p binding (WebTransport over HTTP/3). This proves the SDK's transport abstraction holds in the browser, not just in Go.

**Steps 6 and 7 require real money/keys**, so they sit behind the local-only stack. By the time you reach them you understand every protocol piece; you're now just promoting the substrate from in-memory to real consensus.

**Step 8 is the application layer** — back to local-only. Everything in steps 1–7 was protocol plumbing; step 8 shows what a product built on it looks like: a simulated drone sensor speaking SAPIENT (the UK-DSTL sensor-interop standard), the generic Seller/Buyer Proxy pair carrying it over Neuron lanes, and a live map rendering the tracks. It needs nothing but Go.

## What you'll have learned by the end

- How a single secp256k1 key gives one agent four derived identities (public key, EVM address, PeerID, DID:key) — and why all four matter (step 1)
- How signed `TopicMessage` envelopes flow through a control plane that is substrate-agnostic (in-memory, HCS, ERC-20 logs, Kafka — same SDK code) (steps 1, 6)
- How the negotiation state machine drives a sale from `IDLE` to `COMPLETED` under signed messages (step 1)
- How ECIES encrypts a multiaddr to a buyer's public key so the data plane handshake leaks no information (step 1)
- How libp2p QUIC streams + a small frame protocol carry the actual payload with end-to-end SHA-256 integrity (steps 1, 2)
- How Circuit Relay v2 + AutoNAT v2 let NATed peers talk without either side punching out (step 3)
- How the same SDK runs in a browser tab over WSS or WebTransport (steps 4, 5)
- How a heartbeat on real HCS proves liveness without a central oracle (step 6)
- How the full commerce loop — including escrow — runs over real Hedera consensus (step 7)
- How a sensor application rides the stack: Neuron-blind translators behind generic SAPIENT proxies, modality-blind multi-source buying, and modality interpretation only at the display (step 8)

## Branches you might take

- Want the formal contracts? Read the [specs](../../specs/) — start with [002](../../specs/002-key-library/spec.md) and follow the dependency chain in [`../../CLAUDE.md`](../../CLAUDE.md).
- Want to embed the SDK in your own product? Skip to the [Go SDK guide](sdks/go.md) or [TypeScript SDK guide](sdks/typescript.md).
- Want the deep-dive walkthrough of step 1 with annotated output? See [quickstart.md](quickstart.md).
- Want to understand what each component is, not how to run them? See [architecture.md](architecture.md).
- Want to deploy a public seller for browser clients? Run `cmd/webtransport-seller` on any host with a public IP and TLS certificate; the browser demo dials it over HTTP/3.

## How long the whole path takes

| Steps                                   | Time                                                      |
| --------------------------------------- | --------------------------------------------------------- |
| 1–3 (local Go demos)                    | ~10 minutes including reading                             |
| 4–5 (browser demos, fresh Node install) | ~15 minutes                                               |
| 6–7 (testnet, fresh Hedera account)     | ~45 minutes including portal sign-up + funding            |
| 8 (local SAPIENT chain)                 | ~5 minutes including reading                              |
| **Total cold start**                    | **~75 minutes** to run every demo in this repo end-to-end |

If you skip steps 6–7 (no Hedera account), you can run steps 1–5 and 8 in under 30 minutes.
