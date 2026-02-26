#!/bin/bash
# ============================================================
# Nukara 一键 Docker 启动脚本
# 启动全部服务：postgres + redis + nanobot + gateway
# 启动后 iOS App 可直接对接测试
# ============================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/configs/docker-compose.dev.yml"
GATEWAY_PORT="${NUKARA_GATEWAY_PORT:-8080}"

# ---- 前置检查 ----
if ! command -v docker >/dev/null 2>&1; then
  echo "❌ docker 未安装"
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "❌ Docker 未运行，请先启动 Docker Desktop"
  exit 1
fi

# ---- 加载环境变量 ----
if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
else
  echo "⚠️  未找到 .env 文件，使用默认配置"
fi

# ---- 获取本机 IP（iOS 真机测试用）----
get_lan_ip() {
  # macOS
  if command -v ipconfig >/dev/null 2>&1; then
    ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || echo "127.0.0.1"
    return
  fi
  # Linux
  if command -v hostname >/dev/null 2>&1; then
    hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1"
    return
  fi
  echo "127.0.0.1"
}

echo "============================================================"
echo "  Nukara Backend — Docker 一键启动"
echo "============================================================"
echo ""

# ---- Step 1: 停止旧容器（如果有）----
echo "[1/5] 清理旧容器..."
docker compose -f "$COMPOSE_FILE" down --remove-orphans 2>/dev/null || true

# ---- Step 2: 构建镜像 ----
echo "[2/5] 构建镜像（gateway + nanobot）..."
docker compose -f "$COMPOSE_FILE" build --parallel 2>&1 | tail -5

# ---- Step 3: 启动基础设施 ----
echo "[3/5] 启动 postgres + redis..."
docker compose -f "$COMPOSE_FILE" up -d postgres redis

echo "     等待 postgres 就绪..."
for i in $(seq 1 30); do
  if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U postgres >/dev/null 2>&1; then
    echo "     postgres ✓ (${i}s)"
    break
  fi
  if [[ $i -eq 30 ]]; then
    echo "❌ postgres 启动超时"
    docker compose -f "$COMPOSE_FILE" logs postgres | tail -10
    exit 1
  fi
  sleep 1
done

echo "     等待 redis 就绪..."
for i in $(seq 1 15); do
  if docker compose -f "$COMPOSE_FILE" exec -T redis redis-cli ping >/dev/null 2>&1; then
    echo "     redis ✓ (${i}s)"
    break
  fi
  if [[ $i -eq 15 ]]; then
    echo "❌ redis 启动超时"
    exit 1
  fi
  sleep 1
done

# ---- Step 4: 启动 nanobot ----
echo "[4/5] 启动 nanobot（AI agent）..."
docker compose -f "$COMPOSE_FILE" up -d nanobot
sleep 2

if ! docker compose -f "$COMPOSE_FILE" ps nanobot --format '{{.State}}' 2>/dev/null | grep -q "running"; then
  echo "⚠️  nanobot 可能未正常启动，查看日志："
  docker compose -f "$COMPOSE_FILE" logs nanobot | tail -10
  echo ""
  echo "继续启动 gateway（agent 会 fallback 到 stub 回复）..."
fi

# ---- Step 5: 启动 gateway ----
echo "[5/5] 启动 gateway..."
docker compose -f "$COMPOSE_FILE" up -d gateway

echo "     等待 gateway 就绪..."
for i in $(seq 1 60); do
  if curl -sf "http://localhost:${GATEWAY_PORT}/api/v1/gateway/health" >/dev/null 2>&1; then
    echo "     gateway ✓ (${i}s)"
    break
  fi
  if [[ $i -eq 60 ]]; then
    echo "❌ gateway 启动超时"
    docker compose -f "$COMPOSE_FILE" logs gateway | tail -20
    exit 1
  fi
  sleep 1
done

# ---- 完成 ----
LAN_IP=$(get_lan_ip)

echo ""
echo "============================================================"
echo "  ✅ Nukara 后端已启动"
echo "============================================================"
echo ""
echo "  服务状态："
docker compose -f "$COMPOSE_FILE" ps --format "    {{.Name}}\t{{.State}}\t{{.Ports}}" 2>/dev/null || \
  docker compose -f "$COMPOSE_FILE" ps
echo ""
echo "  接口地址："
echo "    本机:     http://localhost:${GATEWAY_PORT}"
echo "    局域网:   http://${LAN_IP}:${GATEWAY_PORT}"
echo ""
echo "  iOS App 配置："
echo "    将 API Base URL 设为: http://${LAN_IP}:${GATEWAY_PORT}"
echo "    （确保 iPhone 和 Mac 在同一 WiFi 下）"
echo ""
echo "  快速验证："
echo "    curl http://localhost:${GATEWAY_PORT}/api/v1/gateway/health"
echo ""
echo "  查看日志:   docker compose -f $COMPOSE_FILE logs -f"
echo "  停止服务:   docker compose -f $COMPOSE_FILE down"
echo "============================================================"
