# 记忆系统改造：Embedding 相似度 + Entity 三要素 + 延迟聚合

**日期**：2026-03-23
**作者**：Claude (Kiro)
**状态**：设计阶段

## 一、背景与问题

### 当前问题

1. **去重效果差**：基于 Dice 字符频率的相似度计算，无法识别语义相同但表述不同的记忆
   - "我养了一只猫" vs "我家有只橘猫" → 被当作两条记忆

2. **召回不准确**：词法匹配只能匹配字面相同的词，语义泛化能力弱
   - 用户问"我的猫" → 召回不到"我养的橘猫豆腐"

3. **记忆碎片化**：每轮对话都提取记忆，渐进式完善的信息被拆成多条
   - 第1轮："我养了一只猫"
   - 第2轮："它是橘色的"
   - 第3轮："叫豆腐，今年3岁了"
   - → 存成 3 条独立记忆

4. **Entity 未结构化**：虽然有 `Entity` 字段，但 `Type` 未规范，无法按人物/地点/时间精准过滤

### 目标

1. **语义去重**：用 embedding 相似度替代字符频率，识别语义相同的记忆
2. **语义召回**：用 embedding 替代词法匹配，提升召回准确率
3. **记忆完整性**：延迟聚合机制，将同一话题的多轮对话合并提取
4. **结构化 Entity**：规范 Entity.Type，支持按三要素（人物/地点/时间）检索

## 二、整体架构

### 核心模块

```
┌─────────────────────────────────────────────────────────┐
│                    用户对话                              │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│  延迟聚合层 (MemoryBuffer)                               │
│  - 会话级缓存                                            │
│  - 话题切换检测 (LLM)                                    │
│  - 时间窗口兜底 (5轮/10分钟)                             │
└────────────────┬────────────────────────────────────────┘
                 │ [话题切换/超时]
                 ▼
┌─────────────────────────────────────────────────────────┐
│  Recall 召回 (Embedding)                                 │
│  - 检索相关记忆                                          │
│  - 注入到 memory_extract prompt                          │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│  Memory Extract (LLM)                                    │
│  - 聚合多轮对话                                          │
│  - 输出 memory_id (更新) 或空 (新建)                     │
│  - 输出 entities (含 type: person/location/time/org)     │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│  IngestTurn → persistNodesWithMerge                      │
│  - findBestSimilarityNode (Embedding 相似度)             │
│  - 阈值 0.80 (基础) / 0.70 (同session+entity重叠)        │
│  - 合并或新建                                            │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│  异步生成 Embedding                                      │
│  - 存入 memory_embeddings 表 (JSONB 自适应维度)          │
└─────────────────────────────────────────────────────────┘
```

## 三、详细设计

### 3.1 Embedding 相似度计算

#### 文件
- `internal/agentx/memorygraph/similarity.go`
- `internal/agentx/memorygraph/embedding.go` (新增)

#### 核心逻辑

```go
// similarity.go
func nodeSimilarity(a, b store.TemporalMemoryNode) float64 {
    // 1. 类型和类别必须匹配
    if a.NodeType != b.NodeType { return 0 }
    if a.SemanticCategory != "" && b.SemanticCategory != "" &&
       a.SemanticCategory != b.SemanticCategory { return 0 }

    // 2. MergeKey 完全相同 → 直接合并
    if a.MergeKey != "" && a.MergeKey == b.MergeKey { return 1.0 }

    // 3. 计算 embedding 相似度
    embSim := computeEmbeddingSimilarity(a, b)

    // 4. 同会话 + Entity 重叠 → 降低阈值 (0.70)
    if a.SessionID != "" && a.SessionID == b.SessionID {
        if hasEntityOverlap(a.Entities, b.Entities) {
            return embSim * 1.15  // 让 0.70 也能达到 0.80 阈值
        }
    }

    return embSim
}

// embedding.go (新增)
type EmbeddingService interface {
    GetEmbedding(nodeID string) ([]float64, error)
    ComputeSimilarity(embA, embB []float64) float64
}

func computeEmbeddingSimilarity(a, b store.TemporalMemoryNode) float64 {
    embA, err := embeddingService.GetEmbedding(a.ID)
    if err != nil { return 0 }

    embB, err := embeddingService.GetEmbedding(b.ID)
    if err != nil { return 0 }

    return cosineSimilarity(embA, embB)
}

func cosineSimilarity(a, b []float64) float64 {
    if len(a) != len(b) { return 0 }

    var dotProduct, normA, normB float64
    for i := range a {
        dotProduct += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }

    if normA == 0 || normB == 0 { return 0 }
    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

#### 合并阈值

- **基础阈值**：0.80（余弦相似度）
- **同会话 + Entity 重叠**：0.70（相似度 × 1.15 后达到 0.80）
- **MergeKey 相同**：1.0（直接合并）

### 3.2 纯 Embedding 召回

#### 文件
- `internal/agentx/memorygraph/recall.go`

#### 改动

```go
// 原 scoreNode 函数
func scoreNode(node store.TemporalMemoryNode, cue Cue, now time.Time) float64 {
    // 移除：lexical := float64(lexicalHits(text, hints)) * 0.72

    // 新增：embedding 相似度
    embScore := embeddingScore(node, cue)

    typePrior := nodeTypePrior(node)
    recency := recencyScore(node, now)
    openLoop := 0.0
    if node.NodeType == "promise" && node.Status == "active" {
        openLoop = 1.15
    }

    return embScore * 1.2 + typePrior + recency + node.Salience * 0.35 + openLoop
}

// 新增函数
func embeddingScore(node store.TemporalMemoryNode, cue Cue) float64 {
    nodeEmb, err := embeddingService.GetEmbedding(node.ID)
    if err != nil { return 0 }

    // 将 cue 文本转为 embedding
    cueText := cue.QueryText + " " + strings.Join(cue.EntityHints, " ")
    cueEmb, err := embeddingService.EmbedText(cueText)
    if err != nil { return 0 }

    return cosineSimilarity(nodeEmb, cueEmb)
}
```

### 3.3 Entity 三要素拆解

#### Entity.Type 枚举

```go
// internal/store/agentx_data.go
const (
    EntityTypePerson       = "person"       // 人物
    EntityTypeLocation     = "location"     // 地点
    EntityTypeTime         = "time"         // 时间
    EntityTypeOrganization = "organization" // 组织
)
```

#### LLM Prompt 增强

```go
// internal/api/subtasks_prompts.go
func buildMemoryExtractPrompt(...) string {
    return `...
9. entities 字段必须包含 type，可选值：person | location | time | organization
   - person: 人物（用户、bot、第三方人物）
   - location: 地点（城市、地址、场所）
   - time: 时间（具体时间点、时间段）
   - organization: 组织（公司、学校、团体）

示例：
{"entities": [
  {"id": "pet-doufu", "type": "person", "name": "豆腐", "role": "user"},
  {"id": "loc-beijing", "type": "location", "name": "北京", "role": "shared"},
  {"id": "time-recent", "type": "time", "name": "最近", "role": "shared"}
]}
...`
}
```

#### BuildCue 增强

```go
// internal/agentx/memorygraph/recall.go
func BuildCue(query string, recentTexts []string, entities []store.Entity) Cue {
    cue := Cue{
        QueryText:   query,
        EntityHints: make([]string, 0),
        PersonHints: make([]string, 0),
        LocationHints: make([]string, 0),
        TimeHints: make([]string, 0),
    }

    // 按 entity type 分组
    for _, entity := range entities {
        switch entity.Type {
        case "person":
            cue.PersonHints = append(cue.PersonHints, entity.Name)
        case "location":
            cue.LocationHints = append(cue.LocationHints, entity.Name)
        case "time":
            cue.TimeHints = append(cue.TimeHints, entity.Name)
        }
    }

    return cue
}
```

### 3.4 延迟聚合机制

#### 新增文件
- `internal/agentx/memorybuffer/buffer.go`
- `internal/agentx/memorybuffer/topic_detector.go`

#### 核心结构

```go
// buffer.go
type ConversationMemoryBuffer struct {
    ConversationID string
    TopicID        string  // 当前话题标识
    TurnIDs        []string
    UserTexts      []string
    BotTexts       []string
    TopicKeywords  []string  // 话题关键词
    StartedAt      time.Time
    LastUpdatedAt  time.Time
}

type BufferService struct {
    store          map[string]*ConversationMemoryBuffer
    topicDetector  TopicDetector
    maxTurns       int           // 默认 5
    maxDuration    time.Duration // 默认 10 分钟
}

func (s *BufferService) Append(conversationID, turnID, userText, botText string) error {
    buffer := s.getOrCreate(conversationID)
    buffer.TurnIDs = append(buffer.TurnIDs, turnID)
    buffer.UserTexts = append(buffer.UserTexts, userText)
    buffer.BotTexts = append(buffer.BotTexts, botText)
    buffer.LastUpdatedAt = time.Now()
    return nil
}

func (s *BufferService) ShouldFlush(conversationID string) (bool, string) {
    buffer := s.get(conversationID)
    if buffer == nil { return false, "" }

    // A. 话题切换检测
    if len(buffer.TurnIDs) > 0 {
        lastUserText := buffer.UserTexts[len(buffer.UserTexts)-1]
        lastBotText := buffer.BotTexts[len(buffer.BotTexts)-1]

        topicChanged, newKeywords := s.topicDetector.Detect(
            buffer.TopicKeywords, lastUserText, lastBotText)

        if topicChanged {
            return true, "topic_changed"
        }
        buffer.TopicKeywords = newKeywords
    }

    // B. 时间窗口兜底
    if len(buffer.TurnIDs) >= s.maxTurns {
        return true, "max_turns_reached"
    }
    if time.Since(buffer.StartedAt) >= s.maxDuration {
        return true, "max_duration_reached"
    }

    return false, ""
}

func (s *BufferService) Flush(conversationID string) (AggregatedTurn, error) {
    buffer := s.get(conversationID)
    if buffer == nil { return AggregatedTurn{}, nil }

    // 聚合多轮对话
    aggregated := AggregatedTurn{
        ConversationID: conversationID,
        TurnIDs:        buffer.TurnIDs,
        UserTexts:      buffer.UserTexts,
        BotTexts:       buffer.BotTexts,
    }

    // 清空 buffer
    delete(s.store, conversationID)

    return aggregated, nil
}
```

#### 话题检测

```go
// topic_detector.go
type TopicDetector interface {
    Detect(previousKeywords []string, userText, botText string) (changed bool, newKeywords []string)
}

type LLMTopicDetector struct {
    llmClient LLMClient
}

func (d *LLMTopicDetector) Detect(previousKeywords []string, userText, botText string) (bool, []string) {
    prompt := fmt.Sprintf(`[system:topic_detection]
判断本轮对话是否延续上一话题。

上一话题关键词：%s
本轮对话：
用户：%s
机器人：%s

严格输出 JSON：
{"topic_continues": true/false, "new_topic_keywords": ["..."]}
`, strings.Join(previousKeywords, ", "), userText, botText)

    response, err := d.llmClient.Call(prompt)
    if err != nil { return false, previousKeywords }

    var result struct {
        TopicContinues   bool     `json:"topic_continues"`
        NewTopicKeywords []string `json:"new_topic_keywords"`
    }
    json.Unmarshal([]byte(response), &result)

    return !result.TopicContinues, result.NewTopicKeywords
}
```

#### 集成到主流程

```go
// internal/agentx/subtasks/runner.go
func (r *Runner) Run(ctx context.Context, in Input) (Result, error) {
    // 1. 追加到 buffer
    r.bufferService.Append(in.ConversationID, in.TurnID, in.UserText, in.BotText)

    // 2. 检查是否需要聚合
    shouldFlush, reason := r.bufferService.ShouldFlush(in.ConversationID)
    if !shouldFlush {
        return Result{}, nil  // 继续缓存
    }

    // 3. 聚合并提取记忆
    aggregated, err := r.bufferService.Flush(in.ConversationID)
    if err != nil { return Result{}, err }

    // 4. Recall 召回相关记忆
    recalledNodes := r.recallRelatedMemories(ctx, aggregated)

    // 5. Memory Extract（注入召回的记忆）
    raw, err := r.memoryExtractor(ctx, aggregated, recalledNodes)
    if err != nil { return Result{}, err }

    // 6. 解析并写入
    items, err := ParseMemoryItems(raw)
    if err != nil { return Result{}, err }

    savedItems := r.applyMemory(items, aggregated)

    // 7. IngestTurn
    r.memoryGraph.IngestTurn(ctx, memorygraph.IngestTurnInput{
        UserID:         in.UserID,
        BotID:          in.BotID,
        ConversationID: in.ConversationID,
        TurnID:         aggregated.TurnIDs[len(aggregated.TurnIDs)-1],
        Items:          savedItems,
        CompactJSON:    "", // 聚合模式下不需要 compact
        Now:            time.Now().UTC(),
    })

    return Result{SavedMemoryCount: len(savedItems)}, nil
}
```

### 3.5 Recall 注入到 memory_extract

#### 增强 Prompt

```go
// internal/api/subtasks_prompts.go
func buildMemoryExtractPrompt(
    aggregatedTexts string,
    existingMemory string,
    recalledNodes []store.TemporalMemoryNode,
) string {
    recalled := ""
    if len(recalledNodes) > 0 {
        recalled = "\n本次对话相关的记忆（可能需要更新）：\n"
        for _, node := range recalledNodes {
            recalled += fmt.Sprintf("- [ID:%s] %s\n", node.ID, node.Summary)
        }
    }

    return fmt.Sprintf(`[system:memory_extract_json]
你现在负责从聚合的对话中提取长期记忆。

已有长期记忆：
%s
%s
[聚合的对话历史]
%s

如果本轮对话是在补充/修正上述相关记忆，请复用对应的 memory_id。
严格输出 JSON：
{"items":[{
  "memory_id": "tm:xxx",  // 更新已有记忆时填写，新建时留空
  "kind": "...",
  "content": "...",
  "entities": [
    {"id": "...", "type": "person|location|time|organization", "name": "...", "role": "..."}
  ]
}]}
`, existingMemory, recalled, aggregatedTexts)
}
```

## 四、数据存储

### 4.1 Embedding 存储

#### 表结构（已有）

```sql
CREATE TABLE memory_embeddings (
    node_id TEXT PRIMARY KEY REFERENCES memory_nodes(id) ON DELETE CASCADE,
    embedding_model TEXT NOT NULL DEFAULT '',
    embedding_json JSONB NOT NULL DEFAULT '[]'::jsonb,  -- 自适应维度
    embedding vector(1536),  -- pgvector（可选）
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### 存储策略

1. **优先 pgvector**：如果 `vector` 扩展可用，存入 `embedding vector(1536)` 列
2. **降级 JSONB**：否则存入 `embedding_json` JSONB 列
3. **自适应维度**：JSONB 可存任意维度，不限定 1536

#### 写入时机

```go
// internal/agentx/memorygraph/service.go
func (s *Service) IngestTurn(...) (IngestTurnResult, error) {
    // ... 写入节点 ...

    // 异步生成 embedding
    for _, node := range result.Nodes {
        go s.generateEmbedding(node.ID, node.Summary)
    }

    return result, nil
}

func (s *Service) generateEmbedding(nodeID, text string) {
    emb, err := s.embeddingClient.Embed(context.Background(), text)
    if err != nil {
        log.Printf("generate embedding failed: %v", err)
        return
    }

    s.store.UpsertEmbedding(nodeID, emb)
}
```

### 4.2 Buffer 存储

#### 内存存储（默认）

```go
type BufferService struct {
    mu    sync.RWMutex
    store map[string]*ConversationMemoryBuffer
}
```

#### 持久化存储（可选）

如果需要跨进程共享或持久化，可用 Redis：

```
Key: memory_buffer:{conversationID}
Value: JSON(ConversationMemoryBuffer)
TTL: 1 hour
```

## 五、性能优化

### 5.1 批量 Embedding 查询

```go
// Recall 时批量查询
func (s *Service) Recall(ctx context.Context, in RecallInput) (RecallResult, error) {
    allNodes := s.store.ListMemoryNodes(in.UserID, in.BotID, ...)

    // 批量查询 embeddings
    nodeIDs := extractNodeIDs(allNodes)
    embeddings := s.embeddingStore.BatchGetEmbeddings(nodeIDs)

    // 缓存到内存
    s.embeddingCache.SetBatch(embeddings)

    // 后续计算相似度时直接从缓存读取
    seeds := SelectSeeds(cue, allNodes, ...)
    ...
}
```

### 5.2 异步生成 Embedding

- 写入节点时不阻塞主流程
- 用 goroutine 异步调用 embedding API
- 失败时记录日志，下次 Recall 时补生成

### 5.3 降级策略

```go
func (s *Service) computeEmbeddingSimilarity(a, b store.TemporalMemoryNode) float64 {
    embA, err := s.embeddingService.GetEmbedding(a.ID)
    if err != nil {
        // 降级：回退到词法匹配
        return diceCoefficient(similarityText(a), similarityText(b))
    }

    embB, err := s.embeddingService.GetEmbedding(b.ID)
    if err != nil {
        return diceCoefficient(similarityText(a), similarityText(b))
    }

    return cosineSimilarity(embA, embB)
}
```

## 六、兼容性与迁移

### 6.1 渐进式升级

- **已有记忆节点**：无 embedding → 首次召回时补生成
- **配置开关**：`ENABLE_EMBEDDING_RECALL=true/false`
- **双模式运行**：embedding 失败时自动降级到词法匹配

### 6.2 数据迁移

**无需迁移**，embedding 按需生成：

```go
func (s *Service) GetEmbedding(nodeID string) ([]float64, error) {
    // 1. 尝试从 memory_embeddings 读取
    emb, err := s.store.GetEmbedding(nodeID)
    if err == nil { return emb, nil }

    // 2. 不存在 → 补生成
    node, ok := s.store.GetMemoryNode(nodeID)
    if !ok { return nil, errors.New("node not found") }

    emb, err = s.embeddingClient.Embed(context.Background(), node.Summary)
    if err != nil { return nil, err }

    // 3. 存入数据库
    s.store.UpsertEmbedding(nodeID, emb)

    return emb, nil
}
```

## 七、测试计划

### 7.1 单元测试

- `similarity_test.go`：embedding 相似度计算
- `recall_test.go`：embedding 召回评分
- `buffer_test.go`：延迟聚合逻辑
- `topic_detector_test.go`：话题切换检测

### 7.2 集成测试

- 端到端流程：对话 → 缓存 → 聚合 → 提取 → 写入 → 召回
- 渐进式完善场景：多轮对话围绕同一话题
- 话题切换场景：从"猫"切换到"工作"再回到"猫"

### 7.3 性能测试

- Embedding 生成延迟
- 批量召回性能（100+ 节点）
- 内存占用（buffer 缓存）

## 八、风险与缓解

### 8.1 Embedding 服务不可用

**风险**：embedding API 故障导致召回失败

**缓解**：
- 降级到词法匹配
- 本地缓存 embedding
- 监控告警

### 8.2 话题检测误判

**风险**：LLM 误判话题切换，导致过早或过晚聚合

**缓解**：
- 时间窗口兜底（5 轮 / 10 分钟强制聚合）
- 可配置阈值
- 人工审核机制

### 8.3 记忆延迟生效

**风险**：用户聊了 3 轮才写入记忆，短期内无法召回

**缓解**：
- 短期记忆靠 context window（LLM 上下文）
- 长期记忆靠 buffer 聚合后写入
- 用户感知不明显（对话中已有上下文）

## 九、后续优化方向

1. **向量索引**：pgvector 的 HNSW 索引，加速大规模召回
2. **多模态 Embedding**：支持图片、语音记忆
3. **时间范围查询**：按 Entity.Type=time 过滤时间段
4. **地点聚类**：按 Entity.Type=location 聚合地点相关记忆
5. **用户主动标记**：前端"保存记忆"按钮，手动触发聚合

## 十、总结

本次改造通过 **Embedding 相似度 + Entity 三要素 + 延迟聚合** 三大核心机制，从根本上解决了记忆系统的去重、召回、碎片化问题。

**核心价值**：
- 语义去重，减少冗余记忆
- 语义召回，提升准确率
- 延迟聚合，保证记忆完整性
- 结构化 Entity，支持精准检索

**实施策略**：
- 渐进式升级，无需数据迁移
- 降级友好，embedding 失败时回退
- 性能优化，批量查询 + 异步生成

---

**下一步**：进入实施计划阶段，拆解为可执行的开发任务。
