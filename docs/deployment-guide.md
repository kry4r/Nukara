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
- 默认 Provider 的 `name/base_url/api_key/models/priority`

部署完成后会自动执行：

1. `POST /api/admin/providers` 创建默认 Provider
2. `POST /api/admin/providers/{id}/switch` 切换为激活 Provider

如果默认 Provider 参数不完整，会跳过初始化并打印告警。

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
