# 3. Relayed Delivery Demo

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

A standalone Circuit Relay v2 node, plus the buyer-seller demo routed through it. This is the answer to "what happens when neither peer can dial the other directly?" — for example, a NATed buyer behind a home router and a NATed seller behind a corporate firewall.

## What this demo proves

- Circuit Relay v2 successfully relays libp2p traffic between two peers that cannot dial directly (spec [011](../../../specs/011-relay/spec.md))
- AutoNAT v2 lets a peer detect its own reachability and request relay reservations only when needed
- The same buyer-seller commerce flow works unchanged through a relay — the application is unaware of the topology

## When to run it

Run this **after** [Step 2](2-delivery.md). You now know how the data plane works peer-to-peer; this demo shows how it scales when the network fabric isn't friendly.

## Prerequisites

| What | How to check |
|------|--------------|
| Go 1.22+ | `go version` |
| Two terminals open at the repo root | — |
| Steps 1 and 2 already worked on this machine | — |

You do **not** need: a public IP, a deployed VPS, or any external infrastructure. Localhost is enough to demonstrate the topology.

## Run it

This demo needs **two terminals**.

**Terminal A — start the relay node:**

```bash
cd impl/golang
go run ./cmd/relay-node
```

The relay prints its PeerID and listen multiaddrs. Copy the QUIC multiaddr (the one ending in `/quic-v1/p2p/<PeerID>`).

**Terminal B — run the buyer-seller demo through the relay:**

```bash
cd impl/golang
go run ./cmd/buyer-seller-demo --mode=mock \
  --relay /ip4/127.0.0.1/udp/4001/quic-v1/p2p/<RELAY_PEER_ID>
```

The demo runs to completion as in step 1, but with the relay flag active the buyer and seller advertise relay-reachable addresses (`/p2p-circuit/`) and exercise the Circuit Relay v2 reservation flow.

## Expected output

**Terminal A (relay):**

```
=== RELAY READY ===
PeerID:        12D3KooWB...
Listen QUIC:   /ip4/0.0.0.0/udp/4001/quic-v1/p2p/12D3KooWB...
Listen TCP:    /ip4/0.0.0.0/tcp/4001/p2p/12D3KooWB...
Services:      Circuit Relay v2, AutoNAT v2

[RESERVATION] new from peer 16Uiu2HAk... (limit 4 hours, 128 KiB/s)
[RESERVATION] new from peer 16Uiu2HAm... (limit 4 hours, 128 KiB/s)
[CONNECT-REQUEST] 16Uiu2HAm... -> 16Uiu2HAk... — accepted (limit 2 minutes, 1 MiB)
[CONNECT-RESPONSE] forwarded
```

**Terminal B (buyer-seller demo):** runs to completion as in [Step 1](1-buyer-seller-mock.md), with one extra line in the **6 CONNECT** phase noting the relay path.

> Behaviour-level output. Specific line formatting may vary; the structure (reservations precede the connect request, then the connect-response forwards) is stable.

## How to verify success

Pass criteria: the relay logs **two `[RESERVATION]` lines** (one per peer) followed by a `[CONNECT-REQUEST]` and `[CONNECT-RESPONSE]`. The buyer-seller demo in Terminal B reaches `9 VALIDATE OK` exactly as in step 1.

If you stop the relay mid-run, the buyer and seller fall back to direct dialling (step 2 behaviour) when both are reachable; on a real NATed pair, they fail with `dial backoff` instead.

## What this maps to

| Component | Spec | Source |
|-----------|------|--------|
| Circuit Relay v2 service | [011](../../../specs/011-relay/spec.md) | [`cmd/relay-node/main.go`](../../../impl/golang/cmd/relay-node/main.go) |
| AutoNAT v2 reachability detection | libp2p stdlib | wired in [`cmd/relay-node/main.go`](../../../impl/golang/cmd/relay-node/main.go) |
| Buyer/seller relay-aware dialling | [009](../../../specs/009-p2p-data-delivery/spec.md), [011](../../../specs/011-relay/spec.md) | [`internal/delivery/`](../../../impl/golang/internal/delivery/) |

Source: [`cmd/relay-node/main.go`](../../../impl/golang/cmd/relay-node/main.go) and [`cmd/buyer-seller-demo/main.go`](../../../impl/golang/cmd/buyer-seller-demo/main.go).

## Useful flags

**Relay node:**

| Flag | Default | Purpose |
|------|---------|---------|
| `--listen-quic` | `/ip4/0.0.0.0/udp/4001/quic-v1` | QUIC listen multiaddr |
| `--listen-tcp` | `/ip4/0.0.0.0/tcp/4001` | TCP listen multiaddr |
| `--identity` | (ephemeral) | Path to a persistent secp256k1 key file (created if missing). Use this if you want a stable PeerID across restarts |

**Buyer-seller / delivery demo:**

| Flag | Purpose |
|------|---------|
| `--relay <addr>` | Comma-separated list of Circuit Relay v2 multiaddrs to use for autorelay + DCUtR |

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Buyer-seller demo never reaches CONNECT phase | Wrong relay multiaddr. Confirm you used the QUIC line ending in `/quic-v1/p2p/<PeerID>` |
| Relay never logs `[RESERVATION]` | The buyer-seller demo isn't using the `--relay` flag; check the command in Terminal B |
| Port 4001 already in use | Pass `--listen-quic /ip4/0.0.0.0/udp/4002/quic-v1` to the relay; update Terminal B's multiaddr accordingly |
| `cannot dial` between local peers | Localhost demos always have a direct path; relay code still runs but the reservation may not be exercised. Run the relay on a public host for a real cross-NAT scenario |

## Production deployment

For a real-world cross-NAT validation, run the same relay node binary on a public host — only the deployment topology differs.

## Next demo

→ **[Step 4: Browser demo (WSS)](4-browser-wss.md)** — same protocol, but the buyer is now a browser tab.
