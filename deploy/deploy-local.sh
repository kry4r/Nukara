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
DEPLOY_SOURCE_ROOT=""
DEPLOY_SOURCE_SNAPSHOT=""
DEPLOY_SOURCE_COMMIT="unknown"

log()  { echo -e "${GREEN}[Nukara]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }
info() { echo -e "${CYAN}[INFO]${NC} $*"; }
append_env_var() {
  local key="$1"
  local value="${2-}"
  printf '%s=%q\n' "$key" "$value" >> "$ENV_FILE"
}

# --- Parse command line arguments ---
INCREMENTAL_MODE=false
FORCE_FULL_DEPLOY=false
DRY_RUN=false
FORCE_CLEAN=false
NON_INTERACTIVE=false
POSTGRES_SERVICE_NAME="postgresql"
REDIS_SERVICE_NAME="redis"

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
    --non-interactive)
      NON_INTERACTIVE=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      echo "Usage: $0 [--incremental|--full|--dry-run|--force-clean|--non-interactive]"
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
source "$(dirname "$0")/lib/memory-infra.sh"
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

list_systemd_unit_candidates() {
  local pattern="$1"
  systemctl list-unit-files "$pattern" --no-legend 2>/dev/null | awk '{print $1}' | sed 's/\.service$//' | awk 'NF'
}

start_and_enable_service() {
  local result_var="$1"
  shift
  local candidate
  for candidate in "$@"; do
    [ -n "$candidate" ] || continue
    systemctl start "$candidate" 2>/dev/null || continue
    systemctl enable "$candidate" 2>/dev/null || true
    printf -v "$result_var" '%s' "$candidate"
    return 0
  done
  return 1
}

ensure_postgres_cluster() {
  case "$DISTRO_ID" in
    ubuntu|debian)
      return 0
      ;;
  esac

  if [ -f /var/lib/pgsql/data/PG_VERSION ] || compgen -G "/var/lib/pgsql/*/data/PG_VERSION" >/dev/null; then
    return 0
  fi

  local setup_cmd=""
  local candidate
  for candidate in /usr/bin/postgresql-setup /usr/pgsql-*/bin/postgresql-setup; do
    [ -x "$candidate" ] || continue
    setup_cmd="$candidate"
    break
  done

  if [ -n "$setup_cmd" ]; then
    log "Initializing PostgreSQL data directory..."
    "$setup_cmd" --initdb
    return 0
  fi

  warn "PostgreSQL data directory is not initialized and postgresql-setup was not found; initialize PostgreSQL manually if startup fails."
}

generate_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 16
  elif command -v xxd >/dev/null 2>&1; then
    head -c 32 /dev/urandom | xxd -p | head -c 32
  else
    od -An -N16 -tx1 /dev/urandom | tr -d ' \n'
  fi
}

apply_config_defaults() {
  LLM_API_BASE="${LLM_API_BASE:-https://maas-api.cn-huabei-1.xf-yun.com/v2}"
  LLM_MODEL="${LLM_MODEL:-xminimaxm25}"
  LLM_API_MODE="${LLM_API_MODE:-chat_completions}"
  DOMAIN="${DOMAIN:-localhost}"
  HTTP_PORT="${HTTP_PORT:-80}"
  GATEWAY_PORT="${GATEWAY_PORT:-8080}"
  ADMIN_WEB_PORT="${ADMIN_WEB_PORT:-$ADMIN_WEB_PORT_DEFAULT}"
  ADMIN_API_PORT="${ADMIN_API_PORT:-$ADMIN_API_PORT_DEFAULT}"
  JWT_SECRET="${JWT_SECRET:-$(generate_secret)}"
  POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-nukara123}"
  NUKARA_ADMIN_USERNAME="${NUKARA_ADMIN_USERNAME:-admin}"
  PROACTIVE_INTERVAL="${PROACTIVE_INTERVAL:-5m}"
  INACTIVITY_THRESHOLD="${INACTIVITY_THRESHOLD:-30m}"
  PROACTIVE_COOLDOWN="${PROACTIVE_COOLDOWN:-60m}"
  DEFAULT_PROVIDER_NAME="${DEFAULT_PROVIDER_NAME:-astron}"
  DEFAULT_PROVIDER_BASE_URL="${DEFAULT_PROVIDER_BASE_URL:-$LLM_API_BASE}"
  DEFAULT_PROVIDER_API_KEY="${DEFAULT_PROVIDER_API_KEY:-${LLM_API_KEY:-}}"
  DEFAULT_PROVIDER_MODELS="${DEFAULT_PROVIDER_MODELS:-$LLM_MODEL}"
  DEFAULT_PROVIDER_PRIORITY="${DEFAULT_PROVIDER_PRIORITY:-1}"
  DEFAULT_PROVIDER_API_MODE="${DEFAULT_PROVIDER_API_MODE:-$LLM_API_MODE}"
  NUKARA_MEMORY_INFRA_ENABLED="${NUKARA_MEMORY_INFRA_ENABLED:-true}"
  NUKARA_QDRANT_VERSION="${NUKARA_QDRANT_VERSION:-1.16.3}"
  NUKARA_QDRANT_HTTP_PORT="${NUKARA_QDRANT_HTTP_PORT:-6333}"
  NUKARA_QDRANT_GRPC_PORT="${NUKARA_QDRANT_GRPC_PORT:-6334}"
  NUKARA_QDRANT_COLLECTION="${NUKARA_QDRANT_COLLECTION:-agent_memory_v1}"
  NUKARA_EMBEDDING_MODEL="${NUKARA_EMBEDDING_MODEL:-text-embedding-3-small}"
  NUKARA_QDRANT_VECTOR_SIZE="${NUKARA_QDRANT_VECTOR_SIZE:-$(infer_qdrant_vector_size "$NUKARA_EMBEDDING_MODEL")}"
  NUKARA_NEO4J_USER="${NUKARA_NEO4J_USER:-neo4j}"
  NUKARA_NEO4J_PASSWORD="${NUKARA_NEO4J_PASSWORD:-$(generate_secret)}"
  NUKARA_NEO4J_DATABASE="${NUKARA_NEO4J_DATABASE:-neo4j}"
  NUKARA_NEO4J_HTTP_PORT="${NUKARA_NEO4J_HTTP_PORT:-7474}"
  NUKARA_NEO4J_BOLT_PORT="${NUKARA_NEO4J_BOLT_PORT:-7687}"
  NUKARA_NEO4J_ADAPTER_PORT="${NUKARA_NEO4J_ADAPTER_PORT:-17687}"
  NUKARA_QDRANT_URL="${NUKARA_QDRANT_URL:-http://127.0.0.1:${NUKARA_QDRANT_HTTP_PORT}}"
  NUKARA_NEO4J_BOLT_URL="${NUKARA_NEO4J_BOLT_URL:-bolt://127.0.0.1:${NUKARA_NEO4J_BOLT_PORT}}"
  NUKARA_NEO4J_URL="${NUKARA_NEO4J_URL:-http://127.0.0.1:${NUKARA_NEO4J_ADAPTER_PORT}}"
}

validate_config() {
  [ -n "${LLM_API_KEY:-}" ] || err "LLM API Key is required (set LLM_API_KEY or deploy/.env)"
  [ -n "${NUKARA_ADMIN_PASSWORD:-}" ] || err "Admin password is required (set NUKARA_ADMIN_PASSWORD or deploy/.env)"
  case "${NUKARA_QDRANT_VECTOR_SIZE:-}" in
    ''|*[!0-9]*) err "NUKARA_QDRANT_VECTOR_SIZE must be a positive integer" ;;
  esac
}

persist_config() {
  : > "$ENV_FILE"
  printf '# Nukara Local Deploy — generated %s\n' "$(date +%Y-%m-%d)" >> "$ENV_FILE"
  append_env_var LLM_API_KEY "$LLM_API_KEY"
  append_env_var LLM_API_BASE "$LLM_API_BASE"
  append_env_var LLM_MODEL "$LLM_MODEL"
  append_env_var LLM_API_MODE "$LLM_API_MODE"
  append_env_var DOMAIN "$DOMAIN"
  append_env_var HTTP_PORT "$HTTP_PORT"
  append_env_var GATEWAY_PORT "$GATEWAY_PORT"
  append_env_var JWT_SECRET "$JWT_SECRET"
  append_env_var POSTGRES_PASSWORD "$POSTGRES_PASSWORD"
  append_env_var PROACTIVE_INTERVAL "$PROACTIVE_INTERVAL"
  append_env_var INACTIVITY_THRESHOLD "$INACTIVITY_THRESHOLD"
  append_env_var PROACTIVE_COOLDOWN "$PROACTIVE_COOLDOWN"
  append_env_var ADMIN_WEB_PORT "$ADMIN_WEB_PORT"
  append_env_var ADMIN_API_PORT "$ADMIN_API_PORT"
  append_env_var NUKARA_ADMIN_USERNAME "$NUKARA_ADMIN_USERNAME"
  append_env_var NUKARA_ADMIN_PASSWORD "$NUKARA_ADMIN_PASSWORD"
  append_env_var DEFAULT_PROVIDER_NAME "$DEFAULT_PROVIDER_NAME"
  append_env_var DEFAULT_PROVIDER_BASE_URL "$DEFAULT_PROVIDER_BASE_URL"
  append_env_var DEFAULT_PROVIDER_API_KEY "$DEFAULT_PROVIDER_API_KEY"
  append_env_var DEFAULT_PROVIDER_MODELS "$DEFAULT_PROVIDER_MODELS"
  append_env_var DEFAULT_PROVIDER_PRIORITY "$DEFAULT_PROVIDER_PRIORITY"
  append_env_var DEFAULT_PROVIDER_API_MODE "$DEFAULT_PROVIDER_API_MODE"
  append_env_var NUKARA_MEMORY_INFRA_ENABLED "$NUKARA_MEMORY_INFRA_ENABLED"
  append_env_var NUKARA_QDRANT_VERSION "$NUKARA_QDRANT_VERSION"
  append_env_var NUKARA_QDRANT_HTTP_PORT "$NUKARA_QDRANT_HTTP_PORT"
  append_env_var NUKARA_QDRANT_GRPC_PORT "$NUKARA_QDRANT_GRPC_PORT"
  append_env_var NUKARA_QDRANT_URL "$NUKARA_QDRANT_URL"
  append_env_var NUKARA_QDRANT_API_KEY "$NUKARA_QDRANT_API_KEY"
  append_env_var NUKARA_QDRANT_COLLECTION "$NUKARA_QDRANT_COLLECTION"
  append_env_var NUKARA_QDRANT_VECTOR_SIZE "$NUKARA_QDRANT_VECTOR_SIZE"
  append_env_var NUKARA_NEO4J_URL "$NUKARA_NEO4J_URL"
  append_env_var NUKARA_NEO4J_USER "$NUKARA_NEO4J_USER"
  append_env_var NUKARA_NEO4J_PASSWORD "$NUKARA_NEO4J_PASSWORD"
  append_env_var NUKARA_NEO4J_DATABASE "$NUKARA_NEO4J_DATABASE"
  append_env_var NUKARA_NEO4J_HTTP_PORT "$NUKARA_NEO4J_HTTP_PORT"
  append_env_var NUKARA_NEO4J_BOLT_PORT "$NUKARA_NEO4J_BOLT_PORT"
  append_env_var NUKARA_NEO4J_BOLT_URL "$NUKARA_NEO4J_BOLT_URL"
  append_env_var NUKARA_NEO4J_ADAPTER_PORT "$NUKARA_NEO4J_ADAPTER_PORT"
  append_env_var NUKARA_EMBEDDING_MODEL "$NUKARA_EMBEDDING_MODEL"

  log "Config saved to $ENV_FILE"
}

# --- Install system dependencies ---
install_deps() {
  log "Installing system dependencies..."

  case "$DISTRO_ID" in
    ubuntu|debian)
      apt-get update -qq
      pkg_install curl wget git build-essential ca-certificates gnupg lsb-release jq rsync xz-utils lsof openssl
      ;;
    centos|rhel|fedora|rocky|almalinux)
      pkg_install curl wget git gcc make ca-certificates jq rsync xz lsof openssl
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
  ensure_postgres_cluster
  local postgres_units=()
  readarray -t postgres_units < <(list_systemd_unit_candidates 'postgresql*.service')
  start_and_enable_service POSTGRES_SERVICE_NAME "postgresql" "${postgres_units[@]}" || err "Failed to start PostgreSQL service"

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
  local redis_units=()
  readarray -t redis_units < <(list_systemd_unit_candidates 'redis*.service')
  start_and_enable_service REDIS_SERVICE_NAME "redis" "redis-server" "${redis_units[@]}" || warn "Failed to start Redis service automatically"

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
  if command -v go &>/dev/null && go version | grep -Eq 'go1\.(2[2-9]|[3-9][0-9])(\.| |$)'; then
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

  apply_config_defaults

  if [ "$NON_INTERACTIVE" = true ] || [ ! -t 0 ]; then
    log "Using non-interactive config mode"
    validate_config
    persist_config
    return
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
  read -rp "  API mode [${LLM_API_MODE:-chat_completions}] (chat_completions/responses/auto): " input
  LLM_API_MODE="${input:-${LLM_API_MODE:-chat_completions}}"

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
  echo -e "${CYAN}[3/6] Security${NC}"
  read -rp "  JWT Secret [auto-generated]: " input
  JWT_SECRET="${input:-$JWT_SECRET}"

  read -rp "  Postgres password [${POSTGRES_PASSWORD:-nukara123}]: " input
  POSTGRES_PASSWORD="${input:-${POSTGRES_PASSWORD:-nukara123}}"

  echo ""
  echo -e "${CYAN}[4/6] Admin Account${NC}"
  read -rp "  Admin username [${NUKARA_ADMIN_USERNAME:-admin}]: " input
  NUKARA_ADMIN_USERNAME="${input:-${NUKARA_ADMIN_USERNAME:-admin}}"

  read -rsp "  Admin password: " input
  echo ""
  NUKARA_ADMIN_PASSWORD="${input:-${NUKARA_ADMIN_PASSWORD:-}}"
  [ -z "$NUKARA_ADMIN_PASSWORD" ] && err "Admin password is required"

  echo ""
  echo -e "${CYAN}[5/6] Proactive + Default Provider${NC}"
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
  read -rp "  Default provider API mode [${DEFAULT_PROVIDER_API_MODE:-${LLM_API_MODE:-chat_completions}}] (chat_completions/responses/auto): " input
  DEFAULT_PROVIDER_API_MODE="${input:-${DEFAULT_PROVIDER_API_MODE:-${LLM_API_MODE:-chat_completions}}}"

  echo ""
  echo -e "${CYAN}[6/6] Memory Infra${NC}"
  read -rp "  Enable local Qdrant/Neo4j install [${NUKARA_MEMORY_INFRA_ENABLED:-true}] (true/false): " input
  NUKARA_MEMORY_INFRA_ENABLED="${input:-${NUKARA_MEMORY_INFRA_ENABLED:-true}}"
  read -rp "  Qdrant version [${NUKARA_QDRANT_VERSION:-1.16.3}]: " input
  NUKARA_QDRANT_VERSION="${input:-${NUKARA_QDRANT_VERSION:-1.16.3}}"
  read -rp "  Qdrant HTTP port [${NUKARA_QDRANT_HTTP_PORT:-6333}]: " input
  NUKARA_QDRANT_HTTP_PORT="${input:-${NUKARA_QDRANT_HTTP_PORT:-6333}}"
  read -rp "  Qdrant gRPC port [${NUKARA_QDRANT_GRPC_PORT:-6334}]: " input
  NUKARA_QDRANT_GRPC_PORT="${input:-${NUKARA_QDRANT_GRPC_PORT:-6334}}"
  NUKARA_QDRANT_URL="http://127.0.0.1:${NUKARA_QDRANT_HTTP_PORT}"
  read -rp "  Qdrant API key [${NUKARA_QDRANT_API_KEY:-}]: " input
  NUKARA_QDRANT_API_KEY="${input:-${NUKARA_QDRANT_API_KEY:-}}"
  read -rp "  Qdrant collection [${NUKARA_QDRANT_COLLECTION:-agent_memory_v1}]: " input
  NUKARA_QDRANT_COLLECTION="${input:-${NUKARA_QDRANT_COLLECTION:-agent_memory_v1}}"
  read -rp "  Neo4j HTTP port [${NUKARA_NEO4J_HTTP_PORT:-7474}]: " input
  NUKARA_NEO4J_HTTP_PORT="${input:-${NUKARA_NEO4J_HTTP_PORT:-7474}}"
  read -rp "  Neo4j Bolt port [${NUKARA_NEO4J_BOLT_PORT:-7687}]: " input
  NUKARA_NEO4J_BOLT_PORT="${input:-${NUKARA_NEO4J_BOLT_PORT:-7687}}"
  read -rp "  Neo4j adapter port [${NUKARA_NEO4J_ADAPTER_PORT:-17687}]: " input
  NUKARA_NEO4J_ADAPTER_PORT="${input:-${NUKARA_NEO4J_ADAPTER_PORT:-17687}}"
  NUKARA_NEO4J_BOLT_URL="bolt://127.0.0.1:${NUKARA_NEO4J_BOLT_PORT}"
  NUKARA_NEO4J_URL="http://127.0.0.1:${NUKARA_NEO4J_ADAPTER_PORT}"
  read -rp "  Neo4j database [${NUKARA_NEO4J_DATABASE:-neo4j}]: " input
  NUKARA_NEO4J_DATABASE="${input:-${NUKARA_NEO4J_DATABASE:-neo4j}}"
  read -rp "  Neo4j user [${NUKARA_NEO4J_USER:-neo4j}]: " input
  NUKARA_NEO4J_USER="${input:-${NUKARA_NEO4J_USER:-neo4j}}"
  read -rsp "  Neo4j password [auto-generated if empty]: " input
  echo ""
  NUKARA_NEO4J_PASSWORD="${input:-${NUKARA_NEO4J_PASSWORD:-$(generate_secret)}}"
  read -rp "  Embedding model [${NUKARA_EMBEDDING_MODEL:-text-embedding-3-small}]: " input
  NUKARA_EMBEDDING_MODEL="${input:-${NUKARA_EMBEDDING_MODEL:-text-embedding-3-small}}"
  read -rp "  Qdrant vector size [${NUKARA_QDRANT_VECTOR_SIZE:-$(infer_qdrant_vector_size "$NUKARA_EMBEDDING_MODEL")}]: " input
  NUKARA_QDRANT_VECTOR_SIZE="${input:-${NUKARA_QDRANT_VECTOR_SIZE:-$(infer_qdrant_vector_size "$NUKARA_EMBEDDING_MODEL")}}"

  persist_config
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

# --- Resolve and lock deployment source before cleanup ---
source_root_has_components() {
  local root="$1"
  [ -d "$root/deploy" ] && [ -d "$root/Nukara_Backend" ] && [ -d "$root/Nukara_Web" ] && [ -d "$root/Nukara_Admin_Web" ]
}

sync_tree() {
  local src="$1"
  local dst="$2"
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
}

resolve_deploy_source_root() {
  if [ -n "${NUKARA_SOURCE_ROOT:-}" ]; then
    local configured_root
    configured_root="$(cd "$NUKARA_SOURCE_ROOT" && pwd)"
    source_root_has_components "$configured_root" || err "NUKARA_SOURCE_ROOT is invalid: $configured_root"
    printf '%s\n' "$configured_root"
    return 0
  fi

  local candidate_root
  candidate_root="$(cd "$DEPLOY_DIR/.." && pwd)"
  if source_root_has_components "$candidate_root"; then
    printf '%s\n' "$candidate_root"
    return 0
  fi

  err "Unable to resolve deployment source root from $DEPLOY_DIR. Set NUKARA_SOURCE_ROOT and retry."
}

lock_deploy_source() {
  DEPLOY_SOURCE_ROOT="$(resolve_deploy_source_root)"
  if git -C "$DEPLOY_SOURCE_ROOT" rev-parse HEAD >/dev/null 2>&1; then
    DEPLOY_SOURCE_COMMIT="$(git -C "$DEPLOY_SOURCE_ROOT" rev-parse HEAD)"
  fi

  if [[ "$DEPLOY_SOURCE_ROOT" == "$INSTALL_DIR"* ]]; then
    DEPLOY_SOURCE_SNAPSHOT="$(mktemp -d /tmp/nukara-source.XXXXXX)"
    log "Source root is inside install dir; snapshotting to $DEPLOY_SOURCE_SNAPSHOT"
    sync_tree "$DEPLOY_SOURCE_ROOT" "$DEPLOY_SOURCE_SNAPSHOT"
    DEPLOY_SOURCE_ROOT="$DEPLOY_SOURCE_SNAPSHOT"
  fi

  log "Locked deployment source: $DEPLOY_SOURCE_ROOT"
  log "Deployment source commit: $DEPLOY_SOURCE_COMMIT"
}

cleanup_locked_source() {
  if [ -n "$DEPLOY_SOURCE_SNAPSHOT" ] && [ -d "$DEPLOY_SOURCE_SNAPSHOT" ]; then
    rm -rf "$DEPLOY_SOURCE_SNAPSHOT"
    DEPLOY_SOURCE_SNAPSHOT=""
  fi
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
  [ -n "$DEPLOY_SOURCE_ROOT" ] || err "Deployment source root is not locked"

  for component in deploy Nukara_Backend Nukara_Web Nukara_Admin_Web; do
    [ -d "$DEPLOY_SOURCE_ROOT/$component" ] || err "Missing component in deployment source: $DEPLOY_SOURCE_ROOT/$component"
    log "Syncing $component from locked source..."
    sync_tree "$DEPLOY_SOURCE_ROOT/$component" "$INSTALL_DIR/$component"
  done

  cleanup_locked_source
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
    log "Building neo4j adapter..."
    CGO_ENABLED=0 go build -o "$INSTALL_DIR/bin/neo4j-adapter" ./cmd/neo4j_adapter
  else
    log "Skipping backend build (no changes)"
  fi

  # Build frontend - only if changed
  if [ "$REBUILD_WEB" = true ]; then
    cd "$INSTALL_DIR/Nukara_Web"
    log "Building frontend..."
    npm ci --registry https://registry.npmmirror.com
    npm run build
    [ -f "$INSTALL_DIR/Nukara_Web/dist/index.html" ] || err "Frontend build output missing: $INSTALL_DIR/Nukara_Web/dist/index.html"

    cd "$INSTALL_DIR/Nukara_Admin_Web"
    log "Building admin web..."
    npm ci --registry https://registry.npmmirror.com
    npm run build
    [ -f "$INSTALL_DIR/Nukara_Admin_Web/dist/index.html" ] || err "Admin frontend build output missing: $INSTALL_DIR/Nukara_Admin_Web/dist/index.html"
  else
    log "Skipping frontend build (no changes)"
  fi

  log "All services built"
}

# --- Create systemd service files ---
create_services() {
  log "Creating systemd services..."

  local PG_DSN="postgres://nukara:${POSTGRES_PASSWORD}@127.0.0.1:5432/nukara?sslmode=disable"
  local memory_after=""
  local memory_wants=""

  if [ "${NUKARA_MEMORY_INFRA_ENABLED:-true}" = "true" ]; then
    memory_after=" qdrant.service neo4j.service nukara-neo4j-adapter.service"
    memory_wants=" qdrant.service neo4j.service nukara-neo4j-adapter.service"

    cat > /etc/systemd/system/nukara-neo4j-adapter.service <<EOF
[Unit]
Description=Nukara Neo4j HTTP Adapter
After=network.target neo4j.service
Wants=network.target neo4j.service

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/neo4j-adapter
Restart=on-failure
RestartSec=5
Environment=NUKARA_NEO4J_BOLT_URL=${NUKARA_NEO4J_BOLT_URL}
Environment=NUKARA_NEO4J_USER=${NUKARA_NEO4J_USER}
Environment=NUKARA_NEO4J_PASSWORD=${NUKARA_NEO4J_PASSWORD}
Environment=NUKARA_NEO4J_DATABASE=${NUKARA_NEO4J_DATABASE}
Environment=NUKARA_NEO4J_ADAPTER_HOST=127.0.0.1
Environment=NUKARA_NEO4J_ADAPTER_PORT=${NUKARA_NEO4J_ADAPTER_PORT}

[Install]
WantedBy=multi-user.target
EOF
  fi

  # --- Gateway ---
  cat > /etc/systemd/system/nukara-gateway.service <<EOF
[Unit]
Description=Nukara Gateway
After=${POSTGRES_SERVICE_NAME}.service ${REDIS_SERVICE_NAME}.service${memory_after}
Wants=${POSTGRES_SERVICE_NAME}.service ${REDIS_SERVICE_NAME}.service${memory_wants}

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/gateway
Restart=on-failure
RestartSec=5
Environment=NUKARA_JWT_SECRET=${JWT_SECRET}
Environment=NUKARA_POSTGRES_DSN=${PG_DSN}
Environment=NUKARA_REDIS_ADDR=127.0.0.1:6379
Environment=NUKARA_PROACTIVE_INTERVAL=${PROACTIVE_INTERVAL}
Environment=NUKARA_CHAT_BASE_URL=${LLM_API_BASE}
Environment=NUKARA_CHAT_API_KEY=${LLM_API_KEY}
Environment=NUKARA_CHAT_MODEL=${LLM_MODEL}
Environment=NUKARA_CHAT_API_MODE=${LLM_API_MODE}
Environment=NUKARA_QDRANT_URL=${NUKARA_QDRANT_URL:-}
Environment=NUKARA_QDRANT_API_KEY=${NUKARA_QDRANT_API_KEY:-}
Environment=NUKARA_QDRANT_COLLECTION=${NUKARA_QDRANT_COLLECTION:-agent_memory_v1}
Environment=NUKARA_NEO4J_URL=${NUKARA_NEO4J_URL:-}
Environment=NUKARA_NEO4J_USER=${NUKARA_NEO4J_USER:-}
Environment=NUKARA_NEO4J_PASSWORD=${NUKARA_NEO4J_PASSWORD:-}
Environment=NUKARA_EMBEDDING_MODEL=${NUKARA_EMBEDDING_MODEL:-}
Environment=NUKARA_INACTIVITY_THRESHOLD=${INACTIVITY_THRESHOLD}
Environment=NUKARA_PROACTIVE_COOLDOWN=${PROACTIVE_COOLDOWN}

[Install]
WantedBy=multi-user.target
EOF

  # --- Proactive ---
  cat > /etc/systemd/system/nukara-proactive.service <<EOF
[Unit]
Description=Nukara Proactive Messaging
After=nukara-gateway.service${memory_after}
Wants=nukara-gateway.service${memory_wants}

[Service]
Type=simple
ExecStart=$INSTALL_DIR/bin/proactive
Restart=on-failure
RestartSec=5
Environment=NUKARA_JWT_SECRET=${JWT_SECRET}
Environment=NUKARA_POSTGRES_DSN=${PG_DSN}
Environment=NUKARA_REDIS_ADDR=127.0.0.1:6379
Environment=NUKARA_PROACTIVE_INTERVAL=${PROACTIVE_INTERVAL}
Environment=NUKARA_CHAT_BASE_URL=${LLM_API_BASE}
Environment=NUKARA_CHAT_API_KEY=${LLM_API_KEY}
Environment=NUKARA_CHAT_MODEL=${LLM_MODEL}
Environment=NUKARA_CHAT_API_MODE=${LLM_API_MODE}
Environment=NUKARA_QDRANT_URL=${NUKARA_QDRANT_URL:-}
Environment=NUKARA_QDRANT_API_KEY=${NUKARA_QDRANT_API_KEY:-}
Environment=NUKARA_QDRANT_COLLECTION=${NUKARA_QDRANT_COLLECTION:-agent_memory_v1}
Environment=NUKARA_NEO4J_URL=${NUKARA_NEO4J_URL:-}
Environment=NUKARA_NEO4J_USER=${NUKARA_NEO4J_USER:-}
Environment=NUKARA_NEO4J_PASSWORD=${NUKARA_NEO4J_PASSWORD:-}
Environment=NUKARA_EMBEDDING_MODEL=${NUKARA_EMBEDDING_MODEL:-}

[Install]
WantedBy=multi-user.target
EOF

  # --- Admin ---
  cat > /etc/systemd/system/nukara-admin.service <<EOF
[Unit]
Description=Nukara Admin API
After=${POSTGRES_SERVICE_NAME}.service ${REDIS_SERVICE_NAME}.service
Wants=${POSTGRES_SERVICE_NAME}.service ${REDIS_SERVICE_NAME}.service

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
  update_deploy_state "$DEPLOY_SOURCE_COMMIT" "" ""

  echo ""
  echo -e "${BOLD}=========================================${NC}"
  echo -e "${BOLD}  Service Status${NC}"
  echo -e "${BOLD}=========================================${NC}"
  local services=("$POSTGRES_SERVICE_NAME" "$REDIS_SERVICE_NAME" nginx)
  if [ "${NUKARA_MEMORY_INFRA_ENABLED:-true}" = "true" ]; then
    services+=(qdrant neo4j nukara-neo4j-adapter)
  fi
  services+=(nukara-gateway nukara-proactive nukara-admin)
  for svc in "${services[@]}"; do
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

  collect_config

  # Lock deployment source before cleanup, then clear /opt residue and rebuild.
  lock_deploy_source
  cleanup_install_residue
  REBUILD_BACKEND=true
  REBUILD_WEB=true
  RELOAD_CONFIG=true
  log "Forced full rebuild after residue cleanup"

  install_deps
  install_go
  install_node
  prepare_sources
  setup_postgres
  build_services
  install_memory_infra
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
