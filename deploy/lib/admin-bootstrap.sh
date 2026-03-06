#!/usr/bin/env bash

sanitize_provider_id() {
  local raw="${1:-}"
  raw="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]')"
  raw="$(printf '%s' "$raw" | sed -E 's/[^a-z0-9]+/_/g; s/^_+//; s/_+$//; s/_+/_/g')"
  printf '%s\n' "$raw"
}

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
  local provider_api_mode="${DEFAULT_PROVIDER_API_MODE:-${LLM_API_MODE:-chat_completions}}"
  local provider_id
  provider_id="$(sanitize_provider_id "$provider_name")"

  if [ -z "$admin_username" ] || [ -z "$admin_password" ]; then
    warn "Skipping provider bootstrap: admin credentials are empty"
    return 0
  fi
  if [ -z "$provider_base_url" ] || [ -z "$provider_api_key" ] || [ -z "$provider_models" ]; then
    warn "Skipping provider bootstrap: provider base_url/api_key/models are incomplete"
    return 0
  fi
  if [ -z "$provider_id" ] || [ "$provider_id" = "custom" ]; then
    warn "Skipping provider bootstrap: provider name resolves to invalid id"
    return 0
  fi

  wait_for_health "$admin_base_url/health" 30

  local payload
  payload=$(jq -nc \
    --arg name "$provider_name" \
    --arg api_key "$provider_api_key" \
    --arg base_url "$provider_base_url" \
    --arg api_mode "$provider_api_mode" \
    --arg models "$provider_models" \
    --argjson priority "$provider_priority" \
    '{
      name: $name,
      api_key: $api_key,
      base_url: $base_url,
      api_mode: $api_mode,
      models: ($models | split(",") | map(gsub("^\\s+|\\s+$";"")) | map(select(length > 0))),
      is_active: false,
      priority: $priority
    }')

  log "Bootstrapping default provider..."
  local providers_resp existing_id method endpoint response_body
  providers_resp=$(curl --noproxy '*' -fsS \
    -u "${admin_username}:${admin_password}" \
    "$admin_base_url/api/admin/providers")
  existing_id=$(printf '%s' "$providers_resp" | jq -r --arg id "$provider_id" 'map(select(.id == $id)) | .[0].id // empty')

  if [ -n "$existing_id" ]; then
    method="PUT"
    endpoint="$admin_base_url/api/admin/providers/${existing_id}"
    log "Default provider already exists; updating: $existing_id"
  else
    method="POST"
    endpoint="$admin_base_url/api/admin/providers"
    log "Default provider missing; creating: $provider_id"
  fi

  response_body=$(curl --noproxy '*' -fsS \
    -u "${admin_username}:${admin_password}" \
    -H "Content-Type: application/json" \
    -X "$method" "$endpoint" \
    -d "$payload")

  if [ -z "$existing_id" ]; then
    existing_id=$(printf '%s' "$response_body" | jq -r '.id // empty')
  fi
  if [ -z "$existing_id" ]; then
    err "Failed to resolve provider id from bootstrap response"
  fi

  curl --noproxy '*' -fsS \
    -u "${admin_username}:${admin_password}" \
    -H "Content-Type: application/json" \
    -X POST "$admin_base_url/api/admin/providers/${existing_id}/switch" \
    -d "{}" >/dev/null

  log "Default provider bootstrapped and switched active: $existing_id (mode=$provider_api_mode)"
}
