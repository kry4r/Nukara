# AgentX / OpenAI 兼容与本地部署修复设计

> **状态：** 已与用户确认，按此设计实施

## 背景

当前项目存在四类相关问题：

1. `deploy/deploy-local.sh` 在服务端重复部署时会出现“部署出旧版前端”的现象。
2. AgentX 适配状态不透明，部署后网页端对话可能表现为“无回应”。
3. 需要兼容 OpenAI 官方两套接口：`/v1/chat/completions` 与 `/v1/responses`，并允许在 provider 级别显式选择。
4. 需要对网页端代码做走查，识别当前高风险点与明显错误点。

## 已确认目标

- `deploy/deploy-local.sh` 必须以**当前服务器仓库源码**为部署来源，不能静默回退到旧快照。
- 管理端 provider 需要支持选择 API 模式：
  - `chat_completions`
  - `responses`
  - `auto`
- AgentX runtime 与管理端 provider 测试必须共用同一套 provider client/factory，避免“测试能通、真实聊天不通”。
- 网页端在 provider/runtime 出错时不能只表现为“无回应”，需要有明确错误反馈。

## 根因分析

### 1. 本地部署脚本复用旧前端/旧源码的高风险点

`deploy/deploy-local.sh` 会先清空 `/opt/nukara`，然后再执行 `prepare_sources()`。

如果脚本本身就是从 `/opt/nukara/deploy/deploy-local.sh` 运行，则：

- 清理步骤可能删除当前 repo 工作副本；
- 后续源码同步找不到本地 workspace；
- 脚本退回到 `git clone https://github.com/kry4r/Nukara.git` 的临时快照；
- 最终构建的前后端并不一定来自当前服务器 repo 的当前 commit。

这会直接导致“已 clone / 已 pull，但部署出来仍是旧前端”的现象。

### 2. AgentX 无回应的高风险点

当前 runtime/provider 路径存在以下问题：

- runtime 与 admin chat-test 不是完全统一的协议选择层；
- 当前 `OpenAICompatClient` 仅支持 `chat/completions`，不支持 `responses`；
- provider 解析失败或无有效 client 时，用户侧可能只看到“没有回复”，而非明确错误；
- 默认 provider 自举不是完全幂等，重复部署时可能失败，导致新环境 provider 状态不完整。

### 3. provider schema 演进不一致

运行时代码已经按 `providers.id TEXT` 使用 provider slug，但旧 migration `003_create_provider_tables.sql` 里仍保留了 `UUID` 风格 provider 结构，存在新老环境 schema 语义漂移风险。

## 推荐方案

采用“统一适配层 + 部署源修复”的方案：

### A. 部署修复

- 明确“源码根目录”与“安装目录”分离；
- 在清理 `/opt/nukara` 前，先锁定当前部署源；
- 如果当前源码位于 `/opt/nukara` 内，则先复制到 `/tmp` 临时快照，再从该快照同步到安装目录；
- 禁止在本地部署脚本中静默回退到远程 `git clone`，避免部署源漂移；
- 输出部署源路径、commit、构建产物关键信息，便于排查。

### B. Provider API 模式

在 provider 记录中新增字段：`api_mode`。

允许值：

- `chat_completions`
- `responses`
- `auto`

默认值建议为 `chat_completions`，原因：

- 与现有 provider 行为最接近；
- 避免老环境升级后因默认切到 `responses` 导致兼容性回归；
- `auto` 仅用于需要兼容不明供应商行为的场景，不应默认掩盖配置问题。

### C. 统一 OpenAI 兼容客户端

对 `internal/agentx/llm/openai_compat.go` 进行扩展，形成单一兼容层：

- `chat_completions` → 请求 `/chat/completions`
- `responses` → 请求 `/responses`
- `auto` → 按配置优先模式发起；若返回明确不支持错误，再尝试另一种模式，并记录日志

统一支持：

- JSON 返回
- SSE streaming 返回
- 输出文本抽取

### D. 统一 runtime 与管理端测试路径

以下能力必须共用同一套工厂逻辑：

- AgentX runtime
- 管理端 provider `test`
- 管理端 provider `chat-test`

这样可以避免：

- 管理端测通，用户聊天不通；
- 某模式只在部分入口生效。

### E. 错误可见性

网页端需要明确处理 websocket `error` 事件和 runtime/provider 错误：

- 聊天页显示错误提示；
- 消息发送失败时给出可见状态；
- 避免“静默无响应”。

### F. Provider bootstrap 幂等化

`deploy/lib/admin-bootstrap.sh` 改为：

- 如果 provider 已存在，则更新其配置并执行 switch；
- 如果不存在，则创建后 switch；
- 重复部署不会因为 `Provider already exists` 直接失败。

## 数据与接口设计

### Provider 数据结构新增字段

后端 `Provider` 新增：

- `api_mode string`

管理端请求/响应同步包含该字段。

### 数据迁移

新增 migration：

1. 为 `providers` 增加 `api_mode TEXT NOT NULL DEFAULT 'chat_completions'`
2. 统一 provider 相关 schema 与当前运行时代码的 TEXT 语义（必要时补兼容迁移）

## 前端网页端走查结论

### `Nukara_Web/src/composables/useWebSocket.js`

风险：

- 未统一处理后端 `error` 事件；
- 连接错误与 runtime 错误未映射到 UI。

### `Nukara_Web/src/stores/chat.js`

风险：

- HTTP fallback 失败仅标记消息 `failed`，但 websocket 路径没有明确用户提示；
- 用户容易误判为 bot 没回应，而不是后端/provider 故障。

### `Nukara_Web/src/composables/useApi.js`

风险：

- 默认 `res.json()`，对非 JSON 错误响应不够稳健；
- 错误信息抽取不足。

### `Nukara_Admin_Web/src/api/admin.js`

风险：

- 管理员 Basic Auth 长期存储在 `localStorage`；
- 这是明显安全风险，但不作为本次主线改动，只记录为后续优化项。

## 影响范围

### 后端

- `Nukara_Backend/internal/agentx/llm/*`
- `Nukara_Backend/internal/agentx/runtime.go`
- `Nukara_Backend/internal/agentx/provider/*`
- `Nukara_Backend/internal/admin/provider_handler.go`
- `Nukara_Backend/internal/admin/provider_chat_test_handler.go`
- `Nukara_Backend/internal/store/*`
- `Nukara_Backend/migrations/*`

### 管理端

- `Nukara_Admin_Web/src/api/admin.js`
- `Nukara_Admin_Web/src/App.vue`
- 相关样式

### 网页端

- `Nukara_Web/src/composables/useWebSocket.js`
- `Nukara_Web/src/stores/chat.js`
- `Nukara_Web/src/composables/useApi.js`
- 聊天页面相关可见性逻辑

### 部署

- `deploy/deploy-local.sh`
- `deploy/lib/admin-bootstrap.sh`
- 相关验证脚本与文档

## 验收标准

- 从服务端当前 repo 重新执行 `deploy/deploy-local.sh` 后，网页端确实变成当前源码版本。
- provider 可在管理端显式选择 `chat_completions` / `responses` / `auto`。
- AgentX 对话在两种 OpenAI 接口模式下均可正常响应。
- 管理端 provider 测试与真实对话使用一致的协议选择逻辑。
- provider/runtime 出错时，网页端能够显示明确错误提示。
- 重复部署不会因默认 provider 已存在而失败。
- provider schema 不再存在明显的 UUID/TEXT 双轨漂移风险。
