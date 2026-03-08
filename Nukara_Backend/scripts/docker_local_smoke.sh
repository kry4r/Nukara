#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_TAG="${1:-nukara-gateway-local:dev}"
CONTAINER_NAME="${2:-nukara-gateway-local}"
PORT="${3:-18080}"
BASE_URL="http://localhost:${PORT}"
BIN_PATH="$ROOT_DIR/build/gateway-linux-amd64"
GO_BUILD_CACHE_DIR="${NUKARA_DOCKER_LOCAL_GOCACHE:-${TMPDIR:-/tmp}/nukara-go-build}"
GO_MOD_CACHE_DIR="${NUKARA_DOCKER_LOCAL_GOMODCACHE:-}"

cleanup() {
  rm -f "$BIN_PATH"
}
trap cleanup EXIT

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

mkdir -p "$ROOT_DIR/build" "$GO_BUILD_CACHE_DIR"

BUILD_ENV=(
  "GOCACHE=$GO_BUILD_CACHE_DIR"
  "CGO_ENABLED=0"
  "GOOS=linux"
  "GOARCH=amd64"
)
if [[ -n "$GO_MOD_CACHE_DIR" ]]; then
  mkdir -p "$GO_MOD_CACHE_DIR"
  BUILD_ENV+=("GOMODCACHE=$GO_MOD_CACHE_DIR")
fi

echo "[1/5] build linux gateway binary"
(
  cd "$ROOT_DIR"
  env "${BUILD_ENV[@]}" "$GO_BIN" build -o "$BIN_PATH" ./cmd/gateway/main.go
)

echo "[2/5] build runtime image ($IMAGE_TAG)"
docker build -f "$ROOT_DIR/deploy/docker/Dockerfile.runtime" -t "$IMAGE_TAG" "$ROOT_DIR"

echo "[3/5] restart container ($CONTAINER_NAME)"
docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
docker run -d --name "$CONTAINER_NAME" -p "${PORT}:8080" "$IMAGE_TAG" >/dev/null

echo "[4/5] wait for gateway health"
for i in {1..30}; do
  if curl -sS "$BASE_URL/api/v1/gateway/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "[5/5] run smoke test"
bash "$ROOT_DIR/scripts/smoke_backend.sh" "$BASE_URL"

echo "docker local smoke passed"
echo "container: $CONTAINER_NAME"
echo "url: $BASE_URL"
