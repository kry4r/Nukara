#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

large="$(bash -lc 'source "$1"; infer_qdrant_vector_size "TEXT-EMBEDDING-3-LARGE"' _ "$ROOT_DIR/deploy/lib/memory-infra.sh")"
small="$(bash -lc 'source "$1"; infer_qdrant_vector_size "MiniMax-M2"' _ "$ROOT_DIR/deploy/lib/memory-infra.sh")"

[ "$large" = "3072" ]
[ "$small" = "1536" ]

echo "verify_deploy_bash_compat: PASS"
