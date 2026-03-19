<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useApi } from '../composables/useApi'

const route = useRoute()
const router = useRouter()
const api = useApi()

const loading = ref(false)
const loadingImpression = ref(false)
const error = ref('')
const profile = ref({
  bot: null,
  bot_state: null,
  conversation_id: '',
  runtime_state: null,
  recent_impressions: [],
  key_memories: [],
  recent_changes: [],
})
const impression = ref('')

const botID = computed(() => String(route.params.id || '').trim())
const bot = computed(() => profile.value.bot || {})
const personality = computed(() => Array.isArray(bot.value.personality) ? bot.value.personality : [])
const runtimeState = computed(() => profile.value.runtime_state || null)
const recentImpressions = computed(() => Array.isArray(profile.value.recent_impressions) ? profile.value.recent_impressions : [])
const keyMemories = computed(() => Array.isArray(profile.value.key_memories) ? profile.value.key_memories : [])
const recentChanges = computed(() => Array.isArray(profile.value.recent_changes) ? profile.value.recent_changes : [])
const memories = ref([])
const memoriesLoading = ref(false)
const memoriesError = ref('')
const displayImpression = computed(() => impression.value || recentImpressions.value[0]?.content || '')
const selfCognitionList = computed(() => {
  const raw = Array.isArray(bot.value.self_cognition) ? bot.value.self_cognition : []
  const staticTaboos = String(bot.value.taboos_and_preferences || '').trim()
  return raw
    .map((item) => String(item || '').trim())
    .filter((item, index, arr) => item && item !== staticTaboos && arr.indexOf(item) === index)
})

const personaSections = computed(() => [
  { key: 'identity', title: '身份设定', type: 'text', value: bot.value.identity || '' },
  { key: 'personality', title: '性格特征', type: 'chips', value: personality.value },
  { key: 'expression_style', title: '表达风格', type: 'text', value: bot.value.expression_style || '' },
  { key: 'life_context', title: '生活环境', type: 'text', value: bot.value.life_context || '' },
  { key: 'taboos_and_preferences', title: '禁忌与偏好', type: 'text', value: bot.value.taboos_and_preferences || '' },
])

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

function applyProfilePayload(data = {}) {
  profile.value = {
    bot: data.bot || null,
    bot_state: data.bot_state || null,
    conversation_id: data.conversation_id || '',
    runtime_state: data.runtime_state || null,
    recent_impressions: Array.isArray(data.recent_impressions) ? data.recent_impressions : [],
    key_memories: Array.isArray(data.key_memories) ? data.key_memories : [],
    recent_changes: Array.isArray(data.recent_changes) ? data.recent_changes : [],
  }
  if (!impression.value && profile.value.recent_impressions.length > 0) {
    impression.value = String(profile.value.recent_impressions[0]?.content || '').trim()
  }
}

function displayState() {
  const state = profile.value.bot_state || {}
  return {
    emoji: state.status_emoji || '🙂',
    text: state.status_text || '在线',
  }
}

async function loadProfile() {
  if (!botID.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await api.get(`/api/v1/bots/${botID.value}/profile`)
    applyProfilePayload(data)
  } catch (e) {
    error.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function refreshImpression() {
  if (!botID.value || loadingImpression.value) return
  loadingImpression.value = true
  try {
    const data = await api.post(`/api/v1/bots/${botID.value}/impression`, {})
    impression.value = String(data?.impression || '').trim()
    if (data?.memory?.content) {
      const latest = data.memory
      const rest = recentImpressions.value.filter(item => item.id !== latest.id && item.content !== latest.content)
      profile.value.recent_impressions = [latest, ...rest]
    }
  } catch (e) {
    error.value = e?.message || '印象更新失败'
  } finally {
    loadingImpression.value = false
  }
}

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

onMounted(async () => {
  await loadProfile()
  loadMemories()
})
</script>

<template>
  <div class="detail-page" data-testid="bot-detail-page">
    <header class="detail-header">
      <button type="button" class="back-btn" @click="router.push('/bots')">←</button>
      <h2>{{ bot.name || 'Bot 详情' }}</h2>
      <router-link :to="`/bots/${botID}/edit`" class="edit-btn">编辑</router-link>
    </header>

    <main class="detail-main">
      <div v-if="loading" class="empty">加载中...</div>
      <div v-else-if="!profile.bot" class="empty">未找到 Bot</div>
      <template v-else>
        <p v-if="error" class="error">{{ error }}</p>

        <section class="card">
          <h3>当前生活状态</h3>
          <div class="status-row" data-testid="bot-detail-status">
            <span class="status-pill">{{ displayState().emoji }} {{ displayState().text }}</span>
            <span class="status-meta">会话 ID：{{ profile.conversation_id || '无' }}</span>
          </div>
          <p class="paragraph runtime-activity">{{ runtimeState?.activity_text || '暂时没有新的生活状态。' }}</p>
          <div v-if="runtimeState?.basis_tags?.length" class="chips">
            <span v-for="tag in runtimeState.basis_tags" :key="tag" class="chip muted-chip">{{ tag }}</span>
          </div>
        </section>

        <section class="card">
          <h3>人设档案</h3>
          <div v-for="section in personaSections" :key="section.key" class="field">
            <span class="label">{{ section.title }}</span>
            <div v-if="section.type === 'chips'" class="chips">
              <span v-for="item in section.value" :key="`${section.key}-${item}`" class="chip">{{ item }}</span>
              <span v-if="!section.value.length" class="muted">暂无</span>
            </div>
            <p v-else class="paragraph">{{ section.value || '暂无' }}</p>
          </div>
        </section>

        <section class="card">
          <div class="section-head">
            <h3>最近印象</h3>
            <button type="button" class="ghost-btn" :disabled="loadingImpression" @click="refreshImpression">
              {{ loadingImpression ? '刷新中...' : '刷新' }}
            </button>
          </div>
          <p class="impression" data-testid="bot-detail-impression">{{ displayImpression || '暂时还没有新的印象。' }}</p>
          <div v-if="recentImpressions.length > 1" class="stack-list">
            <div v-for="item in recentImpressions.slice(1)" :key="item.id" class="mini-item">
              <span class="mini-kind">{{ item.kind || 'memory' }}</span>
              <span>{{ item.content }}</span>
            </div>
          </div>
        </section>

        <section class="card">
          <h3>关键记忆</h3>
          <div v-if="!keyMemories.length" class="muted">暂无关键记忆</div>
          <div v-else class="stack-list" data-testid="bot-detail-key-memories">
            <div v-for="item in keyMemories" :key="item.id" class="memory-row">
              <div class="memory-head">
                <span class="mini-kind">{{ item.kind || 'memory' }}</span>
                <span v-if="item.status" class="memory-status">{{ item.status }}</span>
              </div>
              <p class="paragraph">{{ item.content }}</p>
            </div>
          </div>
        </section>

        <section class="card">
          <h3>自我认知</h3>
          <div v-if="!selfCognitionList.length" class="muted">暂无最近自我认知变化</div>
          <div v-else class="stack-list" data-testid="bot-detail-self-cognition">
            <div v-for="item in selfCognitionList" :key="item" class="change-row cognition-row">
              <p class="paragraph">{{ item }}</p>
            </div>
          </div>
        </section>

        <section class="card">
          <h3>最近人设变更记录</h3>
          <div v-if="!recentChanges.length" class="muted">暂无最近变化</div>
          <div v-else class="stack-list bounded-stack-list" data-testid="bot-detail-recent-changes">
            <div v-for="item in recentChanges" :key="item.id" class="change-row accepted-row">
              <div class="memory-head">
                <span class="mini-kind">{{ item.field || 'persona' }}</span>
                <span class="memory-status">{{ item.status }}</span>
              </div>
              <p class="paragraph">{{ item.summary_text || item.proposed_value }}</p>
            </div>
          </div>
        </section>

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

      </template>
    </main>
  </div>
</template>

<style scoped>
.detail-page {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg-page);
}

.detail-header {
  display: grid;
  grid-template-columns: 36px 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border-default);
  background: rgba(255, 255, 255, 0.76);
}

.detail-header h2 {
  font-size: 18px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.back-btn {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 1px solid #d5dfc7;
  background: #fff;
  color: #50663f;
  font-size: 18px;
}

.edit-btn {
  text-decoration: none;
  color: var(--accent-primary);
  font-size: 14px;
  font-weight: 600;
}

.detail-main {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding: 14px 14px calc(14px + 76px);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card {
  border: 1px solid #dce6cd;
  border-radius: 14px;
  background: #ffffffd6;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.card h3 {
  font-size: 15px;
  color: var(--text-primary);
}

.status-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.status-pill {
  padding: 6px 10px;
  border-radius: 9999px;
  background: #edf5e1;
  color: #476037;
  font-size: 13px;
}

.status-meta,
.label,
.directive-meta,
.muted {
  font-size: 12px;
  color: var(--text-muted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.paragraph,
.impression,
.directive-content {
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.5;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chip {
  border-radius: 9999px;
  border: 1px solid #dbe5cb;
  padding: 4px 10px;
  font-size: 12px;
  color: #51633f;
  background: #f9fcf4;
}

.muted-chip {
  background: #eef4ff;
}

.directive-list,
.iterate-list,
.stack-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.directive-row,
.memory-row,
.change-row,
.mini-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px dashed #d8e2c8;
  border-radius: 10px;
  padding: 10px;
}

.pending-row {
  background: #fff8f0;
}

.accepted-row {
  background: #f7fbf2;
}

.cognition-row {
  background: #f5f8ff;
}

.bounded-stack-list {
  max-height: 320px;
  overflow-y: auto;
  padding-right: 4px;
}

.memory-head,
.action-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.memory-status,
.mini-kind {
  font-size: 12px;
  color: #6b7280;
}

.directive-main {
  flex: 1;
}

.danger-btn,
.ghost-btn {
  border-radius: 8px;
  font-size: 12px;
  padding: 5px 10px;
}

.danger-btn {
  border: 1px solid #e6b7b1;
  background: #fff5f3;
  color: #b44f43;
}

.ghost-btn {
  border: 1px solid #c7d6b1;
  background: #f6faef;
  color: #4f663d;
}

.ghost-btn:disabled,
.danger-btn:disabled {
  opacity: 0.6;
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.empty {
  text-align: center;
  color: var(--text-muted);
  padding: 50px 0;
}

.error {
  color: #bb4538;
  font-size: 13px;
  line-height: 1.35;
}

.runtime-activity {
  font-weight: 500;
}

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
</style>
