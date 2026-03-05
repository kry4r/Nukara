#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/configs/docker-compose.dev.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  source "$ROOT_DIR/.env"
  set +a
fi

echo "[1/2] starting postgres/redis..."
docker compose -f "$COMPOSE_FILE" up -d postgres redis

echo "[2/2] waiting for postgres..."
for i in {1..30}; do
  if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "[ready] running gateway on :8080"
cd "$ROOT_DIR"
exec go run ./cmd/gateway/main.go
