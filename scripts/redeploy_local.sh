#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_SCRIPT="$ROOT_DIR/deploy/deploy-local.sh"

args=(--force-clean --reset-data --non-interactive "$@")
needs_root=true
for arg in "$@"; do
  if [ "$arg" = "--dry-run" ]; then
    needs_root=false
    break
  fi
done

cd "$ROOT_DIR"

if [ "$needs_root" = true ] && [ "${EUID:-$(id -u)}" -ne 0 ]; then
  exec sudo bash "$DEPLOY_SCRIPT" "${args[@]}"
fi

exec bash "$DEPLOY_SCRIPT" "${args[@]}"
