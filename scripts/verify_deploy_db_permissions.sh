#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

require_line() {
  local file="$1"
  local pattern="$2"
  if ! rg -q "$pattern" "$ROOT_DIR/$file"; then
    echo "missing pattern in $file: $pattern" >&2
    exit 1
  fi
}

require_line "deploy/deploy-local.sh" '^run_nukara_psql\(\) \{$'
require_line "deploy/deploy-local.sh" '^repair_postgres_permissions\(\) \{$'
require_line "deploy/deploy-local.sh" 'ALTER DEFAULT PRIVILEGES FOR USER postgres IN SCHEMA public'
require_line "deploy/deploy-local.sh" 'run_nukara_psql -f "\$migration"'

require_line "deploy/deploy.sh" '^POSTGRES_USER=nukara$'

require_line "deploy/docker-compose.yml" 'POSTGRES_USER: \$\{POSTGRES_USER:-nukara\}'
require_line "deploy/docker-compose.yml" 'postgres://\$\{POSTGRES_USER:-nukara\}:\$\{POSTGRES_PASSWORD:-postgres\}@postgres:5432/nukara\?sslmode=disable'

require_line "Nukara_Backend/configs/docker-compose.dev.yml" 'NUKARA_POSTGRES_DSN: postgres://nukara:nukara@postgres:5432/nukara\?sslmode=disable'
require_line "Nukara_Backend/configs/docker-compose.dev.yml" '^      POSTGRES_USER: nukara$'
require_line "Nukara_Backend/.env.example" '^NUKARA_POSTGRES_DSN=postgres://nukara:nukara@localhost:5432/nukara\?sslmode=disable$'

echo "verify_deploy_db_permissions: PASS"
