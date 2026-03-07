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
const bot = computed(() => profile.value.bot || {})
const directives = computed(() => Array.isArray(profile.value.directives) ? profile.value.directives : [])
const personality = computed(() => Array.isArray(bot.value.personality) ? bot.value.personality : [])

const personaSections = computed(() => [
  { key: 'identity', title: '身份设定', type: 'text', value: bot.value.identity || '' },
  { key: 'personality', title: '性格特征', type: 'chips', value: personality.value },
  { key: 'expression_style', title: '表达风格', type: 'text', value: bot.value.expression_style || '' },
  { key: 'life_context', title: '生活环境', type: 'text', value: bot.value.life_context || '' },
  { key: 'taboos_and_preferences', title: '禁忌与偏好', type: 'text', value: bot.value.taboos_and_preferences || '' },
])

const iterateSections = computed(() => {
  if (!iterateResult.value) return []
  return [
    { key: 'identity_adds', title: '身份设定新增', items: iterateResult.value.identity_adds || [] },
    { key: 'personality_adds', title: '性格特征新增', items: iterateResult.value.personality_adds || [] },
    { key: 'expression_style_adds', title: '表达风格新增', items: iterateResult.value.expression_style_adds || [] },
    { key: 'life_context_adds', title: '生活环境新增', items: iterateResult.value.life_context_adds || [] },
    { key: 'taboos_and_preferences_adds', title: '禁忌与偏好新增', items: iterateResult.value.taboos_and_preferences_adds || [] },
  ].filter(section => Array.isArray(section.items) && section.items.length > 0)
})

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
      <h2>{{ bot.name || 'Bot 详情' }}</h2>
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
          <p class="impression" data-testid="bot-detail-impression">{{ impression || '暂时还没有新的印象。' }}</p>
        </section>

        <section class="card">
          <div class="section-head">
            <h3>自我迭代</h3>
            <button type="button" class="ghost-btn" :disabled="loadingIterate" @click="runIterate">
              {{ loadingIterate ? '迭代中...' : '运行迭代' }}
            </button>
          </div>
          <div v-if="iterateSections.length" class="iterate-list">
            <div v-for="section in iterateSections" :key="section.key" class="field">
              <span class="label">{{ section.title }}</span>
              <div class="chips">
                <span v-for="item in section.items" :key="`${section.key}-${item}`" class="chip">{{ item }}</span>
              </div>
            </div>
          </div>
          <p v-else class="muted">还没有新的迭代结果。</p>
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

.directive-list,
.iterate-list {
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

.ghost-btn:disabled {
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
</style>
