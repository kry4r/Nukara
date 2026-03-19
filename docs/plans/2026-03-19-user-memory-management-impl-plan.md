# 用户可见记忆管理 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让用户在 Web 端查看并删除 AI 记忆，管理员在 Admin Web 也能执行同样操作。

**Architecture:** 后端新增 `DeleteMemoryNode` store 方法 + 3 个 HTTP 端点（用户侧 GET/DELETE，Admin 侧 DELETE）；Web 端在 `BotDetailView.vue` 新增记忆管理卡片；Admin Web 在 `MemoryGraphPanel.vue` 节点行新增删除按钮。

**Tech Stack:** Go 1.22, Vue 3 (Composition API), PostgreSQL

---

## Task 1: Store 层 — 添加 DeleteMemoryNode 接口与内存实现

**Files:**
- Modify: `Nukara_Backend/internal/store/interface.go:91`
- Modify: `Nukara_Backend/internal/store/temporal_memory_graph.go`
- Test: `Nukara_Backend/internal/store/temporal_memory_graph_test.go`

**Step 1: 写失败测试**

在 `temporal_memory_graph_test.go` 末尾追加：

```go
func TestDeleteMemoryNode(t *testing.T) {
	s := NewStore()

	node, err := s.CreateMemoryNode(TemporalMemoryNode{
		UserID:   "user-1",
		BotID:    "bot-1",
		NodeType: "episode",
		Summary:  "test memory",
		Status:   "active",
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}

	// 正常删除
	if err := s.DeleteMemoryNode(node.ID, "user-1", "bot-1"); err != nil {
		t.Fatalf("delete node: %v", err)
	}
	if _, ok := s.GetMemoryNode(node.ID); ok {
		t.Fatal("expected node to be deleted")
	}

	// 删除不存在的 ID
	if err := s.DeleteMemoryNode("nonexistent", "user-1", "bot-1"); err == nil {
		t.Fatal("expected error for nonexistent node")
	}

	// userID 不匹配
	node2, _ := s.CreateMemoryNode(TemporalMemoryNode{
		UserID:   "user-2",
		BotID:    "bot-1",
		NodeType: "episode",
		Summary:  "another memory",
		Status:   "active",
	})
	if err := s.DeleteMemoryNode(node2.ID, "user-1", "bot-1"); err == nil {
		t.Fatal("expected error for mismatched userID")
	}
}
```

**Step 2: 运行测试确认失败**

```bash
cd Nukara_Backend
go test ./internal/store/... -run TestDeleteMemoryNode -v
```

期望：`FAIL — undefined: DeleteMemoryNode`

**Step 3: 在 interface.go 新增方法签名**

在 `store/interface.go` 的 `DataStore` interface 末尾（`ListAllUserIDs` 之前）添加：

```go
DeleteMemoryNode(nodeID, userID, botID string) error
```

**Step 4: 在 temporal_memory_graph.go 新增内存实现**

在文件末尾追加：

```go
func (s *Store) DeleteMemoryNode(nodeID, userID, botID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodeID = strings.TrimSpace(nodeID)
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	node, ok := s.memoryNodesByID[nodeID]
	if !ok {
		return errors.New("memory node not found")
	}
	if node.UserID != userID || node.BotID != botID {
		return errors.New("memory node not found")
	}
	delete(s.memoryNodesByID, nodeID)
	return nil
}
```

**Step 5: 运行测试确认通过**

```bash
go test ./internal/store/... -run TestDeleteMemoryNode -v
```

期望：`PASS`

**Step 6: 提交**

```bash
git add Nukara_Backend/internal/store/interface.go \
        Nukara_Backend/internal/store/temporal_memory_graph.go \
        Nukara_Backend/internal/store/temporal_memory_graph_test.go
git commit -m "feat: add DeleteMemoryNode to store interface and in-memory impl"
```

---

## Task 2: Store 层 — Postgres 实现

**Files:**
- Modify: `Nukara_Backend/internal/store/postgres_temporal_memory_graph.go`

**Step 1: 在 postgres_temporal_memory_graph.go 末尾追加**

```go
func (p *PostgresStore) DeleteMemoryNode(nodeID, userID, botID string) error {
	ctx, cancel := p.withTimeout()
	defer cancel()
	nodeID = strings.TrimSpace(nodeID)
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	result, err := p.db.ExecContext(ctx,
		`DELETE FROM memory_nodes WHERE id = $1 AND user_id = $2 AND bot_id = $3`,
		nodeID, userID, botID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("memory node not found")
	}
	return nil
}
```

注意：`memory_edges` 和 `memory_embeddings` 表已有 `ON DELETE CASCADE`，删除节点会自动级联删除关联边和嵌入，无需额外处理。

**Step 2: 编译确认无错误**

```bash
cd Nukara_Backend
go build ./...
```

期望：无错误输出

**Step 3: 提交**

```bash
git add Nukara_Backend/internal/store/postgres_temporal_memory_graph.go
git commit -m "feat: add DeleteMemoryNode postgres implementation"
```

---

## Task 3: 用户侧 API — GET /api/v1/bots/{botID}/memories

**Files:**
- Modify: `Nukara_Backend/internal/api/server.go`

**Step 1: 在 handleBotByID 的 switch 里新增 "memories" case**

找到 `server.go:514` 的 `switch parts[1]` 块，在 `case "persona-changes":` 之前插入：

```go
case "memories":
    s.handleBotMemories(w, r, userID, botID, parts[2:])
    return
```

**Step 2: 在 server.go 末尾追加 handleBotMemories**

```go
func (s *Server) handleBotMemories(w http.ResponseWriter, r *http.Request, userID, botID string, rest []string) {
	if _, found := s.store.GetBot(userID, botID); !found {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return
	}

	// DELETE /api/v1/bots/{botID}/memories/{memoryID}
	if r.Method == http.MethodDelete && len(rest) > 0 {
		memoryID := strings.TrimSpace(rest[0])
		if memoryID == "" {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": "memory id missing"})
			return
		}
		if err := s.store.DeleteMemoryNode(memoryID, userID, botID); err != nil {
			respondJSON(w, http.StatusNotFound, map[string]any{"error": "memory not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// GET /api/v1/bots/{botID}/memories
	if r.Method == http.MethodGet {
		nodes := s.store.ListMemoryNodes(userID, botID, store.TemporalMemoryNodeFilter{
			Status: "active",
			Limit:  100,
		})
		type memoryItem struct {
			ID             string `json:"id"`
			NodeType       string `json:"node_type"`
			Title          string `json:"title"`
			Summary        string `json:"summary"`
			OccurredAt     string `json:"occurred_at"`
			StabilityLabel string `json:"stability_label"`
		}
		items := make([]memoryItem, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, memoryItem{
				ID:             n.ID,
				NodeType:       n.NodeType,
				Title:          n.Title,
				Summary:        n.Summary,
				OccurredAt:     n.OccurredAt.UTC().Format(time.RFC3339),
				StabilityLabel: n.StabilityLabel,
			})
		}
		respondJSON(w, http.StatusOK, map[string]any{"memories": items})
		return
	}

	methodNotAllowed(w)
}
```

**Step 3: 编译确认**

```bash
cd Nukara_Backend
go build ./...
```

**Step 4: 提交**

```bash
git add Nukara_Backend/internal/api/server.go
git commit -m "feat: add GET/DELETE /api/v1/bots/{botID}/memories endpoints"
```

---

## Task 4: Admin 侧 API — DELETE /api/admin/users/{uid}/bots/{bid}/memories/{id}

**Files:**
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler.go`

**Step 1: 在 handleAdminUserGraphRoutes 新增路由分支**

找到 `memory_graph_handler.go:397`（`if len(parts) == 4 && parts[1] == "bots" && parts[3] == "memory-graph"` 之后），插入：

```go
if len(parts) == 5 && parts[1] == "bots" && parts[3] == "memories" {
    s.handleAdminDeleteMemory(w, r, parts[0], parts[2], parts[4])
    return
}
```

**Step 2: 在文件末尾追加 handleAdminDeleteMemory**

```go
func (s *Server) handleAdminDeleteMemory(w http.ResponseWriter, r *http.Request, userID, botID, memoryID string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	memoryID = strings.TrimSpace(memoryID)
	if userID == "" || botID == "" || memoryID == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}
	result, err := s.db.Exec(
		`DELETE FROM memory_nodes WHERE id = $1 AND user_id = $2 AND bot_id = $3`,
		memoryID, userID, botID,
	)
	if err != nil {
		http.Error(w, "Failed to delete memory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "memory not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 3: 编译确认**

```bash
cd Nukara_Backend
go build ./...
```

**Step 4: 提交**

```bash
git add Nukara_Backend/internal/admin/memory_graph_handler.go
git commit -m "feat: add DELETE /api/admin/users/{uid}/bots/{bid}/memories/{id}"
```

---

## Task 5: Web 端 — BotDetailView.vue 记忆管理卡片

**Files:**
- Modify: `Nukara_Web/src/views/BotDetailView.vue`

**Step 1: 在 `<script setup>` 顶部新增状态变量**

在 `const recentChanges = computed(...)` 之后追加：

```js
const memories = ref([])
const memoriesLoading = ref(false)
const memoriesError = ref('')
```

**Step 2: 新增时间分组工具函数**

在 `function applyProfilePayload` 之前追加：

```js
function groupMemoriesByTime(list) {
  const now = new Date()
  const startOfWeek = new Date(now)
  startOfWeek.setDate(now.getDate() - now.getDay())
  startOfWeek.setHours(0, 0, 0, 0)
  const startOfLastWeek = new Date(startOfWeek)
  startOfLastWeek.setDate(startOfWeek.getDate() - 7)

  const groups = { '本周': [], '上周': [], '更早': [] }
  for (const m of list) {
    const d = new Date(m.occurred_at)
    if (d >= startOfWeek) groups['本周'].push(m)
    else if (d >= startOfLastWeek) groups['上周'].push(m)
    else groups['更早'].push(m)
  }
  return Object.entries(groups).filter(([, items]) => items.length > 0)
}

const groupedMemories = computed(() => groupMemoriesByTime(memories.value))
```

**Step 3: 新增加载和删除函数**

在 `async function refreshImpression` 之后追加：

```js
async function loadMemories() {
  if (!botID.value) return
  memoriesLoading.value = true
  memoriesError.value = ''
  try {
    const data = await api.get(`/api/v1/bots/${botID.value}/memories`)
    memories.value = Array.isArray(data?.memories) ? data.memories : []
  } catch (e) {
    memoriesError.value = e?.message || '加载记忆失败'
  } finally {
    memoriesLoading.value = false
  }
}

async function deleteMemory(memoryID) {
  if (!window.confirm('确定删除这条记忆？AI 将不再记得这件事。')) return
  try {
    await api.del(`/api/v1/bots/${botID.value}/memories/${memoryID}`)
    memories.value = memories.value.filter(m => m.id !== memoryID)
  } catch (e) {
    memoriesError.value = e?.message || '删除失败'
  }
}
```

**Step 4: 在 onMounted 里追加懒加载**

将 `onMounted` 改为：

```js
onMounted(async () => {
  await loadProfile()
  loadMemories()
})
```

**Step 5: 在模板里新增记忆管理卡片**

在"最近人设变更记录"卡片（`</section>` 结束标签）之后，`</template>` 之前插入：

```html
<section class="card">
  <h3>记忆管理</h3>
  <p v-if="memoriesLoading" class="muted">加载中...</p>
  <p v-else-if="memoriesError" class="error">{{ memoriesError }}</p>
  <p v-else-if="!memories.length" class="muted">暂无记忆记录</p>
  <template v-else>
    <div v-for="[group, items] in groupedMemories" :key="group" class="memory-group">
      <p class="memory-group-label">{{ group }}</p>
      <div v-for="m in items" :key="m.id" class="memory-row memory-manage-row">
        <div class="memory-head">
          <span class="mini-kind">{{ m.node_type }}</span>
          <span class="muted" style="font-size:11px">{{ m.occurred_at?.slice(0,10) }}</span>
        </div>
        <p class="paragraph">{{ m.summary || m.title }}</p>
        <button type="button" class="ghost-btn danger-btn" @click="deleteMemory(m.id)">删除</button>
      </div>
    </div>
  </template>
</section>
```

**Step 6: 在 `<style scoped>` 末尾追加样式**

```css
.memory-group-label {
  font-size: 11px;
  color: var(--text-muted);
  margin: 12px 0 4px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.memory-manage-row {
  position: relative;
}
.danger-btn {
  color: var(--color-danger, #e05);
  margin-top: 4px;
}
```

**Step 7: 提交**

```bash
git add Nukara_Web/src/views/BotDetailView.vue
git commit -m "feat: add memory management card to BotDetailView"
```

---

## Task 6: Admin Web — MemoryGraphPanel.vue 删除按钮

**Files:**
- Modify: `Nukara_Admin_Web/src/api/admin.js`
- Modify: `Nukara_Admin_Web/src/components/MemoryGraphPanel.vue`

**Step 1: 在 admin.js 末尾追加 deleteAdminMemory**

```js
export function deleteAdminMemory(userId, botId, memoryId) {
  return request(`/api/admin/users/${userId}/bots/${botId}/memories/${memoryId}`, {
    method: 'DELETE',
  })
}
```

**Step 2: 在 MemoryGraphPanel.vue 的 import 里引入新函数**

找到：
```js
import {
  getMemoryGraph,
  listAdminUsers,
  listUserBots,
} from '../api/admin.js'
```

改为：
```js
import {
  deleteAdminMemory,
  getMemoryGraph,
  listAdminUsers,
  listUserBots,
} from '../api/admin.js'
```

**Step 3: 新增 deletingNodeId 状态和 deleteNode 函数**

在 `const selectedNodeId = ref('')` 之后追加：

```js
const deletingNodeId = ref('')

async function deleteNode(nodeId) {
  if (!selectedUserId.value || !selectedBotId.value) return
  deletingNodeId.value = nodeId
  try {
    await deleteAdminMemory(selectedUserId.value, selectedBotId.value, nodeId)
    await loadGraph()
  } catch (e) {
    errorMessage.value = e?.message || '删除失败'
  } finally {
    deletingNodeId.value = ''
  }
}
```

**Step 4: 在节点列表模板里找到节点行，追加删除按钮**

找到渲染节点列表的 `v-for` 循环（搜索 `node.label` 或 `node.content`），在每行末尾追加：

```html
<button
  type="button"
  :disabled="deletingNodeId === node.id"
  @click.stop="deleteNode(node.id)"
  style="margin-left:8px;color:#e05;background:none;border:none;cursor:pointer;font-size:12px"
>
  {{ deletingNodeId === node.id ? '删除中...' : '删除' }}
</button>
```

**Step 5: 提交**

```bash
git add Nukara_Admin_Web/src/api/admin.js \
        Nukara_Admin_Web/src/components/MemoryGraphPanel.vue
git commit -m "feat: add memory delete button to admin MemoryGraphPanel"
```

---

## Task 7: 验证

**Step 1: 后端全量测试**

```bash
cd Nukara_Backend
go test ./...
```

期望：全部 PASS，无新增失败

**Step 2: 后端编译**

```bash
go build ./cmd/gateway
go build ./cmd/bot
go build ./cmd/conversation
```

**Step 3: 手动冒烟测试（需要本地服务运行）**

```bash
# 获取 token（替换为真实值）
TOKEN="your_jwt_token"
BOT_ID="your_bot_id"

# 列出记忆
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/bots/$BOT_ID/memories

# 删除一条记忆（替换 MEMORY_ID）
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/bots/$BOT_ID/memories/MEMORY_ID
```

**Step 4: 最终提交（如有遗漏文件）**

```bash
git add -A
git commit -m "feat: complete user-visible memory management (read + delete)"
```
