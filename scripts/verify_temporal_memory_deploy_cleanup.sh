#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if rg -n '^[[:space:]]+(neo4j|qdrant):$' deploy/docker-compose.yml >/dev/null; then
  echo "deploy/docker-compose.yml still defines neo4j/qdrant services" >&2
  exit 1
fi

if rg -n 'NUKARA_QDRANT_|NUKARA_NEO4J_|QDRANT_PORT|NEO4J_HTTP_PORT|NEO4J_BOLT_PORT' deploy/docker-compose.yml >/dev/null; then
  echo "deploy/docker-compose.yml still exposes legacy memory infra envs" >&2
  exit 1
fi

if rg -n 'NUKARA_QDRANT_|NUKARA_NEO4J_|NUKARA_MEMORY_INFRA_ENABLED' scripts/redeploy_local.sh >/dev/null; then
  echo "scripts/redeploy_local.sh still preserves legacy memory infra envs" >&2
  exit 1
fi

if rg -n 'qdrant|neo4j' deploy/lib/service-restart.sh >/dev/null; then
  echo "deploy/lib/service-restart.sh still restarts legacy memory infra" >&2
  exit 1
fi

if rg -n 'memory-infra\.sh|NUKARA_QDRANT_|NUKARA_NEO4J_|qdrant|neo4j' deploy/deploy-local.sh >/dev/null; then
  echo "deploy/deploy-local.sh still references legacy memory infra" >&2
  exit 1
fi

if [ -e deploy/lib/memory-infra.sh ]; then
  echo "deploy/lib/memory-infra.sh still exists" >&2
  exit 1
fi

if [ -e Nukara_Backend/cmd/neo4j_adapter/main.go ] || [ -d Nukara_Backend/internal/neo4jadapter ]; then
  echo "legacy neo4j adapter code still exists" >&2
  exit 1
fi

if rg -n 'neo4j-go-driver' Nukara_Backend/go.mod Nukara_Backend/go.sum >/dev/null; then
  echo "Nukara_Backend still depends on neo4j driver" >&2
  exit 1
fi

echo 'verify_temporal_memory_deploy_cleanup: PASS'
