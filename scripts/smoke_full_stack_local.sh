#!/usr/bin/env bash
set -euo pipefail

curl --noproxy '*' -fsS http://localhost:8080/api/v1/gateway/health >/dev/null
curl --noproxy '*' -fsS http://localhost:18081/ >/dev/null
curl --noproxy '*' -fsS http://localhost:9527/ >/dev/null
curl --noproxy '*' -fsS -u "${NUKARA_ADMIN_USERNAME}:${NUKARA_ADMIN_PASSWORD}" http://localhost:9527/api/admin/providers >/dev/null
