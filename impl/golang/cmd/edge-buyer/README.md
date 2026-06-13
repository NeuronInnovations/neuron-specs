# edge-buyer

Spec-built Neuron buyer for the **reverse-connect** topology with **multi-seller aggregation**. Listens on a publicly-reachable libp2p multiaddr, publishes a `ReverseConnectionSetup` to each configured seller's stdIn topic announcing its multiaddrs (encrypted to that seller's pubkey via ECIES), accepts the incoming libp2p stream when each seller dials in, and emits one JSONL record per received frame to a configurable output sink.

## Usage

```text
edge-buyer [--mode=testnet|mock] [--bootstrap-in=<path>] [--output=<sink>]
```

## Required environment

| Variable | Purpose |
|----------|---------|
| `NEURON_EDGE_PRIVATE_KEY` | 32-byte secp256k1 private key, hex-encoded, no `0x`. |
| `HEDERA_OPERATOR_ID` | Hedera testnet account ID. Required for `--mode=testnet`. |
| `HEDERA_OPERATOR_KEY` | Hedera operator private key (ECDSA hex). Required for `--mode=testnet`. |

## Sellers

Two ways to point the buyer at sellers:

| Variable / flag | Behavior |
|-----------------|----------|
| `NEURON_EDGE_SELLERS_BOOTSTRAP=alpha=./a.json,beta=./b.json,gamma=./c.json` | Multi-seller. Each entry is `name=path` (name optional). Path is a `seller-bootstrap.json` written by an `edge-seller`. |
| `NEURON_EDGE_BOOTSTRAP_IN` / `--bootstrap-in` | Legacy single-seller path. Used only when `NEURON_EDGE_SELLERS_BOOTSTRAP` is unset. |

Each seller gets its own goroutine, connection lifecycle, and reconnect loop — a dropped seller does not affect any other.

## Output sink

| `NEURON_EDGE_OUTPUT` value (or `--output`) | Behavior |
|--------------------------------------------|----------|
| `stdout` (default) | One JSONL record per frame on os.Stdout. |
| `file:/path/to/x.jsonl` | One JSONL record per frame to a file (truncates on open). |
| `file+:/path/to/x.jsonl` | Same, but appends if the file exists. |
| `tcp:host:port` | Connects on first frame and writes line-delimited JSON to a TCP target. Reconnects on write failure. |

The JSONL schema and exact field semantics are documented in `docs/edge-demo-runbook.md` § "JSONL record schema".

## Optional environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `NEURON_EDGE_LIBP2P_LISTEN` | `/ip4/0.0.0.0/udp/0/quic-v1` | Buyer must be reachable here for sellers' dials to succeed. |
| `NEURON_EDGE_LIBP2P_ADVERTISED` | (auto) | Comma-separated multiaddrs to advertise instead of `host.Addrs()`. |
| `NEURON_EDGE_HEARTBEAT_PERIOD` | `60s` | Spec 005 recommends 60 s; ≥ 10 s required. |
| `NEURON_EDGE_REQUEST_ID` | `edge-feed-001` | Carried in `ReverseConnectionSetup` (per-seller suffix appended automatically: `…-alpha`, `…-beta`). |
| `NEURON_EDGE_RECONNECT_BACKOFF` | `10s` | Per-seller delay between reconnection attempts when a stream closes or dial-in times out. |
| `NEURON_EDGE_SELLER_DIAL_TIMEOUT` | `60s` | How long the buyer waits for a seller to dial in before re-publishing its setup. |

## What the buyer logs

- On startup: per-seller `loaded seller: name=… evm=… stdIn=…`.
- For each new stream: `[buyer:<name>] accepted stream peer=… transport=…`.
- First 5 frames per seller in detail: `frame#1 from alpha: bytes=14 df=17 icao="4ca853"`.
- Every 5 seconds: aggregate rate per seller: `rate: total=N delta=M (last 5s) [0xAAAAAAAA=K1 0xBBBBBBBB=K2]`.
- On every state transition: `seller alpha [0x4699c1Ec4B] state=connected frames=2410 err=""`.

## ICAO recovery (Phase C.2)

`AggregatedFrame.Meta.Recovered` (bool) is `true` when the ICAO came from the parity-XOR cache rather than the plaintext field. The buyer maintains one shared `feeds.ICAORecoveryCache` (cap=512, ttl=60s by default) across every per-seller stream:

- DF 11 / 17 / 18 frames carry plaintext ICAO → `Recovered=false`. The ICAO is `Observe()`d into the cache.
- DF 0 / 4 / 5 / 16 / 20 / 21 frames carry parity ⊕ ICAO → `TryRecover` extracts a candidate via 24-bit Mode-S CRC, looks it up in the cache. On hit: `meta.ICAO` filled, `Recovered=true`. On miss: `meta.ICAO=""`, `Recovered=false`.

Downstream consumers that need a strict chain of custody should prefer `Recovered=false` sources. The collision risk is bounded by `cache_size / 2²⁴`; with the default 512-entry cache, that's ~3 mis-attributions per 100k parity-XOR'd frames.

Disable recovery by passing `BuyerConfig.DisableICAOCache = true`. Tune via `ICAOCache: feeds.NewICAORecoveryCache(cap, ttl)`.

## Demo

End-to-end demo procedure with terminal-by-terminal commands and the success criteria is in `docs/edge-demo-runbook.md`. Multi-seller specifics are in the "Multi-seller aggregation" section there.
