#!/usr/bin/env bash

# 检测变更范围
detect_changes() {
  local last_commit=$(get_last_commit)
  local current_commit=$(git rev-parse HEAD)

  # 如果是首次部署或强制全量部署
  if [ -z "$last_commit" ] || [ "$FORCE_FULL_DEPLOY" = true ]; then
    REBUILD_BACKEND=true
    REBUILD_NANOBOT=true
    REBUILD_WEB=true
    RELOAD_CONFIG=true
    log "Full deployment mode"
    return
  fi

  # 如果 commit 相同，无需部署
  if [ "$last_commit" = "$current_commit" ]; then
    log "No changes detected (commit: $current_commit)"
    REBUILD_BACKEND=false
    REBUILD_NANOBOT=false
    REBUILD_WEB=false
    RELOAD_CONFIG=false
    return
  fi

  log "Detecting changes between $last_commit and $current_commit"

  # 获取变更文件列表
  local changed_files=$(git diff --name-only "$last_commit" "$current_commit")

  REBUILD_BACKEND=false
  REBUILD_NANOBOT=false
  REBUILD_WEB=false
  RELOAD_CONFIG=false

  # 分析变更文件
  while IFS= read -r file; do
    case "$file" in
      Nukara_Backend/nanobot|Nukara_Backend/nanobot/*|nanobot/*)
        REBUILD_NANOBOT=true
        log "  Nanobot changed: $file"
        ;;
      Nukara_Backend/*)
        REBUILD_BACKEND=true
        log "  Backend changed: $file"
        ;;
      Nukara_Web/*)
        REBUILD_WEB=true
        log "  Web changed: $file"
        ;;
      Nukara_Admin_Web/*)
        REBUILD_WEB=true
        log "  Admin web changed: $file"
        ;;
      configs/*|deploy/*)
        RELOAD_CONFIG=true
        log "  Config changed: $file"
        ;;
    esac
  done <<< "$changed_files"

  # 输出检测结果
  echo ""
  log "Change detection results:"
  log "  REBUILD_BACKEND: $REBUILD_BACKEND"
  log "  REBUILD_NANOBOT: $REBUILD_NANOBOT"
  log "  REBUILD_WEB: $REBUILD_WEB"
  log "  RELOAD_CONFIG: $RELOAD_CONFIG"
  echo ""
}

# Dry run 模式：只检测不执行
dry_run_changes() {
  detect_changes

  echo ""
  echo -e "${BOLD}=========================================${NC}"
  echo -e "${BOLD}  Dry Run - Changes Detected${NC}"
  echo -e "${BOLD}=========================================${NC}"
  echo ""

  if [ "$REBUILD_BACKEND" = true ]; then
    echo -e "  ${YELLOW}●${NC} Backend services will be rebuilt"
  fi
  if [ "$REBUILD_NANOBOT" = true ]; then
    echo -e "  ${YELLOW}●${NC} Nanobot will be rebuilt"
  fi
  if [ "$REBUILD_WEB" = true ]; then
    echo -e "  ${YELLOW}●${NC} Web frontend will be rebuilt"
  fi
  if [ "$RELOAD_CONFIG" = true ]; then
    echo -e "  ${YELLOW}●${NC} Configuration will be reloaded"
  fi

  if [ "$REBUILD_BACKEND" = false ] && [ "$REBUILD_NANOBOT" = false ] && \
     [ "$REBUILD_WEB" = false ] && [ "$RELOAD_CONFIG" = false ]; then
    echo -e "  ${GREEN}●${NC} No changes detected - nothing to deploy"
  fi

  echo ""
  exit 0
}
