#!/usr/bin/env bash

sanitize_provider_id() {
  local raw="${1:-}"
  raw="$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]')"
  raw="$(printf '%s' "$raw" | sed -E 's/[^a-z0-9]+/_/g; s/^_+//; s/_+$//; s/_+/_/g')"
  printf '%s\n' "$raw"
}

admin_api_request() {
  local username="$1"
  local password="$2"
  local method="$3"
  local url="$4"
  local body="${5:-}"
  local tmp
  tmp=$(mktemp)
  local -a args=(--noproxy '*' -sS -o "$tmp" -w '%{http_code}' -u "${username}:${password}" -X "$method" "$url")
  if [ -n "$body" ]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi

  if ! ADMIN_HTTP_STATUS=$(curl "${args[@]}"); then
    ADMIN_HTTP_STATUS="000"
    ADMIN_HTTP_BODY="$(cat "$tmp" 2>/dev/null || true)"
    rm -f "$tmp"
    return 1
  fi
  ADMIN_HTTP_BODY="$(cat "$tmp")"
  rm -f "$tmp"
  return 0
}

wait_for_admin_provider_api() {
  local username="$1"
  local password="$2"
  local base_url="$3"
  local timeout="${4:-45}"
  local elapsed=0
  local url="${base_url}/api/admin/providers"

  log "Waiting for admin provider API readiness: $url"
  while [ "$elapsed" -lt "$timeout" ]; do
    if admin_api_request "$username" "$password" GET "$url" && [ "$ADMIN_HTTP_STATUS" = "200" ]; then
      log "  ✓ Admin provider API ready"
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  err "Admin provider API readiness failed: status=${ADMIN_HTTP_STATUS:-000} body=${ADMIN_HTTP_BODY:-}"
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

  wait_for_admin_provider_api "$admin_username" "$admin_password" "$admin_base_url" 45

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
  admin_api_request "$admin_username" "$admin_password" GET "$admin_base_url/api/admin/providers" || \
    err "Failed to query providers: ${ADMIN_HTTP_BODY:-curl error}"
  [ "$ADMIN_HTTP_STATUS" = "200" ] || err "Failed to query providers: status=$ADMIN_HTTP_STATUS body=$ADMIN_HTTP_BODY"

  local providers_resp existing_id method endpoint response_body
  providers_resp="$ADMIN_HTTP_BODY"
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

  admin_api_request "$admin_username" "$admin_password" "$method" "$endpoint" "$payload" || \
    err "Provider bootstrap request failed: ${ADMIN_HTTP_BODY:-curl error}"
  case "$ADMIN_HTTP_STATUS" in
    200|201)
      response_body="$ADMIN_HTTP_BODY"
      ;;
    *)
      err "Provider bootstrap failed: status=$ADMIN_HTTP_STATUS body=$ADMIN_HTTP_BODY"
      ;;
  esac

  if [ -z "$existing_id" ]; then
    existing_id=$(printf '%s' "$response_body" | jq -r '.id // empty')
  fi
  if [ -z "$existing_id" ]; then
    err "Failed to resolve provider id from bootstrap response: $response_body"
  fi

  admin_api_request "$admin_username" "$admin_password" POST "$admin_base_url/api/admin/providers/${existing_id}/switch" '{}' || \
    err "Provider switch request failed: ${ADMIN_HTTP_BODY:-curl error}"
  case "$ADMIN_HTTP_STATUS" in
    200|204) ;;
    *) err "Provider switch failed: status=$ADMIN_HTTP_STATUS body=$ADMIN_HTTP_BODY" ;;
  esac

  log "Default provider bootstrapped and switched active: $existing_id (mode=$provider_api_mode)"
}
