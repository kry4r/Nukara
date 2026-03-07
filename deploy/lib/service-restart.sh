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

# 重启后端服务
restart_backend_services() {
  log "Restarting backend services..."

  if [ "${NUKARA_MEMORY_INFRA_ENABLED:-true}" = "true" ]; then
    systemctl enable qdrant neo4j nukara-neo4j-adapter 2>/dev/null || true
    systemctl restart qdrant
    systemctl restart neo4j
    systemctl restart nukara-neo4j-adapter
    wait_for_health "http://127.0.0.1:${NUKARA_QDRANT_HTTP_PORT:-6333}/readyz" 45 || \
      err "Service health check failed: http://127.0.0.1:${NUKARA_QDRANT_HTTP_PORT:-6333}/readyz (timeout after 45s)"
    wait_for_health "http://127.0.0.1:${NUKARA_NEO4J_ADAPTER_PORT:-17687}/health" 45 || \
      err "Service health check failed: http://127.0.0.1:${NUKARA_NEO4J_ADAPTER_PORT:-17687}/health (timeout after 45s)"
  fi

  systemctl enable nukara-gateway nukara-proactive nukara-admin 2>/dev/null || true
  systemctl restart nukara-gateway
  systemctl restart nukara-proactive
  systemctl restart nukara-admin

  wait_for_health "http://localhost:${GATEWAY_PORT:-8080}/api/v1/gateway/health" 30 || \
    err "Service health check failed: http://localhost:${GATEWAY_PORT:-8080}/api/v1/gateway/health (timeout after 30s)"
  wait_for_health "http://localhost:${ADMIN_API_PORT:-19527}/health" 30 || \
    err "Service health check failed: http://localhost:${ADMIN_API_PORT:-19527}/health (timeout after 30s)"

  local gateway_hash=$(calculate_hash "$INSTALL_DIR/bin/gateway")
  local proactive_hash=$(calculate_hash "$INSTALL_DIR/bin/proactive")
  update_deploy_state "" "gateway" "$gateway_hash"
  update_deploy_state "" "proactive" "$proactive_hash"

  log "Backend services restarted successfully"
}

# 重载 nginx
reload_nginx() {
  log "Reloading nginx..."

  nginx -t || err "Nginx configuration test failed"
  systemctl reload nginx

  local web_hash=$(calculate_hash "$INSTALL_DIR/Nukara_Web/dist/index.html")
  update_deploy_state "" "web" "$web_hash"

  log "Nginx reloaded successfully"
}

# 选择性重启服务
restart_services() {
  local any_restart=false

  if [ "$REBUILD_BACKEND" = true ] || [ "$RELOAD_CONFIG" = true ]; then
    restart_backend_services
    any_restart=true
  fi

  if [ "$REBUILD_WEB" = true ] || [ "$RELOAD_CONFIG" = true ]; then
    reload_nginx
    any_restart=true
  fi

  if [ "$any_restart" = false ]; then
    log "No services need to be restarted"
  fi
}
