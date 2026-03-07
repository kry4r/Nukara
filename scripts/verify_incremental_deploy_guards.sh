#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
python3 - <<'PY' "$ROOT_DIR/deploy/deploy-local.sh" "$ROOT_DIR/deploy/lib/service-restart.sh" "$ROOT_DIR/scripts/redeploy_local.sh"
from pathlib import Path
import sys

deploy_local = Path(sys.argv[1]).read_text()
service_restart = Path(sys.argv[2]).read_text()
redeploy = Path(sys.argv[3]).read_text()

old_unconditional_cleanup = '''  lock_deploy_source
  cleanup_install_residue
  REBUILD_BACKEND=true
  REBUILD_WEB=true
  RELOAD_CONFIG=true
  log "Forced full rebuild after residue cleanup"'''
if old_unconditional_cleanup in deploy_local:
    raise SystemExit('incremental deploy still contains unconditional cleanup/full-rebuild block')

required_fragments = [
    'No source or config changes detected; skipping deploy.',
    'if [ "$FORCE_CLEAN" = true ] || [ "$RESET_DATA" = true ]; then',
    'cleanup_install_residue',
    'ensure_npm_dependencies "$INSTALL_DIR/Nukara_Web" "frontend"',
    'ensure_npm_dependencies "$INSTALL_DIR/Nukara_Admin_Web" "admin web"',
]
for fragment in required_fragments:
    if fragment not in deploy_local:
        raise SystemExit(f'missing expected deploy guard fragment: {fragment}')

restart_fragments = [
    'if [ "$REBUILD_BACKEND" = true ] || [ "$RELOAD_CONFIG" = true ]; then',
    'if [ "$REBUILD_WEB" = true ] || [ "$RELOAD_CONFIG" = true ]; then',
]
for fragment in restart_fragments:
    if fragment not in service_restart:
        raise SystemExit(f'missing expected restart fragment: {fragment}')

if '--force-clean --reset-data --non-interactive' in redeploy:
    raise SystemExit('redeploy script still hardcodes destructive defaults')
if '--incremental' not in redeploy:
    raise SystemExit('redeploy script no longer defaults to incremental mode')

print('verify_incremental_deploy_guards: PASS')
PY
