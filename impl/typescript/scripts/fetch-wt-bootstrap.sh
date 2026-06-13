#!/usr/bin/env bash
# 2a-wt — scp bootstrap-wt.json from the VPS WebTransport seller into
# the local Vite public dir so the browser can fetch it same-origin.
#
# Requires the Go seller to be running on the VPS with --bootstrap-out
# pointing at $REMOTE_BOOTSTRAP_PATH.
#
# Env vars:
#   VPS_USER              default: root
#   VPS_HOST              REQUIRED — your seller host (no default)
#   VPS_SSH_KEY           REQUIRED — private key for that host (no default)
#   REMOTE_BOOTSTRAP_PATH default: /opt/bootstrap-wt.json
#   LOCAL_BOOTSTRAP_PATH  default: examples/browser-demo-wt/public/bootstrap-wt.json

set -euo pipefail

VPS_USER="${VPS_USER:-root}"
VPS_HOST="${VPS_HOST:?set VPS_HOST to your WebTransport seller host}"
VPS_SSH_KEY="${VPS_SSH_KEY:?set VPS_SSH_KEY to the private key for $VPS_HOST}"
REMOTE_BOOTSTRAP_PATH="${REMOTE_BOOTSTRAP_PATH:-/opt/bootstrap-wt.json}"
LOCAL_BOOTSTRAP_PATH="${LOCAL_BOOTSTRAP_PATH:-examples/browser-demo-wt/public/bootstrap-wt.json}"

if [[ ! -f "$VPS_SSH_KEY" ]]; then
  echo "fetch-wt-bootstrap: SSH key $VPS_SSH_KEY not found" >&2
  echo "set VPS_SSH_KEY to an existing private key, or create $VPS_SSH_KEY" >&2
  exit 1
fi

mkdir -p "$(dirname "$LOCAL_BOOTSTRAP_PATH")"
scp -i "$VPS_SSH_KEY" \
    -o StrictHostKeyChecking=accept-new \
    "$VPS_USER@$VPS_HOST:$REMOTE_BOOTSTRAP_PATH" \
    "$LOCAL_BOOTSTRAP_PATH"

echo "fetch-wt-bootstrap: copied $VPS_USER@$VPS_HOST:$REMOTE_BOOTSTRAP_PATH -> $LOCAL_BOOTSTRAP_PATH"
