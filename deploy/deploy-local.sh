#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Nukara — Local Native Deploy (No Docker)
# Installs: Postgres + Redis + Nginx + Go + Node
# Builds:   Gateway + Proactive + Admin + Frontend
# Manages:  systemd services
# Supports: Ubuntu/Debian, CentOS/RHEL/Fedora
# ============================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$DEPLOY_DIR/.env"
INSTALL_DIR="/opt/nukara"
ADMIN_WEB_PORT_DEFAULT="9527"
ADMIN_API_PORT_DEFAULT="19527"

log()  { echo -e "${GREEN}[Nukara]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }
info() { echo -e "${CYAN}[INFO]${NC} $*"; }

# --- Parse command line arguments ---
INCREMENTAL_MODE=false
FORCE_FULL_DEPLOY=false
DRY_RUN=false
FORCE_CLEAN=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --incremental)
      INCREMENTAL_MODE=true
      shift
      ;;
    --full)
      FORCE_FULL_DEPLOY=true
      shift
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --force-clean)
      FORCE_CLEAN=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [--incremental|--full|--dry-run|--force-clean]"
      exit 1
      ;;
  esac
done

if [ "$DRY_RUN" = true ] && [ -z "${STATE_FILE:-}" ]; then
  STATE_FILE="/tmp/nukara-deploy-state.json"
fi

# --- Load library functions ---
source "$(dirname "$0")/lib/deploy-state.sh"
source "$(dirname "$0")/lib/change-detection.sh"
source "$(dirname "$0")/lib/service-restart.sh"
source "$(dirname "$0")/lib/cleanup.sh"
source "$(dirname "$0")/lib/admin-bootstrap.sh"

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
  log "Detected OS: $DISTRO_ID"
}

# --- Package manager helper ---
pkg_install() {
  case "$DISTRO_ID" in
    ubuntu|debian)
      apt-get install -y -qq "$@"
      ;;
    centos|rhel|fedora|rocky|almalinux)
      if command -v dnf &>/dev/null; then
        dnf install -y "$@"
      else
        yum install -y "$@"
      fi
      ;;
    *)
      err "Unsupported distro: $DISTRO_ID"
      ;;
  esac
}

# --- Install system dependencies ---
install_deps() {
  log "Installing system dependencies..."

  case "$DISTRO_ID" in
    ubuntu|debian)
      apt-get update -qq
      pkg_install curl wget git build-essential ca-certificates gnupg lsb-release
      ;;
    centos|rhel|fedora|rocky|almalinux)
      pkg_install curl wget git gcc make ca-certificates
      ;;
  esac

  # --- PostgreSQL ---
  if command -v psql &>/dev/null; then
    log "PostgreSQL already installed"
  else
    log "Installing PostgreSQL..."
    case "$DISTRO_ID" in
      ubuntu|debian) pkg_install postgresql postgresql-contrib ;;
      *) pkg_install postgresql-server postgresql ;;
    esac
  fi
  systemctl enable postgresql
  systemctl start postgresql

  # --- Redis ---
  if command -v redis-server &>/dev/null; then
    log "Redis already installed"
  else
    log "Installing Redis..."
    case "$DISTRO_ID" in
      ubuntu|debian) pkg_install redis-server ;;
      *) pkg_install redis ;;
    esac
  fi
  systemctl enable redis 2>/dev/null || systemctl enable redis-server 2>/dev/null || true
  systemctl start redis 2>/dev/null || systemctl start redis-server 2>/dev/null || true

  # --- Nginx ---
  if command -v nginx &>/dev/null; then
    log "Nginx already installed"
  else
    log "Installing Nginx..."
    pkg_install nginx
  fi
  systemctl enable nginx

  log "System dependencies installed"
}

# --- Install Go 1.22 ---
install_go() {
  if command -v go &>/dev/null && go version | grep -q "go1.2"; then
    log "Go already installed: $(go version)"
    return
  fi

  log "Installing Go 1.22..."
  local GO_VER="1.22.10"
  local ARCH
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
  esac

  curl -fsSL "https://golang.google.cn/dl/go${GO_VER}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz

  # Ensure PATH
  if ! grep -q '/usr/local/go/bin' /etc/profile.d/go.sh 2>/dev/null; then
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
  fi
  export PATH=$PATH:/usr/local/go/bin
  export GOPROXY=https://goproxy.cn,direct

  log "Go installed: $(go version)"
}

# --- Install Node.js 20 ---
install_node() {
  if command -v node &>/dev/null && node -v | grep -q "v20"; then
    log "Node.js already installed: $(node -v)"
    return
  fi

  log "Installing Node.js 20..."
  local ARCH
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64)  ARCH="x64" ;;
    aarch64) ARCH="arm64" ;;
  esac

  curl -fsSL "https://npmmirror.com/mirrors/node/v20.18.0/node-v20.18.0-linux-${ARCH}.tar.xz" -o /tmp/node.tar.xz
  tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1
  rm /tmp/node.tar.xz
  npm config set registry https://registry.npmmirror.com

  log "Node.js installed: $(node -v)"
}

# --- Install Python 3.12 + uv ---
install_python() {
  if command -v python3 &>/dev/null; then
    log "Python already installed: $(python3 --version)"
  else
    log "Installing Python..."
    case "$DISTRO_ID" in
      ubuntu|debian) pkg_install python3 python3-venv python3-pip ;;
      *) pkg_install python3 python3-pip ;;
    esac
  fi

  if command -v uv &>/dev/null; then
    log "uv already installed"
  else
    log "Installing uv (Python package manager)..."
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="$HOME/.local/bin:$PATH"
  fi

  log "Python ready: $(python3 --version)"
}

# --- Interactive configuration ---
collect_config() {
  if [ -f "$ENV_FILE" ]; then
    log "Found existing .env — loading defaults..."
    set -a; source "$ENV_FILE"; set +a
  fi

  echo ""
  echo -e "${BOLD}=========================================${NC}"
  echo -e "${BOLD}  Nukara — Local Deploy Config${NC}"
  echo -e "${BOLD}=========================================${NC}"
  echo ""

  echo -e "${CYAN}[1/4] LLM Provider${NC}"
  read -rp "  API Key: " input
  LLM_API_KEY="${input:-${LLM_API_KEY:-}}"
  [ -z "$LLM_API_KEY" ] && err "LLM API Key is required"

  read -rp "  API Base URL [${LLM_API_BASE:-https://maas-api.cn-huabei-1.xf-yun.com/v2}]: " input
  LLM_API_BASE="${input:-${LLM_API_BASE:-https://maas-api.cn-huabei-1.xf-yun.com/v2}}"

  read -rp "  Model ID [${LLM_MODEL:-xminimaxm25}]: " input
  LLM_MODEL="${input:-${LLM_MODEL:-xminimaxm25}}"

  echo ""
  echo -e "${CYAN}[2/4] Domain & Ports${NC}"
  read -rp "  Domain [${DOMAIN:-localhost}]: " input
  DOMAIN="${input:-${DOMAIN:-localhost}}"

  read -rp "  HTTP port [${HTTP_PORT:-80}]: " input
  HTTP_PORT="${input:-${HTTP_PORT:-80}}"

  read -rp "  Gateway API port [${GATEWAY_PORT:-8080}]: " input
  GATEWAY_PORT="${input:-${GATEWAY_PORT:-8080}}"
  ADMIN_WEB_PORT="${ADMIN_WEB_PORT:-$ADMIN_WEB_PORT_DEFAULT}"
  ADMIN_API_PORT="${ADMIN_API_PORT:-$ADMIN_API_PORT_DEFAULT}"

  echo ""
  echo -e "${CYAN}[3/5] Security${NC}"
  DEFAULT_JWT=$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | xxd -p | head -c 32)
  read -rp "  JWT Secret [auto-generated]: " input
  JWT_SECRET="${input:-${JWT_SECRET:-$DEFAULT_JWT}}"

  read -rp "  Postgres password [${POSTGRES_PASSWORD:-nukara123}]: " input
  POSTGRES_PASSWORD="${input:-${POSTGRES_PASSWORD:-nukara123}}"

  echo ""
  echo -e "${CYAN}[4/5] Admin Account${NC}"
  read -rp "  Admin username [${NUKARA_ADMIN_USERNAME:-admin}]: " input
  NUKARA_ADMIN_USERNAME="${input:-${NUKARA_ADMIN_USERNAME:-admin}}"

  read -rsp "  Admin password: " input
  echo ""
  NUKARA_ADMIN_PASSWORD="${input:-${NUKARA_ADMIN_PASSWORD:-}}"
  [ -z "$NUKARA_ADMIN_PASSWORD" ] && err "Admin password is required"

  echo ""
  echo -e "${CYAN}[5/5] Proactive + Default Provider${NC}"
  read -rp "  Check interval [${PROACTIVE_INTERVAL:-5m}]: " input
  PROACTIVE_INTERVAL="${input:-${PROACTIVE_INTERVAL:-5m}}"
  read -rp "  Inactivity threshold [${INACTIVITY_THRESHOLD:-30m}]: " input
  INACTIVITY_THRESHOLD="${input:-${INACTIVITY_THRESHOLD:-30m}}"
  read -rp "  Cooldown [${PROACTIVE_COOLDOWN:-60m}]: " input
  PROACTIVE_COOLDOWN="${input:-${PROACTIVE_COOLDOWN:-60m}}"

  read -rp "  Default provider name [${DEFAULT_PROVIDER_NAME:-astron}]: " input
  DEFAULT_PROVIDER_NAME="${input:-${DEFAULT_PROVIDER_NAME:-astron}}"
  read -rp "  Default provider base URL [${DEFAULT_PROVIDER_BASE_URL:-$LLM_API_BASE}]: " input
  DEFAULT_PROVIDER_BASE_URL="${input:-${DEFAULT_PROVIDER_BASE_URL:-$LLM_API_BASE}}"
  read -rp "  Default provider API key [${DEFAULT_PROVIDER_API_KEY:-$LLM_API_KEY}]: " input
  DEFAULT_PROVIDER_API_KEY="${input:-${DEFAULT_PROVIDER_API_KEY:-$LLM_API_KEY}}"
  read -rp "  Default provider models [${DEFAULT_PROVIDER_MODELS:-$LLM_MODEL}]: " input
  DEFAULT_PROVIDER_MODELS="${input:-${DEFAULT_PROVIDER_MODELS:-$LLM_MODEL}}"
  read -rp "  Default provider priority [${DEFAULT_PROVIDER_PRIORITY:-1}]: " input
  DEFAULT_PROVIDER_PRIORITY="${input:-${DEFAULT_PROVIDER_PRIORITY:-1}}"

  cat > "$ENV_FILE" <<EOF
# Nukara Local Deploy — generated $(date +%Y-%m-%d)
LLM_API_KEY=$LLM_API_KEY
LLM_API_BASE=$LLM_API_BASE
LLM_MODEL=$LLM_MODEL
DOMAIN=$DOMAIN
HTTP_PORT=$HTTP_PORT
GATEWAY_PORT=$GATEWAY_PORT
JWT_SECRET=$JWT_SECRET
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
PROACTIVE_INTERVAL=$PROACTIVE_INTERVAL
INACTIVITY_THRESHOLD=$INACTIVITY_THRESHOLD
PROACTIVE_COOLDOWN=$PROACTIVE_COOLDOWN
ADMIN_WEB_PORT=$ADMIN_WEB_PORT
ADMIN_API_PORT=$ADMIN_API_PORT
NUKARA_ADMIN_USERNAME=$NUKARA_ADMIN_USERNAME
NUKARA_ADMIN_PASSWORD=$NUKARA_ADMIN_PASSWORD
DEFAULT_PROVIDER_NAME=$DEFAULT_PROVIDER_NAME
DEFAULT_PROVIDER_BASE_URL=$DEFAULT_PROVIDER_BASE_URL
DEFAULT_PROVIDER_API_KEY=$DEFAULT_PROVIDER_API_KEY
DEFAULT_PROVIDER_MODELS=$DEFAULT_PROVIDER_MODELS
DEFAULT_PROVIDER_PRIORITY=$DEFAULT_PROVIDER_PRIORITY
EOF

  log "Config saved to $ENV_FILE"
}

# --- Setup PostgreSQL database ---
setup_postgres() {
  log "Setting up PostgreSQL database..."

  # Create user and database
  sudo -u postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='nukara'" | grep -q 1 || \
    sudo -u postgres psql -c "CREATE USER nukara WITH PASSWORD '${POSTGRES_PASSWORD}';"

  sudo -u postgres psql -tc "SELECT 1 FROM pg_database WHERE datname='nukara'" | grep -q 1 || \
    sudo -u postgres psql -c "CREATE DATABASE nukara OWNER nukara;"

  sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE nukara TO nukara;" 2>/dev/null || true

  # Execute migrations
  log "Running database migrations..."
  if [ -d "$INSTALL_DIR/Nukara_Backend/migrations" ]; then
    for migration in "$INSTALL_DIR/Nukara_Backend/migrations"/*.sql; do
      if [ -f "$migration" ]; then
        log "  Applying $(basename "$migration")..."
        sudo -u postgres psql -d nukara -f "$migration" || warn "Migration failed: $migration"
      fi
    done
  else
    warn "Migrations directory not found, skipping migrations"
  fi

  log "PostgreSQL database ready"
}

# --- Clear /opt install residue before each deployment ---
cleanup_install_residue() {
  log "Clearing installation residue under $INSTALL_DIR ..."

  if [ "$DRY_RUN" = true ]; then
    log "  [dry-run] would remove: $INSTALL_DIR/* (except .deploy-state.json)"
    return
  fi

  mkdir -p "$INSTALL_DIR"
  find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 ! -name '.deploy-state.json' -exec rm -rf {} +
}

# --- Prepare source code ---
prepare_sources() {
  mkdir -p "$INSTALL_DIR"
  local workspace_root
  workspace_root="$(cd "$DEPLOY_DIR/.." && pwd)"
  local snapshot_root=""
  local snapshot_repo=""

  sync_from_workspace() {
    local name="$1"
    local src="$2"
    local dst="$3"
    [ -d "$src" ] || return 1

    log "Syncing $name from local workspace..."
    mkdir -p "$dst"
    if command -v rsync >/dev/null 2>&1; then
      rsync -a --delete \
        --exclude '.git' \
        --exclude 'node_modules' \
        --exclude 'dist' \
        --exclude '.venv' \
        --exclude '__pycache__' \
        "$src/" "$dst/"
    else
      rm -rf "$dst"
      mkdir -p "$dst"
      cp -a "$src"/. "$dst"/
      rm -rf "$dst/.git" "$dst/node_modules" "$dst/dist" "$dst/.venv" "$dst/__pycache__"
    fi
    return 0
  }

  ensure_repo_snapshot() {
    if [ -n "$snapshot_repo" ] && [ -d "$snapshot_repo" ]; then
      return 0
    fi
    snapshot_root="$(mktemp -d "$INSTALL_DIR/.nukara-src.XXXXXX")"
    snapshot_repo="$snapshot_root/repo"
    log "Fetching latest Nukara snapshot..."
    git clone --depth 1 https://github.com/kry4r/Nukara.git "$snapshot_repo"
  }

  refresh_component_from_snapshot() {
    local name="$1"
    local subdir="$2"
    local dst="$3"
    ensure_repo_snapshot
    log "Refreshing $name from repository snapshot..."
    rm -rf "$dst"
    mkdir -p "$dst"
    cp -a "$snapshot_repo/$subdir"/. "$dst"/
  }

  # Backend
  if ! sync_from_workspace "Nukara_Backend" "$workspace_root/Nukara_Backend" "$INSTALL_DIR/Nukara_Backend"; then
    refresh_component_from_snapshot "Nukara_Backend" "Nukara_Backend" "$INSTALL_DIR/Nukara_Backend"
  fi

  # Web frontend
  if ! sync_from_workspace "Nukara_Web" "$workspace_root/Nukara_Web" "$INSTALL_DIR/Nukara_Web"; then
    refresh_component_from_snapshot "Nukara_Web" "Nukara_Web" "$INSTALL_DIR/Nukara_Web"
  fi

  # Admin web frontend
  if ! sync_from_workspace "Nukara_Admin_Web" "$workspace_root/Nukara_Admin_Web" "$INSTALL_DIR/Nukara_Admin_Web"; then
    refresh_component_from_snapshot "Nukara_Admin_Web" "Nukara_Admin_Web" "$INSTALL_DIR/Nukara_Admin_Web"
  fi

  if [ -n "$snapshot_root" ] && [ -d "$snapshot_root" ]; then
    rm -rf "$snapshot_root"
  fi

  log "Source code ready at $INSTALL_DIR"
}

# --- Build all services ---
build_services() {
  log "Building services..."
  mkdir -p "$INSTALL_DIR/bin"

  # Build Go backend (gateway + proactive) - only if changed
  if [ "$REBUILD_BACKEND" = true ]; then
    cd "$INSTALL_DIR/Nukara_Backend"
    export GOPROXY=https://goproxy.cn,direct
    log "Building gateway..."
    CGO_ENABLED=0 go build -o "$INSTALL_DIR/bin/gateway" ./cmd/gateway
    log "Building proactive..."
    CGO_ENABLED=0 go build -o "$INSTALL_DIR/bin/proactive" ./cmd/proactive
    log "Building admin..."
    CGO_ENABLED=0 go build -o "$INSTALL_DIR/bin/admin" ./cmd/admin
  else
    log "Skipping backend build (no changes)"
  fi

  # Build frontend - only if changed
  if [ "$REBUILD_WEB" = true ]; then
    cd "$INSTALL_DIR/Nukara_Web"
    log "Building frontend..."
    npm ci --registry https://registry.npmmirror.com
    npm run build

    cd "$INSTALL_DIR/Nukara_Admin_Web"
    log "Building admin web..."
    npm ci --registry https://registry.npmmirror.com
    npm run build
  else
    log "Skipping frontend build (no changes)"
  fi

  log "All services built"
}

# --- Create systemd service files ---
create_services() {
  log "Creating systemd services..."

  local PG_DSN="postgres://nukara:${POSTGRES_PASSWORD}@127.0.0.1:5432/nukara?sslmode=disable"

  # --- Gateway ---
  cat > /etc/systemd/system/nukara-gateway.service <<EOF
[Unit]
Description=Nukara Gateway
After=postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/gateway
Restart=on-failure
RestartSec=5
Environment=NUKARA_JWT_SECRET=${JWT_SECRET}
Environment=NUKARA_POSTGRES_DSN=${PG_DSN}
Environment=NUKARA_REDIS_ADDR=127.0.0.1:6379
Environment=NUKARA_PROACTIVE_INTERVAL=${PROACTIVE_INTERVAL}
Environment=NUKARA_INACTIVITY_THRESHOLD=${INACTIVITY_THRESHOLD}
Environment=NUKARA_PROACTIVE_COOLDOWN=${PROACTIVE_COOLDOWN}

[Install]
WantedBy=multi-user.target
EOF

  # --- Proactive ---
  cat > /etc/systemd/system/nukara-proactive.service <<EOF
[Unit]
Description=Nukara Proactive Messaging
After=nukara-gateway.service
Wants=nukara-gateway.service

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/proactive
Restart=on-failure
RestartSec=5
Environment=NUKARA_JWT_SECRET=${JWT_SECRET}
Environment=NUKARA_POSTGRES_DSN=${PG_DSN}
Environment=NUKARA_REDIS_ADDR=127.0.0.1:6379
Environment=NUKARA_PROACTIVE_INTERVAL=${PROACTIVE_INTERVAL}

[Install]
WantedBy=multi-user.target
EOF

  # --- Admin ---
  cat > /etc/systemd/system/nukara-admin.service <<EOF
[Unit]
Description=Nukara Admin API
After=postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/admin
Restart=on-failure
RestartSec=5
Environment=NUKARA_ADMIN_PORT=${ADMIN_API_PORT}
Environment=NUKARA_ADMIN_USERNAME=${NUKARA_ADMIN_USERNAME}
Environment=NUKARA_ADMIN_PASSWORD=${NUKARA_ADMIN_PASSWORD}
Environment=NUKARA_POSTGRES_DSN=${PG_DSN}

[Install]
WantedBy=multi-user.target
EOF

  log "Gateway + Proactive + Admin services created"
}

# --- Configure Nginx ---
configure_nginx() {
  log "Configuring Nginx..."

  cat > /etc/nginx/conf.d/nukara.conf <<EOF
server {
    listen ${HTTP_PORT};
    server_name ${DOMAIN};

    # Vue SPA
    location / {
        root $INSTALL_DIR/Nukara_Web/dist;
        try_files \$uri \$uri/ /index.html;
    }

    # API proxy
    location /api/ {
        proxy_pass http://127.0.0.1:${GATEWAY_PORT}/api/;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # WebSocket proxy
    location /ws/ {
        proxy_pass http://127.0.0.1:${GATEWAY_PORT}/ws/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
EOF

  local admin_template="$DEPLOY_DIR/templates/nginx-admin.conf.template"
  local admin_web_root="$INSTALL_DIR/Nukara_Admin_Web/dist"

  [ -f "$admin_template" ] || err "Missing template: $admin_template"
  sed \
    -e "s|\${ADMIN_WEB_PORT}|${ADMIN_WEB_PORT}|g" \
    -e "s|\${DOMAIN}|${DOMAIN}|g" \
    -e "s|\${ADMIN_WEB_ROOT}|${admin_web_root}|g" \
    -e "s|\${ADMIN_API_PORT}|${ADMIN_API_PORT}|g" \
    "$admin_template" > /etc/nginx/conf.d/nukara-admin.conf

  # Remove default site if it conflicts
  rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true

  nginx -t && systemctl restart nginx
  log "Nginx configured and restarted"
}

# --- Start all services ---
start_services() {
  log "Starting Nukara services..."
  systemctl daemon-reload

  # Use selective restart logic
  restart_services

  # Update global deployment state
  local current_commit=$(git -C "$INSTALL_DIR/Nukara_Backend" rev-parse HEAD 2>/dev/null || echo "unknown")
  update_deploy_state "$current_commit" "" ""

  echo ""
  echo -e "${BOLD}=========================================${NC}"
  echo -e "${BOLD}  Service Status${NC}"
  echo -e "${BOLD}=========================================${NC}"
  for svc in postgresql redis nginx nukara-gateway nukara-proactive nukara-admin; do
    if systemctl is-active --quiet "$svc" 2>/dev/null; then
      echo -e "  ${GREEN}●${NC} $svc"
    else
      echo -e "  ${RED}●${NC} $svc (not running)"
    fi
  done
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
  echo -e "${BOLD}  Local Native Deploy (No Docker)${NC}"
  echo ""

  if [[ "$EUID" -ne 0 ]] && [ "$DRY_RUN" != true ]; then
    err "Please run as root: sudo bash $0"
  fi
  if [[ "$EUID" -ne 0 ]] && [ "$DRY_RUN" = true ]; then
    warn "Running in dry-run mode without root privileges"
  fi

  detect_distro

  # Initialize deployment state
  init_deploy_state

  if [ "$FORCE_CLEAN" = true ]; then
    cleanup_pre_deploy "$DRY_RUN"
  fi

  # Dry run mode
  if [ "$DRY_RUN" = true ]; then
    dry_run_changes
  fi

  # Incremental deployment mode
  if [ "$INCREMENTAL_MODE" = true ]; then
    log "Running in incremental deployment mode"
    detect_changes

    # No-change is no longer an early-exit condition:
    # clean deployment is enforced every run.
    if [ "$REBUILD_BACKEND" = false ] && \
       [ "$REBUILD_WEB" = false ] && [ "$RELOAD_CONFIG" = false ]; then
      log "No source changes detected, but clean deploy is enforced; continuing."
    fi
  else
    # Full deployment mode
    log "Running in full deployment mode"
    REBUILD_BACKEND=true
    REBUILD_WEB=true
    RELOAD_CONFIG=true
  fi

  # Always clear /opt residue and perform full rebuild.
  cleanup_install_residue
  REBUILD_BACKEND=true
  REBUILD_WEB=true
  RELOAD_CONFIG=true
  log "Forced full rebuild after residue cleanup"

  install_deps
  install_go
  install_node
  collect_config
  setup_postgres
  prepare_sources
  build_services
  create_services
  configure_nginx
  start_services
  bootstrap_default_provider

  echo -e "${GREEN}${BOLD}Nukara deployed successfully!${NC}"
  echo ""
  info "Web UI:     http://${DOMAIN}:${HTTP_PORT}"
  info "Gateway:    http://${DOMAIN}:${GATEWAY_PORT}"
  info "Admin UI:   http://${DOMAIN}:${ADMIN_WEB_PORT}"
  echo ""
  info "Manage services:"
  info "  systemctl status nukara-gateway"
  info "  systemctl status nukara-admin"
  info "  journalctl -u nukara-gateway -f    # follow logs"
  info "  sudo bash $0                       # re-deploy / update"
  echo ""
}

main "$@"
