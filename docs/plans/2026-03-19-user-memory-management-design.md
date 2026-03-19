# 用户可见记忆管理 — 设计文档

**日期**: 2026-03-19
**状态**: 已批准
**范围**: 方案 A（最小可行）

---

## 一、背景与目标

当前记忆系统（`TemporalMemoryNode`）完全在后端内部运行，用户无法知道 AI 记住了什么，也无法纠正或清除不想要的记忆。

目标：让用户能在 Web 端查看并删除 AI 对自己的记忆，同时让管理员在 Admin Web 也能执行同样操作。

---

## 二、范围决策

| 决策项 | 结论 |
|--------|------|
| 操作权限 | 读 + 删除（不支持编辑和新增） |
| 平台 | Nukara_Web（用户侧）+ Nukara_Admin_Web（管理侧） |
| 展示方式 | 按时间分组（本周 / 上周 / 更早） |
| 删除方式 | 硬删除 |
| 数据源 | `TemporalMemoryNode`（不碰旧的 `MemoryItem`） |

---

## 三、架构概览

```
用户侧 (Nukara_Web)
  BotDetailView.vue
    └── 新增"记忆管理"卡片
          ├── GET  /api/v1/bots/{botID}/memories
          └── DELETE /api/v1/bots/{botID}/memories/{memoryID}

管理侧 (Nukara_Admin_Web)
  MemoryGraphPanel.vue
    └── 节点行新增删除按钮
          └── DELETE /api/admin/users/{uid}/bots/{bid}/memories/{memoryID}

后端
  internal/api/server.go     ← 用户侧端点
  internal/store/interface.go ← 新增 DeleteMemoryNode 方法
  internal/store/store.go     ← 内存实现
  internal/store/postgres_temporal_memory_graph.go ← Postgres 实现
  internal/admin/memory_graph_handler.go ← Admin 端点
```

---

## 四、后端 API

### 4.1 用户侧

**GET `/api/v1/bots/{botID}/memories`**

- 认证：Bearer token（`authUserID`）
- 查询参数：`limit`（默认 100）
- 返回：

```json
{
  "memories": [
    {
      "id": "string",
      "node_type": "episode|habit|fact|session_summary",
      "title": "string",
      "summary": "string",
      "occurred_at": "2026-03-01T10:00:00Z",
      "stability_label": "stable|volatile|core"
    }
  ]
}
```

**DELETE `/api/v1/bots/{botID}/memories/{memoryID}`**

- 认证：Bearer token
- 校验：`userID + botID + memoryID` 三重匹配，不匹配返回 404
- 成功返回：`204 No Content`

路由挂载在现有 `handleBotByID` 的子资源分发里（`case "memories":`），与 `directives`、`profile` 同级。

### 4.2 Store 层

在 `store/interface.go` 新增：

```go
DeleteMemoryNode(nodeID, userID, botID string) error
```

内存实现：从 `memoryNodesByID` map 中删除，校验 userID 和 botID。

Postgres 实现：
```sql
DELETE FROM memory_nodes
WHERE id = $1 AND user_id = $2 AND bot_id = $3
```

### 4.3 管理侧

**DELETE `/api/admin/users/{uid}/bots/{bid}/memories/{memoryID}`**

- 认证：Basic Auth（现有 admin 认证）
- 直接走 DB，不经过 store 接口
- 成功返回：`204 No Content`

在 `handleAdminUserGraphRoutes` 路由分发里新增：
```go
if len(parts) == 5 && parts[1] == "bots" && parts[3] == "memories" {
    s.handleAdminDeleteMemory(w, r, parts[0], parts[2], parts[4])
    return
}
```

---

## 五、前端

### 5.1 Nukara_Web — BotDetailView.vue

在现有"关键记忆"卡片下方新增"记忆管理"卡片：

- 进入页面时懒加载（不阻塞主内容）
- 按时间分组：本周 / 上周 / 更早
- 每条显示：`node_type` 标签 + `summary` + `occurred_at` 相对时间
- 删除：点击删除按钮 → `window.confirm` 确认 → 调用 DELETE → 成功后从列表移除
- 复用现有 `useApi()` 的 `api.get()` 和 `api.del()`

### 5.2 Nukara_Admin_Web — MemoryGraphPanel.vue

在现有节点列表每行右侧新增删除按钮：

- 点击直接调用 admin DELETE 端点（无确认弹窗）
- 成功后重新加载图谱数据（调用现有 `loadGraph()`）
- 在 `admin.js` 新增 `deleteAdminMemory(userId, botId, memoryId)` 函数

---

## 六、错误处理

| 场景 | 处理 |
|------|------|
| 删除不存在的记忆 | 后端 404，前端显示"记忆不存在或已删除" |
| 越权删除 | 后端三重校验，不匹配返回 404（不暴露存在性） |
| 网络失败 | 前端 catch，显示错误文字，不移除列表项 |
| 空列表 | 显示"暂无记忆记录" |

---

## 七、测试

后端单元测试（`store_test.go` 模式）：
- `DeleteMemoryNode` 正常删除
- `DeleteMemoryNode` ID 不存在 → 返回 error
- `DeleteMemoryNode` userID 不匹配 → 返回 error

前端：不新增测试（与现有 Web 端保持一致）。
