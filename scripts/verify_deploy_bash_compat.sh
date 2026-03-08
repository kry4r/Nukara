#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

bash -n \
  deploy/deploy-local.sh \
  deploy/lib/deploy-state.sh \
  deploy/lib/change-detection.sh \
  deploy/lib/service-restart.sh \
  deploy/lib/cleanup.sh \
  deploy/lib/admin-bootstrap.sh

echo "verify_deploy_bash_compat: PASS"
