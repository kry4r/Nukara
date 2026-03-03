#!/usr/bin/env bash

# 等待服务健康检查
wait_for_health() {
  local url=$1
  local timeout=${2:-30}
  local elapsed=0

  log "Waiting for health check: $url"

  while [ $elapsed -lt $timeout ]; do
    if curl --noproxy '*' -sf "$url" > /dev/null 2>&1; then
      log "  ✓ Service healthy"
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  return 1
}

# Validate current nanobot runtime. Return non-zero if project/runtime is broken.
validate_nanobot_runtime() {
  local py_bin="$INSTALL_DIR/nanobot/.venv/bin/python"
  local context_py="$INSTALL_DIR/nanobot/nanobot/agent/context.py"
  local provider_py="$INSTALL_DIR/nanobot/nanobot/providers/litellm_provider.py"
  local project_toml="$INSTALL_DIR/nanobot/pyproject.toml"
  local project_setup="$INSTALL_DIR/nanobot/setup.py"

  [ -f "$project_toml" ] || [ -f "$project_setup" ] || return 1
  [ -x "$py_bin" ] || return 1
  [ -f "$context_py" ] || return 1
  [ -f "$provider_py" ] || return 1

  "$py_bin" -m py_compile "$context_py" >/tmp/nukara-nanobot-compile.log 2>&1 || return 1
  "$py_bin" -m py_compile "$provider_py" >/tmp/nukara-nanobot-compile.log 2>&1
}

# Rebuild nanobot virtualenv and reinstall package from source.
rebuild_nanobot_runtime() {
  [ -d "$INSTALL_DIR/nanobot" ] || err "Nanobot source missing: $INSTALL_DIR/nanobot"
  [ -f "$INSTALL_DIR/nanobot/pyproject.toml" ] || [ -f "$INSTALL_DIR/nanobot/setup.py" ] || \
    err "Nanobot source invalid (missing pyproject.toml/setup.py): $INSTALL_DIR/nanobot"
  command -v uv >/dev/null 2>&1 || err "uv is required but not found in PATH"

  log "Rebuilding nanobot virtualenv..."
  rm -rf "$INSTALL_DIR/nanobot/.venv"

  export UV_INDEX_URL="${UV_INDEX_URL:-https://mirrors.aliyun.com/pypi/simple/}"
  uv venv "$INSTALL_DIR/nanobot/.venv"
  uv pip install --python "$INSTALL_DIR/nanobot/.venv/bin/python" .
  "$INSTALL_DIR/nanobot/.venv/bin/python" -m py_compile "$INSTALL_DIR/nanobot/nanobot/agent/context.py"
  "$INSTALL_DIR/nanobot/.venv/bin/python" -m py_compile "$INSTALL_DIR/nanobot/nanobot/providers/litellm_provider.py"
}

# 重启后端服务
restart_backend_services() {
  log "Restarting backend services..."

  systemctl enable nukara-gateway nukara-proactive nukara-admin 2>/dev/null || true
  systemctl restart nukara-gateway
  systemctl restart nukara-proactive
  systemctl restart nukara-admin

  # 健康检查（gateway 实际路由为 /api/v1/gateway/health）
  wait_for_health "http://localhost:${GATEWAY_PORT:-8080}/api/v1/gateway/health" 30 || \
    err "Service health check failed: http://localhost:${GATEWAY_PORT:-8080}/api/v1/gateway/health (timeout after 30s)"
  wait_for_health "http://localhost:${ADMIN_API_PORT:-19527}/health" 30 || \
    err "Service health check failed: http://localhost:${ADMIN_API_PORT:-19527}/health (timeout after 30s)"

  # 更新部署状态
  local gateway_hash=$(calculate_hash "$INSTALL_DIR/bin/gateway")
  local proactive_hash=$(calculate_hash "$INSTALL_DIR/bin/proactive")
  update_deploy_state "" "gateway" "$gateway_hash"
  update_deploy_state "" "proactive" "$proactive_hash"

  log "Backend services restarted successfully"
}

# 重启 nanobot 服务
restart_nanobot_service() {
  log "Restarting nanobot service..."

  if ! validate_nanobot_runtime; then
    warn "Nanobot runtime validation failed; attempting automatic rebuild..."
    [ -f /tmp/nukara-nanobot-compile.log ] && warn "Last compile error: $(tail -n 1 /tmp/nukara-nanobot-compile.log)"
    (
      cd "$INSTALL_DIR/nanobot"
      rebuild_nanobot_runtime
    )
  fi

  systemctl enable nukara-nanobot 2>/dev/null || true
  systemctl restart nukara-nanobot

  # 健康检查
  if ! wait_for_health "http://localhost:8081/health" 90; then
    warn "Nanobot health check failed, dumping recent logs..."
    journalctl -u nukara-nanobot -n 120 --no-pager 2>/dev/null || true
    err "Service health check failed: http://localhost:8081/health (timeout after 90s)"
  fi

  # 更新部署状态
  local nanobot_hash=$(calculate_hash "$INSTALL_DIR/nanobot/.venv/bin/nanobot")
  update_deploy_state "" "nanobot" "$nanobot_hash"

  log "Nanobot service restarted successfully"
}

# 重载 nginx
reload_nginx() {
  log "Reloading nginx..."

  nginx -t || err "Nginx configuration test failed"
  systemctl reload nginx

  # 更新部署状态
  local web_hash=$(calculate_hash "$INSTALL_DIR/Nukara_Web/dist/index.html")
  update_deploy_state "" "web" "$web_hash"

  log "Nginx reloaded successfully"
}

# 选择性重启服务
restart_services() {
  local any_restart=false

  if [ "$REBUILD_BACKEND" = true ]; then
    restart_backend_services
    any_restart=true
  fi

  if [ "$REBUILD_NANOBOT" = true ]; then
    restart_nanobot_service
    any_restart=true
  fi

  if [ "$REBUILD_WEB" = true ]; then
    reload_nginx
    any_restart=true
  fi

  # 增量部署场景下，即使未重建 nanobot，也确保其处于可用状态
  if ! systemctl is-active --quiet nukara-nanobot 2>/dev/null; then
    warn "Nanobot service is not active; attempting recovery restart..."
    restart_nanobot_service
    any_restart=true
  fi

  if [ "$any_restart" = false ]; then
    log "No services need to be restarted"
  fi
}
