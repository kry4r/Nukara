#!/usr/bin/env bash

cleanup_pre_deploy() {
  local dry_run=${1:-false}
  local services=(nukara-gateway nukara-proactive nukara-admin)
  local ports=(80 8080 9527 19527)

  log "Stopping old services..."
  for service in "${services[@]}"; do
    if [ "$dry_run" = true ]; then
      log "  [dry-run] systemctl stop $service"
      continue
    fi
    systemctl stop "$service" 2>/dev/null || true
  done

  log "Cleaning up stale ports: ${ports[*]}"
  for port in "${ports[@]}"; do
    local pids=""
    if command -v lsof >/dev/null 2>&1; then
      pids=$(lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || true)
    elif command -v ss >/dev/null 2>&1; then
      pids=$(ss -ltnp "sport = :$port" 2>/dev/null | awk -F'pid=' 'NR>1 && NF>1 {split($2,a,","); print a[1]}' | sort -u)
    fi

    [ -z "$pids" ] && continue
    for pid in $pids; do
      if [ "$dry_run" = true ]; then
        log "  [dry-run] kill $pid (port $port)"
        continue
      fi
      kill "$pid" 2>/dev/null || true
      sleep 1
      if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null || true
      fi
    done
  done
}
