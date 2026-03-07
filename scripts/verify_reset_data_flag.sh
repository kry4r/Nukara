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
require_line "deploy/deploy-local.sh" '^reset_nukara_data\(\) \{$'
require_line "deploy/deploy-local.sh" 'reset_nukara_data'
require_line "docs/deployment-guide.md" '`--reset-data`'

echo "verify_reset_data_flag: PASS"
