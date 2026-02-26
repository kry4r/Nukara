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

export NANOBOT_SRC_DIR="${NANOBOT_SRC_DIR:-/Users/nidhogg/Desktop/nanobot}"

echo "[1/4] starting postgres/redis..."
docker compose -f "$COMPOSE_FILE" up -d postgres redis

echo "[2/4] waiting for postgres..."
for i in {1..30}; do
  if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "[3/4] building & starting nanobot gateway..."
docker compose -f "$COMPOSE_FILE" up -d --build nanobot

echo "[4/4] waiting for nanobot health..."
for i in {1..30}; do
  if curl -sf http://localhost:9091/health >/dev/null 2>&1; then
    echo "nanobot ready"
    break
  fi
  sleep 2
done

echo "[ready] running gateway on :8080"
cd "$ROOT_DIR"
exec go run ./cmd/gateway/main.go
