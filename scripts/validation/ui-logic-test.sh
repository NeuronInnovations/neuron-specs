#!/usr/bin/env bash
# ui-logic-test.sh — node --test gate for the FID display's pure UI logic
# (static/logic.js). Optional: skips cleanly when node is unavailable (the Go
# gates do not depend on it).
set -uo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
if ! command -v node >/dev/null 2>&1; then
  echo "SKIP: node not available — UI logic tests not run"
  exit 0
fi
exec node --test "$ROOT/scripts/validation/ui-logic.test.mjs"
