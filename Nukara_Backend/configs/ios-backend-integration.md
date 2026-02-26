# iOS 前后端联调指南

## 1. 启动后端

```bash
cd Nukara_Backend
./scripts/deploy_gateway.sh
```

或本地直接运行：

```bash
cd Nukara_Backend
./scripts/dev_up.sh
```

如果网络受限、无法拉取 Docker 基础镜像，可用离线 docker 测试脚本：

```bash
cd Nukara_Backend
./scripts/docker_local_smoke.sh
```

如果要常驻部署后端（容器退出自动重启）：

```bash
cd Nukara_Backend
./scripts/docker_local_up.sh
./scripts/docker_local_status.sh
```

## 2. iOS App 环境配置

Nukara App 已支持真实后端模式（默认 real）。

在 Xcode Scheme -> Run -> Arguments -> Environment Variables 中设置：

- `NUKARA_ENV=real`
- `NUKARA_BASE_URL=http://localhost:8080`

如果使用真机调试，请将 `localhost` 替换为开发机局域网 IP，例如：

- `NUKARA_BASE_URL=http://192.168.1.25:8080`

## 3. 联调验证顺序

1. 登录/注册（验证码固定 `123456`）
2. 创建 Bot
3. 会话列表拉取
4. 发送消息（HTTP `/send` + WS `/ws/chat`）
5. 主动消息触发（`/api/v1/gateway/test/proactive`）

## 4. 后端自动化冒烟

```bash
cd Nukara_Backend
./scripts/smoke_backend.sh http://localhost:8080
```

## 5. Astron MaaS 模型配置（可选）

在后端 `.env` 中配置：

```bash
NUKARA_ASTRON_BASE_URL=https://maas-api.cn-huabei-1.xf-yun.com/v2
NUKARA_ASTRON_API_KEY=<your_api_key>
NUKARA_ASTRON_CHAT_MODEL=xopglm47blth2
NUKARA_ASTRON_PROACTIVE_MODEL=xopglm47blth2
NUKARA_ASTRON_EMBEDDING_MODEL=xopglm5
```

配置后：
- Dify 不可用时，聊天/主动消息自动回退到 Astron MaaS
- RAG embedding 优先走 Astron embedding API，失败时回退本地 hash embedding
