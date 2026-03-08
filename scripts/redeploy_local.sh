#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_SCRIPT="$ROOT_DIR/deploy/deploy-local.sh"

args=("$@")
needs_root=true
has_mode=false
has_non_interactive=false
preserved_env_keys=(
  LLM_API_KEY
  LLM_API_BASE
  LLM_MODEL
  LLM_API_MODE
  DOMAIN
  HTTP_PORT
  GATEWAY_PORT
  ADMIN_WEB_PORT
  ADMIN_API_PORT
  JWT_SECRET
  POSTGRES_PASSWORD
  NUKARA_ADMIN_USERNAME
  NUKARA_ADMIN_PASSWORD
  NUKARA_SMTP_FROM_EMAIL
  NUKARA_SMTP_PASSWORD
  NUKARA_SMTP_USERNAME
  NUKARA_SMTP_HOST
  NUKARA_SMTP_PORT
  NUKARA_SMTP_FROM_NAME
  NUKARA_EMAIL_CODE_TTL_SECONDS
  DEFAULT_PROVIDER_NAME
  DEFAULT_PROVIDER_BASE_URL
  DEFAULT_PROVIDER_API_KEY
  DEFAULT_PROVIDER_MODELS
  DEFAULT_PROVIDER_PRIORITY
  DEFAULT_PROVIDER_API_MODE
  NUKARA_MEMORY_INFRA_ENABLED
  NUKARA_QDRANT_VERSION
  NUKARA_QDRANT_HTTP_PORT
  NUKARA_QDRANT_GRPC_PORT
  NUKARA_QDRANT_URL
  NUKARA_QDRANT_API_KEY
  NUKARA_QDRANT_COLLECTION
  NUKARA_QDRANT_VECTOR_SIZE
  NUKARA_NEO4J_URL
  NUKARA_NEO4J_USER
  NUKARA_NEO4J_PASSWORD
  NUKARA_NEO4J_DATABASE
  NUKARA_NEO4J_HTTP_PORT
  NUKARA_NEO4J_BOLT_PORT
  NUKARA_NEO4J_BOLT_URL
  NUKARA_NEO4J_ADAPTER_PORT
  NUKARA_EMBEDDING_MODEL
  PROACTIVE_INTERVAL
  INACTIVITY_THRESHOLD
  PROACTIVE_COOLDOWN
  NUKARA_SOURCE_ROOT
  STATE_FILE
)
sudo_env=()

for arg in "$@"; do
  case "$arg" in
    --dry-run)
      needs_root=false
      ;;
    --incremental|--full)
      has_mode=true
      ;;
    --non-interactive)
      has_non_interactive=true
      ;;
  esac
done

if [ "$has_mode" = false ]; then
  args=(--incremental "${args[@]}")
fi
if [ "$has_non_interactive" = false ]; then
  args=(--non-interactive "${args[@]}")
fi

for key in "${preserved_env_keys[@]}"; do
  if [ "${!key+x}" = "x" ]; then
    sudo_env+=("${key}=${!key}")
  fi
done

cd "$ROOT_DIR"

if [ "$needs_root" = true ] && [ "${EUID:-$(id -u)}" -ne 0 ]; then
  exec sudo "${sudo_env[@]}" bash "$DEPLOY_SCRIPT" "${args[@]}"
fi

exec bash "$DEPLOY_SCRIPT" "${args[@]}"
