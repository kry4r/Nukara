# Nukara Web

Nukara 聊天前端（Vue 3 + Vite），默认通过 Nginx 在 `18081` 对外提供服务。

## 本地开发

```bash
npm install
npm run dev
```

## 本地构建

```bash
npm run build
npm run preview -- --host 127.0.0.1 --port 18081
```

## 与本地一键部署联动

在仓库根目录执行：

```bash
sudo bash deploy/deploy-local.sh --force-clean
```

部署脚本会同时构建：

- `Nukara_Web`（聊天前端，`18081`）
- `Nukara_Admin_Web`（管理前端，`9527`）
- 管理 API（`19527`，由 `9527` 反向代理 `/api/admin/*`）

## 全链路 smoke

仓库根目录可执行：

```bash
bash scripts/smoke_full_stack_local.sh
```

需要提前设置：

- `NUKARA_ADMIN_USERNAME`
- `NUKARA_ADMIN_PASSWORD`
