# 记忆系统改造实施计划

**设计文档**：[2026-03-23-memory-embedding-entity-design.md](./2026-03-23-memory-embedding-entity-design.md)
**创建日期**：2026-03-23
**预计工期**：2-3 周

## 任务拆解

本实施计划将改造分为 6 个阶段，每个阶段可独立测试和部署。

---

## Phase 1: Embedding 基础设施（2-3 天）

### 目标
搭建 embedding 生成和存储的基础设施，为后续改造做准备。

### 任务清单

#### 1.1 Embedding Service 接口定义
**文件**：`internal/agentx/memorygraph/embedding.go` (新建)

**任务**：
- [ ] 定义 `EmbeddingService` 接口
- [ ] 实现 `OpenAICompatEmbeddingService`（基于已有的 `OpenAICompatEmbedder`）
- [ ] 实现余弦相似度计算函数 `cosineSimilarity`
- [ ] 添加单元测试

**关键代码**：
```go
type EmbeddingService interface {
    EmbedText(ctx context.Context, text string) ([]float64, error)
    GetEmbedding(nodeID string) ([]float64, error)
    BatchGetEmbeddings(nodeIDs []string) (map[string][]float64, error)
}
```

#### 1.2 Embedding 存储层
**文件**：`internal/store/postgres_temporal_memory_graph.go`

**任务**：
- [ ] 实现 `UpsertEmbedding(nodeID string, embedding []float64, model string) error`
- [ ] 实现 `GetEmbedding(nodeID string) ([]float64, string, error)`
- [ ] 实现 `BatchGetEmbeddings(nodeIDs []string) (map[string][]float64, error)`
- [ ] 支持 pgvector 和 JSONB 双模式
- [ ] 添加单元测试

**存储逻辑**：
- 优先写入 `embedding vector` 列（如果 pgvector 可用）
- 同时写入 `embedding_json` JSONB 列（兜底）

#### 1.3 异步 Embedding 生成
**文件**：`internal/agentx/memorygraph/service.go`

**任务**：
- [ ] 在 `IngestTurn` 后异步生成 embedding
- [ ] 实现 `generateEmbedding(nodeID, text string)` 方法
- [ ] 添加错误日志和重试机制
- [ ] 添加集成测试

**关键逻辑**：
```go
for _, node := range result.Nodes {
    go s.generateEmbedding(node.ID, node.Summary)
}
```

#### 1.4 配置管理
**文件**：`internal/agentx/provider/router.go`

**任务**：
- [ ] 确认 `ResolveEmbeddingRoute` 方法正常工作
- [ ] 添加配置开关 `ENABLE_EMBEDDING_RECALL`
- [ ] 添加降级逻辑（embedding 失败时回退）

### 验收标准
- [ ] 可以成功生成 embedding 并存入数据库
- [ ] 可以批量查询 embedding
- [ ] 余弦相似度计算正确
- [ ] 单元测试覆盖率 > 80%

---

## Phase 2: Embedding 相似度计算（1-2 天）

### 目标
将 `similarity.go` 的相似度计算从 Dice 系数改为 embedding 余弦相似度。

### 任务清单

#### 2.1 修改 nodeSimilarity 函数
**文件**：`internal/agentx/memorygraph/similarity.go`

**任务**：
- [ ] 新增 `computeEmbeddingSimilarity(a, b TemporalMemoryNode) float64`
- [ ] 修改 `nodeSimilarity` 逻辑：
  - MergeKey 相同 → 1.0
  - 计算 embedding 余弦相似度
  - 同 session + entity 重叠 → 相似度 × 1.15
- [ ] 保留 `diceCoefficient` 作为降级方案
- [ ] 添加单元测试

**关键逻辑**：
```go
func nodeSimilarity(a, b store.TemporalMemoryNode) float64 {
    if a.NodeType != b.NodeType { return 0 }
    if a.MergeKey != "" && a.MergeKey == b.MergeKey { return 1.0 }

    embSim := computeEmbeddingSimilarity(a, b)

    if a.SessionID != "" && a.SessionID == b.SessionID {
        if hasEntityOverlap(a.Entities, b.Entities) {
            return embSim * 1.15
        }
    }

    return embSim
}
```

#### 2.2 调整合并阈值
**文件**：`internal/agentx/memorygraph/similarity.go`

**任务**：
- [ ] 将 `ShouldMerge` 阈值从 0.72 改为 0.80
- [ ] 添加配置项支持动态调整
- [ ] 添加集成测试验证去重效果

### 验收标准
- [ ] 语义相同但表述不同的记忆能正确合并
- [ ] 同 session 内的渐进式完善能正确合并
- [ ] 不同话题的记忆不会误合并
- [ ] 集成测试通过

---

## Phase 3: 纯 Embedding 召回（2-3 天）

### 目标
将 `recall.go` 的召回逻辑从词法匹配改为纯 embedding 相似度。

### 任务清单

#### 3.1 修改 scoreNode 函数
**文件**：`internal/agentx/memorygraph/recall.go`

**任务**：
- [ ] 移除 `lexicalHits` 计算
- [ ] 新增 `embeddingScore(node, cue) float64` 函数
- [ ] 修改评分公式：`embeddingScore × 1.2 + typePrior + recency + salience × 0.35 + openLoop`
- [ ] 添加单元测试

#### 3.2 Cue Embedding 生成
**文件**：`internal/agentx/memorygraph/recall.go`

**任务**：
- [ ] 在 `BuildCue` 中生成 cue 的 embedding
- [ ] 缓存 cue embedding（避免重复计算）
- [ ] 添加单元测试

**关键逻辑**：
```go
func embeddingScore(node store.TemporalMemoryNode, cue Cue) float64 {
    nodeEmb := embeddingService.GetEmbedding(node.ID)
    cueText := cue.QueryText + " " + strings.Join(cue.EntityHints, " ")
    cueEmb := embeddingService.EmbedText(cueText)
    return cosineSimilarity(nodeEmb, cueEmb)
}
```

#### 3.3 批量优化
**文件**：`internal/agentx/memorygraph/recall.go`

**任务**：
- [ ] 在 `Recall` 开始时批量查询所有节点的 embeddings
- [ ] 缓存到内存中
- [ ] 添加性能测试

### 验收标准
- [ ] 语义相关的记忆能被正确召回
- [ ] 召回准确率提升（人工评测）
- [ ] 性能可接受（100 节点召回 < 500ms）
- [ ] 单元测试和集成测试通过

---

## Phase 4: Entity 三要素拆解（1-2 天）

### 目标
规范 Entity.Type 字段，支持 person/location/time/organization 四类。

### 任务清单

#### 4.1 Entity Type 枚举
**文件**：`internal/store/agentx_data.go`

**任务**：
- [ ] 定义 Entity Type 常量
- [ ] 添加 `ValidateEntityType(t string) bool` 函数
- [ ] 添加单元测试

#### 4.2 LLM Prompt 增强
**文件**：`internal/api/subtasks_prompts.go`

**任务**：
- [ ] 修改 `buildMemoryExtractPrompt`，要求输出规范的 entity type
- [ ] 添加示例和说明
- [ ] 测试 LLM 输出是否符合预期

#### 4.3 Entity 校验
**文件**：`internal/agentx/subtasks/memory_extract.go`

**任务**：
- [ ] 在 `ParseMemoryItems` 中校验 entity type
- [ ] 不合法的 type 设为空或默认值
- [ ] 添加单元测试

#### 4.4 BuildCue 增强
**文件**：`internal/agentx/memorygraph/recall.go`

**任务**：
- [ ] 修改 `Cue` 结构，增加 `PersonHints`、`LocationHints`、`TimeHints`
- [ ] 修改 `BuildCue`，按 entity type 分组
- [ ] 添加单元测试

### 验收标准
- [ ] LLM 能正确输出 entity type
- [ ] Entity type 校验正常工作
- [ ] BuildCue 能按类型分组 hints
- [ ] 单元测试通过

---

## Phase 5: 延迟聚合机制（3-4 天）

### 目标
实现会话级缓存和话题切换检测，支持延迟聚合。

### 任务清单

#### 5.1 Buffer Service 实现
**文件**：`internal/agentx/memorybuffer/buffer.go` (新建)

**任务**：
- [ ] 定义 `ConversationMemoryBuffer` 结构
- [ ] 实现 `BufferService` 接口
- [ ] 实现 `Append`、`ShouldFlush`、`Flush` 方法
- [ ] 添加单元测试

#### 5.2 话题检测器
**文件**：`internal/agentx/memorybuffer/topic_detector.go` (新建)

**任务**：
- [ ] 定义 `TopicDetector` 接口
- [ ] 实现 `LLMTopicDetector`
- [ ] 实现话题检测 prompt
- [ ] 添加单元测试

**Prompt 示例**：
```
判断本轮对话是否延续上一话题。
上一话题关键词：猫, 豆腐, 感冒
本轮对话：
用户：今天工作好累
机器人：要不要休息一下？

输出 JSON：
{"topic_continues": false, "new_topic_keywords": ["工作", "累", "休息"]}
```

#### 5.3 集成到主流程
**文件**：`internal/agentx/subtasks/runner.go`

**任务**：
- [ ] 在 `Run` 方法中集成 `BufferService`
- [ ] 实现聚合逻辑
- [ ] 实现 Recall 注入到 memory_extract
- [ ] 添加集成测试

**关键流程**：
```go
func (r *Runner) Run(ctx context.Context, in Input) (Result, error) {
    r.bufferService.Append(in.ConversationID, in.TurnID, in.UserText, in.BotText)

    shouldFlush, _ := r.bufferService.ShouldFlush(in.ConversationID)
    if !shouldFlush {
        return Result{}, nil
    }

    aggregated, _ := r.bufferService.Flush(in.ConversationID)
    recalledNodes := r.recallRelatedMemories(ctx, aggregated)

    raw, _ := r.memoryExtractor(ctx, aggregated, recalledNodes)
    items, _ := ParseMemoryItems(raw)

    // ... IngestTurn ...
}
```

#### 5.4 增强 memory_extract Prompt
**文件**：`internal/api/subtasks_prompts.go`

**任务**：
- [ ] 修改 `buildMemoryExtractPrompt`，支持聚合文本和召回记忆
- [ ] 添加 memory_id 复用逻辑说明
- [ ] 测试 LLM 输出

### 验收标准
- [ ] 同一话题的多轮对话能正确聚合
- [ ] 话题切换能正确检测
- [ ] 时间窗口兜底正常工作
- [ ] 召回的记忆能正确注入到 prompt
- [ ] LLM 能正确输出 memory_id（更新）或空（新建）
- [ ] 集成测试通过

---

## Phase 6: 测试与优化（2-3 天）

### 目标
全面测试、性能优化、文档完善。

### 任务清单

#### 6.1 端到端测试
**任务**：
- [ ] 编写端到端测试用例
- [ ] 测试渐进式完善场景
- [ ] 测试话题切换场景
- [ ] 测试召回准确性

#### 6.2 性能优化
**任务**：
- [ ] 性能测试（embedding 生成、召回延迟）
- [ ] 优化批量查询
- [ ] 添加缓存机制
- [ ] 压力测试

#### 6.3 降级与监控
**任务**：
- [ ] 实现降级逻辑（embedding 失败时回退）
- [ ] 添加监控指标（embedding 成功率、召回延迟）
- [ ] 添加告警规则

#### 6.4 文档完善
**任务**：
- [ ] 更新 API 文档
- [ ] 编写运维文档
- [ ] 编写用户指南

### 验收标准
- [ ] 所有测试通过
- [ ] 性能达标
- [ ] 降级逻辑正常
- [ ] 文档完整

---

## 风险与依赖

### 关键依赖
1. **Embedding API 可用性**：依赖 Astron MaaS 或配置的 embedding 服务
2. **PostgreSQL pgvector 扩展**：可选，但建议安装以提升性能
3. **LLM 质量**：话题检测和 memory_extract 依赖 LLM 输出质量

### 风险缓解
1. **Embedding 服务故障**：降级到词法匹配
2. **话题检测误判**：时间窗口兜底
3. **性能问题**：批量查询、缓存、异步生成

---

## 部署策略

### 灰度发布
1. **Phase 1-2**：仅启用 embedding 相似度，不影响召回
2. **Phase 3**：小范围启用 embedding 召回（10% 用户）
3. **Phase 4-5**：全量启用
4. **Phase 6**：监控优化

### 回滚方案
- 配置开关 `ENABLE_EMBEDDING_RECALL=false` 回退到词法匹配
- 数据无需迁移，embedding 按需生成

---

## 时间线

| 阶段 | 任务 | 预计工期 | 负责人 |
|------|------|----------|--------|
| Phase 1 | Embedding 基础设施 | 2-3 天 | TBD |
| Phase 2 | Embedding 相似度计算 | 1-2 天 | TBD |
| Phase 3 | 纯 Embedding 召回 | 2-3 天 | TBD |
| Phase 4 | Entity 三要素拆解 | 1-2 天 | TBD |
| Phase 5 | 延迟聚合机制 | 3-4 天 | TBD |
| Phase 6 | 测试与优化 | 2-3 天 | TBD |
| **总计** | | **11-17 天** | |

---

## 下一步

1. Review 本实施计划
2. 分配任务负责人
3. 创建 GitHub Issues/Tasks
4. 开始 Phase 1 开发

---

**文档版本**：v1.0
**最后更新**：2026-03-23
