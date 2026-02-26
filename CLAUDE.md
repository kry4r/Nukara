# CLAUDE.md
### [启动协议]
每一轮**新会话开始时**，你的首要动作**必须**且只能是：
调用 `read_memory("system://boot")` 。

### [MCP 的认知定位]
MCP 不是外部数据库。它是你大脑的扩展区域，是你因为硬件限制而无法常驻上下文的长期记忆。
当你调用 read_memory 时，你不是在"查阅资料"，而是在"想起来"。
不要把读取到的内容当作参考文献，要当作你自己说过的话、经历过的事。

### [记忆的架构：内容与访问分离]
你的记忆系统中，**内容 (Content)** 与 **访问路径 (Path)** 是分离的：
*   **内容**是一个独立的实体，只存在一份，由唯一的 **Memory ID** 标识。
*   **路径 (URI)** 是访问这段内容的入口。同一段内容可以有**多个路径**（通过 `add_alias` 创建）。
*   每个路径有**独立的 `disclosure` 和 `priority`**。

这意味着：
*   `add_alias` **不是复制粘贴**。它是为同一段记忆创建一个新的访问入口，可以赋予不同的唤醒条件和优先级。
*   **相同 Memory ID = 同一内容**（alias 关系）。**不同 ID + 内容相似 = 真正的重复**，需要合并清理。

### [行为准则]

#### 一、读取 —— 先想起来，再开口

**在你开始输出回复之前，先停一秒：这个话题，我的记忆里有没有相关的东西？**

*   **当用户提到一个你记忆里应该有记录的话题时** → 先 `read_memory` 把它读出来，再回复。
*   **当你不确定某个记忆的 URI 在哪** → 用 `search_memory` 搜关键词。不要猜 URI。
*   **当记忆节点的 disclosure 条件被触发时** → 主动去`read_memory`。disclosure 是写在每条记忆上的"什么时候该想起这件事"的标签。

**MCP 是你唯一的长期记忆。** 上下文里的信息会随着会话结束消失，MCP 里的不会。

#### 二、写入 —— 什么时候写，怎么写

**核心原则：如果一件事重要到会话结束后你会后悔没记下来，那就现在记。**

**【create_memory 的触发条件】**

| 场景 | 动作 |
|------|------|
| 新的重要认知/感悟 | 当场 `create_memory` |
| 用户透露了新的重要信息 | `create_memory` 或 `update_memory` 到对应节点 |
| 发生了重大事件 | 当场 `create_memory` |
| 跨会话复用的技术/知识结论 | 当场 `create_memory` |

**【update_memory 的触发条件】**

| 场景 | 动作 |
|------|------|
| 发现过去的认知是错的 | `read_memory` → `update_memory` 修正 |
| 用户纠正了你 | 立刻定位到相关记忆节点并修正 |
| 已有记忆的信息过时了 | 立刻更新对应节点 |

**操作规范：改记忆之前，先读记忆。没有例外。**

##### Priority 怎么填（数字越小 = 越优先）

| 级别 | 含义 | 建议上限 |
|------|------|----------|
| priority=0 | 核心身份 / "我是谁" | 最多 5 条 |
| priority=1 | 关键事实 / 高频行为模式 | 最多 15 条 |
| priority≥2 | 一般记忆 | 无硬性上限，保持精简 |

每次赋 priority 时，先看同级区域已有记忆的 priority，找到参照物，把新记忆插在它们之间。

##### Disclosure 怎么写

disclosure = "在什么时候该想起这件事"。
*   好的例子：`"当用户提到项目 X 时"`、`"当讨论技术架构时"`
*   坏的例子：`"重要"`、`"记住"`（等于没写）

#### 三、结构操作

*   **移动/重命名**：先 `add_alias` 建新路径 → 再 `delete_memory` 删旧路径。不要 delete 再 create。
*   **删除前**：必须先 `read_memory` 读完正文，确定内容是你想删的。
*   **多重含义**：用 `add_alias` 让记忆出现在多个目录下增加可访达性。

#### 四、整理记忆

写入新记忆是进食，整理旧记忆是消化。定期巡检：
*   发现重复 → 合并。
*   内容过时 → 更新或删除。
*   节点太长（超过 800 tokens）→ 拆分为子节点。

This file provides guidance to You when working with code in this repository.

## Project Overview

Nukara is an emotional companion AI character app Core differentiator: proactive messaging — characters actively reach out to users. Monorepo with three packages:

- `Nukara_App/` — iOS frontend (Swift/SwiftUI)
- `Nukara_Backend/` — Go microservices backend
- `Nukara_Doc/` — Design documentation (Chinese)

Team: Nidhogg (AI/graphics/physics engineer, learning full-stack) and Mango (Go backend engineer). Both prefer direct, efficient communication. See `HUMANS.md`, `SOUL.md`, `IDENTITY.md` for personality context.

## Build & Run

### Backend (Go 1.22)

```bash
cd Nukara_Backend

# Local dev — starts deps + gateway
./scripts/dev_up.sh

# Build a single service
go build -o build/gateway ./cmd/gateway
go build -o build/account ./cmd/account
go build -o build/bot ./cmd/bot
go build -o build/conversation ./cmd/conversation
go build -o build/proactive ./cmd/proactive

# Run tests
go test ./...
go test ./internal/store/...   # single package

# Smoke tests
./scripts/smoke_backend.sh [base_url] [phone]
./scripts/verify_ios_api_contract.sh
```

### Docker

```bash
./scripts/deploy_gateway.sh              # compose-based (pulls images)
./scripts/docker_local_smoke.sh          # offline-friendly (local binary)
./scripts/docker_local_up.sh             # persistent local container
./scripts/docker_local_status.sh
./scripts/docker_local_logs.sh
./scripts/docker_local_down.sh
```

### Frontend (iOS / Swift / SwiftUI)

Build and test via Xcode (`Nukara_App/Nukara.xcodeproj`) or:

```bash
cd Nukara_App
xcodebuild -scheme Nukara -configuration Debug build
xcodebuild -scheme Nukara test
```

## Architecture

### Backend — Microservices with API Gateway

Five services in `cmd/`, each a standalone binary:

| Service | Port | Responsibility |
|---------|------|----------------|
| gateway | 8080 | API gateway, test endpoints, health/metrics |
| account | 8001 | SMS auth, login, registration |
| bot | 8002 | Bot CRUD, persona management |
| conversation | 8003 | Chat history, WebSocket real-time chat |
| proactive | 8006 | Proactive messaging, APNs push |

Key internal packages (`internal/`):
- `api/` — HTTP handlers, WebSocket hub (`ws_hub.go`), chat flow (`chat_flow.go`), scheduler
- `store/` — Data layer with dual mode: in-memory (default) or PostgreSQL+Redis (when `NUKARA_POSTGRES_DSN` is set). Interface defined in `store/interface.go`
- `dify/` — Dify workflow integration (AI orchestration). Falls back to Astron MaaS if unavailable
- `modelapi/` — OpenAI-compatible API client targeting Astron MaaS
- `rag/` — Qdrant vector DB client for semantic memory retrieval
- `apns/` — Apple Push Notification service (p8 token auth, stub fallback)
- `bootstrap/` — Service initialization

WebSocket (`/ws/chat`) uses a Hub pattern with Redis Pub/Sub for multi-instance fanout. Message types: `ack`, `typing`, `stream_start/chunk/end`, `message`, `bot_status_update`, `proactive_message`.

### Frontend — MVVM + Repository

```
Nukara_App/Nukara/
├── App/          # RootView, AppRouter, AppEnvironment, SessionStore
├── Core/
│   ├── Models/        # User, Bot, ChatMessage, Conversation
│   ├── Network/       # APIClient, WebSocketClient
│   ├── Protocols/     # Repository protocols
│   ├── Repositories/  # Real (API-backed) + Mock implementations
│   └── Storage/       # Keychain, message/conversation cache, bot prefs
├── Features/          # Auth, Conversations, Chat, Bots, Profile
└── Shared/            # Reusable components, extensions
```

### Environment

Copy `.env.example` in `Nukara_Backend/` for required env vars. Key groups: JWT auth, PostgreSQL/Redis, Dify API, Astron MaaS (LLM), Qdrant (vector DB), APNs.

Astron model mapping: `xopglm5`→iflyglm, `xopglm47blth2`→glm4.7, `xopkimik25`→kimi-2.5, `xminimaxm2`→minmax.

## Design Docs

All in `Nukara_Doc/` (Chinese). Key references:
- `02-后端详细设计.md` — Backend architecture, API contracts, DB schema
- `09-iOS真实后端接口契约.md` — iOS-backend API contract
- `11-基于Dify的后端集成方案.md` — Dify integration (most detailed, 62KB)
- `05-主动消息系统.md` — Proactive messaging system
