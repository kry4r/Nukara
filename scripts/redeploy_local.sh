#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_SCRIPT="$ROOT_DIR/deploy/deploy-local.sh"

args=("$@")
needs_root=true
has_mode=false
has_non_interactive=false

for arg in "$@"; do
  case "$arg" in
    --dry-run)
      needs_root=false
      ;;
    --incremental|--full)
      has_mode=true
      ;;
    --non-interactive)
      has_non_interactive=true
      ;;
  esac
done

if [ "$has_mode" = false ]; then
  args=(--incremental "${args[@]}")
fi
if [ "$has_non_interactive" = false ]; then
  args=(--non-interactive "${args[@]}")
fi

cd "$ROOT_DIR"

if [ "$needs_root" = true ] && [ "${EUID:-$(id -u)}" -ne 0 ]; then
  exec sudo bash "$DEPLOY_SCRIPT" "${args[@]}"
fi

exec bash "$DEPLOY_SCRIPT" "${args[@]}"
