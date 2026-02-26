#!/bin/bash
set -euo pipefail

CONTAINER_NAME="${1:-nukara-gateway-local}"
TAIL_LINES="${2:-200}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

docker logs --tail "$TAIL_LINES" "$CONTAINER_NAME"
