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

require_line "Nukara_Backend/scripts/docker_local_smoke.sh" 'NUKARA_DOCKER_LOCAL_GOCACHE'
require_line "Nukara_Backend/scripts/docker_local_smoke.sh" 'NUKARA_DOCKER_LOCAL_GOMODCACHE'
require_line "Nukara_Backend/scripts/docker_local_up.sh" 'NUKARA_DOCKER_LOCAL_GOCACHE'
require_line "Nukara_Backend/scripts/docker_local_up.sh" 'NUKARA_DOCKER_LOCAL_GOMODCACHE'
forbid_line "Nukara_Backend/scripts/docker_local_smoke.sh" 'GOMODCACHE=/tmp/go-mod'
forbid_line "Nukara_Backend/scripts/docker_local_up.sh" 'GOMODCACHE=/tmp/go-mod'

echo "verify_backend_docker_local_cache: PASS"
