<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useApi } from '../composables/useApi'

const route = useRoute()
const router = useRouter()
const api = useApi()

const loading = ref(false)
const loadingImpression = ref(false)
const loadingIterate = ref(false)
const error = ref('')
const profile = ref({
  bot: null,
  bot_state: null,
  directives: [],
  conversation_id: '',
})
const impression = ref('')
const iterateResult = ref(null)

const botID = computed(() => String(route.params.id || '').trim())

const speakingSegments = computed(() => splitSegments(profile.value.bot?.speaking_style))
const backgroundSegments = computed(() => splitSegments(profile.value.bot?.background))
const summarySegments = computed(() => splitSegments(profile.value.bot?.summary))
const traits = computed(() => Array.isArray(profile.value.bot?.traits) ? profile.value.bot.traits : [])
const directives = computed(() => Array.isArray(profile.value.directives) ? profile.value.directives : [])

function splitSegments(value) {
  return String(value || '')
    .split('|')
    .map(v => v.trim())
    .filter(Boolean)
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
    profile.value = {
      bot: data.bot || null,
      bot_state: data.bot_state || null,
      directives: Array.isArray(data.directives) ? data.directives : [],
      conversation_id: data.conversation_id || '',
    }
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
  } catch (e) {
    error.value = e?.message || '印象更新失败'
  } finally {
    loadingImpression.value = false
  }
}

async function runIterate() {
  if (!botID.value || loadingIterate.value) return
  loadingIterate.value = true
  try {
    const data = await api.post(`/api/v1/bots/${botID.value}/iterate`, { message_limit: 30 })
    iterateResult.value = data || null
    if (data?.bot) {
      profile.value.bot = data.bot
    }
  } catch (e) {
    error.value = e?.message || '自我迭代失败'
  } finally {
    loadingIterate.value = false
  }
}

async function revokeDirective(id) {
  if (!id || !botID.value) return
  try {
    await api.del(`/api/v1/bots/${botID.value}/directives/${id}`)
    profile.value.directives = directives.value.filter(item => item.id !== id)
  } catch (e) {
    error.value = e?.message || '撤销失败'
  }
}

onMounted(async () => {
  await loadProfile()
  await refreshImpression()
})
</script>

<template>
  <div class="detail-page" data-testid="bot-detail-page">
    <header class="detail-header">
      <button type="button" class="back-btn" @click="router.push('/bots')">←</button>
      <h2>{{ profile.bot?.name || 'Bot 详情' }}</h2>
      <router-link :to="`/bots/${botID}/edit`" class="edit-btn">编辑</router-link>
    </header>

    <main class="detail-main">
      <div v-if="loading" class="empty">加载中...</div>
      <div v-else-if="!profile.bot" class="empty">未找到 Bot</div>
      <template v-else>
        <p v-if="error" class="error">{{ error }}</p>

        <section class="card">
          <h3>自我状态</h3>
          <div class="status-row" data-testid="bot-detail-status">
            <span class="status-pill">{{ displayState().emoji }} {{ displayState().text }}</span>
            <span class="status-meta">会话 ID：{{ profile.conversation_id || '无' }}</span>
          </div>
        </section>

        <section class="card">
          <h3>原始人设</h3>
          <div class="field">
            <span class="label">简介</span>
            <div class="chips">
              <span v-for="item in summarySegments" :key="`summary-${item}`" class="chip">{{ item }}</span>
              <span v-if="!summarySegments.length" class="muted">暂无</span>
            </div>
          </div>
          <div class="field">
            <span class="label">说话风格</span>
            <div class="chips">
              <span v-for="item in speakingSegments" :key="`speaking-${item}`" class="chip">{{ item }}</span>
              <span v-if="!speakingSegments.length" class="muted">暂无</span>
            </div>
          </div>
          <div class="field">
            <span class="label">背景</span>
            <div class="chips">
              <span v-for="item in backgroundSegments" :key="`background-${item}`" class="chip">{{ item }}</span>
              <span v-if="!backgroundSegments.length" class="muted">暂无</span>
            </div>
          </div>
          <div class="field">
            <span class="label">特质</span>
            <div class="chips">
              <span v-for="item in traits" :key="`trait-${item}`" class="chip">{{ item }}</span>
              <span v-if="!traits.length" class="muted">暂无</span>
            </div>
          </div>
        </section>

        <section class="card">
          <h3>行为指令</h3>
          <div v-if="!directives.length" class="muted">暂无指令</div>
          <div v-else class="directive-list">
            <div v-for="item in directives" :key="item.id" class="directive-row">
              <div class="directive-main">
                <p class="directive-content">{{ item.content }}</p>
                <p class="directive-meta">{{ item.category || 'style' }} · {{ item.source || 'conversation' }}</p>
              </div>
              <button type="button" class="danger-btn" @click="revokeDirective(item.id)">撤销</button>
            </div>
          </div>
        </section>

        <section class="card">
          <div class="section-head">
            <h3>用户印象</h3>
            <button type="button" class="ghost-btn" :disabled="loadingImpression" @click="refreshImpression">
              {{ loadingImpression ? '刷新中...' : '刷新' }}
            </button>
          </div>
          <p class="impression" data-testid="bot-detail-impression">{{ impression || '暂无印象' }}</p>
        </section>

        <section class="card">
          <div class="section-head">
            <h3>自我迭代</h3>
            <button type="button" class="ghost-btn" :disabled="loadingIterate" @click="runIterate">
              {{ loadingIterate ? '处理中...' : '运行' }}
            </button>
          </div>
          <div v-if="iterateResult" class="iterate-result" data-testid="bot-detail-iterate-result">
            <p>说话风格新增：{{ (iterateResult.speaking_style_adds || []).join('、') || '无' }}</p>
            <p>背景新增：{{ (iterateResult.background_adds || []).join('、') || '无' }}</p>
            <p>特质新增：{{ (iterateResult.trait_adds || []).join('、') || '无' }}</p>
            <p>性别调整：{{ iterateResult.gender || '无' }}</p>
          </div>
          <p v-else class="muted">点击“运行”后会展示新增项。</p>
        </section>
      </template>
    </main>
  </div>
</template>

<style scoped>
.detail-page {
  flex: 1;
  display: flex;
  flex-direction: column;
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
  overflow-y: auto;
  padding: 14px;
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

.status-meta {
  font-size: 12px;
  color: var(--text-muted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.label {
  font-size: 12px;
  color: var(--text-muted);
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

.directive-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.directive-row {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  border: 1px dashed #d8e2c8;
  border-radius: 10px;
  padding: 10px;
}

.directive-main {
  flex: 1;
}

.directive-content {
  color: var(--text-primary);
  font-size: 13px;
  line-height: 1.35;
}

.directive-meta {
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
}

.danger-btn {
  border: 1px solid #e6b7b1;
  border-radius: 8px;
  background: #fff5f3;
  color: #b44f43;
  font-size: 12px;
  padding: 4px 8px;
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.ghost-btn {
  border: 1px solid #c7d6b1;
  background: #f6faef;
  color: #4f663d;
  border-radius: 8px;
  font-size: 12px;
  padding: 5px 10px;
}

.ghost-btn:disabled {
  opacity: 0.6;
}

.impression {
  font-size: 14px;
  color: var(--text-primary);
  line-height: 1.5;
}

.iterate-result p {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.4;
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

.muted {
  color: var(--text-muted);
  font-size: 13px;
}
</style>
