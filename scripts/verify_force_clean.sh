#!/usr/bin/env bash
set -euo pipefail

bash deploy/deploy-local.sh --dry-run --force-clean | rg "Stopping old services" >/dev/null
