# Nukara Web Frontend 设计文档

> 日期：2026-02-26
> 状态：已批准

## 1. 概述

为 Nukara 情感陪伴 AI 应用构建 Web 前端，功能对标 iOS 原生 App，支持一键部署到裸 Linux 服务器。

**技术栈**：Vue 3 + Vite + Composition API
**部署方式**：Nginx 反代 + Docker Compose 编排

## 2. 整体架构

```
┌──────────────────────────────────────────────┐
│                Linux Server                   │
│  ┌──────────┐   ┌──────────┐   ┌───────────┐│
│  │  Nginx   │──▶│ Backend  │──▶│  Nanobot  ││
│  │ :80/:443 │   │  :8080   │   │   :9091   ││
│  └────┬─────┘   └──────────┘   └───────────┘│
│       │                                       │
│  ┌────┴─────┐                                │
│  │ Vue SPA  │  /var/www/nukara/dist/         │
│  └──────────┘                                │
└──────────────────────────────────────────────┘
```

路由规则：
- `/api/*` → 反代 Go backend :8080
- `/ws/*` → WebSocket 反代 Go backend :8080
- `/*` → 静态文件（SPA fallback index.html）

## 3. 前端功能设计

### 3.1 页面结构

参照 iOS App 的页面结构：

| 页面 | 路由 | 功能 |
|------|------|------|
| 登录/注册 | `/auth` | 手机号 + 短信验证码，登录/注册切换 |
| 会话列表 | `/` | 所有会话，显示最后一条消息、未读数、Bot 状态 |
| 聊天 | `/chat/:convId` | 消息流、输入框、Bot 状态指示器 |
| Bot 列表 | `/bots` | Bot 卡片列表 + 创建入口 |
| Bot 创建/编辑 | `/bots/new` `/bots/:id/edit` | 名称、简介、性格、说话风格 |
| 个人设置 | `/settings` | 通知设置、主动消息频率、免打扰时段 |

### 3.2 WebSocket 协议对接

连接端点：`/ws/chat?token={accessToken}`

接收事件类型：

```
ack              — 消息确认
typing           — 正在输入指示
multi_reply_start — 多句回复开始（含 reply_group_id）
message          — 单条消息（含 reply_group_id + sequence）
multi_reply_end  — 多句回复结束
bot_status_update — Bot 情绪状态变化
proactive_message — 主动消息推送
ping             — 心跳
```

发送消息格式：
```json
{
  "type": "message",
  "conversation_id": "xxx",
  "content": { "type": "text", "text": "..." }
}
```

### 3.3 核心交互机制

**多句回复（句子切分）**：
- 收到 `multi_reply_start` 后创建消息组容器
- 每条 `message`（同 `reply_group_id`）逐条追加显示，带打字动画
- 收到 `multi_reply_end` 标记组完成

**用户消息聚合**：
- 用户快速连续发送多条消息时，后端 aggregator 会在 100-500ms 窗口内合并
- 前端即时显示每条用户消息（不做前端合并），后端负责聚合后统一处理

**主动消息**：
- 通过 WebSocket 接收 `proactive_message` 事件
- 前端设置页可配置：主动消息开关、频率（高/正常/低）、免打扰时段
- 默认：3 分钟无回复触发主动消息（可通过后端 API 配置）

**Bot 状态**：
- 收到 `bot_status_update` 更新 Bot 情绪状态（happy/excited/love/sad/angry/anxious/gentle）
- 在聊天页和会话列表显示对应 emoji

**心跳与重连**：
- 25 秒发送一次 ping
- 断线后指数退避重连（2^n 秒，上限 30 秒，最多 8 次）

## 4. 一键部署脚本

### 4.1 设计目标

一个 shell 脚本 `deploy.sh`，在裸 Linux 服务器上完成全部部署，交互式收集配置。

### 4.2 支持的系统

| 系统 | 包管理器 |
|------|----------|
| Ubuntu / Debian | apt |
| CentOS / RHEL / Fedora | dnf / yum |
| Arch Linux | pacman |
| Alpine | apk |

脚本自动检测 `/etc/os-release` 选择对应包管理器。

### 4.3 交互式配置

脚本运行时交互收集：
- 域名（用于 Nginx server_name 和可选 SSL）
- Astron API Key（写入 `.env`）
- Astron API Base URL
- 是否启用 SSL（自动申请 Let's Encrypt）
- 主动消息间隔（默认 180 秒）

### 4.4 部署流程

```
1. 检测系统 → 安装 Docker + Docker Compose
2. 交互收集配置 → 生成 .env 和 nginx.conf
3. 构建前端 dist（Docker multi-stage build）
4. docker compose up -d（nginx + backend + nanobot）
5. 健康检查 → 输出访问地址
```

### 4.5 Docker Compose 编排

三个服务：

| 服务 | 镜像 | 端口 |
|------|------|------|
| nginx | nginx:alpine | 80, 443 |
| backend | 本地构建 Go binary | 8080（内部） |
| nanobot | 本地构建 | 9091（内部） |

前端 dist 通过 volume 挂载到 nginx 容器。

## 5. Chrome DevTools E2E 验证计划

使用 chrome-devtools MCP 工具对完整用户流程进行自动化验证。

### 5.1 验证流程

```
Step 1: 注册
  → 输入手机号 → 发送验证码 → 填入验证码 → 注册成功 → 自动跳转会话列表

Step 2: 创建 Bot
  → 进入 Bot 列表 → 点击创建 → 填写名称/简介/性格 → 提交 → 自动创建会话

Step 3: 多轮对话（5+ 轮）
  → 发送消息 → 等待 Bot 回复 → 验证消息显示
  → 重复多轮，验证历史消息滚动

Step 4: 消息切分验证
  → 发送触发长回复的消息 → 验证 multi_reply_start/message/multi_reply_end
  → 确认多条消息逐条显示

Step 5: 用户消息聚合验证
  → 快速连续发送 3 条短消息 → 验证前端即时显示
  → 验证 Bot 回复是对合并后内容的响应

Step 6: 主动消息验证
  → 停止发送消息 → 等待配置的间隔时间
  → 验证收到 proactive_message 事件并显示

Step 7: 设置验证
  → 进入设置页 → 修改主动消息频率 → 修改免打扰时段
  → 验证设置保存成功
```

### 5.2 验证要点

- 每步截图对比
- 检查 WebSocket 连接状态（Network 面板）
- 检查 Console 无报错
- 验证消息顺序和内容正确性
