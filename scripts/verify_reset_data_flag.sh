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

require_line "deploy/deploy-local.sh" '^[[:space:]]*--reset-data\)'
require_line "deploy/deploy-local.sh" '--reset-data'
require_line "deploy/deploy-local.sh" '^confirm_reset_data\(\) \{$'
require_line "deploy/deploy-local.sh" '^reset_neo4j_auth_and_data\(\) \{$'
require_line "deploy/deploy-local.sh" '^reset_nukara_data\(\) \{$'
require_line "deploy/deploy-local.sh" 'Neo4j password mismatch detected; resetting local Neo4j auth and data'
require_line "deploy/deploy-local.sh" '/var/lib/neo4j/data/dbms/auth\*'
require_line "deploy/deploy-local.sh" 'wait_for_neo4j_ready 30 \|\|'
require_line "deploy/deploy-local.sh" 'reset_nukara_data'
require_line "docs/deployment-guide.md" 'Neo4j 认证失败时'

echo "verify_reset_data_flag: PASS"
