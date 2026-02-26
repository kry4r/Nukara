#!/bin/bash
set -euo pipefail

CONTAINER_NAME="${1:-nukara-gateway-local}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

docker ps --filter "name=^${CONTAINER_NAME}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
