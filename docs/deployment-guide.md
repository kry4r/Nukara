# Nukara 部署指南

## 本地一键部署（全链路）

```bash
sudo bash deploy/deploy-local.sh --force-clean
```

参数说明：

- `--incremental`：按变更增量重建
- `--full`：强制全量重建
- `--dry-run`：仅检测变更，不执行部署
- `--force-clean`：部署前停止旧服务并清理关键端口
- `--non-interactive`：不走交互输入，直接使用环境变量或 `deploy/.env`

非交互示例：

```bash
export LLM_API_KEY=your-key
export NUKARA_ADMIN_PASSWORD=your-admin-password
sudo bash deploy/deploy-local.sh --force-clean --non-interactive
```

核心端口：

- `80`：用户前端（Nukara_Web）
- `8080`：Gateway API
- `9527`：管理端前端（Nukara_Admin_Web）
- `19527`：管理 API（nukara-admin，供 9527 反向代理）

## 增量部署

增量部署只重建变更的服务，保持数据持久性。

### 使用方式

```bash
# 增量部署（推荐）
sudo bash deploy/deploy-local.sh --incremental

# 强制全量部署
sudo bash deploy/deploy-local.sh --full

# 仅检测变更，不执行部署
sudo bash deploy/deploy-local.sh --dry-run

# 部署前清理旧服务 + 端口
sudo bash deploy/deploy-local.sh --force-clean
```

### 数据持久化保证

- **PostgreSQL**: 数据库不会被删除或重置
- **Redis**: 缓存数据保持不变
- **文件存储**: `/opt/nukara/data/uploads/` 中的用户文件保持不变

### 变更检测逻辑

脚本会自动检测以下目录的变更：

- `Nukara_Backend/**` → 重建 Go 服务（gateway, proactive）
- `Nukara_Web/**` → 重建前端并重载 nginx
- `Nukara_Admin_Web/**` → 重建管理前端并重载 nginx
- `configs/**` → 重载配置

## 管理员账号与默认 Provider 初始化

部署脚本会交互收集：

- `NUKARA_ADMIN_USERNAME`
- `NUKARA_ADMIN_PASSWORD`
- 默认 Provider 的 `name/base_url/api_key/models/priority/api_mode`
- 可选记忆基础设施：`NUKARA_QDRANT_URL / NUKARA_QDRANT_API_KEY / NUKARA_QDRANT_COLLECTION / NUKARA_NEO4J_URL / NUKARA_NEO4J_USER / NUKARA_NEO4J_PASSWORD / NUKARA_EMBEDDING_MODEL`

部署完成后会自动执行：

1. 若默认 Provider 不存在，则 `POST /api/admin/providers` 创建
2. 若默认 Provider 已存在，则 `PUT /api/admin/providers/{id}` 更新
3. `POST /api/admin/providers/{id}/switch` 切换为激活 Provider

如果默认 Provider 参数不完整，会跳过初始化并打印告警。

## 部署前置依赖

`deploy/deploy-local.sh` 现在会自动安装并校验以下关键依赖，避免首次部署中途失败：

- `jq`：provider bootstrap / deploy state 写入依赖
- `rsync`：源码同步依赖
- `xz-utils`（或 `xz`）：Node.js 二进制解压依赖
- `lsof`：清理旧端口时优先使用
- `openssl`：JWT 默认密钥生成依赖（缺失时也会回退到系统随机源）

另外，脚本已补齐两类 Linux 兼容问题：

- 在 RHEL / CentOS 系发行版首次安装 PostgreSQL 后，会自动尝试 `postgresql-setup --initdb`
- systemd 依赖会按实际服务名适配 `postgresql*` / `redis*`，避免 Ubuntu 上 `redis-server.service` 与固定写死 `redis.service` 不匹配

另外，脚本会在前端构建后显式校验：

- `Nukara_Web/dist/index.html`
- `Nukara_Admin_Web/dist/index.html`

若构建产物缺失，部署会直接失败而不是把旧前端继续挂到 nginx。

## 部署源码来源

`deploy/deploy-local.sh` 会先锁定当前仓库源码，再清理 `/opt/nukara` 并重建服务；不会在本地部署过程中静默回退到远程仓库快照。

同时，脚本现在会先同步源码、再执行 PostgreSQL migrations，避免首次部署时 `/opt/nukara/Nukara_Backend/migrations` 尚不存在导致迁移被跳过。

如果当前源码目录本身位于 `/opt/nukara` 下，脚本会先将源码快照到 `/tmp`，再继续部署，避免把当前 checkout 自己删掉。

## 先停旧服务（Force Clean）

`--force-clean` 会在部署前执行：

- 停止 systemd 服务：`nukara-gateway`、`nukara-proactive`、`nukara-admin`
- 清理端口：`80`、`8080`、`9527`、`19527`

风险提示：

- 该操作会中断当前在线流量，请在维护窗口执行。
- 非 `--dry-run` 下会直接终止占用目标端口的旧进程。

### 部署状态

部署状态存储在 `/opt/nukara/.deploy-state.json`，包含：

- 上次部署的 commit hash
- 各服务的二进制文件 hash
- 最后重启时间

### 故障排查

**问题：健康检查失败**

```bash
# 查看服务日志
journalctl -u nukara-gateway -n 50
journalctl -u nukara-admin -n 50

# 手动测试健康检查
curl http://localhost:8080/api/v1/gateway/health
curl http://localhost:19527/health
```

**问题：变更检测不准确**

```bash
# 查看部署状态
cat /opt/nukara/.deploy-state.json

# 手动重置状态（强制全量部署）
sudo rm /opt/nukara/.deploy-state.json
sudo bash deploy/deploy-local.sh --full
```
