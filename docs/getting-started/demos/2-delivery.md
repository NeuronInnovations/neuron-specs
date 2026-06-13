# 2. P2P Delivery Demo

← Back to [Getting Started](../README.md) · [Demo map](../README.md#demo-map) · [Learning path](../learning-path.md)

The data plane in isolation, across two separate OS processes. Step 1 ran the libp2p stream in-memory; this demo proves the same code paths work over real QUIC sockets.

## What this demo proves

- libp2p QUIC streams cross process boundaries cleanly (spec [009](../../../specs/009-p2p-data-delivery/spec.md))
- The frame protocol (4 MiB max per frame) carries arbitrary payloads with stable framing
- PeerID derivation produces a stable libp2p identity from the same secp256k1 key (spec [002](../../../specs/002-key-library/spec.md))
- Connection state transitions (`CONNECTED` → `DISCONNECTED`) propagate to the buyer's `DeliveryAdapter`

## When to run it

Run this **after** [Step 1](1-buyer-seller-mock.md). Step 1 ran the libp2p stream in-process; this demo answers the obvious next question — is the libp2p code path actually real, or simulated like the topic bus? It is real. Watch two processes talk over QUIC.

## Prerequisites

| What | How to check |
|------|--------------|
| Go 1.22+ | `go version` |
| Two terminals open at the repo root | — |
| Local UDP traffic allowed (loopback) | usually yes; corporate firewalls occasionally block this |

You do **not** need: any infrastructure, env vars, or external network access.

## Run it

This demo needs **two terminals**, both inside `impl/golang/`.

**Terminal A — start the seller:**

```bash
cd impl/golang
go run ./cmd/delivery-demo --mode seller --listen /ip4/127.0.0.1/udp/0/quic-v1
```

The seller prints its multiaddr. Copy the line that starts with `Multiaddr:`.

**Terminal B — connect with the buyer:**

```bash
cd impl/golang
go run ./cmd/delivery-demo --mode buyer \
  --peer /ip4/127.0.0.1/udp/<PORT>/quic-v1/p2p/<PEER_ID> \
  --frames 3
```

Paste the seller's multiaddr into `--peer`. The buyer sends three test frames and disconnects.

## Expected output

**Terminal A (seller):**

```
=== SELLER READY ===
PeerID:    16Uiu2HAmHrZN71djaRPe3dqNmuR1bJ4CcE93Jbas9c5gptAtJUK1
Multiaddr: /ip4/127.0.0.1/udp/52988/quic-v1/p2p/16Uiu2HAmHrZN71djaRPe3dqNmuR1bJ4CcE93Jbas9c5gptAtJUK1
Protocol:  /neuron/demo/1.0.0

Waiting for buyer connection... (Ctrl+C to stop)

[CONNECTED stream=1] Buyer 16Uiu2HAm7Zi2ezyHhHp4SQ3yt24PtDwpyc6t5PjSzCC7v1Ec7eh4 connected via quic-v1 (remote=/ip4/127.0.0.1/udp/63196/quic-v1 limited=false)
[RECV stream=1] 61 bytes: {"seq":0,"type":"adsb","data":"test-payload","ts":1777436326}
[RECV stream=1] 61 bytes: {"seq":1,"type":"adsb","data":"test-payload","ts":1777436326}
[RECV stream=1] 61 bytes: {"seq":2,"type":"adsb","data":"test-payload","ts":1777436326}
[DISCONNECTED stream=1] Buyer stream closed: delivery.Receive: [StreamError] EOF
```

**Terminal B (buyer):**

```
=== BUYER CONNECTING ===
Buyer PeerID:  16Uiu2HAm7Zi2ezyHhHp4SQ3yt24PtDwpyc6t5PjSzCC7v1Ec7eh4
Seller PeerID: 16Uiu2HAmHrZN71djaRPe3dqNmuR1bJ4CcE93Jbas9c5gptAtJUK1
Protocol:      /neuron/demo/1.0.0

Connected: state=CONNECTED, transport=quic-v1, remote=/ip4/127.0.0.1/udp/52988/quic-v1

Sending 3 test frames...
[1/3] Sent 61 bytes: {"seq":0,"type":"adsb","data":"test-payload","ts":1777436326}
[2/3] Sent 61 bytes: {"seq":1,"type":"adsb","data":"test-payload","ts":1777436326}
[3/3] Sent 61 bytes: {"seq":2,"type":"adsb","data":"test-payload","ts":1777436326}

All frames sent. Disconnecting...
state=DISCONNECTED
Buyer done.
```

> Captured 2026-04-29 from a coordinated two-terminal run on macOS.

## How to verify success

Pass criteria: the seller logs **three `[RECV]` lines** matching the buyer's three `[Sent]` lines, and the disconnect propagates cleanly to both sides (`Buyer done.` on the buyer, `Buyer stream closed` on the seller).

If the byte counts match (`61 bytes` on both sides for default JSON frames), framing is intact.

## What this maps to

| Component | Spec | Source |
|-----------|------|--------|
| QUIC transport, libp2p host setup | [009](../../../specs/009-p2p-data-delivery/spec.md) | [`internal/delivery/adapter_libp2p.go`](../../../impl/golang/internal/delivery/) |
| Frame protocol (length-prefixed, 4 MiB cap) | [009](../../../specs/009-p2p-data-delivery/spec.md) FR-D22 | [`internal/delivery/frame.go`](../../../impl/golang/internal/delivery/) |
| PeerID derivation | [002](../../../specs/002-key-library/spec.md) | [`internal/keylib/peer_id.go`](../../../impl/golang/internal/keylib/) |
| Connection state machine | [009](../../../specs/009-p2p-data-delivery/spec.md) | [`internal/delivery/state.go`](../../../impl/golang/internal/delivery/) |

Source: [`impl/golang/cmd/delivery-demo/main.go`](../../../impl/golang/cmd/delivery-demo/main.go).

## Useful flags

| Flag | What it does |
|------|--------------|
| `--frames N` | Buyer sends N frames (default 5) |
| `--payload-size N` | Buyer sends raw bytes of size N instead of default JSON frames |
| `--disconnect-after N` | Seller force-closes after receiving N frames (test disconnect handling) |
| `--protocol /your/proto/1.0.0` | Override the libp2p stream protocol ID |
| `--key <hex>` | Use a specific secp256k1 key instead of a random one (deterministic PeerID) |
| `--relay <addr>` | Route through a Circuit Relay v2 — see [Step 3](3-relay.md) |
| `--second-stream-after-ms N` | Buyer waits N ms then opens a second stream — useful for testing post-DCUtR path selection |

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Buyer hangs at `Connected:` then nothing | Wrong multiaddr. Make sure you copied the full line including `/p2p/<PeerID>` |
| `dial backoff` errors | Seller crashed or the port is already in use; restart the seller |
| Local UDP blocked | macOS occasionally blocks new UDP listeners on first run; allow it in System Settings → Privacy → Local Network |
| `peer not found` | The seller's PeerID changed because the key is randomised on every run. Always copy the multiaddr from the same seller process you're connecting to |
| Connection succeeds but no `[RECV]` lines | Frame send failed silently; rerun with `--frames 1 --payload-size 1024` to isolate framing |

## Next demo

→ **[Step 3: Relayed delivery](3-relay.md)** — same data plane, but through a Circuit Relay v2 hop for NAT traversal.
