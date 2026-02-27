#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Nukara Web — One-Click Deploy Script
# Supports: Ubuntu/Debian, CentOS/RHEL/Fedora, Arch, Alpine
# ============================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[Nukara]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# --- Detect distro ---
detect_distro() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO_ID="${ID:-unknown}"
  elif [ -f /etc/redhat-release ]; then
    DISTRO_ID="rhel"
  else
    DISTRO_ID="unknown"
  fi
  log "Detected distro: $DISTRO_ID"
}

# --- Install Docker if missing ---
install_docker() {
  if command -v docker &>/dev/null; then
    log "Docker already installed: $(docker --version)"
    return
  fi

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
      if command -v dnf &>/dev/null; then
        PKG=dnf
      else
        PKG=yum
      fi
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

# --- Install Docker Compose plugin if missing ---
ensure_compose() {
  if docker compose version &>/dev/null; then
    log "Docker Compose plugin available"
    COMPOSE="docker compose"
  elif command -v docker-compose &>/dev/null; then
    log "Using standalone docker-compose"
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
}

# --- Interactive config ---
collect_config() {
  DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
  ENV_FILE="$DEPLOY_DIR/.env"

  if [ -f "$ENV_FILE" ]; then
    log "Found existing .env — loading..."
    source "$ENV_FILE"
  fi

  echo ""
  echo "========================================="
  echo "  Nukara Web — Deployment Configuration"
  echo "========================================="
  echo ""

  read -rp "Backend API URL (e.g. https://api.example.com): " input_api_url
  NUKARA_API_URL="${input_api_url:-${NUKARA_API_URL:-}}"
  [ -z "$NUKARA_API_URL" ] && err "API URL is required"

  read -rp "API Token: " input_api_token
  NUKARA_API_TOKEN="${input_api_token:-${NUKARA_API_TOKEN:-}}"
  [ -z "$NUKARA_API_TOKEN" ] && err "API Token is required"

  read -rp "Domain for this web frontend (e.g. web.example.com) [localhost]: " input_domain
  NUKARA_DOMAIN="${input_domain:-${NUKARA_DOMAIN:-localhost}}"

  read -rp "HTTP port [80]: " input_http_port
  NUKARA_HTTP_PORT="${input_http_port:-${NUKARA_HTTP_PORT:-80}}"

  read -rp "HTTPS port (0 to disable) [0]: " input_https_port
  NUKARA_HTTPS_PORT="${input_https_port:-${NUKARA_HTTPS_PORT:-0}}"

  # Write .env
  cat > "$ENV_FILE" <<EOF
NUKARA_API_URL=$NUKARA_API_URL
NUKARA_API_TOKEN=$NUKARA_API_TOKEN
NUKARA_DOMAIN=$NUKARA_DOMAIN
NUKARA_HTTP_PORT=$NUKARA_HTTP_PORT
NUKARA_HTTPS_PORT=$NUKARA_HTTPS_PORT
EOF

  log "Config saved to $ENV_FILE"
}

# --- Generate nginx config from template ---
generate_nginx_conf() {
  log "Generating nginx.conf..."
  sed \
    -e "s|\${NUKARA_API_URL}|$NUKARA_API_URL|g" \
    -e "s|\${NUKARA_DOMAIN}|$NUKARA_DOMAIN|g" \
    "$DEPLOY_DIR/nginx.conf.template" > "$DEPLOY_DIR/nginx.conf"
}

# --- Build and deploy ---
deploy() {
  cd "$DEPLOY_DIR"

  log "Building and starting containers..."
  $COMPOSE -f docker-compose.yml --env-file .env down --remove-orphans 2>/dev/null || true
  $COMPOSE -f docker-compose.yml --env-file .env up -d --build

  echo ""
  log "========================================="
  log "  Deployment complete!"
  log "========================================="
  log "  Web UI: http://$NUKARA_DOMAIN:$NUKARA_HTTP_PORT"
  log "  API proxy: http://$NUKARA_DOMAIN:$NUKARA_HTTP_PORT/api/"
  log ""
  log "  Manage:"
  log "    cd $DEPLOY_DIR"
  log "    $COMPOSE logs -f        # view logs"
  log "    $COMPOSE restart        # restart"
  log "    $COMPOSE down           # stop"
  log "========================================="
}

# --- Main ---
main() {
  [ "$(id -u)" -ne 0 ] && err "Please run as root: sudo bash deploy.sh"

  detect_distro
  install_docker
  ensure_compose
  collect_config
  generate_nginx_conf
  deploy
}

main "$@"
