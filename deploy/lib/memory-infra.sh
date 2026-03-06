#!/usr/bin/env bash

infer_qdrant_vector_size() {
  local model="${1:-}"
  case "${model,,}" in
    text-embedding-3-large)
      printf '3072\n'
      ;;
    text-embedding-3-small|text-embedding-ada-002|*minimax*|*)
      printf '1536\n'
      ;;
  esac
}

ensure_ubuntu_universe() {
  case "$DISTRO_ID" in
    ubuntu)
      if ! grep -RqsE '^[[:space:]]*deb .+ubuntu.+ universe([[:space:]]|$)' /etc/apt/sources.list /etc/apt/sources.list.d/*.list 2>/dev/null; then
        if ! command -v add-apt-repository >/dev/null 2>&1; then
          pkg_install software-properties-common
        fi
        add-apt-repository -y universe
      fi
      ;;
  esac
}

install_java_runtime() {
  if command -v java >/dev/null 2>&1; then
    log "Java runtime already installed: $(java -version 2>&1 | head -n 1)"
    return
  fi

  log "Installing Java runtime for Neo4j..."
  case "$DISTRO_ID" in
    ubuntu|debian)
      pkg_install openjdk-17-jre-headless
      ;;
    centos|rhel|fedora|rocky|almalinux)
      pkg_install java-17-openjdk-headless
      ;;
    *)
      err "Unsupported distro for Java runtime: $DISTRO_ID"
      ;;
  esac
}

qdrant_request() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local tmp
  tmp=$(mktemp)
  local status
  local -a args=(--noproxy '*' -sS -o "$tmp" -w '%{http_code}' -X "$method" "$url")
  if [ -n "${NUKARA_QDRANT_API_KEY:-}" ]; then
    args+=(-H "api-key: ${NUKARA_QDRANT_API_KEY}")
  fi
  if [ -n "$body" ]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  if ! status=$(curl "${args[@]}"); then
    QDRANT_HTTP_STATUS="000"
    QDRANT_HTTP_BODY="$(cat "$tmp" 2>/dev/null || true)"
    rm -f "$tmp"
    return 1
  fi
  QDRANT_HTTP_STATUS="$status"
  QDRANT_HTTP_BODY="$(cat "$tmp")"
  rm -f "$tmp"
  return 0
}

wait_for_qdrant_ready() {
  local timeout="${1:-45}"
  local elapsed=0
  local ready_url="http://127.0.0.1:${NUKARA_QDRANT_HTTP_PORT}/readyz"

  log "Waiting for Qdrant readiness: $ready_url"
  while [ "$elapsed" -lt "$timeout" ]; do
    if qdrant_request GET "$ready_url" && [ "$QDRANT_HTTP_STATUS" = "200" ]; then
      log "  ✓ Qdrant ready"
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  err "Qdrant readiness check failed: status=${QDRANT_HTTP_STATUS:-000} body=${QDRANT_HTTP_BODY:-}"
}

ensure_qdrant_collection() {
  [ -n "${NUKARA_QDRANT_COLLECTION:-}" ] || return 0
  case "${NUKARA_QDRANT_VECTOR_SIZE:-}" in
    ''|*[!0-9]*) err "NUKARA_QDRANT_VECTOR_SIZE must be a positive integer" ;;
  esac

  local base_url="${NUKARA_QDRANT_URL%/}"
  local collection_url="${base_url}/collections/${NUKARA_QDRANT_COLLECTION}"
  if qdrant_request GET "$collection_url"; then
    if [ "$QDRANT_HTTP_STATUS" = "200" ]; then
      log "Qdrant collection already exists: ${NUKARA_QDRANT_COLLECTION}"
      return 0
    fi
    if [ "$QDRANT_HTTP_STATUS" != "404" ]; then
      err "Failed checking Qdrant collection: status=$QDRANT_HTTP_STATUS body=$QDRANT_HTTP_BODY"
    fi
  else
    err "Failed checking Qdrant collection readiness: ${QDRANT_HTTP_BODY:-curl error}"
  fi

  local payload
  payload=$(jq -nc --argjson size "$NUKARA_QDRANT_VECTOR_SIZE" '{vectors:{size:$size,distance:"Cosine"}}')
  qdrant_request PUT "$collection_url" "$payload" || err "Failed creating Qdrant collection: ${QDRANT_HTTP_BODY:-curl error}"
  case "$QDRANT_HTTP_STATUS" in
    200|201)
      log "Qdrant collection created: ${NUKARA_QDRANT_COLLECTION} (size=${NUKARA_QDRANT_VECTOR_SIZE})"
      ;;
    *)
      err "Failed creating Qdrant collection: status=$QDRANT_HTTP_STATUS body=$QDRANT_HTTP_BODY"
      ;;
  esac
}

install_qdrant() {
  local version="${NUKARA_QDRANT_VERSION:-1.16.3}"
  version="${version#v}"
  local arch asset
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)
      asset='qdrant-x86_64-unknown-linux-musl.tar.gz'
      ;;
    aarch64|arm64)
      asset='qdrant-aarch64-unknown-linux-musl.tar.gz'
      ;;
    *)
      err "Unsupported architecture for Qdrant: $arch"
      ;;
  esac

  local url="https://github.com/qdrant/qdrant/releases/download/v${version}/${asset}"
  local tmpdir nologin_shell qdrant_bin
  tmpdir=$(mktemp -d)
  nologin_shell="$(command -v nologin || true)"
  nologin_shell="${nologin_shell:-/usr/sbin/nologin}"

  log "Installing Qdrant v${version} from ${url} ..."
  curl -fsSL "$url" -o "$tmpdir/qdrant.tar.gz"
  tar -xzf "$tmpdir/qdrant.tar.gz" -C "$tmpdir"
  qdrant_bin="$(find "$tmpdir" -type f -name qdrant | head -n 1)"
  [ -n "$qdrant_bin" ] || err "Qdrant binary not found in release asset"
  install -m 0755 "$qdrant_bin" /usr/local/bin/qdrant

  if ! getent group qdrant >/dev/null 2>&1; then
    groupadd --system qdrant >/dev/null 2>&1 || true
  fi
  if ! id -u qdrant >/dev/null 2>&1; then
    useradd --system --gid qdrant --home-dir /var/lib/qdrant --shell "$nologin_shell" qdrant >/dev/null 2>&1 || true
  fi
  mkdir -p /etc/qdrant /var/lib/qdrant/storage /var/log/qdrant
  chown -R qdrant:qdrant /var/lib/qdrant /var/log/qdrant 2>/dev/null || true

  cat > /etc/qdrant/config.yaml <<EOF
log_level: INFO
storage:
  storage_path: /var/lib/qdrant/storage
service:
  host: 127.0.0.1
  http_port: ${NUKARA_QDRANT_HTTP_PORT}
  grpc_port: ${NUKARA_QDRANT_GRPC_PORT}
EOF
  if [ -n "${NUKARA_QDRANT_API_KEY:-}" ]; then
    printf '  api_key: %s\n' "${NUKARA_QDRANT_API_KEY}" >> /etc/qdrant/config.yaml
  fi

  cat > /etc/systemd/system/qdrant.service <<EOF
[Unit]
Description=Qdrant Vector Database
After=network.target
Wants=network.target

[Service]
Type=simple
User=qdrant
Group=qdrant
ExecStart=/usr/local/bin/qdrant --config-path /etc/qdrant/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535
WorkingDirectory=/var/lib/qdrant

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable qdrant
  systemctl restart qdrant
  wait_for_qdrant_ready 45
  ensure_qdrant_collection
  rm -rf "$tmpdir"
}

set_neo4j_conf() {
  local key="$1"
  local value="$2"
  local conf="/etc/neo4j/neo4j.conf"
  local escaped_key
  escaped_key="$(printf '%s' "$key" | sed -e 's/[][\\.^$*+?(){}|/]/\\&/g')"
  [ -f "$conf" ] || err "Neo4j config not found: $conf"
  if grep -Eq "^[#[:space:]]*${escaped_key}=" "$conf"; then
    sed -i -E "s|^[#[:space:]]*${escaped_key}=.*|${key}=${value}|" "$conf"
  else
    printf '\n%s=%s\n' "$key" "$value" >> "$conf"
  fi
}

wait_for_neo4j_ready() {
  local timeout="${1:-60}"
  local elapsed=0
  local address="bolt://127.0.0.1:${NUKARA_NEO4J_BOLT_PORT}"

  log "Waiting for Neo4j readiness: ${address}"
  while [ "$elapsed" -lt "$timeout" ]; do
    if command -v cypher-shell >/dev/null 2>&1 && \
       cypher-shell -a "$address" -u "${NUKARA_NEO4J_USER}" -p "${NUKARA_NEO4J_PASSWORD}" -d "${NUKARA_NEO4J_DATABASE}" 'RETURN 1;' >/dev/null 2>&1; then
      log "  ✓ Neo4j ready"
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done

  err "Neo4j readiness check failed. Verify NUKARA_NEO4J_PASSWORD matches the installed database password."
}

install_neo4j_repository() {
  case "$DISTRO_ID" in
    ubuntu|debian)
      ensure_ubuntu_universe
      mkdir -p /etc/apt/keyrings
      curl -fsSL https://debian.neo4j.com/neotechnology.gpg.key | gpg --dearmor -o /etc/apt/keyrings/neotechnology.gpg
      cat > /etc/apt/sources.list.d/neo4j.list <<EOF
deb [signed-by=/etc/apt/keyrings/neotechnology.gpg] https://debian.neo4j.com stable latest
EOF
      apt-get update -qq
      ;;
    centos|rhel|fedora|rocky|almalinux)
      rpm --import https://debian.neo4j.com/neotechnology.gpg.key
      cat > /etc/yum.repos.d/neo4j.repo <<EOF
[neo4j]
name=Neo4j Yum Repo
baseurl=https://yum.neo4j.com/stable/latest
enabled=1
gpgcheck=1
gpgkey=https://debian.neo4j.com/neotechnology.gpg.key
EOF
      ;;
    *)
      err "Unsupported distro for Neo4j repository: $DISTRO_ID"
      ;;
  esac
}

install_neo4j() {
  install_java_runtime
  install_neo4j_repository

  if command -v neo4j >/dev/null 2>&1; then
    log "Neo4j already installed: $(neo4j --version 2>/dev/null || echo installed)"
  else
    log "Installing Neo4j package..."
    pkg_install neo4j
  fi

  set_neo4j_conf server.default_listen_address 127.0.0.1
  set_neo4j_conf server.http.listen_address "127.0.0.1:${NUKARA_NEO4J_HTTP_PORT}"
  set_neo4j_conf server.bolt.listen_address "127.0.0.1:${NUKARA_NEO4J_BOLT_PORT}"
  set_neo4j_conf initial.dbms.default_database "${NUKARA_NEO4J_DATABASE}"

  if ! compgen -G '/var/lib/neo4j/data/dbms/auth*' >/dev/null; then
    if command -v neo4j-admin >/dev/null 2>&1; then
      neo4j-admin dbms set-initial-password "${NUKARA_NEO4J_PASSWORD}" >/dev/null 2>&1 || true
    fi
  fi

  systemctl enable neo4j
  systemctl restart neo4j
  wait_for_neo4j_ready 90
}

install_memory_infra() {
  if [ "${NUKARA_MEMORY_INFRA_ENABLED:-true}" != "true" ]; then
    log "Skipping local memory infra installation (NUKARA_MEMORY_INFRA_ENABLED=${NUKARA_MEMORY_INFRA_ENABLED})"
    return 0
  fi

  log "Installing local memory infra (Qdrant + Neo4j)..."
  install_qdrant
  install_neo4j
}
