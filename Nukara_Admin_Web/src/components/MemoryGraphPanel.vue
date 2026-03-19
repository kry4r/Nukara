<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  deleteAdminMemory,
  getMemoryGraph,
  listAdminUsers,
  listUserBots,
} from '../api/admin.js'
import {
  buildGraphNeighborSet,
  buildMemoryGraphLayout,
} from '../utils/memory-graph-layout.js'

const searchQuery = ref('')
const loadingUsers = ref(false)
const loadingBots = ref(false)
const loadingGraph = ref(false)
const statusMessage = ref('')
const errorMessage = ref('')
const users = ref([])
const bots = ref([])
const filters = reactive({
  kind: '',
  status: 'active',
})

function emptyGraph() {
  return {
    nodes: [],
    edges: [],
    summary: { memory_count: 0, topic_count: 0, graph_source: 'store', kind_filter: '', status_filter: 'active' },
    runtime_state: null,
    recent_impressions: [],
    recent_changes: [],
    pending_persona_changes: [],
  }
}

const graph = ref(emptyGraph())
const selectedUserId = ref('')
const selectedBotId = ref('')
const selectedNodeId = ref('')
const deletingNodeId = ref('')

async function deleteNode(nodeId) {
  if (!selectedUserId.value || !selectedBotId.value) return
  deletingNodeId.value = nodeId
  try {
    await deleteAdminMemory(selectedUserId.value, selectedBotId.value, nodeId)
    await loadSelectedBotGraph()
  } catch (e) {
    errorMessage.value = e?.message || '删除失败'
  } finally {
    deletingNodeId.value = ''
  }
}

const selectedNode = computed(() => graph.value.nodes.find((node) => node.id === selectedNodeId.value) || null)
const runtimeState = computed(() => graph.value.runtime_state || null)
const recentImpressions = computed(() => Array.isArray(graph.value.recent_impressions) ? graph.value.recent_impressions : [])
const recentChanges = computed(() => Array.isArray(graph.value.recent_changes) ? graph.value.recent_changes : [])
const pendingPersonaChanges = computed(() => Array.isArray(graph.value.pending_persona_changes) ? graph.value.pending_persona_changes : [])

const memoryKindOptions = [
  { value: '', label: '全部类型' },
  { value: 'promise', label: 'Promise' },
  { value: 'event', label: 'Event' },
  { value: 'self_fact', label: 'Self Fact' },
  { value: 'user_fact', label: 'User Fact' },
  { value: 'habit', label: 'Habit' },
]
const statusOptions = [
  { value: 'active', label: 'Active' },
  { value: 'fulfilled', label: 'Fulfilled' },
  { value: 'pending', label: 'Pending' },
  { value: 'accepted', label: 'Accepted' },
  { value: 'all', label: 'All' },
]

const canvasWidth = 1160
const canvasHeight = 780

const graphLayout = computed(() => buildMemoryGraphLayout(graph.value.nodes, graph.value.edges, {
  width: canvasWidth,
  height: canvasHeight,
}))
const relatedNodeIds = computed(() => buildGraphNeighborSet(graph.value.edges, selectedNodeId.value))
const hasRelatedSelection = computed(() => relatedNodeIds.value.size > 0)
const selectedNodeNeighbors = computed(() => {
  if (!selectedNode.value) {
    return []
  }
  const ids = buildGraphNeighborSet(graph.value.edges, selectedNode.value.id)
  ids.delete(selectedNode.value.id)
  return graph.value.nodes.filter((node) => ids.has(node.id)).slice(0, 12)
})

async function refreshUsers() {
  loadingUsers.value = true
  errorMessage.value = ''
  try {
    const payload = await listAdminUsers({ q: searchQuery.value, limit: 50, offset: 0 })
    users.value = Array.isArray(payload.items) ? payload.items : []
    if (selectedUserId.value && !users.value.some((item) => item.user_id === selectedUserId.value)) {
      selectedUserId.value = ''
      selectedBotId.value = ''
      bots.value = []
      graph.value = emptyGraph()
    }
    statusMessage.value = `已加载 ${users.value.length} 位用户。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loadingUsers.value = false
  }
}

async function selectUser(user) {
  if (!user?.user_id) return
  selectedUserId.value = user.user_id
  selectedBotId.value = ''
  selectedNodeId.value = ''
  graph.value = emptyGraph()
  loadingBots.value = true
  errorMessage.value = ''
  try {
    const payload = await listUserBots(user.user_id)
    bots.value = Array.isArray(payload.items) ? payload.items : []
    statusMessage.value = `${user.nickname || user.email} 下共有 ${bots.value.length} 个机器人。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loadingBots.value = false
  }
}

async function loadSelectedBotGraph() {
  if (!selectedUserId.value || !selectedBotId.value) return
  loadingGraph.value = true
  errorMessage.value = ''
  try {
    graph.value = await getMemoryGraph(selectedUserId.value, selectedBotId.value, {
      kind: filters.kind,
      status: filters.status,
    })
    if (graph.value.nodes?.length) {
      selectedNodeId.value = graph.value.nodes[0].id
    }
    statusMessage.value = `图谱已加载：${graph.value.summary?.memory_count || 0} 条记忆，${graph.value.summary?.topic_count || 0} 个主题。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loadingGraph.value = false
  }
}

async function selectBot(bot) {
  if (!selectedUserId.value || !bot?.bot_id) return
  selectedBotId.value = bot.bot_id
  selectedNodeId.value = ''
  await loadSelectedBotGraph()
}

function edgePath(item) {
  const source = item.source
  const target = item.target
  const dx = target.x - source.x
  const dy = target.y - source.y
  const distance = Math.hypot(dx, dy) || 1
  const curve = item.type === 'topic' ? 22 : 14
  const controlX = (source.x + target.x) / 2 - (dy / distance) * curve
  const controlY = (source.y + target.y) / 2 + (dx / distance) * curve
  return `M ${source.x} ${source.y} Q ${controlX} ${controlY} ${target.x} ${target.y}`
}

function topicDiamondPath(node) {
  const radius = node.radius || 24
  return [
    `M ${node.x} ${node.y - radius}`,
    `L ${node.x + radius} ${node.y}`,
    `L ${node.x} ${node.y + radius}`,
    `L ${node.x - radius} ${node.y}`,
    'Z',
  ].join(' ')
}

function isNodeDimmed(node) {
  return hasRelatedSelection.value && !relatedNodeIds.value.has(node.id)
}

function isEdgeDimmed(item) {
  return hasRelatedSelection.value
    && !(relatedNodeIds.value.has(item.source.id) && relatedNodeIds.value.has(item.target.id))
}

function formatDateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

onMounted(() => {
  refreshUsers()
})
</script>

<template>
  <article class="memory-graph-card">
    <header class="graph-header">
      <div>
        <p class="panel-eyebrow">记忆图谱</p>
        <h2>按 User → Robot 查看记忆图</h2>
      </div>
      <button type="button" class="ghost" :disabled="loadingUsers" @click="refreshUsers">
        {{ loadingUsers ? '刷新中...' : '刷新用户' }}
      </button>
    </header>

    <div class="graph-topbar">
      <label class="search-field">
        <span>搜索用户</span>
        <input v-model.trim="searchQuery" placeholder="按邮箱 / 昵称 / 用户 ID 搜索" @keyup.enter="refreshUsers" />
      </label>
      <label class="mini-filter">
        <span>Kind</span>
        <select v-model="filters.kind">
          <option v-for="item in memoryKindOptions" :key="item.value || 'all'" :value="item.value">{{ item.label }}</option>
        </select>
      </label>
      <label class="mini-filter">
        <span>Status</span>
        <select v-model="filters.status">
          <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select>
      </label>
      <button type="button" class="ghost" :disabled="loadingGraph || !selectedBotId" @click="loadSelectedBotGraph">应用筛选</button>
    </div>

    <div class="graph-layout">
      <aside class="selector-column">
        <section class="selector-group">
          <h3>Users</h3>
          <div v-if="loadingUsers" class="mini-empty">加载中...</div>
          <button
            v-for="user in users"
            :key="user.user_id"
            type="button"
            class="selector-card"
            :class="{ active: user.user_id === selectedUserId }"
            @click="selectUser(user)"
          >
            <strong>{{ user.nickname || '未命名用户' }}</strong>
            <span>{{ user.email }}</span>
          </button>
          <div v-if="!loadingUsers && users.length === 0" class="mini-empty">暂无匹配用户</div>
        </section>

        <section class="selector-group">
          <h3>Robots</h3>
          <div v-if="loadingBots" class="mini-empty">机器人加载中...</div>
          <button
            v-for="bot in bots"
            :key="bot.bot_id"
            type="button"
            class="selector-card"
            :class="{ active: bot.bot_id === selectedBotId }"
            @click="selectBot(bot)"
          >
            <strong>{{ bot.name || '未命名机器人' }}</strong>
            <span>{{ bot.bot_id }}</span>
          </button>
          <div v-if="selectedUserId && !loadingBots && bots.length === 0" class="mini-empty">该用户暂无机器人</div>
          <div v-if="!selectedUserId" class="mini-empty">先选择一个用户</div>
        </section>
      </aside>

      <section class="graph-stage-card">
        <div class="graph-summary">
          <span class="summary-pill">Memory：{{ graph.summary?.memory_count || 0 }}</span>
          <span class="summary-pill">Topic：{{ graph.summary?.topic_count || 0 }}</span>
          <span class="summary-pill">Source：{{ graph.summary?.graph_source || 'store' }}</span>
        </div>

        <div v-if="loadingGraph" class="graph-empty">图谱加载中...</div>
        <div v-else-if="!selectedBotId" class="graph-empty">先选择一个 Robot 查看图谱</div>
        <div v-else-if="!graph.nodes?.length" class="graph-empty">当前 Robot 暂无记忆图数据</div>
        <div v-else class="graph-stage">
          <div class="graph-stage-toolbar">
            <div class="legend-group">
              <span class="legend-item"><i class="legend-dot topic"></i>主题节点</span>
              <span class="legend-item"><i class="legend-dot memory"></i>记忆节点</span>
              <span class="legend-item"><i class="legend-line"></i>关联边</span>
            </div>
            <span class="legend-note">点击节点查看详情，并高亮直接关联节点</span>
          </div>

          <svg :viewBox="`0 0 ${graphLayout.width} ${graphLayout.height}`" class="graph-svg" role="img" aria-label="memory graph">
            <path
              v-for="item in graphLayout.edges"
              :key="item.id"
              :d="edgePath(item)"
              class="graph-edge"
              :class="{ dimmed: isEdgeDimmed(item), active: !isEdgeDimmed(item) && hasRelatedSelection }"
            />
            <g
              v-for="node in graphLayout.nodes"
              :key="node.id"
              class="graph-node"
              :class="[
                `node-${node.type}`,
                { active: node.id === selectedNodeId, dimmed: isNodeDimmed(node) },
              ]"
              @click="selectedNodeId = node.id"
            >
              <path v-if="node.shape === 'diamond'" :d="topicDiamondPath(node)" class="node-shape" />
              <circle v-else :cx="node.x" :cy="node.y" :r="node.radius" class="node-shape" />
              <text :x="node.x" :y="node.y + node.radius + 18" text-anchor="middle" class="node-label">{{ node.shortLabel }}</text>
              <text
                v-if="node.type === 'memory'"
                :x="node.x"
                :y="node.y + node.radius + 34"
                text-anchor="middle"
                class="node-sub"
              >
                {{ node.metaLabel }}
              </text>
            </g>
          </svg>
        </div>
      </section>

      <aside class="detail-column">
        <section class="detail-block">
          <h3>节点详情</h3>
          <div v-if="!selectedNode" class="mini-empty">点击图中节点查看详细内容</div>
          <div v-else class="detail-card">
            <p class="detail-type">{{ selectedNode.type === 'memory' ? 'Memory' : 'Topic' }}</p>
            <h4>{{ selectedNode.label }}</h4>
            <p v-if="selectedNode.content" class="detail-content">{{ selectedNode.content }}</p>
            <div v-if="selectedNodeNeighbors.length" class="neighbor-list">
              <span v-for="item in selectedNodeNeighbors" :key="item.id" class="neighbor-chip">{{ item.label }}</span>
            </div>
            <p v-if="selectedNode.kind"><strong>Kind：</strong>{{ selectedNode.kind }}</p>
            <p v-if="selectedNode.status"><strong>Status：</strong>{{ selectedNode.status }}</p>
            <p v-if="selectedNode.owner"><strong>Owner：</strong>{{ selectedNode.owner }}</p>
            <p v-if="selectedNode.importance"><strong>Importance：</strong>{{ selectedNode.importance }}</p>
            <p v-if="selectedNode.occurred_at"><strong>Occurred：</strong>{{ formatDateTime(selectedNode.occurred_at) }}</p>
            <p v-if="selectedNode.topics?.length"><strong>Topics：</strong>{{ selectedNode.topics.join(' / ') }}</p>
            <button
              type="button"
              :disabled="deletingNodeId === selectedNode.id"
              @click.stop="deleteNode(selectedNode.id)"
              style="margin-left:8px;color:#e05;background:none;border:none;cursor:pointer;font-size:12px"
            >
              {{ deletingNodeId === selectedNode.id ? '删除中...' : '删除' }}
            </button>
          </div>
        </section>

        <section class="detail-block">
          <h3>当前状态</h3>
          <div v-if="!runtimeState" class="mini-empty">暂无 runtime state</div>
          <div v-else class="detail-card">
            <p class="detail-content">{{ runtimeState.activity_text }}</p>
            <p v-if="runtimeState.basis_tags?.length"><strong>Basis：</strong>{{ runtimeState.basis_tags.join(' / ') }}</p>
            <p><strong>Updated：</strong>{{ formatDateTime(runtimeState.updated_at) }}</p>
          </div>
        </section>

        <section class="detail-block">
          <h3>最近印象</h3>
          <div v-if="!recentImpressions.length" class="mini-empty">暂无 recent impressions</div>
          <div v-else class="stack-list">
            <div v-for="item in recentImpressions" :key="item.id" class="detail-card compact-card">
              <p class="detail-content">{{ item.content }}</p>
              <p><strong>Kind：</strong>{{ item.kind }}</p>
            </div>
          </div>
        </section>

        <section class="detail-block">
          <h3>最近变更</h3>
          <div v-if="!recentChanges.length" class="mini-empty">暂无 recent changes</div>
          <div v-else class="stack-list">
            <div v-for="item in recentChanges" :key="item.id" class="detail-card compact-card">
              <p class="detail-content">{{ item.field }} · {{ item.summary_text || item.proposed_value }}</p>
              <p><strong>Status：</strong>{{ item.status }}</p>
            </div>
          </div>
        </section>

        <section class="detail-block">
          <h3>待确认人设</h3>
          <div v-if="!pendingPersonaChanges.length" class="mini-empty">暂无 pending persona changes</div>
          <div v-else class="stack-list">
            <div v-for="item in pendingPersonaChanges" :key="item.id" class="detail-card compact-card pending-card">
              <p class="detail-content">{{ item.field }} · {{ item.summary_text || item.proposed_value }}</p>
              <p><strong>Status：</strong>{{ item.status }}</p>
            </div>
          </div>
        </section>
      </aside>
    </div>

    <p v-if="statusMessage" class="status-inline">{{ statusMessage }}</p>
    <p v-if="errorMessage" class="error-inline">{{ errorMessage }}</p>
  </article>
</template>

<style scoped>
.memory-graph-card {
  display: grid;
  gap: 16px;
  padding: 20px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(148, 163, 184, 0.24);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.08);
}

.graph-header,
.graph-topbar,
.graph-summary,
.graph-stage-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.graph-topbar,
.graph-summary,
.graph-stage-toolbar,
.legend-group {
  flex-wrap: wrap;
}

.graph-topbar {
  align-items: end;
}

.search-field,
.mini-filter {
  display: grid;
  gap: 8px;
}

.search-field {
  flex: 1;
}

.search-field input,
.mini-filter select {
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 14px;
  padding: 12px 14px;
}

.graph-layout {
  display: grid;
  grid-template-columns: 190px minmax(0, 1.65fr) 260px;
  gap: 14px;
}

.selector-column,
.detail-column,
.graph-stage-card {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 20px;
  padding: 16px;
  background: #fff;
}

.selector-group {
  display: grid;
  gap: 10px;
}

.selector-group + .selector-group {
  margin-top: 18px;
}

.selector-card {
  display: grid;
  gap: 4px;
  text-align: left;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: #fff;
  cursor: pointer;
}

.selector-card.active {
  border-color: #6366f1;
  background: #eef2ff;
}

.selector-card span,
.mini-empty,
.detail-content,
.detail-card p,
.legend-note {
  color: #475569;
}

.graph-stage {
  overflow: auto;
  border-radius: 18px;
  min-height: 820px;
  padding: 18px;
  background:
    radial-gradient(circle at top, rgba(224, 231, 255, 0.95), rgba(248, 250, 252, 0.98) 44%),
    linear-gradient(180deg, #f8fafc, #eef2ff);
}

.graph-stage-toolbar {
  margin-bottom: 12px;
}

.graph-svg {
  width: 100%;
  min-width: 1100px;
  min-height: 780px;
}

.summary-pill,
.legend-item,
.neighbor-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(241, 245, 249, 0.96);
  color: #334155;
  font-size: 12px;
}

.legend-dot,
.legend-line {
  display: inline-block;
  flex: 0 0 auto;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
}

.legend-dot.topic {
  background: #8b5cf6;
}

.legend-dot.memory {
  background: #60a5fa;
}

.legend-line {
  width: 16px;
  height: 2px;
  background: #94a3b8;
}

.graph-edge {
  fill: none;
  stroke: #cbd5e1;
  stroke-width: 1.6;
  opacity: 0.85;
  transition: opacity 0.18s ease, stroke 0.18s ease, stroke-width 0.18s ease;
}

.graph-edge.active {
  stroke: #94a3b8;
  stroke-width: 2.3;
  opacity: 0.95;
}

.graph-edge.dimmed {
  opacity: 0.14;
}

.graph-node {
  cursor: pointer;
  transition: opacity 0.18s ease;
}

.graph-node.dimmed {
  opacity: 0.24;
}

.node-shape {
  fill: #ffffff;
  stroke: rgba(148, 163, 184, 0.65);
  stroke-width: 1.6;
  transition: stroke 0.18s ease, stroke-width 0.18s ease, filter 0.18s ease, fill 0.18s ease;
}

.node-memory .node-shape {
  fill: rgba(255, 255, 255, 0.98);
  stroke: #60a5fa;
}

.node-topic .node-shape {
  fill: rgba(237, 233, 254, 0.96);
  stroke: #8b5cf6;
}

.graph-node.active .node-shape {
  stroke: #4f46e5;
  stroke-width: 3;
  filter: drop-shadow(0 0 12px rgba(79, 70, 229, 0.24));
}

.node-label,
.node-sub {
  pointer-events: none;
}

.node-label {
  font-size: 12px;
  font-weight: 700;
  fill: #0f172a;
}

.node-sub {
  font-size: 11px;
  fill: #64748b;
}

.detail-column {
  display: grid;
  gap: 14px;
  align-content: start;
}

.detail-block {
  display: grid;
  gap: 10px;
}

.detail-card {
  display: grid;
  gap: 8px;
  border-radius: 16px;
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.compact-card {
  gap: 6px;
}

.pending-card {
  background: #fff7ed;
}

.stack-list {
  display: grid;
  gap: 10px;
  max-height: 240px;
  overflow: auto;
  padding-right: 4px;
}

.neighbor-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.detail-type {
  font-size: 12px;
  color: #6366f1;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.detail-content,
.detail-card p,
.selector-card span,
.selector-card strong,
.legend-note,
.neighbor-chip {
  overflow-wrap: anywhere;
  word-break: break-word;
}

button {
  border: none;
  border-radius: 14px;
  padding: 12px 16px;
  background: linear-gradient(135deg, #111827, #1f2937);
  color: white;
  font-weight: 600;
  cursor: pointer;
}

button.ghost {
  background: #eef2ff;
  color: #3730a3;
}

.graph-empty {
  display: grid;
  place-items: center;
  min-height: 320px;
  color: #64748b;
}

.status-inline,
.error-inline {
  margin: 0;
  font-size: 14px;
}

.status-inline {
  color: #0f766e;
}

.error-inline {
  color: #b91c1c;
}

@media (max-width: 1480px) {
  .graph-layout {
    grid-template-columns: 1fr;
  }

  .graph-stage {
    min-height: 760px;
  }

  .graph-svg {
    min-width: 920px;
    min-height: 720px;
  }
}
</style>
