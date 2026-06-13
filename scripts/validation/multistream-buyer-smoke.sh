#!/usr/bin/env bash
# multistream-buyer smoke harness — verifies the fused-buyer topology:
# ONE buyer process with N parallel seller sessions feeding ONE
# consolidated TaggedFrame JSONL stream to fid-display.
#
# Topology:
#
#   adsb-seller     -> /jetvision/basestation/1.0.0    \
#                                                  -> multistream-buyer -> fid-display
#   remoteid-seller -> /ds240/basestation/1.0.0 /
#
# Hard invariant asserted: exactly ONE `multistream-buyer` process is
# running, yet BOTH `normalizedTracks[]`
# (ADS-B) and `drones[]` (Remote ID) appear in /state.json.
#
# Mode: fully local (fixture-direct + synthetic sources + memory
# backends, no on-chain interaction).
#
# Usage: from repo root,
#   scripts/validation/multistream-buyer-smoke.sh [BIN_DIR]
# If BIN_DIR is supplied, the script uses pre-built binaries from that
# directory. Otherwise it builds the four CLIs into a fresh /tmp dir.
#
# Exits 0 on PASS, 1 on FAIL.
set -uo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN_DIR="${1:-}"
WORKDIR="$(mktemp -d -t multi-buy.XXXXXX)"
EVD="$WORKDIR/evidence"
mkdir -p "$EVD"

if [ -z "$BIN_DIR" ]; then
  BIN_DIR="$WORKDIR/bin"
  mkdir -p "$BIN_DIR"
  echo "===> Building four CLIs into $BIN_DIR"
  ( cd "$REPO_ROOT/impl/golang" && \
    go build -o "$BIN_DIR/adsb-seller"       ./cmd/adsb-seller && \
    go build -o "$BIN_DIR/remoteid-seller"   ./cmd/remoteid-seller && \
    go build -o "$BIN_DIR/multistream-buyer" ./cmd/multistream-buyer && \
    go build -o "$BIN_DIR/fid-display"       ./cmd/fid-display ) || {
      echo "FAIL: build failed" >&2
      exit 1
    }
fi

# Ports off the standard 8080/9090 default so the script never collides
# with a developer's existing fid-display.
HTTP_PORT=18080
TCP_PORT=19090

# Process IDs initialised to empty so the cleanup trap is safe to run
# before any process has started.
FID_PID=""; ADSB_S_PID=""; RID_S_PID=""; MS_PID=""

cleanup() {
  for p in "$MS_PID" "$ADSB_S_PID" "$RID_S_PID" "$FID_PID"; do
    [ -n "${p:-}" ] && kill -INT "$p" 2>/dev/null || true
  done
  sleep 1
  for p in "$MS_PID" "$ADSB_S_PID" "$RID_S_PID" "$FID_PID"; do
    [ -n "${p:-}" ] && kill -KILL "$p" 2>/dev/null || true
  done
}
trap cleanup EXIT

await_multiaddr() {
  local file="$1"
  local who="$2"
  local i
  for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
    if [ -s "$file" ]; then
      local ma
      ma="$(grep -m1 '^/ip4/' "$file" || true)"
      if [ -n "$ma" ]; then
        printf '%s\n' "$ma"
        return 0
      fi
    fi
    sleep 0.5
  done
  echo "FAIL: $who multiaddr did not appear in $file" >&2
  return 1
}

echo "===> [1/7] fid-display on http://127.0.0.1:$HTTP_PORT  + tcp://127.0.0.1:$TCP_PORT"
"$BIN_DIR/fid-display" --http=127.0.0.1:$HTTP_PORT --tcp=127.0.0.1:$TCP_PORT \
  >"$EVD/fid-display.stdout" 2>"$EVD/fid-display.stderr" &
FID_PID=$!
sleep 2

echo "===> [2/7] adsb-seller (synthetic, 2 aircraft, 4 fps)"
"$BIN_DIR/adsb-seller" --mode=fixture-direct \
  --feed-source=synthetic --synth-aircraft=2 --synth-fps=4 \
  --listen=/ip4/127.0.0.1/udp/0/quic-v1 \
  >"$EVD/adsb-seller.stdout" 2>"$EVD/adsb-seller.stderr" &
ADSB_S_PID=$!
ADSB_MA="$(await_multiaddr "$EVD/adsb-seller.stdout" adsb-seller)" || exit 1
echo "    adsb-seller multiaddr: $ADSB_MA"

echo "===> [3/7] remoteid-seller (synthetic, 2 drones, 4 fps)"
"$BIN_DIR/remoteid-seller" --mode=fixture-direct \
  --synth --synth-drones=2 --synth-fps=4 --advertise-basestation-protocol \
  --listen=/ip4/127.0.0.1/udp/0/quic-v1 \
  >"$EVD/remoteid-seller.stdout" 2>"$EVD/remoteid-seller.stderr" &
RID_S_PID=$!
RID_MA="$(await_multiaddr "$EVD/remoteid-seller.stdout" remoteid-seller)" || exit 1
echo "    remoteid-seller multiaddr: $RID_MA"

echo "===> [4/7] multistream-buyer (one process, two sessions, single output sink)"
"$BIN_DIR/multistream-buyer" --mode=fixture-direct \
  --commerce-mode=registration-only \
  --listen=/ip4/127.0.0.1/udp/0/quic-v1 \
  --output=tcp:127.0.0.1:$TCP_PORT \
  --seller="role=adsb,multiaddr=$ADSB_MA,protocol=/jetvision/basestation/1.0.0" \
  --seller="role=remoteid,multiaddr=$RID_MA,protocol=/ds240/basestation/1.0.0" \
  >"$EVD/multistream-buyer.stdout" 2>"$EVD/multistream-buyer.stderr" &
MS_PID=$!

# Give the multistream-buyer time to dial both sellers, open both
# streams, and drain a few frames into fid-display. ~10s is generous
# for libp2p QUIC handshake + first-frame delivery on loopback.
sleep 10

echo "===> [5/7] curl /state.json"
curl -s "http://127.0.0.1:$HTTP_PORT/state.json" >"$EVD/state.json"
echo "    /state.json $(wc -c <"$EVD/state.json" | tr -d ' ') bytes"

echo "===> [6/7] consolidated-output invariant — exactly ONE multistream-buyer process"
# Spec asks for `pgrep -c -f 'multistream-buyer'` → expect 1. BSD
# pgrep on macOS doesn't accept `-c -f` together, so we filter
# `pgrep -fl <basename>` output: include only PIDs whose argv[0]
# basename equals `multistream-buyer` (matches the BIN_DIR/multistream-buyer
# path exactly without picking up sibling processes whose CWD/args
# contain the substring).
MS_PROC_COUNT="$(pgrep -fl multistream-buyer 2>/dev/null \
    | awk '{ split($2, parts, "/"); base = parts[length(parts)]; if (base == "multistream-buyer") print $1 }' \
    | wc -l \
    | tr -d ' ')"
echo "    multistream-buyer pid: $MS_PID  multistream-buyer process count: $MS_PROC_COUNT"
if [ "$MS_PROC_COUNT" != "1" ]; then
  echo "===> FAIL: expected exactly 1 multistream-buyer process; got $MS_PROC_COUNT" >&2
  pgrep -fl multistream-buyer >&2 || true
  exit 1
fi

echo "===> [7/7] Verify both keyspaces populated"
DRONE_COUNT=$(grep -o '"droneId"' "$EVD/state.json" | wc -l | tr -d ' ')
TRACK_COUNT=$(grep -o '"entityID"' "$EVD/state.json" | wc -l | tr -d ' ')
echo "    drones (Remote ID, green):                  $DRONE_COUNT"
echo "    normalized-tracks (ADS-B BaseStation, orange): $TRACK_COUNT"

if [ "$DRONE_COUNT" -gt 0 ] && [ "$TRACK_COUNT" -gt 0 ]; then
  echo "===> PASS: Phase 2 multistream-buyer — one process, both keyspaces populated"
  echo "===> Evidence directory: $EVD"
  exit 0
fi
echo "===> FAIL: one or both keyspaces empty (see $EVD/state.json)" >&2
exit 1
