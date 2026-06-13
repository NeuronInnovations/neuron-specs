# edge-seller

Spec-built Neuron seller for the **reverse-connect** topology (NAT'd seller dials publicly-reachable buyer). Reads BEAST Mode-S frames from a TCP source (default `127.0.0.1:10003`, the JetVision Air!Squitter feed) and forwards each frame as a length-prefixed binary envelope (`feeds.EncodeFeedFrame`) over a libp2p QUIC stream.

This is the spec-derived counterpart to the deployed production seller `/usr/bin/neuron-sdk` v0.52.101. It is intentionally **not** wire-compatible with v0.52.101 (heartbeat envelopes diverge) — see `docs/heartbeat-prod-vs-spec.md` for the diff.

## Usage

```text
edge-seller [--mode=testnet|mock] [--bootstrap-out=<path>]
```

## Required environment

| Variable | Purpose |
|----------|---------|
| `NEURON_EDGE_PRIVATE_KEY` | 32-byte secp256k1 private key, hex-encoded, no `0x`. Signs all topic messages. Derives PeerID + EVM address. |
| `HEDERA_OPERATOR_ID` | Hedera testnet account ID (e.g. `0.0.X`). Required for `--mode=testnet`. |
| `HEDERA_OPERATOR_KEY` | Hedera operator private key (ECDSA hex). Required for `--mode=testnet`. |

## Optional environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `NEURON_EDGE_FEED_SOURCE` | `tcp` | One of `tcp`, `replay`, `synth`. |
| `NEURON_EDGE_FEED_HOSTPORT` | `127.0.0.1:10003` | When source=tcp. JetVision Air!Squitter or rcd default. |
| `NEURON_EDGE_FEED_REPLAY_FILE` | (none) | When source=replay. Path to a captured BEAST byte stream. |
| `NEURON_EDGE_FEED_REPLAY_SPEEDUP` | `1.0` | Replay rate multiplier. |
| `NEURON_EDGE_FEED_SYNTH_FPS` | `50` | When source=synth. Synthetic frame rate. |
| `NEURON_EDGE_LIBP2P_LISTEN` | `/ip4/0.0.0.0/udp/0/quic-v1` | Listen multiaddr (outbound-only is fine). |
| `NEURON_EDGE_HEARTBEAT_PERIOD` | `60s` | Spec 005 recommends 60 s; ≥ 10 s required. |
| `NEURON_EDGE_BOOTSTRAP_OUT` | `./seller-bootstrap.json` | Where to write the bootstrap file the buyer reads. |

## Bootstrap file format

After `edge-seller` creates its three HCS topics it writes a JSON file (per `--bootstrap-out`) containing:

```json
{
  "evmAddress":    "0x…",
  "publicKeyHex":  "<65-byte uncompressed secp256k1 point in hex>",
  "stdInLocator":  "0.0.X",
  "stdOutLocator": "0.0.Y",
  "stdErrLocator": "0.0.Z",
  "backendKind":   "hcs",
  "networkLabel":  "testnet"
}
```

The `edge-buyer` reads this file via `--bootstrap-in` to learn the seller's pubkey (for ECIES encryption) and stdIn topic (where to publish the `ReverseConnectionSetup`).

## Spec-005 envelope (NOT v0.52.101)

The seller publishes spec-005 v1.0.0 heartbeats to its stdOut: `{type:"heartbeat", role:"seller", capabilities.natReachability:false, nextHeartbeatDeadline:"…", …}`. **This is intentionally different from the production v0.52.101 envelope.** Production buyers will not parse our envelope. See `docs/heartbeat-prod-vs-spec.md`.

## Connection-manager tuning (Phase C.2)

`RunSeller` builds the libp2p host with `WithConnManager(320, 384, 90s)` and protects every active stream's underlying connection (`host.ConnManager().Protect(peerID, "neuron-active-stream")`). This mirrors the deployed seller's burst-investigation mitigation for `0x1005` `ConnGarbageCollected` close-waves under fan-in. Watermarks come from `internal/edgeapp/config.go`'s `DefaultConnMgr*` constants.

## Running

The canonical end-to-end demo script is `docs/edge-demo-runbook.md`.

## Binary size

The seller is intended for ARMv7 SBC deployment (e.g. JetVision Air!Squitter, ~499 MB RAM). Cross-compile with the recommended flags:

```bash
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' \
  -o build/edge-seller-armv7 ./cmd/edge-seller
```

Reference sizes (from a clean checkout, multi-seller aggregation already merged):

| Target | Stripped size | Notes |
|--------|---------------|-------|
| `linux/arm/v7` | ~33 MB | The intended on-device target. |
| `linux/amd64`  | ~35 MB | Useful for VPS-based smoke tests of the seller half. |
| `darwin/arm64` | ~34 MB | Useful for laptop-based development. |

Dominant weight (per `go tool nm -size`):

- **`hiero-sdk-go` + transitive gRPC + protobuf** — ~3 MB. This is the HCS control plane and is unavoidable.
- **`libp2p` + transitive QUIC / pion / mDNS** — ~1 MB. This is the data plane and is unavoidable.
- **`net/http` + `crypto/tls` + `math/big`** — ~1 MB combined. Pulled in by hiero-sdk-go's mirror-node REST client.

### How to measure

```bash
# Stripped, for deployment.
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o /tmp/edge-seller ./cmd/edge-seller
ls -lh /tmp/edge-seller

# With symbols, for inspection. ~30% larger.
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 \
  go build -trimpath -o /tmp/edge-seller-syms ./cmd/edge-seller

# Symbol-bytes-by-package breakdown.
go tool nm -size /tmp/edge-seller-syms | awk 'NF >= 4 {
  pkg = $NF; n = split(pkg, parts, ".");
  pkg = parts[1];
  for (i = 2; i < n; i++) pkg = pkg "." parts[i];
  sub(/\([^)]*\)$/, "", pkg);
  sub(/\.[^.]*$/, "", pkg);
  sums[pkg] += $2 + 0
} END { for (k in sums) printf "%10d %s\n", sums[k], k }' | sort -rn | head -20
```

### Further reductions

- **UPX compression** can drop the binary to ~10 MB (`upx --best --lzma /tmp/edge-seller`), at the cost of breaking ASLR and adding ~100 ms cold-start time. Worth it for shipping over slow update channels; not enabled by default.
- **Stripping libp2p transports** down to QUIC-only (skipping pion/webrtc and pion/sctp) is estimated at ~1 MB — left as a future optimization if the binary becomes a real constraint. Requires constructing the libp2p host with `libp2p.NoTransports` + explicit QUIC, which currently lives in `internal/delivery/libp2p_host.go`.

The multi-seller aggregation work is **buyer-side only** — the seller binary's size is unchanged before/after that change.
