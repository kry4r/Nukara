#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/configs/docker-compose.dev.yml"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  source "$ROOT_DIR/.env"
  set +a
fi

echo "deploying all services..."
docker compose -f "$COMPOSE_FILE" up -d --build

echo "gateway deployment complete"
echo "health:   http://localhost:8080/api/v1/gateway/health"
