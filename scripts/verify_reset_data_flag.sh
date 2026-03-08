#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_line() {
  local file="$1"
  local pattern="$2"
  if ! rg -q -- "$pattern" "$ROOT_DIR/$file"; then
    echo "missing pattern in $file: $pattern" >&2
    exit 1
  fi
}

forbid_line() {
  local file="$1"
  local pattern="$2"
  if rg -q -- "$pattern" "$ROOT_DIR/$file"; then
    echo "unexpected pattern in $file: $pattern" >&2
    exit 1
  fi
}

require_line "deploy/deploy-local.sh" '^[[:space:]]*--reset-data\)'
require_line "deploy/deploy-local.sh" '--reset-data'
require_line "deploy/deploy-local.sh" '^confirm_reset_data\(\) \{$'
require_line "deploy/deploy-local.sh" '^reset_postgres_data\(\) \{$'
require_line "deploy/deploy-local.sh" '^reset_redis_data\(\) \{$'
require_line "deploy/deploy-local.sh" '^reset_nukara_data\(\) \{$'
require_line "deploy/deploy-local.sh" 'Temporal memory graph data is stored in PostgreSQL and will be removed together'
require_line "deploy/deploy-local.sh" 'reset_postgres_data'
require_line "deploy/deploy-local.sh" 'reset_redis_data'
require_line "docs/deployment-guide.md" '重建 PostgreSQL `nukara` 数据库（会一并清空时序记忆图'
forbid_line "deploy/deploy-local.sh" 'reset_qdrant_data'
forbid_line "deploy/deploy-local.sh" 'reset_neo4j_data'
forbid_line "deploy/deploy-local.sh" 'Neo4j'

echo "verify_reset_data_flag: PASS"
