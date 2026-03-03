#!/usr/bin/env bash
set -euo pipefail

# ============================================================
# Nukara — One-Click Full Stack Deploy
# Deploys: Postgres + Redis + Nanobot + Gateway + Proactive + Web
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
  echo -e "${CYAN}[2/5] Domain & Ports${NC}"
  read -rp "  Domain [${DOMAIN:-localhost}]: " input
  DOMAIN="${input:-${DOMAIN:-localhost}}"

  read -rp "  HTTP port [${HTTP_PORT:-80}]: " input
  HTTP_PORT="${input:-${HTTP_PORT:-80}}"

  read -rp "  Gateway API port [${GATEWAY_PORT:-8080}]: " input
  GATEWAY_PORT="${input:-${GATEWAY_PORT:-8080}}"

  # --- Security ---
  echo ""
  echo -e "${CYAN}[3/5] Security${NC}"
  DEFAULT_JWT=$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | xxd -p | head -c 32)
  read -rp "  JWT Secret [auto-generated]: " input
  JWT_SECRET="${input:-${JWT_SECRET:-$DEFAULT_JWT}}"

  read -rp "  Postgres password [${POSTGRES_PASSWORD:-postgres}]: " input
  POSTGRES_PASSWORD="${input:-${POSTGRES_PASSWORD:-postgres}}"

  # --- APNs (optional) ---
  echo ""
  echo -e "${CYAN}[4/5] APNs Push Notifications (optional, press Enter to skip)${NC}"
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
  echo -e "${CYAN}[5/5] Proactive Messaging${NC}"
  read -rp "  Check interval [${PROACTIVE_INTERVAL:-5m}]: " input
  PROACTIVE_INTERVAL="${input:-${PROACTIVE_INTERVAL:-5m}}"
  read -rp "  Inactivity threshold [${INACTIVITY_THRESHOLD:-30m}]: " input
  INACTIVITY_THRESHOLD="${input:-${INACTIVITY_THRESHOLD:-30m}}"
  read -rp "  Cooldown [${PROACTIVE_COOLDOWN:-60m}]: " input
  PROACTIVE_COOLDOWN="${input:-${PROACTIVE_COOLDOWN:-60m}}"

  # --- Write .env ---
  cat > "$ENV_FILE" <<EOF
# Nukara Deploy Config — generated $(date +%Y-%m-%d)
LLM_API_KEY=$LLM_API_KEY
LLM_API_BASE=$LLM_API_BASE
LLM_MODEL=$LLM_MODEL
DOMAIN=$DOMAIN
HTTP_PORT=$HTTP_PORT
GATEWAY_PORT=$GATEWAY_PORT
JWT_SECRET=$JWT_SECRET
POSTGRES_USER=postgres
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
APNS_KEY_ID=$APNS_KEY_ID
APNS_TEAM_ID=$APNS_TEAM_ID
APNS_P8_BASE64=$APNS_P8_BASE64
APNS_SANDBOX=$APNS_SANDBOX
APNS_TOPIC=com.nukara.app
PROACTIVE_INTERVAL=$PROACTIVE_INTERVAL
INACTIVITY_THRESHOLD=$INACTIVITY_THRESHOLD
PROACTIVE_COOLDOWN=$PROACTIVE_COOLDOWN
NANOBOT_TOKEN=
EOF

  log "Config saved to $ENV_FILE"
}

# --- Seed nanobot runtime config from deploy inputs ---
seed_nanobot_config() {
  local config_path="$1"

  python3 - "$config_path" "$LLM_API_KEY" "$LLM_API_BASE" "$LLM_MODEL" <<'PY'
import json
import sys
from pathlib import Path

config_path = Path(sys.argv[1])
api_key = sys.argv[2]
api_base = sys.argv[3]
model = sys.argv[4]

data = {}
if config_path.exists():
    raw = config_path.read_text(encoding="utf-8").strip()
    if raw:
        loaded = json.loads(raw)
        if isinstance(loaded, dict):
            data = loaded

agents = data.get("agents")
if not isinstance(agents, dict):
    agents = {}
data["agents"] = agents

defaults = agents.get("defaults")
if not isinstance(defaults, dict):
    defaults = {}
agents["defaults"] = defaults

providers = data.get("providers")
if not isinstance(providers, dict):
    providers = {}
data["providers"] = providers

custom = providers.get("custom")
if not isinstance(custom, dict):
    custom = {}
providers["custom"] = custom

custom["api_key"] = api_key
custom["api_base"] = api_base
if model.strip():
    defaults["model"] = model.strip()

config_path.parent.mkdir(parents=True, exist_ok=True)
config_path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY
}

# --- Prepare source code ---
prepare_sources() {
  cd "$DEPLOY_DIR"

  is_python_project_dir() {
    local dir="$1"
    [ -f "$dir/pyproject.toml" ] || [ -f "$dir/setup.py" ]
  }

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

  # Nanobot
  local nanobot_ready=false
  if [ -d "./nanobot" ]; then
    if is_python_project_dir "./nanobot"; then
      log "nanobot found locally"
      nanobot_ready=true
    else
      warn "Local ./nanobot exists but is missing pyproject.toml/setup.py, will fallback."
    fi
  fi

  if [ "$nanobot_ready" = false ] && [ -d "./Nukara_Backend/nanobot" ]; then
    log "Using embedded nanobot from Nukara_Backend..."
    mkdir -p ./nanobot
    if command -v rsync >/dev/null 2>&1; then
      rsync -a --delete \
        --exclude '.git' \
        --exclude '.venv' \
        --exclude '__pycache__' \
        ./Nukara_Backend/nanobot/ ./nanobot/
    else
      rm -rf ./nanobot
      mkdir -p ./nanobot
      cp -a ./Nukara_Backend/nanobot/. ./nanobot/
      rm -rf ./nanobot/.git ./nanobot/.venv ./nanobot/__pycache__
    fi

    if is_python_project_dir "./nanobot"; then
      nanobot_ready=true
    else
      warn "Embedded nanobot source is incomplete (submodule likely not initialized), will fallback."
      rm -rf ./nanobot
    fi
  fi

  if [ "$nanobot_ready" = false ]; then
    log "Cloning nanobot..."
    rm -rf ./nanobot
    git clone --depth 1 -b multi-thread https://github.com/kry4r/nanobot.git ./nanobot
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

  # Copy nanobot config
  if [ -f "./Nukara_Backend/configs/nanobot/config.json" ]; then
    cp ./Nukara_Backend/configs/nanobot/config.json ./nanobot-config.json
    log "Copied nanobot config"
  else
    warn "nanobot config not found, using default"
    echo '{}' > ./nanobot-config.json
  fi
  seed_nanobot_config "./nanobot-config.json"
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

  echo ""
  echo -e "${GREEN}${BOLD}Nukara deployed successfully!${NC}"
  echo ""
  info "Web UI:     http://${DOMAIN}:${HTTP_PORT}"
  info "Gateway:    http://${DOMAIN}:${GATEWAY_PORT}"
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
