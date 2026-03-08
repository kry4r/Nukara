#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Nukara — One-Click Full Stack Deploy
# Deploys: Postgres + Redis + Gateway + Proactive + Web + Admin API + Admin Web
# Supports: Ubuntu/Debian, CentOS/RHEL/Fedora, Arch, Alpine
# ============================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$DEPLOY_DIR/.env"
COMPOSE="docker compose"

log()  { echo -e "${GREEN}[Nukara]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }
info() { echo -e "${CYAN}[INFO]${NC} $*"; }

# --- Detect distro ---
detect_distro() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_ID="${ID:-unknown}"
  elif [ -f /etc/redhat-release ]; then
    DISTRO_ID="rhel"
  elif [[ "$(uname)" == "Darwin" ]]; then
    DISTRO_ID="macos"
  else
    DISTRO_ID="unknown"
  fi
  log "Detected OS: $DISTRO_ID"
}

# --- Install Docker if missing ---
install_docker() {
  if command -v docker &>/dev/null; then
    log "Docker already installed: $(docker --version)"
    return
  fi

  [[ "$DISTRO_ID" == "macos" ]] && err "Please install Docker Desktop for Mac first: https://docs.docker.com/desktop/mac/install/"

  log "Installing Docker..."
  case "$DISTRO_ID" in
    ubuntu|debian)
      apt-get update -qq
      apt-get install -y -qq ca-certificates curl gnupg lsb-release
      install -m 0755 -d /etc/apt/keyrings
      curl -fsSL "https://download.docker.com/linux/$DISTRO_ID/gpg" | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
      chmod a+r /etc/apt/keyrings/docker.gpg
      echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$DISTRO_ID $(lsb_release -cs) stable" \
        > /etc/apt/sources.list.d/docker.list
      apt-get update -qq
      apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
      ;;
    centos|rhel|fedora|rocky|almalinux)
      if command -v dnf &>/dev/null; then PKG=dnf; else PKG=yum; fi
      $PKG install -y yum-utils
      yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo 2>/dev/null || true
      $PKG install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
      ;;
    arch|manjaro)
      pacman -Sy --noconfirm docker docker-compose
      ;;
    alpine)
      apk add --no-cache docker docker-cli-compose
      ;;
    *)
      err "Unsupported distro: $DISTRO_ID. Install Docker manually, then re-run."
      ;;
  esac

  systemctl enable docker 2>/dev/null || rc-update add docker default 2>/dev/null || true
  systemctl start docker 2>/dev/null || service docker start 2>/dev/null || true
  log "Docker installed successfully"
}

# --- Configure Docker registry mirrors (China) ---
configure_mirrors() {
  local DAEMON_JSON="/etc/docker/daemon.json"

  # Skip if already configured
  if [ -f "$DAEMON_JSON" ] && grep -q "registry-mirrors" "$DAEMON_JSON" 2>/dev/null; then
    log "Docker mirrors already configured"
    return
  fi

  echo ""
  echo -e "${CYAN}Docker Hub 在国内访问受限，是否配置镜像加速？${NC}"
  read -rp "  Configure registry mirrors? [Y/n]: " yn
  if [[ "$yn" =~ ^[Nn] ]]; then
    return
  fi

  log "Configuring Docker registry mirrors..."

  # Merge with existing daemon.json if present
  if [ -f "$DAEMON_JSON" ]; then
    cp "$DAEMON_JSON" "${DAEMON_JSON}.bak"
    warn "Backed up existing daemon.json"
  fi

  mkdir -p /etc/docker
  cat > "$DAEMON_JSON" <<'MIRRORS'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.xuanyuan.me",
    "https://docker.m.daocloud.io"
  ]
}
MIRRORS

  # Restart Docker to apply
  systemctl daemon-reload 2>/dev/null || true
  systemctl restart docker 2>/dev/null || service docker restart 2>/dev/null || true
  sleep 2
  log "Docker mirrors configured and service restarted"
}

# --- Ensure docker compose is available ---
ensure_compose() {
  if docker compose version &>/dev/null; then
    COMPOSE="docker compose"
  elif command -v docker-compose &>/dev/null; then
    COMPOSE="docker-compose"
  else
    log "Installing Docker Compose plugin..."
    mkdir -p /usr/local/lib/docker/cli-plugins
    COMPOSE_VER=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep tag_name | cut -d'"' -f4)
    curl -SL "https://github.com/docker/compose/releases/download/${COMPOSE_VER}/docker-compose-$(uname -s)-$(uname -m)" \
      -o /usr/local/lib/docker/cli-plugins/docker-compose
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
    COMPOSE="docker compose"
  fi
  log "Compose: $($COMPOSE version)"
}

# --- Interactive configuration ---
collect_config() {
  if [ -f "$ENV_FILE" ]; then
    log "Found existing .env — loading defaults..."
    set -a; source "$ENV_FILE"; set +a
  fi

  echo ""
  echo -e "${BOLD}=========================================${NC}"
  echo -e "${BOLD}  Nukara — Full Stack Deploy Config${NC}"
  echo -e "${BOLD}=========================================${NC}"
  echo ""

  # --- Required: LLM API ---
  echo -e "${CYAN}[1/5] LLM Provider (Astron MaaS / OpenAI-compatible)${NC}"
  read -rp "  API Key: " input
  LLM_API_KEY="${input:-${LLM_API_KEY:-}}"
  [ -z "$LLM_API_KEY" ] && err "LLM API Key is required"

  read -rp "  API Base URL [${LLM_API_BASE:-https://maas-api.cn-huabei-1.xf-yun.com/v2}]: " input
  LLM_API_BASE="${input:-${LLM_API_BASE:-https://maas-api.cn-huabei-1.xf-yun.com/v2}}"

  read -rp "  Model ID [${LLM_MODEL:-xminimaxm25}]: " input
  LLM_MODEL="${input:-${LLM_MODEL:-xminimaxm25}}"

  # --- Domain & Ports ---
  echo ""
  echo -e "${CYAN}[2/6] Domain & Ports${NC}"
  read -rp "  Domain [${DOMAIN:-localhost}]: " input
  DOMAIN="${input:-${DOMAIN:-localhost}}"

  read -rp "  HTTP port [${HTTP_PORT:-80}]: " input
  HTTP_PORT="${input:-${HTTP_PORT:-80}}"

  read -rp "  Gateway API port [${GATEWAY_PORT:-8080}]: " input
  GATEWAY_PORT="${input:-${GATEWAY_PORT:-8080}}"

  read -rp "  Admin Web port [${ADMIN_WEB_PORT:-9527}]: " input
  ADMIN_WEB_PORT="${input:-${ADMIN_WEB_PORT:-9527}}"

  read -rp "  Admin API port [${ADMIN_API_PORT:-19527}]: " input
  ADMIN_API_PORT="${input:-${ADMIN_API_PORT:-19527}}"

  # --- Security ---
  echo ""
  echo -e "${CYAN}[3/6] Security${NC}"
  DEFAULT_JWT=$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | xxd -p | head -c 32)
  read -rp "  JWT Secret [auto-generated]: " input
  JWT_SECRET="${input:-${JWT_SECRET:-$DEFAULT_JWT}}"

  read -rp "  Postgres password [${POSTGRES_PASSWORD:-postgres}]: " input
  POSTGRES_PASSWORD="${input:-${POSTGRES_PASSWORD:-postgres}}"

  read -rp "  Admin username [${NUKARA_ADMIN_USERNAME:-admin}]: " input
  NUKARA_ADMIN_USERNAME="${input:-${NUKARA_ADMIN_USERNAME:-admin}}"

  read -rsp "  Admin password [${NUKARA_ADMIN_PASSWORD:-admin123}]: " input
  echo ""
  NUKARA_ADMIN_PASSWORD="${input:-${NUKARA_ADMIN_PASSWORD:-admin123}}"
  [ -z "$NUKARA_ADMIN_PASSWORD" ] && err "Admin password is required"

  # --- APNs (optional) ---
  echo ""
  echo -e "${CYAN}[4/6] APNs Push Notifications (optional, press Enter to skip)${NC}"
  read -rp "  APNs Key ID [${APNS_KEY_ID:-}]: " input
  APNS_KEY_ID="${input:-${APNS_KEY_ID:-}}"
  read -rp "  APNs Team ID [${APNS_TEAM_ID:-}]: " input
  APNS_TEAM_ID="${input:-${APNS_TEAM_ID:-}}"
  read -rp "  APNs P8 Base64 [${APNS_P8_BASE64:-}]: " input
  APNS_P8_BASE64="${input:-${APNS_P8_BASE64:-}}"
  read -rp "  APNs Sandbox? (true/false) [${APNS_SANDBOX:-true}]: " input
  APNS_SANDBOX="${input:-${APNS_SANDBOX:-true}}"

  # --- Proactive messaging ---
  echo ""
  echo -e "${CYAN}[5/6] Proactive Messaging${NC}"
  read -rp "  Check interval [${PROACTIVE_INTERVAL:-5m}]: " input
  PROACTIVE_INTERVAL="${input:-${PROACTIVE_INTERVAL:-5m}}"
  read -rp "  Inactivity threshold [${INACTIVITY_THRESHOLD:-30m}]: " input
  INACTIVITY_THRESHOLD="${input:-${INACTIVITY_THRESHOLD:-30m}}"
  read -rp "  Cooldown [${PROACTIVE_COOLDOWN:-60m}]: " input
  PROACTIVE_COOLDOWN="${input:-${PROACTIVE_COOLDOWN:-60m}}"

  echo ""
  echo -e "${CYAN}[6/6] Embedding / Runtime Defaults${NC}"
  read -rp "  Embedding model [${NUKARA_EMBEDDING_MODEL:-text-embedding-3-small}]: " input
  NUKARA_EMBEDDING_MODEL="${input:-${NUKARA_EMBEDDING_MODEL:-text-embedding-3-small}}"

  # --- Write .env ---
  cat > "$ENV_FILE" <<EOF
# Nukara Deploy Config — generated $(date +%Y-%m-%d)
LLM_API_KEY=$LLM_API_KEY
LLM_API_BASE=$LLM_API_BASE
LLM_MODEL=$LLM_MODEL
NUKARA_ASTRON_API_KEY=$LLM_API_KEY
NUKARA_ASTRON_BASE_URL=$LLM_API_BASE
NUKARA_ASTRON_CHAT_MODEL=$LLM_MODEL
DOMAIN=$DOMAIN
HTTP_PORT=$HTTP_PORT
GATEWAY_PORT=$GATEWAY_PORT
ADMIN_WEB_PORT=$ADMIN_WEB_PORT
ADMIN_API_PORT=$ADMIN_API_PORT
JWT_SECRET=$JWT_SECRET
POSTGRES_USER=nukara
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
NUKARA_ADMIN_USERNAME=$NUKARA_ADMIN_USERNAME
NUKARA_ADMIN_PASSWORD=$NUKARA_ADMIN_PASSWORD
NUKARA_EMBEDDING_MODEL=$NUKARA_EMBEDDING_MODEL
APNS_KEY_ID=$APNS_KEY_ID
APNS_TEAM_ID=$APNS_TEAM_ID
APNS_P8_BASE64=$APNS_P8_BASE64
APNS_SANDBOX=$APNS_SANDBOX
APNS_TOPIC=com.nukara.app
PROACTIVE_INTERVAL=$PROACTIVE_INTERVAL
INACTIVITY_THRESHOLD=$INACTIVITY_THRESHOLD
PROACTIVE_COOLDOWN=$PROACTIVE_COOLDOWN
EOF

  log "Config saved to $ENV_FILE"
}

# --- Prepare source code ---
prepare_sources() {
  cd "$DEPLOY_DIR"

  # Backend
  if [ -d "./Nukara_Backend" ]; then
    log "Nukara_Backend found locally"
  elif [ -d "../Nukara_Backend" ]; then
    ln -sf ../Nukara_Backend ./Nukara_Backend
    log "Linked Nukara_Backend from parent dir"
  else
    log "Cloning Nukara_Backend..."
    git clone --depth 1 https://github.com/kry4r/Nukara.git _tmp_nukara
    mv _tmp_nukara/Nukara_Backend ./Nukara_Backend
    rm -rf _tmp_nukara
  fi

  # Web frontend
  if [ -d "./Nukara_Web" ]; then
    log "Nukara_Web found locally"
  elif [ -d "../Nukara_Web" ]; then
    ln -sf ../Nukara_Web ./Nukara_Web
    log "Linked Nukara_Web from parent dir"
  else
    log "Cloning Nukara_Web..."
    git clone --depth 1 https://github.com/kry4r/Nukara.git _tmp_nukara2
    mv _tmp_nukara2/Nukara_Web ./Nukara_Web
    rm -rf _tmp_nukara2
  fi

  # Admin frontend
  if [ -d "./Nukara_Admin_Web" ]; then
    log "Nukara_Admin_Web found locally"
  elif [ -d "../Nukara_Admin_Web" ]; then
    ln -sf ../Nukara_Admin_Web ./Nukara_Admin_Web
    log "Linked Nukara_Admin_Web from parent dir"
  else
    log "Cloning Nukara_Admin_Web..."
    git clone --depth 1 https://github.com/kry4r/Nukara.git _tmp_nukara3
    mv _tmp_nukara3/Nukara_Admin_Web ./Nukara_Admin_Web
    rm -rf _tmp_nukara3
  fi
}

# --- Generate nginx config from template ---
generate_nginx_conf() {
  if [ ! -f "$DEPLOY_DIR/nginx.conf.template" ]; then
    err "nginx.conf.template not found in $DEPLOY_DIR"
  fi
  sed "s/\${NUKARA_DOMAIN}/$DOMAIN/g" "$DEPLOY_DIR/nginx.conf.template" > "$DEPLOY_DIR/nginx.conf"
  log "Generated nginx.conf for domain: $DOMAIN"
}

# --- Deploy with docker compose ---
deploy() {
  cd "$DEPLOY_DIR"

  log "Building images..."
  $COMPOSE --env-file "$ENV_FILE" build

  log "Starting services..."
  $COMPOSE --env-file "$ENV_FILE" up -d

  echo ""
  log "Waiting for services to become healthy..."
  sleep 5

  echo ""
  echo -e "${BOLD}=========================================${NC}"
  echo -e "${BOLD}  Service Status${NC}"
  echo -e "${BOLD}=========================================${NC}"
  $COMPOSE ps
  echo ""

  # Quick health check
  local gw_url="http://localhost:${GATEWAY_PORT:-8080}/api/v1/gateway/health"
  if curl -sf "$gw_url" &>/dev/null; then
    log "Gateway health check: ${GREEN}OK${NC}"
  else
    warn "Gateway not yet responding at $gw_url (may still be starting)"
  fi

  local admin_url="http://localhost:${ADMIN_API_PORT:-19527}/health"
  if curl -sf "$admin_url" &>/dev/null; then
    log "Admin health check: ${GREEN}OK${NC}"
  else
    warn "Admin not yet responding at $admin_url (may still be starting)"
  fi

  echo ""
  echo -e "${GREEN}${BOLD}Nukara deployed successfully!${NC}"
  echo ""
  info "Web UI:     http://${DOMAIN}:${HTTP_PORT}"
  info "Gateway:    http://${DOMAIN}:${GATEWAY_PORT}"
  info "Admin UI:   http://${DOMAIN}:${ADMIN_WEB_PORT}"
  info "Admin API:  http://${DOMAIN}:${ADMIN_API_PORT}"
  echo ""
  info "Useful commands:"
  info "  $COMPOSE --env-file $ENV_FILE logs -f        # follow logs"
  info "  $COMPOSE --env-file $ENV_FILE ps             # service status"
  info "  $COMPOSE --env-file $ENV_FILE restart <svc>  # restart a service"
  info "  $COMPOSE --env-file $ENV_FILE down            # stop all"
  echo ""
}

# --- Main ---
main() {
  echo ""
  echo -e "${GREEN}${BOLD}"
  echo "  _   _       _                    "
  echo " | \ | |_   _| | ____ _ _ __ __ _  "
  echo " |  \| | | | | |/ / _\` | '__/ _\` | "
  echo " | |\  | |_| |   < (_| | | | (_| | "
  echo " |_| \_|\__,_|_|\_\__,_|_|  \__,_| "
  echo -e "${NC}"
  echo -e "${BOLD}  One-Click Full Stack Deploy${NC}"
  echo ""

  # Check root on Linux
  if [[ "$(uname)" != "Darwin" ]] && [[ "$EUID" -ne 0 ]]; then
    warn "Not running as root. Docker install may fail."
    read -rp "Continue anyway? [y/N]: " yn
    [[ "$yn" =~ ^[Yy] ]] || exit 0
  fi

  detect_distro
  install_docker
  configure_mirrors
  ensure_compose
  collect_config
  prepare_sources
  generate_nginx_conf
  deploy

  log "Done! Enjoy Nukara."
}

main "$@"
