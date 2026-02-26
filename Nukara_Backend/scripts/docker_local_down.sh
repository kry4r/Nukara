#!/bin/bash
set -euo pipefail

CONTAINER_NAME="${1:-nukara-gateway-local}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

if docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1; then
  echo "container removed: $CONTAINER_NAME"
else
  echo "container not found: $CONTAINER_NAME"
fi
