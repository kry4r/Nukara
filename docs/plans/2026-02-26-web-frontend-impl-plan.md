# Nukara Web Frontend — 实现计划

> 基于设计文档：`docs/plans/2026-02-26-web-frontend-design.md`
> 日期：2026-02-26

## 项目结构

```
Nukara_Web/
├── index.html
├── package.json
├── vite.config.js
├── public/
│   └── favicon.ico
├── src/
│   ├── main.js
│   ├── App.vue
│   ├── router/
│   │   └── index.js              # Vue Router 配置 + 路由守卫
│   ├── stores/
│   │   ├── auth.js               # 认证状态（Pinia）
│   │   ├── chat.js               # 聊天状态 + WebSocket 事件处理
│   │   ├── conversations.js      # 会话列表
│   │   ├── bots.js               # Bot 管理
│   │   └── settings.js           # 用户设置
│   ├── composables/
│   │   ├── useWebSocket.js       # WebSocket 连接管理 + 重连
│   │   └── useApi.js             # HTTP API 封装
│   ├── views/
│   │   ├── AuthView.vue          # 登录/注册
│   │   ├── ConversationsView.vue # 会话列表
│   │   ├── ChatView.vue          # 聊天页
│   │   ├── BotsView.vue          # Bot 列表
│   │   ├── BotFormView.vue       # Bot 创建/编辑
│   │   └── SettingsView.vue      # 设置页
│   ├── components/
│   │   ├── MessageBubble.vue     # 消息气泡
│   │   ├── MessageInput.vue      # 输入框
│   │   ├── BotStatusBadge.vue    # Bot 状态 emoji
│   │   ├── ConversationItem.vue  # 会话列表项
│   │   ├── NavBar.vue            # 底部导航
│   │   └── TypingIndicator.vue   # 正在输入指示器
│   └── utils/
│       └── constants.js          # 常量定义
├── deploy/
│   ├── deploy.sh                 # 一键部署脚本
│   ├── docker-compose.yml
│   ├── Dockerfile.frontend       # 前端构建
│   ├── Dockerfile.backend        # 后端构建
│   ├── nginx.conf.template       # Nginx 模板
│   └── .env.template             # 环境变量模板
└── README.md
```

---

## Step 1: 项目脚手架

**目标**：初始化 Vue 3 + Vite 项目，安装依赖，配置基础路由。

**操作**：
1. `npm create vite@latest Nukara_Web -- --template vue`
2. 安装依赖：`vue-router`, `pinia`
3. 配置 `vite.config.js`：API 代理到 `localhost:8080`
4. 创建 `src/router/index.js`：定义所有路由 + auth 守卫
5. 创建 `src/main.js`：挂载 Pinia + Router
6. 创建 `src/App.vue`：RouterView + NavBar

**验证**：`npm run dev` 启动成功，访问各路由正常跳转。

---

## Step 2: API 层 + 认证 Store

**目标**：封装 HTTP 客户端，实现登录/注册流程。

**文件**：
- `src/composables/useApi.js` — fetch 封装，自动注入 Bearer token，错误处理
- `src/stores/auth.js` — Pinia store：login/register/requestSMS/logout/restoreSession
- `src/utils/constants.js` — API base URL、emotion emoji 映射等

**API 对接**：
```
POST /api/v1/auth/sms/send     { phone, purpose }
POST /api/v1/auth/login         { phone, sms_code }
POST /api/v1/auth/register      { phone, sms_code, nickname }
```

**关键逻辑**：
- Token 存 `localStorage`，页面刷新自动恢复
- `useApi` 返回 401 时自动清除 token 并跳转登录页
- Router 守卫：未登录 → `/auth`，已登录访问 `/auth` → `/`

**验证**：Chrome DevTools 验证注册 → 登录 → token 持久化。

---

## Step 3: 登录/注册页面

**目标**：实现 AuthView，对标 iOS 的登录/注册切换 UI。

**文件**：
- `src/views/AuthView.vue` — 手机号输入、验证码输入、登录/注册切换

**UI 要素**：
- 顶部 Logo + 应用名
- 手机号输入框 + 发送验证码按钮（60 秒倒计时）
- 验证码输入框
- 登录/注册切换（tab 或链接）
- 注册时额外显示昵称输入框
- 错误提示

**验证**：Chrome DevTools 完成注册全流程。

---

## Step 4: WebSocket 连接管理

**目标**：实现 WebSocket composable，支持心跳、重连、事件分发。

**文件**：
- `src/composables/useWebSocket.js`

**核心逻辑**：
- 连接：`ws://{host}/ws/chat?token={token}`
- 心跳：每 25 秒发送 `{"type":"ping"}`
- 重连：断线后指数退避（2^n 秒，上限 30 秒，最多 8 次）
- 事件解析：JSON parse → 按 `type` 字段分发到对应 handler
- 发送：`send(obj)` 自动 JSON.stringify
- 状态暴露：`isConnected`, `reconnectAttempts`

**事件类型处理**：
```
ack              → chatStore.handleAck()
typing           → chatStore.handleTyping()
multi_reply_start → chatStore.handleMultiReplyStart()
message          → chatStore.handleMessage()
multi_reply_end  → chatStore.handleMultiReplyEnd()
bot_status_update → conversationsStore.handleBotStatus()
proactive_message → chatStore.handleProactiveMessage()
pong             → 重置心跳计时器
error            → 显示错误提示
```

**验证**：Network 面板确认 WebSocket 连接建立、心跳正常。

---

## Step 5: 会话列表 + Bot 管理

**目标**：实现会话列表页和 Bot CRUD。

**文件**：
- `src/stores/conversations.js` — 会话列表 store
- `src/stores/bots.js` — Bot 管理 store
- `src/views/ConversationsView.vue` — 会话列表
- `src/views/BotsView.vue` — Bot 列表
- `src/views/BotFormView.vue` — Bot 创建/编辑表单
- `src/components/ConversationItem.vue` — 会话列表项
- `src/components/BotStatusBadge.vue` — Bot 状态 emoji
- `src/components/NavBar.vue` — 底部导航栏

**API 对接**：
```
GET    /api/v1/conversations
GET    /api/v1/bots
POST   /api/v1/bots              { name, summary, traits, gender, ... }
GET    /api/v1/bots/{id}
PATCH  /api/v1/bots/{id}         { speaking_style_adds, trait_adds, ... }
```

**会话列表项显示**：
- Bot 名称 + 状态 emoji
- 最后一条消息预览
- 未读数角标
- 主动消息标记

**验证**：Chrome DevTools 创建 Bot → 确认会话自动生成 → 列表显示正确。

---

## Step 6: 聊天页核心

**目标**：实现聊天页，支持消息收发、多句回复、Bot 状态。

**文件**：
- `src/stores/chat.js` — 聊天状态 store（action-based reducer 模式）
- `src/views/ChatView.vue` — 聊天页主视图
- `src/components/MessageBubble.vue` — 消息气泡（用户/Bot 区分）
- `src/components/MessageInput.vue` — 输入框 + 发送按钮
- `src/components/TypingIndicator.vue` — "正在输入" 动画

**Chat Store 状态**：
```js
{
  conversationId: '',
  botName: '',
  botStatus: { emoji: '', text: '' },
  messages: [],          // { id, senderType, content, timestamp, status, replyGroupId, sequence }
  inputText: '',
  isRemoteTyping: false,
  isLoading: false,
  activeReplyGroups: {}, // replyGroupId → { count, received }
}
```

**关键行为**：
1. 乐观更新：发送消息立即显示（status=sending），收到 ack 后更新为 sent
2. 多句回复：`multi_reply_start` → 设置 typing → 逐条 `message` 追加 → `multi_reply_end` 清除 typing
3. HTTP 降级：WebSocket 发送失败时 fallback 到 `POST /conversations/{id}/send`
4. 自动滚动：新消息到达时滚动到底部
5. 历史加载：进入聊天页时 `GET /conversations/{id}/messages?limit=50`

**验证**：Chrome DevTools 发送 5+ 轮消息，验证多句回复逐条显示。

---

## Step 7: 主动消息 + 设置页

**目标**：实现主动消息接收显示、设置页面（通知频率、免打扰）。

**文件**：
- `src/stores/settings.js` — 用户设置 store
- `src/views/SettingsView.vue` — 设置页面

**API 对接**：
```
GET /api/v1/users/notification-settings
PUT /api/v1/users/notification-settings  { proactive_enabled, dnd_start, dnd_end, frequency }
GET /api/v1/users/status
PUT /api/v1/users/status                 { emoji, text }
```

**主动消息处理**：
- WebSocket 收到 `proactive_message` → 追加到对应会话消息列表
- 更新会话列表的 `lastMessage` + `unreadCount`
- 如果用户不在该会话页，显示通知提示

**设置页内容**：
- 主动消息开关
- 频率选择：高（2h）/ 正常（4h）/ 低（8h）
- 免打扰时段：开始时间 + 结束时间
- 用户状态：emoji + 文字

**验证**：Chrome DevTools 修改设置 → 等待主动消息触发 → 验证显示。

---

## Step 8: 一键部署脚本

**目标**：编写 `deploy.sh` + Docker Compose 配置，支持裸 Linux 服务器一键部署。

**文件**：
- `deploy/deploy.sh` — 主部署脚本
- `deploy/docker-compose.yml` — 三服务编排
- `deploy/Dockerfile.frontend` — Vue 前端 multi-stage build
- `deploy/Dockerfile.backend` — Go 后端构建
- `deploy/nginx.conf.template` — Nginx 配置模板（envsubst 替换变量）
- `deploy/.env.template` — 环境变量模板

**deploy.sh 流程**：
```bash
1. 检测 /etc/os-release → 选择包管理器（apt/dnf/yum/pacman/apk）
2. 安装 Docker + Docker Compose（如未安装）
3. 交互收集：域名、API Key、API Base URL、是否 SSL、主动消息间隔
4. 从模板生成 .env 和 nginx.conf
5. git clone 仓库（含 submodule）
6. docker compose build && docker compose up -d
7. 等待健康检查通过
8. 输出访问地址
```

**验证**：脚本语法检查 + 本地 Docker Compose 构建测试。

---

## Step 9: Chrome DevTools E2E 验证

**目标**：使用 chrome-devtools MCP 工具完成全流程自动化验证。

**验证步骤**：

| # | 步骤 | 验证点 |
|---|------|--------|
| 1 | 注册 | 输入手机号 → 发送验证码 → 填入验证码 → 注册成功 → 跳转会话列表 |
| 2 | 创建 Bot | 进入 Bot 列表 → 创建 Bot（名称/简介/性格）→ 会话自动生成 |
| 3 | 多轮对话 | 发送 5+ 轮消息 → 验证消息显示 → 历史消息滚动 |
| 4 | 消息切分 | 发送触发长回复的消息 → 验证多条消息逐条显示（multi_reply） |
| 5 | 消息聚合 | 快速连续发 3 条短消息 → 验证前端即时显示 → Bot 回复合并内容 |
| 6 | 主动消息 | 停止发送 → 等待配置间隔 → 验证 proactive_message 显示 |
| 7 | 设置 | 修改主动消息频率 → 修改免打扰时段 → 验证保存成功 |

**每步检查**：
- 截图对比
- Network 面板 WebSocket 帧检查
- Console 无报错

---

## 依赖关系

```
Step 1 (脚手架)
  └→ Step 2 (API + Auth Store)
       └→ Step 3 (登录页)
       └→ Step 4 (WebSocket)
            └→ Step 5 (会话列表 + Bot)
                 └→ Step 6 (聊天页)
                      └→ Step 7 (主动消息 + 设置)
Step 8 (部署脚本) — 可与 Step 3-7 并行
Step 9 (E2E 验证) — 依赖 Step 1-7 全部完成
```
