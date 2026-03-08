# 记忆与人设系统重构设计文档

**版本**: v1.0
**创建时间**: 2026-03-08
**状态**: 设计阶段

## 一、问题背景

### 1.1 当前系统存在的问题

基于用户提供的对话案例分析，当前系统存在以下核心问题：

**问题 1：记忆创建缺失**
- 用户提到机器人的专业（"不是纯金融，更偏研究型"），但没有创建记忆节点
- 用户问到养猫（小蜜），机器人回答了，但也没有创建记忆
- 会话摘要永远只是一个节点，没有拆分成多个独立的记忆节点

**问题 2：人设更迭过度触发和上下文膨胀**
- 当前逻辑：每3轮对话 OR 有记忆保存时就触发人设更迭（`runner.go:122`）
- 用户需求：只有"高危"信息才应该触发人设更迭
- 现状：人设更迭只是不断往"人设档案"里追加文本，导致上下文膨胀
- 期望：人设更迭应该伴随记忆的创建/更新，而不是单纯填充文本
- 新需求：人设变更不需要用户确认，直接自动更迭

**问题 3：身份认知混乱**
- 用户说"叔叔阿姨"，从用户视角这指的是机器人的父母
- 但机器人也称呼为"叔叔阿姨"，没有理解这是在说自己的父母
- 缺少关系上下文映射机制

**问题 4：HTTP 403 错误**
- 用户在人设确认页面遇到 HTTP 403 错误
- 需要排查前端调用 persona-changes API 的权限问题

### 1.2 根本原因分析

**记忆提取层面**：
1. 关键词硬过滤过于保守（`memory_extract.go:81-108`）
2. LLM prompt 没有明确要求提取专业、宠物、家庭关系等信息
3. 只依赖关键词匹配，无法覆盖所有场景

**人设更迭层面**：
1. 触发条件过于宽松（每3轮或有记忆保存）
2. 没有区分"记忆"和"人设"的边界
3. 缺少智能判断机制，导致不必要的人设更新

**记忆结构层面**：
1. `session_summary` 总是单个节点，没有拆分
2. 缺少语义相似度去重机制
3. 没有关系图谱来表达实体和关系

## 二、设计目标

### 2.1 核心原则

1. **记忆与人设按稳定性区分**
   - 人设：核心身份、性格特征（稳定、精简，10-20条）
   - 记忆：具体事件、状态、习惯（记忆图管理，可变化）

2. **多模型协同**
   - 主模型（LLM）：提取候选记忆
   - 轻量模型：分类、特征提取、综合判断
   - Embedding模型：相似度计算、去重

3. **智能触发机制**
   - 基于记忆类型 + 全维度综合判断
   - 减少不必要的人设更迭
   - 人设变更自动应用，无需用户确认

4. **关系图谱 + 视角转换**
   - 提取实体和关系
   - 在对话生成时提醒机器人注意视角转换

### 2.2 成功标准

1. **记忆覆盖率提升**：专业、宠物、家庭关系等信息能被正确提取
2. **人设更迭频率降低**：只有真正需要时才触发，减少上下文膨胀
3. **会话摘要结构化**：一个会话产生多个独立的记忆节点
4. **身份认知准确**：机器人能正确理解关系上下文并使用第一人称视角

## 三、系统架构设计

### 3.1 多模型协同流程

```
用户对话
  ↓
[主模型 LLM] 提取候选记忆（memory_extract）
  - 输入：用户消息 + 机器人回复 + 已有记忆
  - 输出：候选记忆列表（items）
  ↓
[轻量模型] 分类和特征提取
  - 记忆类型：self_fact/habit/user_fact/event/promise/state_basis
  - 语义类别：身份/性格/生活背景/表达风格/偏好边界
  - 稳定性判断：是否包含时间限定词（"最近"、"临时"等）
  ↓
[Embedding模型] 计算相似度
  - 与已有记忆的相似度（去重/更新判断）
  - 与现有人设的相似度（新维度判断）
  ↓
[轻量模型] 综合判断人设更迭
  - 输入：记忆类型 + 重要性 + 语义类别 + 稳定性 + 相似度
  - 输出：should_update_persona + target_field + reason
  ↓
保存到记忆图 + 可能的人设更新（自动应用）
```

### 3.2 人设更迭触发机制

**分层映射规则（基础规则）**：
- `self_fact` → 可能触发 `identity`, `personality`, `life_context`
- `habit` → 可能触发 `expression_style`
- `user_fact` → 可能触发 `taboos_and_preferences`
- `event`, `promise`, `state_basis` → 不触发人设更迭

**全维度综合判断（轻量模型）**：

输入特征：
```json
{
  "memory_type": "self_fact",
  "content": "不是纯金融，更偏研究型",
  "importance": 85,
  "semantic_category": "identity",
  "stability": "stable",
  "similarity_to_existing_memories": 0.3,
  "similarity_to_persona_identity": 0.2,
  "similarity_to_persona_life_context": 0.4
}
```

输出决策：
```json
{
  "should_update_persona": true,
  "target_field": "identity",
  "reason": "核心身份信息，稳定性高，与现有人设相似度低，属于新维度",
  "risk": "low"
}
```

**风险分类调整**：
- 移除 `pending_confirm` 流程
- 所有人设更迭直接自动应用（`auto_apply`）
- 保留风险分类用于日志和审计

### 3.3 双层记忆结构

**层级 1：详细记忆节点**（来自 `memory_extract`）
- 每个 `item` 创建独立的 `TemporalMemoryNode`
- 节点类型：`episode`, `promise`, `state_snapshot`, `user_fact`
- 包含完整的元数据：重要性、发生时间、主题标签

**层级 2：会话摘要节点**（来自 `compact_update`）
- 每个会话创建一个 `session_summary` 节点
- 从 `compact_update` 的 `facts` 数组中提取每个 fact
- 为每个 fact 创建独立的记忆节点
- `session_summary` 通过 `summarizes` edge 连接到各个 fact 节点

**关系结构**：
```
session_summary (会话摘要)
  ├─[summarizes]→ episode_1 (机器人的专业)
  ├─[summarizes]→ user_fact_1 (机器人养猫)
  └─[summarizes]→ episode_2 (机器人的居住情况)

episode_1 ─[follows]→ episode_2 (时间顺序)
```

### 3.4 关系图谱 + 视角转换

**实体识别和关系提取**：
- 在记忆提取时，同时提取实体和关系
- 实体类型：人物、宠物、地点、组织
- 关系类型：家庭关系、所有权、居住关系等

**案例**：
```json
实体：
- 机器人（bot_self）
- 父母（bot_parents）
- 小蜜（pet_cat）

关系：
- bot_self -[父母]-> bot_parents
- bot_self -[养]-> pet_cat
- bot_self -[居住于]-> bot_parents的家
```

**视角转换提醒机制**：
- 在召回记忆时，检测关系图谱中的实体
- 在 system prompt 中注入视角转换提示：

```
[关系上下文]
- 用户提到的"叔叔阿姨"是指你的父母
- 用户提到的"小蜜"是指你养的猫
- 回复时请使用第一人称视角："我的父母"、"我养的猫"
```

## 四、详细设计

### 4.1 记忆提取改进

**4.1.1 移除关键词硬过滤**

修改 `memory_extract.go` 的 `shouldPersistMemory` 函数：

```go
func shouldPersistMemory(kind, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	// 过滤明显的低价值寒暄
	for _, lowValue := range []string{"你好", "你好呀", "早安", "晚安", "吃了吗", "我有点困", "我有点忙", "嗯", "哦", "好的", "知道了"} {
		if content == lowValue {
			return false
		}
	}
	// user_fact 和 self_fact：信任 LLM 判断，不再用关键词硬过滤
	if kind == "user_fact" || kind == "self_fact" {
		return true
	}
	// habit：放宽过滤，只要内容有实质信息就保存
	if kind == "habit" {
		return len([]rune(content)) >= 4
	}
	// 其他类型：默认保存
	return true
}
```

**4.1.2 改进 LLM Prompt**

在 `subtasks_prompts.go` 的 `buildMemoryExtractPrompt` 中：
- 明确列出 bot 自我认知的关键信息类型
- 添加 kind 分类说明
- 强调专业、宠物、家庭关系等信息的重要性

### 4.2 Embedding 相似度计算

**4.2.1 新增模块**

创建 `internal/agentx/memorygraph/similarity.go`：

```go
type SimilarityCalculator struct {
	embedder *llm.OpenAICompatEmbedder
}

func (c *SimilarityCalculator) FindSimilarMemories(
	ctx context.Context,
	newMemory store.MemoryItem,
	existingMemories []store.TemporalMemoryNode,
) ([]SimilarityMatch, error)

type SimilarityMatch struct {
	NodeID     string
	Similarity float64
	ShouldMerge bool // similarity > 0.85
}
```

**4.2.2 集成到记忆保存流程**

在 `runner.go` 的 `applyMemory` 中：
1. 计算新记忆与已有记忆的相似度
2. 如果相似度 > 0.85，更新已有记忆而不是创建新记忆
3. 如果相似度 < 0.85，创建新记忆节点

### 4.3 双层记忆结构实现

**4.3.1 修改 BuildSessionSummaryNode**

在 `ingest.go` 中：

```go
func BuildSessionSummaryNodesFromCompact(in IngestTurnInput, now time.Time) (store.TemporalMemoryNode, []store.TemporalMemoryNode, error) {
	// 解析 compact JSON
	var payload compactEnvelope
	json.Unmarshal([]byte(in.CompactJSON), &payload)

	// 创建 session_summary 节点
	summaryNode := store.TemporalMemoryNode{
		ID: sessionSummaryNodeID(in.ConversationID),
		NodeType: "session_summary",
		Title: "会话摘要",
		Summary: payload.Summary,
		...
	}

	// 从 facts 数组中提取每个 fact，创建独立节点
	factNodes := make([]store.TemporalMemoryNode, 0, len(payload.Facts))
	for idx, fact := range payload.Facts {
		factNode := store.TemporalMemoryNode{
			ID: fmt.Sprintf("fact:%s:%d", in.TurnID, idx),
			NodeType: "episode",
			Title: deriveNodeTitle("episode", fact),
			Summary: fact,
			...
		}
		factNodes = append(factNodes, factNode)
	}

	return summaryNode, factNodes, nil
}
```

**4.3.2 创建 summarizes edges**

```go
func BuildSessionSummaryEdges(summary store.TemporalMemoryNode, factNodes []store.TemporalMemoryNode) []store.TemporalMemoryEdge {
	edges := make([]store.TemporalMemoryEdge, 0, len(factNodes))
	for _, node := range factNodes {
		edges = append(edges, store.TemporalMemoryEdge{
			ID: fmt.Sprintf("edge:%s:%s", summary.ID, node.ID),
			SourceID: summary.ID,
			TargetID: node.ID,
			EdgeType: "summarizes",
			Weight: 0.8,
			...
		})
	}
	return edges
}
```

### 4.4 轻量模型综合判断

**4.4.1 新增 subtask**

创建 `persona_update_decision` subtask：

```go
// subtasks_prompts.go
func buildPersonaUpdateDecisionPrompt(features MemoryFeatures, currentPersona PersonaSnapshot) string {
	return fmt.Sprintf(`[system:persona_update_decision_json]
你需要判断这条记忆是否需要更新机器人的人设。

记忆特征：
- 类型：%s
- 内容：%s
- 重要性：%d
- 语义类别：%s
- 稳定性：%s
- 与已有记忆相似度：%.2f
- 与人设 identity 相似度：%.2f
- 与人设 life_context 相似度：%.2f

当前人设快照：
- identity: %s
- personality: %s
- life_context: %s

判断规则：
1. 只有稳定的、核心的身份信息才需要更新人设
2. 临时状态、短期习惯只需要保存为记忆，不更新人设
3. 如果与现有人设相似度高（>0.7），说明已有覆盖，不需要更新
4. 如果是新维度的核心信息，需要更新人设

严格输出 JSON：
{"should_update_persona":true/false,"target_field":"identity|personality|life_context|expression_style|taboos_and_preferences","reason":"...","risk":"low|high"}
`, features.Type, features.Content, features.Importance, features.SemanticCategory, features.Stability,
   features.SimilarityToMemories, features.SimilarityToIdentity, features.SimilarityToLifeContext,
   currentPersona.Identity, currentPersona.Personality, currentPersona.LifeContext)
}
```

**4.4.2 特征提取函数**

```go
type MemoryFeatures struct {
	Type                    string
	Content                 string
	Importance              int
	SemanticCategory        string  // identity/personality/life_background/expression/preference
	Stability               string  // stable/temporary
	SimilarityToMemories    float64
	SimilarityToIdentity    float64
	SimilarityToLifeContext float64
}

func extractMemoryFeatures(
	ctx context.Context,
	item store.MemoryItem,
	existingMemories []store.TemporalMemoryNode,
	currentPersona store.Bot,
	embedder *llm.OpenAICompatEmbedder,
) (MemoryFeatures, error)
```

**4.4.3 集成到 runner.go**

修改 `runner.go` 的人设更迭逻辑：

```go
// 移除 turnCount%3 触发
// 改为基于轻量模型判断

if len(savedItems) > 0 {
	for _, item := range savedItems {
		// 提取特征
		features, err := extractMemoryFeatures(ctx, item, existingMemories, bot, embedder)
		if err != nil {
			continue
		}

		// 轻量模型判断
		decision, err := r.personaUpdateDecider(ctx, features, bot)
		if err != nil || !decision.ShouldUpdatePersona {
			continue
		}

		// 自动应用人设更新（移除 pending_confirm）
		patch := buildPersonaPatchFromDecision(decision, item)
		_, _ = r.store.Store.ApplyBotPersonaPatch(in.UserID, in.BotID, patch)

		// 记录变更事件（用于审计）
		_, _ = r.store.Store.CreatePersonaChangeEvent(store.PersonaChangeEvent{
			UserID: in.UserID,
			BotID: in.BotID,
			Field: decision.TargetField,
			ChangeType: "append",
			ProposedValue: item.Content,
			Risk: decision.Risk,
			Status: "accepted", // 直接接受
			...
		})
	}
}
```

### 4.5 关系图谱提取

**4.5.1 扩展 MemoryItem 结构**

```go
// store/interface.go
type MemoryItem struct {
	ID         string
	Kind       string
	Owner      string
	Content    string
	Importance int
	OccurredAt time.Time
	Status     string
	Topics     []string
	Entities   []Entity   // 新增
	Relations  []Relation // 新增
	...
}

type Entity struct {
	ID   string
	Type string // person/pet/place/organization
	Name string
	Role string // 从谁的视角（user/bot）
}

type Relation struct {
	SourceEntityID string
	TargetEntityID string
	RelationType   string // parent/child/owns/lives_at
}
```

**4.5.2 LLM 提取实体和关系**

在 `memory_extract` prompt 中添加：

```
9. 实体和关系提取：
   - 识别对话中的重要实体（人物、宠物、地点）
   - 提取实体之间的关系
   - 注意视角：用户说的"叔叔阿姨"是指 bot 的父母

输出格式：
{"items":[{
  "memory_id":"...",
  "kind":"self_fact",
  "content":"...",
  "entities":[
    {"id":"bot_parents","type":"person","name":"父母","role":"bot"},
    {"id":"pet_cat_xiaomi","type":"pet","name":"小蜜","role":"bot"}
  ],
  "relations":[
    {"source":"bot_self","target":"bot_parents","type":"parent"},
    {"source":"bot_self","target":"pet_cat_xiaomi","type":"owns"}
  ]
}]}
```

**4.5.3 存储实体和关系**

在 `TemporalMemoryNode` 中添加字段：

```go
type TemporalMemoryNode struct {
	...
	EntitiesJSON  string // JSON 序列化的 entities
	RelationsJSON string // JSON 序列化的 relations
}
```

### 4.6 视角转换提醒

**4.6.1 召回时检测关系**

在 `chat_flow.go` 中：

```go
func buildRelationshipContext(
	userID, botID string,
	recentMemories []store.TemporalMemoryNode,
) string {
	// 从记忆中提取实体和关系
	entities := extractEntitiesFromMemories(recentMemories)
	relations := extractRelationsFromMemories(recentMemories)

	// 生成视角转换提示
	hints := []string{}
	for _, rel := range relations {
		if rel.RelationType == "parent" && rel.SourceEntityID == "bot_self" {
			hints = append(hints, "用户提到的'叔叔阿姨'是指你的父母，回复时请用'我的父母'或'我爸妈'")
		}
		if rel.RelationType == "owns" && rel.TargetEntityID == "pet_cat_xiaomi" {
			hints = append(hints, "用户提到的'小蜜'是指你养的猫，回复时请用'我养的猫小蜜'")
		}
	}

	if len(hints) == 0 {
		return ""
	}

	return "[关系上下文]\n" + strings.Join(hints, "\n")
}
```

**4.6.2 注入到 system prompt**

```go
func buildSystemPrompt(bot store.Bot, memories []store.TemporalMemoryNode, ...) string {
	relationshipContext := buildRelationshipContext(userID, botID, memories)

	return fmt.Sprintf(`%s

%s

%s`, bot.PersonaPrompt, memoryContext, relationshipContext)
}
```

## 五、实现计划

### 5.1 实现顺序（自底向上）

1. **改进记忆提取**（优先级：高）
   - 修改 `shouldPersistMemory` 函数
   - 更新 `buildMemoryExtractPrompt`
   - 测试验证：确保"金融专业"、"养猫"等信息能被提取

2. **添加 Embedding 相似度计算**（优先级：高）
   - 创建 `similarity.go` 模块
   - 集成到记忆保存流程
   - 实现去重和更新逻辑

3. **实现双层记忆结构**（优先级：中）
   - 修改 `BuildSessionSummaryNode`
   - 从 facts 数组提取独立节点
   - 创建 summarizes edges

4. **添加轻量模型综合判断**（优先级：高）
   - 创建 `persona_update_decision` subtask
   - 实现特征提取函数
   - 修改 `runner.go` 的触发逻辑
   - 移除 `pending_confirm` 流程

5. **实现关系图谱**（优先级：中）
   - 扩展 `MemoryItem` 结构
   - 更新 `memory_extract` prompt
   - 存储实体和关系

6. **实现视角转换提醒**（优先级：中）
   - 实现关系检测函数
   - 生成视角转换提示
   - 注入到 system prompt

7. **修复 HTTP 403 错误**（优先级：低）
   - 排查前端 API 调用
   - 验证权限配置

### 5.2 测试验证

**单元测试**：
- `shouldPersistMemory` 函数测试
- Embedding 相似度计算测试
- 特征提取函数测试
- 关系检测函数测试

**集成测试**：
- 端到端记忆提取测试（使用用户提供的对话案例）
- 人设更迭触发测试
- 双层记忆结构验证
- 视角转换提醒验证

**性能测试**：
- Embedding 计算性能
- 轻量模型调用延迟
- 记忆图查询性能

## 六、风险和缓解措施

### 6.1 风险识别

1. **Embedding 计算成本**
   - 风险：每次保存记忆都需要计算 embedding，可能增加延迟和成本
   - 缓解：批量计算、缓存、异步处理

2. **轻量模型判断准确性**
   - 风险：模型可能误判，导致不必要的人设更新或遗漏重要更新
   - 缓解：充分测试、调整 prompt、保留审计日志

3. **关系图谱提取准确性**
   - 风险：LLM 可能提取错误的实体或关系
   - 缓解：保守策略、人工审核、逐步优化

4. **移除用户确认的风险**
   - 风险：自动应用人设更新可能导致不符合用户预期的变化
   - 缓解：保留审计日志、提供撤销机制、逐步放开

### 6.2 回滚计划

如果新系统出现问题，可以通过以下方式回滚：
1. 保留旧的 `shouldPersistMemory` 逻辑作为 fallback
2. 通过配置开关控制新功能的启用
3. 保留完整的审计日志用于问题排查

## 七、后续优化方向

1. **记忆整理和压缩**
   - 定期合并相似记忆
   - 淡化过时记忆
   - 压缩长期记忆

2. **人设精简机制**
   - 定期审查人设字段
   - 移除冗余或过时的人设条目
   - 保持人设在 10-20 条以内

3. **关系图谱增强**
   - 支持更复杂的关系类型
   - 时间维度的关系演化
   - 关系强度和置信度

4. **多轮对话的记忆聚合**
   - 跨会话的记忆关联
   - 主题聚类
   - 记忆时间线

## 八、附录

### 8.1 相关文件清单

**需要修改的文件**：
- `Nukara_Backend/internal/agentx/subtasks/memory_extract.go`
- `Nukara_Backend/internal/api/subtasks_prompts.go`
- `Nukara_Backend/internal/agentx/memorygraph/ingest.go`
- `Nukara_Backend/internal/agentx/subtasks/runner.go`
- `Nukara_Backend/internal/api/chat_flow.go`
- `Nukara_Backend/internal/store/interface.go`

**需要新建的文件**：
- `Nukara_Backend/internal/agentx/memorygraph/similarity.go`
- `Nukara_Backend/internal/agentx/subtasks/persona_decision.go`
- `Nukara_Backend/internal/agentx/memorygraph/relationship.go`

### 8.2 配置项

**新增系统配置**：
- `embedding_similarity_threshold`：相似度阈值（默认 0.85）
- `persona_update_enabled`：是否启用新的人设更迭机制（默认 true）
- `relationship_extraction_enabled`：是否启用关系提取（默认 true）
- `persona_auto_apply`：是否自动应用人设更新（默认 true）

### 8.3 数据库变更

**TemporalMemoryNode 表**：
- 添加 `entities_json` 字段（TEXT）
- 添加 `relations_json` 字段（TEXT）

**PersonaChangeEvent 表**：
- 移除 `pending` 状态的使用
- 所有变更直接标记为 `accepted`
