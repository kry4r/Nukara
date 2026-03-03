#!/usr/bin/env bash

bootstrap_default_provider() {
  local admin_port="${ADMIN_API_PORT:-19527}"
  local admin_base_url="http://127.0.0.1:${admin_port}"
  local admin_username="${NUKARA_ADMIN_USERNAME:-}"
  local admin_password="${NUKARA_ADMIN_PASSWORD:-}"
  local provider_name="${DEFAULT_PROVIDER_NAME:-astron}"
  local provider_base_url="${DEFAULT_PROVIDER_BASE_URL:-${LLM_API_BASE:-}}"
  local provider_api_key="${DEFAULT_PROVIDER_API_KEY:-${LLM_API_KEY:-}}"
  local provider_models="${DEFAULT_PROVIDER_MODELS:-${LLM_MODEL:-}}"
  local provider_priority="${DEFAULT_PROVIDER_PRIORITY:-1}"

  if [ -z "$admin_username" ] || [ -z "$admin_password" ]; then
    warn "Skipping provider bootstrap: admin credentials are empty"
    return 0
  fi
  if [ -z "$provider_base_url" ] || [ -z "$provider_api_key" ] || [ -z "$provider_models" ]; then
    warn "Skipping provider bootstrap: provider base_url/api_key/models are incomplete"
    return 0
  fi

  wait_for_health "$admin_base_url/health" 30

  local payload
  payload=$(jq -nc \
    --arg name "$provider_name" \
    --arg api_key "$provider_api_key" \
    --arg base_url "$provider_base_url" \
    --arg models "$provider_models" \
    --argjson priority "$provider_priority" \
    '{
      name: $name,
      api_key: $api_key,
      base_url: $base_url,
      models: ($models | split(",") | map(gsub("^\\s+|\\s+$";"")) | map(select(length > 0))),
      is_active: false,
      priority: $priority
    }')

  log "Bootstrapping default provider..."
  local create_resp
  create_resp=$(curl --noproxy '*' -fsS \
    -u "${admin_username}:${admin_password}" \
    -H "Content-Type: application/json" \
    -X POST "$admin_base_url/api/admin/providers" \
    -d "$payload")

  local provider_id
  provider_id=$(printf '%s' "$create_resp" | jq -r '.id // empty')
  if [ -z "$provider_id" ]; then
    err "Failed to parse provider id from bootstrap response"
  fi

  curl --noproxy '*' -fsS \
    -u "${admin_username}:${admin_password}" \
    -X POST "$admin_base_url/api/admin/providers/${provider_id}/switch" >/dev/null

  log "Default provider bootstrapped and switched active: $provider_id"
}
