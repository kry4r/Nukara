#!/usr/bin/env bash

STATE_FILE="${STATE_FILE:-/opt/nukara/.deploy-state.json}"

# 初始化部署状态文件
init_deploy_state() {
  if [ ! -f "$STATE_FILE" ]; then
    mkdir -p "$(dirname "$STATE_FILE")"
    cat > "$STATE_FILE" <<EOF
{
  "last_commit": "",
  "last_deploy_time": "",
  "services": {
    "gateway": {"binary_hash": "", "last_restart": ""},
    "proactive": {"binary_hash": "", "last_restart": ""},
    "web": {"binary_hash": "", "last_restart": ""}
  }
}
EOF
    log "Initialized deploy state file"
  fi
}

# 获取上次部署的 commit
get_last_commit() {
  if [ -f "$STATE_FILE" ]; then
    jq -r '.last_commit' "$STATE_FILE"
  else
    echo ""
  fi
}

# 更新部署状态
update_deploy_state() {
  local commit=$1
  local service=$2
  local binary_hash=$3

  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  if [ -z "$service" ]; then
    # 更新全局状态
    jq --arg commit "$commit" --arg time "$timestamp" \
      '.last_commit = $commit | .last_deploy_time = $time' \
      "$STATE_FILE" > "$STATE_FILE.tmp" && mv "$STATE_FILE.tmp" "$STATE_FILE"
  else
    # 更新服务状态
    jq --arg service "$service" --arg hash "$binary_hash" --arg time "$timestamp" \
      '.services[$service].binary_hash = $hash | .services[$service].last_restart = $time' \
      "$STATE_FILE" > "$STATE_FILE.tmp" && mv "$STATE_FILE.tmp" "$STATE_FILE"
  fi
}

# 计算文件 hash
calculate_hash() {
  local file=$1
  if [ -f "$file" ]; then
    sha256sum "$file" | awk '{print $1}'
  else
    echo ""
  fi
}
