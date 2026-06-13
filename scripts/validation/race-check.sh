#!/usr/bin/env bash
# Race-condition validation gate — mandatory after any runtime, concurrency,
# or scheduling change. Added 2026-05-13 after a race-condition crash on a
# deployed buyer in the field. Wired from CLAUDE.md → Commands → "Race detection".
#
# Runs `go test -race ... -count=1 -timeout 180s` across the buyer/seller/
# topic/payment/delivery/edge/FID surfaces in three groups so failures
# localise cleanly. ALWAYS runs all groups; exits non-zero if any group
# fails. Equivalent single-command form (for copy/paste):
#
#   cd impl/golang && go test -race \
#       ./internal/dapp/remoteid/... ./internal/dapp/adsb/... \
#       ./internal/topic/...        ./internal/payment/... \
#       ./internal/delivery/...     ./internal/edgeapp/... ./internal/feeds/sbs/... \
#       ./cmd/remoteid-seller/...   ./cmd/adsb-seller/... ./cmd/fid-display/... \
#       ./cmd/multistream-buyer/... \
#       -count=1 -timeout 180s

set -uo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
GO_DIR="$REPO_ROOT/impl/golang"

run_group() {
  local name="$1"; shift
  printf '\n===> Race-check group %s: %s\n' "$name" "$*"
  if ( cd "$GO_DIR" && go test -race "$@" -count=1 -timeout 180s ); then
    printf 'PASS: group %s\n' "$name"
    return 0
  fi
  printf 'FAIL: group %s\n' "$name"
  return 1
}

failed=()

run_group A-protocol \
  ./internal/dapp/remoteid/... ./internal/dapp/adsb/... ./internal/dapp/sapient/... \
  ./internal/topic/... ./internal/payment/... \
  || failed+=(A-protocol)

run_group B-runtime \
  ./internal/delivery/... ./internal/edgeapp/... ./internal/feeds/sbs/... \
  ./internal/feeds/remoteid/... \
  || failed+=(B-runtime)

run_group C-cmd \
  ./cmd/remoteid-seller/... ./cmd/adsb-seller/... \
  ./cmd/fid-display/... ./cmd/multistream-buyer/... \
  ./cmd/sapient-buyer/... ./cmd/sapient-rid-seller/... ./cmd/sapient-task/... \
  ./cmd/sapient-fid-consumer/... ./cmd/sapient-agent-explorer/... ./cmd/sapient-fid-display/... \
  ./cmd/sapient-explorer/... ./cmd/sapient-jv-seller/... ./cmd/sapient-feed-replay/... \
  || failed+=(C-cmd)

printf '\n===> Race-check summary\n'
if [ ${#failed[@]} -eq 0 ]; then
  printf 'All race-check groups PASSED.\n'
  exit 0
fi
printf 'FAILED groups: %s\n' "${failed[*]}"
printf 'Race-check FAILED — do not ship the change. See per-group output above.\n'
exit 1
