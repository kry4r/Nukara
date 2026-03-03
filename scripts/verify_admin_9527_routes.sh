#!/usr/bin/env bash
set -euo pipefail

curl --noproxy '*' -fsS http://localhost:9527/ >/dev/null
curl --noproxy '*' -fsS http://localhost:9527/health >/dev/null
