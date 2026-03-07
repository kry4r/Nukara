#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_FILE="$(mktemp -t nukara-redeploy-safe-defaults)"
CURRENT_COMMIT="$(git -C "$ROOT_DIR" rev-parse HEAD)"

cleanup() {
  rm -f "$STATE_FILE"
}
trap cleanup EXIT

cat > "$STATE_FILE" <<EOF
{
  "last_commit": "$CURRENT_COMMIT",
  "last_deploy_time": "",
  "services": {
    "gateway": {"binary_hash": "", "last_restart": ""},
    "proactive": {"binary_hash": "", "last_restart": ""},
    "web": {"binary_hash": "", "last_restart": ""}
  }
}
EOF

output="$(cd "$ROOT_DIR" && STATE_FILE="$STATE_FILE" bash scripts/redeploy_local.sh --dry-run 2>&1)"

if echo "$output" | rg -q 'Stopping old services|Resetting Nukara application data|Full deployment mode'; then
  echo "redeploy script still triggers destructive cleanup during default dry-run" >&2
  exit 1
fi

echo "$output" | rg -q 'No changes detected - nothing to deploy' || {
  echo "redeploy script did not no-op on unchanged dry-run" >&2
  exit 1
}

if echo "$output" | rg -q 'Stopping old services|Resetting Nukara application data'; then
  echo "redeploy script still triggers destructive cleanup during default dry-run" >&2
  exit 1
fi

echo "verify_redeploy_safe_defaults: PASS"
