#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${NUKARA_SMOKE_GATEWAY_URL:-http://localhost:8080}"
WEB_URL="${NUKARA_SMOKE_WEB_URL:-http://localhost:18081}"
ADMIN_URL="${NUKARA_SMOKE_ADMIN_URL:-http://localhost:9527}"

: "${NUKARA_ADMIN_USERNAME:?missing NUKARA_ADMIN_USERNAME}"
: "${NUKARA_ADMIN_PASSWORD:?missing NUKARA_ADMIN_PASSWORD}"

curl --noproxy '*' -fsS "$GATEWAY_URL/api/v1/gateway/health" >/dev/null
curl --noproxy '*' -fsS "$WEB_URL/" >/dev/null
curl --noproxy '*' -fsS "$ADMIN_URL/" >/dev/null
curl --noproxy '*' -fsS "$ADMIN_URL/health" >/dev/null
curl --noproxy '*' -fsS -u "${NUKARA_ADMIN_USERNAME}:${NUKARA_ADMIN_PASSWORD}" "$ADMIN_URL/api/admin/providers" >/dev/null

echo "smoke_full_stack_local: PASS"
