#!/usr/bin/env bash
set -euo pipefail

test -n "${NUKARA_ADMIN_USERNAME:-}" || (echo "missing admin user" && exit 1)
test -n "${NUKARA_ADMIN_PASSWORD:-}" || (echo "missing admin pass" && exit 1)
