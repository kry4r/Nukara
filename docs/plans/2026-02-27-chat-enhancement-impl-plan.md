# 聊天增强实现计划

日期：2026-02-27
基于设计文档：`2026-02-27-chat-enhancement-design.md`

## 依赖关系

```
Step 1 (消息聚合) ──┐
Step 2 (情绪追踪) ──┤
                    ├── Step 5 (Playwright E2E)
Step 3 (主动消息) ──┤
Step 4 (聊天Skill) ─┘
```

Step 1-4 互相独立，可并行。Step 5 依赖全部完成。

---

## Step 1：智能消息聚合

**目标文件：** `Nukara_Backend/internal/api/ws_chat.go`（lines 59-146 messageAggregator）

### 1.1 重构 messageAggregator 结构体

当前结构体只有 `prompts []string` 和固定 `aggregateDelay`。需要扩展：

```go
type messageAggregator struct {
    mu            sync.Mutex
    buffers       map[string]*aggregateBuffer
    flushCallback func(convID, userID string, prompts []string)
}

type aggregateBuffer struct {
    prompts      []string
    timer        *time.Timer
    hardTimer    *time.Timer    // 新增：8s硬上限计时器
    isTyping     bool           // 新增：用户是否正在typing
    msgCount     int            // 新增：消息计数
    firstMsgTime time.Time      // 新增：首条消息时间
    userID       string
}
```

### 1.2 实现动态窗口计算

新增函数 `calcDelay(msgLen int) time.Duration`：
- `< 10字` → 2s
- `10-50字` → 1.5s
- `> 50字` → 1s

### 1.3 实现typing感知

- 在 `handleWSChat()` 中识别 `type: "typing"` 消息
- typing=true → 暂停聚合计时器（保留硬上限）
- typing=false → 恢复计时器 + 500ms缓冲

### 1.4 实现硬上限

- 首条消息到达时启动8s `hardTimer`
- `hardTimer` 到期 → 强制flush，不管typing状态
- 消息计数达10条 → 强制flush
- flush后如果用户还在typing → 开启新一轮聚合

### 1.5 改进合并格式

flush时不再用 `strings.Join(buf.prompts, "\n")`，改为：
- 单条消息 → 直接传原文
- 多条消息 → 格式化为 `[用户连续发送了N条消息]\n1. xxx\n2. xxx`

### 1.6 前端typing事件

确认 `Nukara_Web` 的 `useWebSocket.js` 是否已发送typing事件。如果没有，在 `chat.js` 的 `sendMessage` 中增加typing状态发送。

### 验证

- 单元测试：修改 `ws_chat_test.go`，覆盖动态窗口、typing暂停、硬上限三种场景
- `go test ./internal/api/...`

---

## Step 2：用户情绪追踪

**新建文件：** `Nukara_Backend/internal/api/emotion_tracker.go`

### 2.1 定义 EmotionContext 结构体

```go
type EmotionContext struct {
    RecentEmotions []string  // 最近5条消息的emotion
    EmotionTrend   string    // "positive" / "negative" / "neutral"
    LastTone       string    // 最后一条消息的语气
}
```

### 2.2 实现LLM批量情绪分析

每收集5条用户消息后，调一次LLM分析情绪趋势。

**流程：**
- 用户每发一条消息 → 追加到Redis消息缓冲 `emotion_buf:{userID}:{botID}`
- 缓冲达5条 → 触发LLM情绪分析
- LLM返回整体情绪趋势 → 更新 `EmotionContext` → 清空缓冲

**LLM分析prompt：**

```
分析以下用户消息的情绪趋势，返回JSON格式：

用户消息：
1. {msg1}
2. {msg2}
3. {msg3}
4. {msg4}
5. {msg5}

返回格式：{"emotions": ["happy","neutral",...], "trend": "positive/negative/neutral", "tone": "描述语气"}
只输出JSON，不要解释。
```

**异步执行：** 情绪分析不阻塞聊天流程，用goroutine异步调用。

### 2.3 Redis存储

两个Key：
- 消息缓冲：`emotion_buf:{userID}:{botID}` — List类型，存最近未分析的消息，无TTL（分析后清空）
- 情绪结果：`emotion:{userID}:{botID}` — JSON序列化的 `EmotionContext`，TTL 24h

函数：
- `BufferMessage(userID, botID, text string)` — RPUSH到缓冲，检查长度>=5则触发分析
- `AnalyzeEmotionBatch(userID, botID string)` — 读缓冲 → 调LLM → 更新EmotionContext → 清缓冲
- `GetEmotionContext(userID, botID string) EmotionContext` — 读Redis

### 2.4 接入点

在 `ws_chat.go` 的 `handleWSChatMessage()` 中，用户消息存储后调用 `BufferMessage()`。达到5条时异步触发 `AnalyzeEmotionBatch()`。

### 验证

- 单元测试：`emotion_tracker_test.go`，覆盖LLM批量分析、Redis缓冲读写、EmotionContext更新
- `go test ./internal/api/...`

---

## Step 3：主动消息触发器扩展

**目标文件：** `Nukara_Backend/internal/api/scheduler.go`（lines 14-276）

### 3.1 扩展触发器类型

当前 `detectTrigger()` 只有3种。扩展为5种：

| 触发器 | 检测逻辑 |
|--------|----------|
| `morning_care` | 8-9点 + 昨天有对话（现有，微调条件） |
| `evening_care` | 21-22点 + 今天有对话（现有，微调条件） |
| `curiosity_after_silence` | 沉默30min-2h + 24h内消息数>5 |
| `worry_after_long_silence` | 沉默3h+ + EmotionTrend=="negative" |
| `random_share` | 10-12点或14-17点 + 随机概率30% |

### 3.2 扩展冷却机制

当前按频率设置统一冷却。改为按触发器类型独立冷却：

```go
var triggerCooldowns = map[string]time.Duration{
    "morning_care":             24 * time.Hour,
    "evening_care":             24 * time.Hour,
    "curiosity_after_silence":  4 * time.Hour,
    "worry_after_long_silence": 8 * time.Hour,
    "random_share":             24 * time.Hour,
}
```

Redis Key: `proactive_cooldown:{userID}:{botID}:{triggerType}`

### 3.3 扩展 Agent.Proactive()

当前 `agent.go` 的 `Proactive()` 方法只支持3种trigger type的简单prompt。需要：

- 接收 `EmotionContext` 参数
- 接收 `lastUserMessage` 和 `recentTopicSummary` 参数
- 使用设计文档中的增强prompt模板（含trigger_description）

### 3.4 scheduler集成情绪追踪

在 `processUser()` 中，检测到候选触发后：
- 调用 `GetEmotionContext()` 获取情绪上下文
- `worry_after_long_silence` 仅在 `EmotionTrend=="negative"` 时触发
- 将 `EmotionContext` 传递给 `Agent.Proactive()`

### 验证

- 单元测试：扩展 `scheduler_test.go`（如果存在）或新建，覆盖5种触发器的条件判断
- `go test ./internal/api/...`

---

## Step 4：Nanobot聊天风格Skill

**目标：** 给Nanobot注册一个system-level的聊天风格Skill，约束回复行为。

### 4.1 确认Nanobot Skill注册方式

需要先了解Nanobot的skill注册API/配置方式，确定：
- Skill定义格式（JSON/YAML/其他）
- 注册方式（API调用/配置文件/启动参数）
- Skill是否支持system-level注入（每次对话自动生效）

### 4.2 编写聊天风格Skill内容

四个维度的规则文本（来自设计文档）：

**维度一：回复长度匹配**
- 对方一句话 → 一两句话回复
- 简单问题 → 简短直接
- 深入话题 → 可以多说
- 语气词/表情 → 语气词/表情回应

**维度二：语气自然度**
- 用口语、省略、语气词
- 禁止列清单、分点回答、主动科普
- 禁止"首先...其次...最后..."结构

**维度三：记忆与上下文感知**
- 延续话题时不要当新话题
- 自然提及用户提过的偏好/习惯
- 不要生硬展示记忆，自然带出

**维度四：人设一致性**
- 每句回复符合角色设定和语言风格
- 不同人设对同一场景用完全不同的表达
- 任何情况下不跳出人设用客服语气
- 人设 > 信息准确性

### 4.3 注册Skill到Nanobot

根据4.1确认的注册方式，将Skill内容注册到Nanobot。确保每次对话自动生效。

### 4.4 备选：system_context注入

如果Nanobot的Skill系统不支持system-level自动注入，则在 `agent.go` 的 `BuildSystemContext()` 中将聊天风格规则追加到 `system_context` 字段。

### 验证

- 手动测试：发送简单问题（"吃了吗"、"天气怎么样"），验证回复长度
- 对比测试：同一问题发给不同人设bot，验证风格差异

---

## Step 5：Playwright E2E测试

**目标：** 通过Vue.js Web前端验证所有模块的实际效果。

### 5.1 测试方式

直接通过Claude Code的Playwright MCP工具操作浏览器，无需安装Playwright依赖：
- 用 `browser_navigate` 打开本地dev环境的Web前端
- 用 `browser_snapshot` / `browser_click` / `browser_type` 操作UI
- 用 `browser_wait_for` 等待bot回复出现
- 用 `browser_console_messages` 检查WebSocket消息

### 5.2 测试辅助：环境变量调整

主动消息的沉默阈值在生产环境是30min-2h，E2E测试中需要缩短：
- `NUKARA_INACTIVITY_THRESHOLD=30s`
- `NUKARA_PROACTIVE_SCAN_INTERVAL=10s`（扫描间隔从5min缩短到10s）

### 5.3 测试场景

通过Playwright MCP手动执行以下场景：

**测试1：消息聚合 - 连发短消息**
- 登录 → 进入对话
- 快速发送3条短消息（间隔<500ms）
- 断言：只收到1次bot回复（而非3次）

**测试2：消息聚合 - 硬上限**
- 持续发送消息，间隔1s，共10条
- 断言：在8s左右收到第一次bot回复，后续消息触发第二轮

**测试3：主动消息 - 沉默触发**
- 发送几条消息后静默
- 等待30-60s（测试环境缩短阈值）
- 断言：收到bot主动发来的消息，且`is_proactive=true`

**测试4：回复质量 - 短问短答**
- 发送简单问题："吃了吗"、"天气怎么样"
- 断言：bot回复不超过2句话

**测试5：回复质量 - 人设一致**
- 同一问题发给不同人设的bot
- 断言：回复风格明显不同（温柔型用"呢/呀"，傲娇型用"哼/才不是"等）

### 验证

- 所有测试场景通过Playwright MCP在浏览器中手动验证
- 截图记录关键测试结果
