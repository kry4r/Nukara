# Auth / Memory / Admin Bugs Design

**Date:** 2026-03-08

## Goal

一次性修复三个线上问题：

1. 邮箱注册阶段重复发送验证码。
2. AgentX 对话链路中 compact / temporal memory 使用了错误的 ID 语义，导致 UUID 解析错误、对话失败、记忆图无节点。
3. Admin 侧 provider 管理入口信息密度过低，常规 provider 测试入口缺失，需要改成更紧凑的卡牌 + expander 结构。

## Confirmed Root Causes

### 1. 对话与记忆 ID 语义混淆

当前代码仍残留旧的 `NanobotConvID` / `nukara:user:bot:conv` 复合会话 ID 习惯。部分 AgentX 运行时入口把这个 provider-facing 会话 ID 直接传给 runtime context，而 runtime context 又会据此读取：

- conversation compact
- temporal memory recall
- activation trace / session summary 相关节点

这会把本地 conversation id 与 provider 会话 id 混为一谈。

### 2. Temporal memory schema 仍把多处业务字符串 ID 建成 UUID

当前 temporal memory / compact schema 把以下字段建成 UUID，但代码里实际写入的是业务字符串：

- `conversation_compacts.conversation_id`
- `memory_nodes.session_id`
- `memory_nodes.source_turn_id`
- `activation_traces.conversation_id`
- `activation_traces.turn_id`

同时 memory node id 代码允许 materialized id / `session-summary:*` 等字符串节点 ID，因此 DB schema 与代码模型不一致。

### 3. Admin 图谱仍主要展示旧 memory_items 视角

新的 temporal memory graph 已是主系统，但 admin 图谱仍主要围绕旧 `memory_items` 结构构图，所以新机器人在只产生 temporal memory node 时会看起来像“空图”。

### 4. 邮件重复发送

重复发送问题优先按“后端幂等 / 短冷却”修复：即使前端重复触发，也只允许短时间内产生一封有效验证码邮件，避免用户收到双邮件。

## Design Decisions

### A. 拆分两类会话 ID

- `localConversationID`：Nukara 本地会话 ID，仅用于 store / compact / recall / trace。
- `providerConversationID`：给 provider / legacy agent client 用的会话 ID。

AgentX runtime 上下文、compact、memory recall 一律只使用 `localConversationID`。

### B. 统一 temporal memory / compact 为 TEXT 语义

把上述错误建模为 UUID 的业务字段改为 `TEXT`，并增加兼容迁移：

- 新部署直接建正确类型。
- 已部署实例通过 migration 将旧列转为 `TEXT`。

### C. Admin memory graph 切到 temporal memory graph 主数据源

Admin 图谱改为：

- 节点：`memory_nodes`
- 边：`memory_edges`
- 辅助侧栏：recent impressions / persona changes / runtime state

旧 `memory_items` 仅作为兼容信息，不再作为主图来源。

### D. 邮件发送增加短冷却 / 幂等保护

在 `auth/email/send` 链路增加基于 `(email, purpose)` 的短时间发送保护，避免双击 / 重试 / 双端重复触发时重复发信。

### E. Admin provider 页面压缩为卡牌 + expander

左侧 provider 区域改为：

- 每个 provider 一张紧凑卡牌
- 头部展示名称、状态、模式、模型、用户数
- 展开后展示编辑表单、切换模式、连接测试、chat 测试等

这样恢复常规 provider 测试能力，同时减少纵向占用。

## Validation

需要验证：

1. 注册连续点击发送验证码，不会收到双邮件。
2. WS / HTTP 聊天链路不再因 compact / session-summary UUID 错误失败。
3. 新机器人完成一轮对话后，admin memory graph 能看到 temporal nodes / edges。
4. Admin provider 页面仍可：查看、编辑、切换模式、执行测试。
5. 前端构建与后端相关测试通过。
