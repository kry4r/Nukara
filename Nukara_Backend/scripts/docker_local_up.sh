#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_TAG="${1:-nukara-gateway-local:stable}"
CONTAINER_NAME="${2:-nukara-gateway-local}"
HOST_PORT="${3:-8080}"
BIN_PATH="$ROOT_DIR/build/gateway-linux-amd64"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found"
  exit 1
fi

if ! command -v go >/dev/null 2>&1 && ! command -v /opt/homebrew/bin/go >/dev/null 2>&1; then
  echo "go not found"
  exit 1
fi

GO_BIN="$(command -v go || true)"
if [[ -z "$GO_BIN" ]]; then
  GO_BIN="/opt/homebrew/bin/go"
fi

mkdir -p "$ROOT_DIR/build"

echo "[1/4] build linux gateway binary"
(
  cd "$ROOT_DIR"
  GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$GO_BIN" build -o "$BIN_PATH" ./cmd/gateway/main.go
)

echo "[2/4] build runtime image ($IMAGE_TAG)"
docker build -f "$ROOT_DIR/deploy/docker/Dockerfile.runtime" -t "$IMAGE_TAG" "$ROOT_DIR"

echo "[3/4] recreate container ($CONTAINER_NAME)"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

RUN_ARGS=(
  -d
  --name "$CONTAINER_NAME"
  --restart unless-stopped
  -p "${HOST_PORT}:8080"
  -e "NUKARA_GATEWAY_PORT=8080"
)

if [[ -f "$ROOT_DIR/.env" ]]; then
  RUN_ARGS+=(--env-file "$ROOT_DIR/.env")
fi

# If compose network exists, join it and override nanobot URLs to use service names
COMPOSE_NETWORK="configs_default"
if docker network inspect "$COMPOSE_NETWORK" >/dev/null 2>&1; then
  RUN_ARGS+=(
    --network "$COMPOSE_NETWORK"
    -e "NUKARA_NANOBOT_HTTP_URL=http://nanobot:8081"
    -e "NUKARA_NANOBOT_WS_URL=ws://nanobot:8081/ws/chat"
    -e "NUKARA_POSTGRES_DSN=postgres://nukara:nukara@postgres:5432/nukara?sslmode=disable"
    -e "NUKARA_REDIS_ADDR=redis:6379"
  )
  echo "  → joined compose network ($COMPOSE_NETWORK)"
fi

docker run "${RUN_ARGS[@]}" "$IMAGE_TAG" >/dev/null

echo "[4/4] wait for health endpoint"
for i in {1..30}; do
  if curl -sS "http://localhost:${HOST_PORT}/api/v1/gateway/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "gateway is running"
echo "container: $CONTAINER_NAME"
echo "image: $IMAGE_TAG"
echo "url: http://localhost:${HOST_PORT}"
