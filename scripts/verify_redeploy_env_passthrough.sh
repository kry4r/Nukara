#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d -t nukara-redeploy-env)"
trap 'rm -rf "$TMP_DIR"' EXIT

cat > "$TMP_DIR/sudo" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$@"
EOF
chmod +x "$TMP_DIR/sudo"

output="$(cd "$ROOT_DIR" && PATH="$TMP_DIR:$PATH" LLM_API_KEY=test-key NUKARA_ADMIN_PASSWORD=test-pass bash scripts/redeploy_local.sh --full 2>&1 || true)"

echo "$output" | rg -q '^LLM_API_KEY=test-key$' || {
  echo "redeploy script did not pass LLM_API_KEY through sudo" >&2
  exit 1
}

echo "$output" | rg -q '^NUKARA_ADMIN_PASSWORD=test-pass$' || {
  echo "redeploy script did not pass NUKARA_ADMIN_PASSWORD through sudo" >&2
  exit 1
}

echo "$output" | rg -q '^bash$' || {
  echo "redeploy script did not invoke bash through sudo" >&2
  exit 1
}

echo "$output" | rg -q 'deploy/deploy-local.sh' || {
  echo "redeploy script did not target deploy-local.sh through sudo" >&2
  exit 1
}

echo 'verify_redeploy_env_passthrough: PASS'
