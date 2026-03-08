<script setup>
import { computed, onMounted, ref } from 'vue'
import {
  getMemoryGraph,
  listAdminUsers,
  listUserBots,
} from '../api/admin.js'

const searchQuery = ref('')
const loadingUsers = ref(false)
const loadingBots = ref(false)
const loadingGraph = ref(false)
const statusMessage = ref('')
const errorMessage = ref('')
const users = ref([])
const bots = ref([])
const graph = ref({ nodes: [], edges: [], summary: { memory_count: 0, topic_count: 0, graph_source: 'store' } })
const selectedUserId = ref('')
const selectedBotId = ref('')
const selectedNodeId = ref('')

const selectedNode = computed(() => graph.value.nodes.find((node) => node.id === selectedNodeId.value) || null)
const canvasWidth = 920
const canvasHeight = 620

const graphLayout = computed(() => {
  const memoryNodes = graph.value.nodes.filter((node) => node.type === 'memory')
  const topicNodes = graph.value.nodes.filter((node) => node.type === 'topic')
  const centerX = canvasWidth / 2
  const centerY = canvasHeight / 2
  const positioned = []
  const topicRadius = 180
  const memoryRadius = 300

  topicNodes.forEach((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(topicNodes.length, 1)
    positioned.push({ ...node, x: centerX + Math.cos(angle) * topicRadius, y: centerY + Math.sin(angle) * topicRadius })
  })
  memoryNodes.forEach((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(memoryNodes.length, 1)
    positioned.push({ ...node, x: centerX + Math.cos(angle) * memoryRadius, y: centerY + Math.sin(angle) * memoryRadius })
  })

  const nodeMap = new Map(positioned.map((node) => [node.id, node]))
  const edges = graph.value.edges
    .map((edge) => ({ edge, source: nodeMap.get(edge.source), target: nodeMap.get(edge.target) }))
    .filter((item) => item.source && item.target)

  return { nodes: positioned, edges }
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
      graph.value = { nodes: [], edges: [], summary: { memory_count: 0, topic_count: 0, graph_source: 'store' } }
    }
    statusMessage.value = `已加载 ${users.value.length} 位用户。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loadingUsers.value = false
  }
}

async function selectUser(user) {
  if (!user?.user_id) {
    return
  }
  selectedUserId.value = user.user_id
  selectedBotId.value = ''
  selectedNodeId.value = ''
  graph.value = { nodes: [], edges: [], summary: { memory_count: 0, topic_count: 0, graph_source: 'store' } }
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

async function selectBot(bot) {
  if (!selectedUserId.value || !bot?.bot_id) {
    return
  }
  selectedBotId.value = bot.bot_id
  selectedNodeId.value = ''
  loadingGraph.value = true
  errorMessage.value = ''
  try {
    graph.value = await getMemoryGraph(selectedUserId.value, bot.bot_id)
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

function nodeRect(node) {
  return node.type === 'memory'
    ? { width: 150, height: 64, rx: 18 }
    : { width: 96, height: 44, rx: 22 }
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
      <button type="button" class="ghost" :disabled="loadingUsers" @click="refreshUsers">搜索</button>
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
          <span>Memory：{{ graph.summary?.memory_count || 0 }}</span>
          <span>Topic：{{ graph.summary?.topic_count || 0 }}</span>
          <span>Source：{{ graph.summary?.graph_source || 'store' }}</span>
        </div>

        <div v-if="loadingGraph" class="graph-empty">图谱加载中...</div>
        <div v-else-if="!selectedBotId" class="graph-empty">先选择一个 Robot 查看图谱</div>
        <div v-else-if="!graph.nodes?.length" class="graph-empty">当前 Robot 暂无记忆图数据</div>
        <div v-else class="graph-stage">
          <svg :viewBox="`0 0 ${canvasWidth} ${canvasHeight}`" class="graph-svg">
            <line
              v-for="item in graphLayout.edges"
              :key="item.edge.id"
              :x1="item.source.x"
              :y1="item.source.y"
              :x2="item.target.x"
              :y2="item.target.y"
              stroke="#cbd5e1"
              stroke-width="2"
            />
            <g
              v-for="node in graphLayout.nodes"
              :key="node.id"
              class="graph-node"
              :class="[`node-${node.type}`, { active: node.id === selectedNodeId }]"
              @click="selectedNodeId = node.id"
            >
              <rect
                :x="node.x - nodeRect(node).width / 2"
                :y="node.y - nodeRect(node).height / 2"
                :width="nodeRect(node).width"
                :height="nodeRect(node).height"
                :rx="nodeRect(node).rx"
              />
              <text :x="node.x" :y="node.y - 4" text-anchor="middle" class="node-label">{{ node.label }}</text>
              <text v-if="node.type === 'memory'" :x="node.x" :y="node.y + 16" text-anchor="middle" class="node-sub">{{ node.importance || 0 }} 分</text>
            </g>
          </svg>
        </div>
      </section>

      <aside class="detail-column">
        <h3>节点详情</h3>
        <div v-if="!selectedNode" class="mini-empty">点击图中节点查看详细内容</div>
        <div v-else class="detail-card">
          <p class="detail-type">{{ selectedNode.type === 'memory' ? 'Memory' : 'Topic' }}</p>
          <h4>{{ selectedNode.label }}</h4>
          <p v-if="selectedNode.content" class="detail-content">{{ selectedNode.content }}</p>
          <p v-if="selectedNode.owner"><strong>Owner：</strong>{{ selectedNode.owner }}</p>
          <p v-if="selectedNode.importance"><strong>Importance：</strong>{{ selectedNode.importance }}</p>
          <p v-if="selectedNode.occurred_at"><strong>Occurred：</strong>{{ new Date(selectedNode.occurred_at).toLocaleString() }}</p>
          <p v-if="selectedNode.topics?.length"><strong>Topics：</strong>{{ selectedNode.topics.join(' / ') }}</p>
        </div>
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
.graph-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.graph-topbar {
  align-items: end;
}

.search-field {
  display: grid;
  gap: 8px;
  flex: 1;
}

.search-field input {
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 14px;
  padding: 12px 14px;
}

.graph-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr) 260px;
  gap: 16px;
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
.detail-card p {
  color: #475569;
}

.graph-stage {
  overflow: auto;
  border-radius: 18px;
  background: linear-gradient(180deg, #f8fafc, #eef2ff);
}

.graph-svg {
  width: 100%;
  min-height: 620px;
}

.graph-node rect {
  fill: white;
  stroke: rgba(148, 163, 184, 0.55);
  stroke-width: 1.5;
}

.graph-node.node-memory rect {
  fill: #ffffff;
}

.graph-node.node-topic rect {
  fill: #ede9fe;
  stroke: #8b5cf6;
}

.graph-node.active rect {
  stroke: #4f46e5;
  stroke-width: 3;
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

@media (max-width: 1280px) {
  .graph-layout {
    grid-template-columns: 1fr;
  }
}
</style>
