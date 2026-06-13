#!/usr/bin/env bash
# Local SAPIENT Remote ID demo (Neuron reverse-connect topology):
#
#   DS240 simulator ──MQTT──▶ neuron-rid-bridge ──SAPIENT protobuf feed (4-byte LE)──▶
#     sapient-rid-seller ──(seller DIALS buyer)──p2p──▶ sapient-buyer (generic Buyer Proxy)
#       ──SAPIENT edge (4-byte LE protobuf)──▶ sapient-fid-consumer (018 display)
#         ──remote-id TaggedFrame (TCP JSONL)──▶ fid-display (map)
#
# FR-S90 split: the Buyer Proxy is rid-blind and only forwards SapientMessages;
# the fid-consumer is the sole component that parses rid.* (→ map + CoT).
# Fully isolated from the public reference demo: distinct ports, additive binaries,
# fid-display unchanged, /ds240/* untouched. Simulator-driven — no hardware.
#
# Usage:
#   scripts/demo/sapient-rid-demo.sh                       # interactive: run until Ctrl-C
#   scripts/demo/sapient-rid-demo.sh --check               # CI/proof: assert a drone, exit 0/1
#   scripts/demo/sapient-rid-demo.sh --cot-output <file>   # also emit CoT XML; assert ≥1 event, exit 0/1
#   scripts/demo/sapient-rid-demo.sh --check-task-stop-start  # exercise the SAPIENT control plane (STOP/START + StatusReport)
#   scripts/demo/sapient-rid-demo.sh --register            # register the seller's EIP-8004 Agent Card (local/simulated) + show the explorer table
#   scripts/demo/sapient-rid-demo.sh --friendly            # CoT demo profile: classify targets friendly (a-f-A) + node_id provenance; assert + exit
#   scripts/demo/sapient-rid-demo.sh --sapient-display     # rich SAPIENT UI (cmd/sapient-fid-display, http://127.0.0.1:8193); implies --register
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GO_DIR="$ROOT/impl/golang"
BRIDGE_DIR="$ROOT/docs/rid-to-sapient/neuron-rid-bridge"
SIM_DIR="$ROOT/docs/rid-to-sapient/ds240-simulator"

# Ports — deliberately distinct from the public demo.
FID_TCP="127.0.0.1:19191"            # fid-consumer -> fid-display TaggedFrame JSONL
FID_HTTP="127.0.0.1:8192"            # browser
BUYER_LISTEN="/ip4/127.0.0.1/udp/19192/quic-v1"
SAPIENT_EDGE="127.0.0.1:19193"       # Buyer Proxy -> fid-consumer SAPIENT edge (4-byte LE protobuf)
SFID_TCP="127.0.0.1:19194"           # fid-consumer -> sapient-fid-display rich sapient-track JSONL
SFID_HTTP="127.0.0.1:8193"           # rich SAPIENT UI (browser)
MQTT="127.0.0.1:21883"
SAPIENT_ADDR="127.0.0.1:29999"
LAT="50.1027"; LON="-5.6705"         # Land's End — the simulator drone orbit centre

# --- arg parsing (additive; --check stays byte-identical) ---
CHECK=0; TASK=0; COT_FILE=""; REGISTER=0; FRIENDLY=0; SAPIENT_DISPLAY=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)                 CHECK=1; shift;;
    --check-task-stop-start) TASK=1; shift;;
    --cot-output)            COT_FILE="${2:?--cot-output needs a file path}"; shift 2;;
    --register)              REGISTER=1; shift;;
    --friendly)              FRIENDLY=1; shift;;
    --sapient-display)       SAPIENT_DISPLAY=1; shift;;
    *) echo "unknown arg: $1" >&2; exit 2;;
  esac
done
# The rich UI needs the agent evidence → registration comes with it.
[[ "$SAPIENT_DISPLAY" == "1" ]] && REGISTER=1

TMP="$(mktemp -d)"; PIDS=()
CONTROL_LANE="$TMP/control.ndjson"
# --friendly needs CoT output to assert against; auto-enable one if not given.
[[ "$FRIENDLY" == "1" && -z "$COT_FILE" ]] && COT_FILE="$TMP/cot-friendly.xml"
# --cot-output (incl. the auto-enabled friendly one) runs as an assert-and-exit check.
[[ -n "$COT_FILE" && "$TASK" == "0" ]] && CHECK=1
cleanup(){ set +e; for p in "${PIDS[@]:-}"; do kill "$p" 2>/dev/null; done; wait 2>/dev/null; rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

say(){ printf '\n\033[1;36m[demo]\033[0m %s\n' "$*"; }
fail(){ printf '\n\033[1;31m[demo FAIL]\033[0m %s\n' "$*" >&2
        for f in bridge seller buyer consumer fid sfid sim; do [[ -f "$TMP/$f.log" ]] && { echo "--- $f.log ---"; tail -8 "$TMP/$f.log"; }; done
        exit 1; }

wait_port(){ # host:port timeout_s
  local hp="$1" t="${2:-15}" end; end=$(( $(date +%s) + t ))
  local h="${hp%:*}" p="${hp##*:}"
  while [[ $(date +%s) -lt $end ]]; do nc -z "$h" "$p" 2>/dev/null && return 0; sleep 0.2; done
  return 1
}

say "building binaries ..."
( cd "$GO_DIR" && go build -o "$TMP/fid-display" ./cmd/fid-display \
              && go build -o "$TMP/sapient-buyer" ./cmd/sapient-buyer \
              && go build -o "$TMP/sapient-fid-consumer" ./cmd/sapient-fid-consumer \
              && go build -o "$TMP/sapient-rid-seller" ./cmd/sapient-rid-seller ) || fail "go build (impl/golang)"
[[ "$TASK" == "1" ]] && { ( cd "$GO_DIR" && go build -o "$TMP/sapient-task" ./cmd/sapient-task ) || fail "build sapient-task"; }
[[ "$REGISTER" == "1" ]] && { ( cd "$GO_DIR" && go build -o "$TMP/sapient-agent-explorer" ./cmd/sapient-agent-explorer ) || fail "build sapient-agent-explorer"; }
[[ "$SAPIENT_DISPLAY" == "1" ]] && { ( cd "$GO_DIR" && go build -o "$TMP/sapient-fid-display" ./cmd/sapient-fid-display ) || fail "build sapient-fid-display"; }
( cd "$BRIDGE_DIR" && go build -o "$TMP/neuron-rid-bridge" ./cmd/neuron-rid-bridge ) || fail "bridge build"

if [[ ! -x "$SIM_DIR/.venv/bin/python" ]]; then
  say "creating DS240 simulator venv ..."
  ( cd "$SIM_DIR" && uv venv .venv --python 3.13 >/dev/null 2>&1 && uv pip install --python .venv/bin/python -e . >/dev/null 2>&1 ) \
    || ( cd "$SIM_DIR" && python3 -m venv .venv && .venv/bin/pip install -e . >/dev/null 2>&1 ) || fail "simulator venv setup"
fi

say "starting fid-display (map) on http://$FID_HTTP ..."
"$TMP/fid-display" --tcp "$FID_TCP" --http "$FID_HTTP" --lat "$LAT" --lon "$LON" --zoom 12 --evict 10m >"$TMP/fid.log" 2>&1 &
PIDS+=($!)
wait_port "$FID_TCP" 15 || fail "fid-display TCP not up"
wait_port "$FID_HTTP" 15 || fail "fid-display HTTP not up"

if [[ "$SAPIENT_DISPLAY" == "1" ]]; then
  say "starting sapient-fid-display (rich SAPIENT UI) on http://$SFID_HTTP ..."
  "$TMP/sapient-fid-display" --tcp "$SFID_TCP" --http "$SFID_HTTP" --lat "$LAT" --lon "$LON" --zoom 13 --evict 10m >"$TMP/sfid.log" 2>&1 &
  PIDS+=($!)
  wait_port "$SFID_TCP" 15 || fail "sapient-fid-display TCP not up"
  wait_port "$SFID_HTTP" 15 || fail "sapient-fid-display HTTP not up"
fi

say "starting sapient-buyer (generic Buyer Proxy; listens, serves the SAPIENT edge) ..."
"$TMP/sapient-buyer" --listen "$BUYER_LISTEN" --sapient-edge "$SAPIENT_EDGE" >"$TMP/buyer.out" 2>"$TMP/buyer.log" &
PIDS+=($!)
BUYER_MA=""; end=$(( $(date +%s) + 15 ))
while [[ $(date +%s) -lt $end ]]; do
  BUYER_MA="$(grep -m1 '/p2p/' "$TMP/buyer.out" 2>/dev/null || true)"
  [[ -n "$BUYER_MA" ]] && break; sleep 0.2
done
[[ -n "$BUYER_MA" ]] || fail "buyer did not print a multiaddr"
say "buyer multiaddr: $BUYER_MA"

say "starting sapient-fid-consumer (dials the SAPIENT edge; renders to fid-display) ..."
wait_port "$SAPIENT_EDGE" 15 || fail "buyer SAPIENT edge not up"
CONSUMER_ARGS=(--edge "$SAPIENT_EDGE" --output "tcp:$FID_TCP")
[[ -n "$COT_FILE" ]] && CONSUMER_ARGS+=(--cot-output "file:$COT_FILE")
[[ "$FRIENDLY" == "1" ]] && CONSUMER_ARGS+=(--cot-affiliation friendly --cot-provenance)
[[ "$SAPIENT_DISPLAY" == "1" ]] && CONSUMER_ARGS+=(--sapient-output "tcp:$SFID_TCP" --agent-evidence "$TMP/agents/seller.json")
"$TMP/sapient-fid-consumer" "${CONSUMER_ARGS[@]}" >"$TMP/consumer.log" 2>&1 &
PIDS+=($!)

say "starting neuron-rid-bridge (SAPIENT protobuf feed [4-byte LE] on $SAPIENT_ADDR) ..."
"$TMP/neuron-rid-bridge" --mqtt-listen "$MQTT" --sbs-listen ":20003" --sapient-listen ":29999" \
    --sapient-format protobuf --log-level warn >"$TMP/bridge.log" 2>&1 &
PIDS+=($!)
wait_port "$SAPIENT_ADDR" 15 || fail "bridge SAPIENT feed not up"

say "starting DS240 simulator (publishing OpenDroneID over MQTT) ..."
PYTHONPATH="$SIM_DIR" "$SIM_DIR/.venv/bin/python" -m ds240_simulator.simulator \
    --broker "${MQTT%:*}" --port "${MQTT##*:}" --rate-hz 4 --duration 0 --log-level WARNING >"$TMP/sim.log" 2>&1 &
PIDS+=($!)

say "starting sapient-rid-seller (DIALS the buyer; sources the bridge) ..."
SELLER_ARGS=(--bridge-addr "$SAPIENT_ADDR" --buyer "$BUYER_MA")
if [[ "$TASK" == "1" ]]; then
  SELLER_ARGS+=(--control-lane "file:$CONTROL_LANE" --session-id hldmm --feed-source synthetic --status-interval 2s)
fi
if [[ "$REGISTER" == "1" ]]; then
  mkdir -p "$TMP/agents"
  SELLER_ARGS+=(--register --agent-card-out "$TMP/seller-card.json" --registry-out "$TMP/agents/seller.json")
  # Guarded real-EVM registry passthrough (the gated testnet window). Default
  # (env unset) keeps the byte-identical local/simulated path.
  if [[ "${SAPIENT_DEMO_REGISTRY_BACKEND:-}" == "evm" ]]; then
    [[ -n "${SAPIENT_DEMO_REGISTRY_ADDRESS:-}" ]] || fail "SAPIENT_DEMO_REGISTRY_BACKEND=evm needs SAPIENT_DEMO_REGISTRY_ADDRESS"
    SELLER_ARGS+=(--registry-backend evm --registry-address "$SAPIENT_DEMO_REGISTRY_ADDRESS")
    [[ -n "${SAPIENT_DEMO_RPC_URL:-}" ]] && SELLER_ARGS+=(--rpc-url "$SAPIENT_DEMO_RPC_URL")
  fi
fi
"$TMP/sapient-rid-seller" "${SELLER_ARGS[@]}" >"$TMP/seller.log" 2>&1 &
PIDS+=($!)

say "waiting for a drone to reach the map ..."
DRONES=0; end=$(( $(date +%s) + 30 ))
while [[ $(date +%s) -lt $end ]]; do
  curl -s "http://$FID_HTTP/state.json" -o "$TMP/state.json" 2>/dev/null || true
  DRONES="$(python3 -c "import json;print(len(json.load(open('$TMP/state.json')).get('drones',[])))" 2>/dev/null || echo 0)"
  [[ "${DRONES:-0}" -ge 1 ]] && break; sleep 0.5
done

[[ "${DRONES:-0}" -ge 1 ]] || fail "no drone reached the map within 30s"
say "✅ ${DRONES} drone(s) on the map from SAPIENT data:"
python3 -c "import json;d=json.load(open('$TMP/state.json'));
for x in d.get('drones',[]): print('   drone', x.get('droneId'), 'lat', x.get('lat'), 'lon', x.get('lon'), 'src', x.get('frameSource'))
for o in d.get('operators',[]): print('   pilot', o.get('operatorId'), 'lat', o.get('lat'), 'lon', o.get('lon'))" 2>/dev/null || true

# --- Agent Card registration (EIP-8004 evidence; local/simulated) ---
if [[ "$REGISTER" == "1" ]]; then
  end=$(( $(date +%s) + 8 ))
  while [[ $(date +%s) -lt $end ]]; do [[ -s "$TMP/agents/seller.json" ]] && break; sleep 0.3; done
  [[ -s "$TMP/agents/seller.json" ]] || fail "seller wrote no agent evidence ($TMP/agents/seller.json)"
  grep -q '/sapient/detection/2.0.0' "$TMP/seller-card.json" || fail "agent card missing /sapient/detection/2.0.0"
  grep -q '"neuron.rid/1"' "$TMP/seller-card.json" || fail "agent card missing the neuron.rid/1 extension"
  if [[ "${SAPIENT_DEMO_REGISTRY_BACKEND:-}" == "evm" ]]; then
    say "✅ seller Agent Card registered (on-chain, evm backend). Explorer view:"
  else
    say "✅ seller Agent Card registered (local/simulated). Explorer view:"
  fi
  "$TMP/sapient-agent-explorer" --dir "$TMP/agents" | sed 's/^/   /'
fi

# --- Rich SAPIENT UI assertion (sapient-fid-display state) ---
if [[ "$SAPIENT_DISPLAY" == "1" ]]; then
  say "waiting for an enriched sapient track on http://$SFID_HTTP ..."
  SOK=0; end=$(( $(date +%s) + 25 ))
  while [[ $(date +%s) -lt $end ]]; do
    curl -s "http://$SFID_HTTP/state.json" -o "$TMP/sapient-state.json" 2>/dev/null || true
    if python3 - "$TMP/sapient-state.json" <<'PYEOF' 2>/dev/null
import json, sys
tracks = json.load(open(sys.argv[1])).get("tracks", [])
t = tracks[0] if tracks else {}
ok = (bool((t.get("agent") or {}).get("agentId"))          # EIP-8004 identity arrived
      and bool((t.get("cot") or {}).get("type"))           # CoT metadata present
      and (t.get("rf") or {}).get("rssiDbm") is not None   # RF signal present
      and (t.get("rid") or {}).get("operatorLat") is not None)  # operator/pilot present
sys.exit(0 if ok else 1)
PYEOF
    then SOK=1; break; fi
    sleep 0.5
  done
  [[ "$SOK" == "1" ]] || fail "no enriched sapient track (agent+cot+rf+operator) on $SFID_HTTP within 25s"
  say "✅ rich SAPIENT track live (agent + CoT + RF + operator). Track summary:"
  python3 - "$TMP/sapient-state.json" <<'PYEOF' | sed 's/^/   /'
import json, sys
t = json.load(open(sys.argv[1]))["tracks"][0]
a, c, rf, rid = t.get("agent") or {}, t.get("cot") or {}, t.get("rf") or {}, t.get("rid") or {}
cls = t.get("classification") or {}
print(f"uid {t.get('uid')}  class {cls.get('type','—')}({cls.get('confidence','—')})  feed {t.get('feedSource','—')}  wire {t.get('wire','—')}")
print(f"cot {c.get('type','—')}/{c.get('how','—')} affiliation={c.get('affiliation','—')} demoProfile={c.get('demoProfile', False)}")
print(f"rf rssi {rf.get('rssiDbm','—')} dBm  freq {rf.get('frequencyHz','—')} Hz  ch {rf.get('channel','—')}  {rf.get('transport','—')}")
print(f"agent id {a.get('agentId','—')}  evm {a.get('sellerEVM','—')}  sim {a.get('simulated','—')}  protocol {a.get('protocol','—')}")
print(f"operator {rid.get('operatorId','—')} @ {rid.get('operatorLat','—')},{rid.get('operatorLon','—')}")
PYEOF
  if [[ "$FRIENDLY" == "1" ]]; then
    grep -q '"type":"a-f-A"' "$TMP/sapient-state.json" || fail "friendly profile: sapient track cot.type is not a-f-A"
    say "✅ sapient track carries the friendly demo CoT profile (a-f-A)"
  fi
  say "rich SAPIENT UI:  http://$SFID_HTTP/"
fi

# --- CoT assertion ---
if [[ -n "$COT_FILE" ]]; then
  end=$(( $(date +%s) + 10 ))
  while [[ $(date +%s) -lt $end ]]; do grep -q '<event' "$COT_FILE" 2>/dev/null && break; sleep 0.3; done
  grep -q '<event' "$COT_FILE" 2>/dev/null || fail "no CoT <event> written to $COT_FILE"
  say "✅ CoT events written to $COT_FILE ($(grep -c '<event' "$COT_FILE") events):"
  grep -m1 '<event' "$COT_FILE" | sed 's/^/   /'
  if [[ "$FRIENDLY" == "1" ]]; then
    grep -q 'a-f-A' "$COT_FILE" 2>/dev/null || fail "friendly profile: no a-f-A CoT event in $COT_FILE"
    say "✅ friendly CoT events present (type=a-f-A; library default stays a-u-A)"
  fi
fi

# --- Tasking (CONTROL_STOP / CONTROL_START) assertion ---
if [[ "$TASK" == "1" ]]; then
  NID="$(grep -o 'node_id=[0-9a-fA-F-]*' "$TMP/seller.log" | head -1 | cut -d= -f2)"
  [[ -n "$NID" ]] || fail "could not read seller ASM node_id from seller.log"
  say "seller ASM node_id=$NID"

  # Wait for at least one StatusReport (feedSource) on the auditable lane.
  end=$(( $(date +%s) + 8 ))
  while [[ $(date +%s) -lt $end ]]; do grep -q 'neuron.feedSource=synthetic' "$CONTROL_LANE" 2>/dev/null && break; sleep 0.3; done
  grep -q 'neuron.feedSource=synthetic' "$CONTROL_LANE" 2>/dev/null || fail "no StatusReport neuron.feedSource=synthetic on the auditable lane"
  say "✅ StatusReport with neuron.feedSource=synthetic present on the auditable lane"

  say "issuing CONTROL_STOP via sapient-task ..."
  "$TMP/sapient-task" --lane "file:$CONTROL_LANE" --asm-node-id "$NID" --from-node-id hldmm --control stop --wait 6s >"$TMP/task-stop.log" 2>&1 || { cat "$TMP/task-stop.log"; fail "STOP task errored"; }
  grep -q 'ACCEPTED' "$TMP/task-stop.log" || fail "STOP not ACCEPTED: $(cat "$TMP/task-stop.log")"
  say "✅ CONTROL_STOP accepted: $(grep TaskAck "$TMP/task-stop.log")"

  say "issuing CONTROL_START via sapient-task ..."
  "$TMP/sapient-task" --lane "file:$CONTROL_LANE" --asm-node-id "$NID" --from-node-id hldmm --control start --wait 6s >"$TMP/task-start.log" 2>&1 || { cat "$TMP/task-start.log"; fail "START task errored"; }
  grep -q 'ACCEPTED' "$TMP/task-start.log" || fail "START not ACCEPTED: $(cat "$TMP/task-start.log")"
  say "✅ CONTROL_START accepted: $(grep TaskAck "$TMP/task-start.log")"
  say "control-plane check passed (auditable lane: $CONTROL_LANE)"
fi

if [[ "$CHECK" == "1" || "$TASK" == "1" ]]; then say "check passed"; exit 0; fi

say "open the map:  http://$FID_HTTP/   (drone orbiting + pilot at Land's End)"
say "Ctrl-C to stop."
wait
