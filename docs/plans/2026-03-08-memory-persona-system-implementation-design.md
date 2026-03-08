# 记忆与人设系统实现设计文档

**关联文档**：`docs/plans/2026-03-08-memory-persona-system-redesign.md`
**创建时间**：2026-03-08
**状态**：已定稿，可进入实施
**兼容策略**：不保留旧的人设待确认主流程；以新 pipeline 替换旧编排

## 一、目标与范围

本设计将当前“记忆提取 + 人设更迭”链路重构为一条新的 post-turn pipeline：

1. 从一轮对话中提取候选事实、实体、关系。
2. 通过相似度和图入库规则，把候选事实写入 temporal memory graph。
3. 将 persona 视为“由稳定事实派生出的精简档案”，而不是与记忆混合维护的文本堆积。
4. 去掉 `pending_confirm` 产品主流程，人设变更自动应用并写审计记录。
5. 在 runtime recall 时利用关系图谱生成视角转换提示，解决“叔叔阿姨 / 小蜜”类身份错位问题。

本次实现同时覆盖后端与 Web 端中依赖旧待确认流程的界面。

## 二、最终架构决策

采用“新编排层 + 复用现有基础设施”的方案：

- **重写行为编排**：替换 `internal/agentx/subtasks/runner.go` 中旧的 `turnCount%3` 触发、人设迭代、`pending_confirm` 分支。
- **复用底层能力**：继续使用已有 `temporal_memory_graph` store、`internal/agentx/memorygraph` recall/card 基础、`persona_change_events` 审计表。
- **事实与人设分层**：记忆图是事实唯一来源，persona 是稳定事实的派生视图。
- **主链路去确认化**：Web 不再依赖 `POST /api/v1/bots/{botID}/persona-changes/{id}/accept|reject` 完成产品功能。

## 三、模块边界

### 3.1 Extraction Layer

职责：从 turn 中提取候选记忆、实体和关系，不直接决定是否更新 persona。

涉及模块：

- `Nukara_Backend/internal/agentx/subtasks/memory_extract.go`
- `Nukara_Backend/internal/api/subtasks_runtime.go`

输出建议统一为中间结构 `CandidateMemory`：

```go
type CandidateMemory struct {
	Kind             string
	Owner            string
	Content          string
	Importance       int
	OccurredAt       time.Time
	SemanticCategory string
	Stability        string
	Topics           []string
	Entities         []store.Entity
	Relations        []store.Relation
	EvidenceTurnID   string
}
```

### 3.2 Memory Graph Layer

职责：去重 / 合并 / 新建、物化 `session_summary` 和事实节点、创建关系边。

涉及模块：

- `Nukara_Backend/internal/agentx/memorygraph/similarity.go`（新增）
- `Nukara_Backend/internal/agentx/memorygraph/ingest.go`
- `Nukara_Backend/internal/agentx/memorygraph/service.go`
- `Nukara_Backend/internal/store/temporal_memory_graph.go`

这一层负责把候选事实转成可召回的图节点，并保留来源与合并证据。

### 3.3 Persona Decision Layer

职责：仅根据“稳定的、核心的、未被现有人设覆盖的事实”产出 persona 更新决策。

涉及模块：

- `Nukara_Backend/internal/agentx/subtasks/persona_decision.go`（新增）
- `Nukara_Backend/internal/agentx/subtasks/runner.go`

输出结构：

```go
type PersonaDecision struct {
	ShouldUpdatePersona bool
	TargetField         string
	Reason              string
	Risk                string
	SourceNodeIDs       []string
}
```

### 3.4 Audit / Application Layer

职责：自动应用 persona patch，并记录 `accepted / skipped / failed` 审计事件。

涉及模块：

- `Nukara_Backend/internal/store/agentx_data.go`
- `Nukara_Backend/internal/api/bot_profile.go`
- `Nukara_Backend/internal/api/server.go`

`persona_change_events` 保留，但不再作为待确认任务队列使用。

### 3.5 Runtime Recall Layer

职责：利用图关系生成精简 prompt cards 与视角转换提示。

涉及模块：

- `Nukara_Backend/internal/api/runtime_context.go`
- `Nukara_Backend/internal/agentx/memorygraph/recall.go`

## 四、数据模型调整

### 4.1 `store.MemoryItem`

保留旧存储以兼容现有辅助逻辑，但新增图相关字段，供迁移期与 runtime portrait 复用：

```go
type Entity struct {
	ID   string
	Type string
	Name string
	Role string
}

type Relation struct {
	SourceEntityID string
	TargetEntityID string
	RelationType   string
	RoleHint       string
}
```

`MemoryItem` 增加：

- `SemanticCategory string`
- `Stability string`
- `Entities []Entity`
- `Relations []Relation`

### 4.2 `TemporalMemoryNode`

在现有节点字段基础上增加：

- `SourceTurnID string`
- `SourceKind string`
- `SemanticCategory string`
- `StabilityLabel string`
- `MergeKey string`
- `EvidenceCount int`
- `Entities []Entity`

节点类型统一收敛为：

- `episode`
- `user_fact`
- `state_snapshot`
- `promise`
- `session_summary`
- `habit`

### 4.3 `TemporalMemoryEdge`

边类型收敛为：

- `summarizes`
- `follows`
- `relates_to`
- `family_of`
- `owns`
- `lives_with`
- `supported_by`

### 4.4 `PersonaChangeEvent`

状态统一为：

- `accepted`：已自动应用
- `skipped`：已评估但无需改 persona
- `failed`：应更新但应用失败

不再将 `pending` 作为主流程状态。

## 五、核心执行流程

一条 turn 的固定执行顺序如下：

1. **提取候选**：从用户消息 + 机器人回复中得到 `CandidateMemory[]`。
2. **低价值过滤**：仅过滤无信息量寒暄与语气词，不再对 `self_fact/user_fact` 做硬关键词门控。
3. **相似度决策**：对候选与已有图节点做 embedding + lexical 混合比对，决定 `create/merge/supersede`。
4. **图入库**：创建事实节点与关系边。
5. **摘要物化**：把 `compact_update` 的 `summary + facts[]` 物化为 `session_summary + fact nodes + summarizes edges`。
6. **persona 决策**：仅对稳定核心事实进行字段映射与相似度判断。
7. **自动应用**：直接执行 `ApplyBotPersonaPatch`，并写入 `persona_change_events` 审计。
8. **runtime 提示**：在后续召回中注入关系视角提示，而不是平铺原始称谓。

## 六、Persona 决策规则

### 6.1 基础映射

- `self_fact`
  - 当 `stability=stable` 且语义分类属于 `identity / personality / life_context` 时，允许进入 persona 判断。
- `habit`
  - 单次出现默认只入图；重复出现并经 consolidate 升级后，才允许更新 `expression_style` 或 `life_context`。
- `user_fact`
  - 默认只描述“机器人对用户的认识”；仅在明确边界/禁忌场景下映射到 `taboos_and_preferences`。
- `event / promise / state_snapshot`
  - 默认只入图，不直接更新 persona。

### 6.2 拒绝更新条件

出现以下任一条件则输出 `skipped`：

- 存在时间限定词，如“最近 / 暂时 / 这周 / 刚刚 / 今天”。
- 与目标 persona 字段相似度 > 0.7。
- 信息只描述短期事件、短期心情、临时安排。
- 文本无法稳定归类到 persona 五大字段之一。

### 6.3 自动应用规则

- 不再走 `pending_confirm`。
- `risk` 仅用于日志、管理面板和审计展示，不影响是否自动应用。

## 七、关系图谱与视角转换

实体抽取至少覆盖：

- `person`
- `pet`
- `place`
- `organization`

关系抽取至少覆盖：

- 家庭关系
- 所有权关系
- 居住关系
- 共同活动关系

在 runtime system prompt 中新增一段关系提示，例如：

```text
[关系上下文]
- 用户提到的“叔叔阿姨”是指你的父母
- 用户提到的“小蜜”是指你养的猫
- 回复时请使用第一人称视角：“我的父母”“我养的猫”
```

## 八、错误处理与降级

- **提取失败**：跳过本轮候选生成，不影响主对话。
- **Embedding 失败**：退化为精确归一 + lexical overlap，不做激进 merge。
- **单条入图失败**：按 item 粒度容错，整轮记录 `partial_success`。
- **persona 决策失败**：默认 `skipped`，保留事实节点，不做 persona patch。
- **persona patch 失败**：写 `PersonaChangeEvent{status:"failed"}`，保留 `source_node_ids` 便于补偿。

## 九、API 与前端收口

### 9.1 后端

- `GET /api/v1/bots/{botID}/profile`
  - 仅返回 `recent_changes`，不再依赖 `pending_persona_changes`。
- `POST /api/v1/bots/{botID}/persona-changes/{id}/accept|reject`
  - 从产品主流程中移除；若保留，仅作为内部兼容或调试接口。

### 9.2 Web

- `Nukara_Web/src/views/BotDetailView.vue`
  - 去掉“待确认的人设变更”区块和接受/拒绝按钮。
  - 将“最近人设更迭”改为“最近自动应用的人设变更记录”。
- `Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts`
  - 改为断言页面展示自动应用事件，不再点击 accept/reject。

这样可以从业务流程上根除 403 页面错误，而不是仅修补一次接口权限。

## 十、测试与验收

### 10.1 必测范围

- 记忆提取覆盖专业、宠物、家庭关系、居住信息。
- `session_summary` 能拆分出多个独立 fact nodes。
- 高相似候选会 merge，低相似候选会 create。
- persona 只在稳定核心事实出现时更新。
- runtime prompt 会注入视角转换提示。
- Web 详情页不再依赖 pending confirmation API。

### 10.2 验收标准

- “不是纯金融，更偏研究型” 会进入事实层，并可更新 `identity`。
- “我养了一只猫叫小蜜” 会生成事实节点和 `owns` 关系。
- “最近住在爸妈这边” 会进入图记忆，但默认不直接写入 persona。
- 用户提到“叔叔阿姨”时，runtime prompt 能提醒 bot 使用“我的父母”。
- Bot 详情页不再触发 persona confirm 请求，自然消除该链路上的 403。

## 十一、非目标

本次不做以下事项：

- 不引入新的外部数据库或消息队列。
- 不重做整个 recall ranking 算法，只在现有 `memorygraph` 基础上增强关系与相似度。
- 不保留旧的 `turnCount%3` 人设迭代语义。
- 不围绕旧 pending 审核交互继续打补丁。

## 十二、实施顺序

1. 补齐数据模型和图节点元数据。
2. 改写 memory extraction 输出与 `compact_update` 物化。
3. 接入 similarity + ingest merge/create 逻辑。
4. 替换旧 persona iterator，接入 decision + auto apply。
5. 改 runtime context 的关系提示。
6. 收口 bot profile API 和 Web 详情页。
7. 跑 focused tests，再跑跨模块回归。
