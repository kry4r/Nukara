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

require_line "deploy/docker-compose.yml" '^  admin:$'
require_line "deploy/docker-compose.yml" '^  admin_frontend:$'
require_line "deploy/docker-compose.yml" 'SERVICE: admin'
require_line "deploy/docker-compose.yml" 'dockerfile: deploy/Dockerfile.frontend'
require_line "deploy/deploy.sh" 'Admin Web port'
require_line "deploy/deploy.sh" 'Admin API port'
require_line "deploy/deploy.sh" 'NUKARA_ADMIN_USERNAME='
require_line "deploy/deploy.sh" 'NUKARA_ADMIN_PASSWORD='
require_line "deploy/deploy.sh" 'NUKARA_ASTRON_API_KEY='
require_line "deploy/deploy.sh" 'Nukara_Admin_Web found locally'
require_line "scripts/smoke_full_stack_local.sh" 'NUKARA_SMOKE_ADMIN_URL'
require_line "Nukara_Admin_Web/deploy/nginx.conf" 'proxy_pass http://admin:8080/api/admin/'

echo "verify_docker_deploy_admin: PASS"
