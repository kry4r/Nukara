<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  clearUserProviderSetting,
  createProvider,
  deleteProvider,
  getAdminCredentials,
  listProviders,
  listUserProviderSettings,
  testProvider,
  updateUserProviderSetting,
} from './api/admin.js'

const loading = ref(false)
const refreshing = ref(false)
const bulkSaving = ref(false)
const statusMessage = ref('')
const errorMessage = ref('')
const searchQuery = ref('')
const creatingProvider = ref(false)
const showCreateProvider = ref(false)
const createStatus = ref('')
const selectedSourceId = ref('')

const providers = ref([])
const rows = ref([])
const totalUsers = ref(0)
const defaultProviderId = ref('')
const defaultModel = ref('')

function isVisibleProvider(provider) {
  return Boolean(provider?.id) && provider.id !== 'custom'
}

function normalizeProviders(list) {
  if (!Array.isArray(list)) {
    return []
  }
  return list.filter(isVisibleProvider)
}

const drafts = reactive({})
const savingByUser = reactive({})
const newProvider = reactive({
  name: '',
  base_url: '',
  models: '',
})

const activeProvider = computed(() =>
  providers.value.find((provider) => provider.is_active) ||
  providers.value.find((provider) => provider.id === defaultProviderId.value) ||
  null,
)

const activeProviderModel = computed(() => {
  const firstModel = Array.isArray(activeProvider.value?.models)
    ? activeProvider.value.models[0] || ''
    : ''
  return firstModel || defaultModel.value || '-'
})

const dirtyCount = computed(() =>
  Object.values(drafts).filter((draft) => draft && draft.dirty).length,
)
const providerUsageMap = computed(() => {
  const usage = {}
  rows.value.forEach((item) => {
    const sourceID = item.effective_provider_id || item.provider_id || defaultProviderId.value
    if (!sourceID) {
      return
    }
    usage[sourceID] = (usage[sourceID] || 0) + 1
  })
  return usage
})
const rowsForDisplay = computed(() => {
  if (!selectedSourceId.value) {
    return rows.value
  }
  return rows.value.filter((item) => {
    const sourceID = item.effective_provider_id || item.provider_id || defaultProviderId.value
    return sourceID === selectedSourceId.value
  })
})
const selectedSourceName = computed(() => {
  if (!selectedSourceId.value) {
    return '全部来源'
  }
  const target = providers.value.find((provider) => provider.id === selectedSourceId.value)
  return target?.name || selectedSourceId.value
})

function defaultProviderName() {
  const target = providers.value.find((provider) => provider.id === defaultProviderId.value)
  return target?.name || defaultProviderId.value || '未配置'
}

function modelOptionsForProvider(providerId) {
  const provider = providers.value.find((item) => item.id === providerId)
  return Array.isArray(provider?.models) ? provider.models.filter(Boolean) : []
}

function providerUsersCount(providerId) {
  return providerUsageMap.value[providerId] || 0
}

function ensureDraft(user) {
  if (!drafts[user.user_id]) {
    drafts[user.user_id] = {
      provider_id: user.provider_id || '',
      model: user.model || '',
      dirty: false,
    }
  }
  return drafts[user.user_id]
}

function resetDraftsFromRows() {
  Object.keys(drafts).forEach((key) => {
    delete drafts[key]
  })
  rows.value.forEach((item) => {
    drafts[item.user_id] = {
      provider_id: item.provider_id || '',
      model: item.model || '',
      dirty: false,
    }
  })
}

function markDirty(userId) {
  const target = drafts[userId]
  if (!target) {
    return
  }
  target.dirty = true
}

function onProviderChange(item) {
  const draft = ensureDraft(item)
  const models = modelOptionsForProvider(draft.provider_id)
  if (!draft.provider_id) {
    draft.model = ''
  } else if (!draft.model || !models.includes(draft.model)) {
    draft.model = models[0] || ''
  }
  markDirty(item.user_id)
}

function isSaving(userId) {
  return Boolean(savingByUser[userId])
}

function isRowDirty(userId) {
  return Boolean(drafts[userId]?.dirty)
}

async function refreshAll() {
  loading.value = true
  refreshing.value = true
  errorMessage.value = ''

  try {
    const credentials = getAdminCredentials()
    if (!credentials.username || !credentials.password) {
      throw new Error('请先在浏览器 localStorage 中设置 nukara_admin_username / nukara_admin_password')
    }

    const settingPayload = await listUserProviderSettings({
      q: searchQuery.value,
      limit: 100,
      offset: 0,
    })

    const options = Array.isArray(settingPayload.providers) ? settingPayload.providers : []
    if (options.length > 0) {
      providers.value = normalizeProviders(options)
    } else {
      try {
        providers.value = normalizeProviders(await listProviders())
      } catch {
        providers.value = []
      }
    }
    rows.value = Array.isArray(settingPayload.items) ? settingPayload.items : []
    totalUsers.value = Number(settingPayload.total || rows.value.length)
    defaultProviderId.value = settingPayload.default_provider_id || ''
    if (!providers.value.some((provider) => provider.id === defaultProviderId.value)) {
      defaultProviderId.value = providers.value.find((provider) => provider.is_active)?.id || providers.value[0]?.id || ''
    }
    defaultModel.value = settingPayload.default_model || ''
    if (selectedSourceId.value && !providers.value.some((provider) => provider.id === selectedSourceId.value)) {
      selectedSourceId.value = ''
    }

    resetDraftsFromRows()
    statusMessage.value = '数据已刷新。'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function saveUser(item) {
  const draft = ensureDraft(item)
  if (!draft.dirty || isSaving(item.user_id)) {
    return
  }

  savingByUser[item.user_id] = true
  errorMessage.value = ''

  try {
    let updated = null
    if (!draft.provider_id) {
      await clearUserProviderSetting(item.user_id)
      updated = {
        ...item,
        provider_id: '',
        model: '',
        is_override: false,
        effective_provider_id: defaultProviderId.value,
        effective_model: defaultModel.value,
        updated_at: null,
      }
    } else {
      updated = await updateUserProviderSetting(item.user_id, {
        provider_id: draft.provider_id,
        model: draft.model,
      })
    }

    const idx = rows.value.findIndex((row) => row.user_id === item.user_id)
    if (idx >= 0) {
      rows.value[idx] = {
        ...rows.value[idx],
        ...updated,
      }
    }
    drafts[item.user_id] = {
      provider_id: rows.value[idx]?.provider_id || '',
      model: rows.value[idx]?.model || '',
      dirty: false,
    }
    statusMessage.value = `已保存 ${item.nickname || item.phone} 的配置。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    savingByUser[item.user_id] = false
  }
}

async function saveAllDirty() {
  if (bulkSaving.value || dirtyCount.value === 0) {
    return
  }

  bulkSaving.value = true
  errorMessage.value = ''

  try {
    const targets = rows.value.filter((item) => drafts[item.user_id]?.dirty)
    for (const item of targets) {
      await saveUser(item)
      if (errorMessage.value) {
        break
      }
    }
    if (!errorMessage.value) {
      statusMessage.value = `已保存 ${targets.length} 条变更。`
    }
  } finally {
    bulkSaving.value = false
  }
}

function rollbackAll() {
  resetDraftsFromRows()
  statusMessage.value = '未提交更改已撤销。'
}

async function quickTestProvider(provider) {
  if (!provider?.id || creatingProvider.value) {
    return
  }
  creatingProvider.value = true
  errorMessage.value = ''
  try {
    const result = await testProvider(provider.id)
    if (result.status !== 'ok') {
      throw new Error(result.error_message || 'Provider 连通测试失败。')
    }
    statusMessage.value = `${provider.name} 连通测试成功（${result.latency_ms ?? '-'} ms）。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    creatingProvider.value = false
  }
}

async function testAndCreateProvider() {
  if (creatingProvider.value) {
    return
  }
  if (!newProvider.name.trim()) {
    errorMessage.value = 'Provider 名称不能为空。'
    return
  }

  creatingProvider.value = true
  errorMessage.value = ''
  createStatus.value = ''

  let created = null
  try {
    created = await createProvider({
      name: newProvider.name.trim(),
      base_url: newProvider.base_url.trim(),
      models: newProvider.models,
      priority: providers.value.length + 1,
      is_active: false,
      api_key: '',
    })

    const testResult = await testProvider(created.id)
    if (testResult.status !== 'ok') {
      throw new Error(testResult.error_message || 'Provider 连通测试失败。')
    }

    createStatus.value = `连通测试成功（${testResult.latency_ms ?? '-'} ms）`
    newProvider.name = ''
    newProvider.base_url = ''
    newProvider.models = ''
    showCreateProvider.value = false
    await refreshAll()
    statusMessage.value = `Provider 已创建并通过测试：${created.name || created.id}`
  } catch (error) {
    if (created?.id) {
      try {
        await deleteProvider(created.id)
      } catch {
        // ignore rollback failure, keep original error for user
      }
    }
    errorMessage.value = error.message
  } finally {
    creatingProvider.value = false
  }
}

onMounted(() => {
  refreshAll()
})
</script>

<template>
  <main class="admin-shell">
    <header class="top-bar">
      <div class="title-stack">
        <p class="eyebrow">Admin Console / AgentX Routing</p>
        <h1>User Provider Assignment</h1>
      </div>
      <div class="top-actions">
        <span class="status-chip">AgentX Runtime Connected</span>
        <button type="button" :disabled="refreshing" @click="refreshAll">
          {{ refreshing ? '刷新中...' : '刷新数据' }}
        </button>
      </div>
    </header>

    <p v-if="statusMessage" class="status-message">{{ statusMessage }}</p>
    <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

    <section class="main-area">
      <aside class="left-column">
        <article class="panel-card provider-list-card">
          <div class="provider-list-header">
            <p class="panel-eyebrow">来源 Provider 列表</p>
            <button type="button" class="ghost mini plus-button" @click="showCreateProvider = !showCreateProvider">
              +
            </button>
          </div>

          <div class="provider-card-list">
            <article
              v-for="provider in providers"
              :key="provider.id"
              :class="['provider-item-card', { selected: selectedSourceId === provider.id }]"
              @click="selectedSourceId = selectedSourceId === provider.id ? '' : provider.id"
            >
              <div class="provider-item-top">
                <strong>{{ provider.name }}</strong>
                <span :class="['provider-state', { active: provider.is_active }]">
                  {{ provider.is_active ? 'Active' : 'Standby' }}
                </span>
              </div>
              <p class="provider-model">
                {{ Array.isArray(provider.models) && provider.models.length > 0 ? provider.models[0] : '未配置 model' }}
              </p>
              <p class="provider-count">{{ providerUsersCount(provider.id) }} 位用户</p>
              <div class="provider-item-actions">
                <button
                  type="button"
                  class="ghost mini"
                  :disabled="creatingProvider"
                  @click.stop="quickTestProvider(provider)"
                >
                  连通测试
                </button>
              </div>
            </article>
          </div>

          <div class="default-summary">
            <div class="kv-row">
              <span>Runtime Default</span>
              <strong>{{ activeProvider?.name || defaultProviderName() }}</strong>
            </div>
            <div class="kv-row">
              <span>Model</span>
              <strong>{{ activeProviderModel }}</strong>
            </div>
          </div>

          <div v-if="showCreateProvider" class="provider-create-inline">
            <p class="panel-eyebrow">新增 Provider（创建时自动联通测试）</p>
            <label class="mini-form-field">
              <span>Name</span>
              <input v-model.trim="newProvider.name" placeholder="例如：OpenAI Compatible" />
            </label>
            <label class="mini-form-field">
              <span>Base URL</span>
              <input v-model.trim="newProvider.base_url" placeholder="https://api.example.com/v1" />
            </label>
            <label class="mini-form-field">
              <span>Models（逗号分隔）</span>
              <input v-model.trim="newProvider.models" placeholder="gpt-4o-mini, gpt-4.1-mini" />
            </label>
            <div class="create-actions">
              <button type="button" class="ghost" :disabled="creatingProvider" @click="showCreateProvider = false">取消</button>
              <button type="button" :disabled="creatingProvider" @click="testAndCreateProvider">
                {{ creatingProvider ? '测试中...' : '测试并创建' }}
              </button>
            </div>
            <p v-if="createStatus" class="status-inline">{{ createStatus }}</p>
          </div>
          <div v-else class="panel-desc">
            点击右上角 <strong>+</strong> 新建 provider，保存前会自动执行连通测试。
          </div>
        </article>
      </aside>

      <section class="right-column">
        <article class="assignment-card">
          <header class="assignment-header">
            <h2>用户来源分配</h2>
            <p>按用户维度覆盖默认 Provider 路由</p>
          </header>

          <div class="filter-row">
            <label class="search-field">
              <span>检索用户</span>
              <input
                v-model.trim="searchQuery"
                placeholder="按手机号 / 昵称 / 用户ID 搜索"
                @keyup.enter="refreshAll"
              />
            </label>
            <button type="button" class="ghost" :disabled="refreshing" @click="refreshAll">
              {{ refreshing ? '搜索中...' : '搜索用户' }}
            </button>
          </div>

          <div class="table-head">
            <span>用户</span>
            <span>目标 Provider</span>
            <span>Model</span>
            <span>操作</span>
          </div>

          <div v-if="loading" class="empty-state">加载中...</div>
          <div v-else-if="rowsForDisplay.length === 0" class="empty-state">暂无匹配用户</div>
          <div v-else class="table-body">
            <div v-for="item in rowsForDisplay" :key="item.user_id" class="table-row">
              <div class="user-cell">
                <strong>{{ item.phone }} · {{ item.nickname || '未命名用户' }}</strong>
                <small>{{ item.user_id }}</small>
              </div>

              <div class="provider-cell">
                <select
                  v-model="ensureDraft(item).provider_id"
                  :disabled="isSaving(item.user_id)"
                  @change="onProviderChange(item)"
                >
                  <option value="">使用默认（{{ defaultProviderName() }}）</option>
                  <option v-for="provider in providers" :key="provider.id" :value="provider.id">
                    {{ provider.name }}
                  </option>
                </select>
              </div>

              <div class="model-cell">
                <input
                  v-model.trim="ensureDraft(item).model"
                  :placeholder="modelOptionsForProvider(ensureDraft(item).provider_id)[0] || defaultModel || '模型可选'"
                  :disabled="isSaving(item.user_id)"
                  @input="markDirty(item.user_id)"
                />
              </div>

              <div class="action-cell">
                <button
                  type="button"
                  class="mini"
                  :disabled="isSaving(item.user_id) || !isRowDirty(item.user_id)"
                  @click="saveUser(item)"
                >
                  {{ isSaving(item.user_id) ? '保存中...' : '保存' }}
                </button>
              </div>
            </div>
          </div>

          <div class="summary-bar">来源 {{ selectedSourceName }} 下 {{ rowsForDisplay.length }} 位用户（总 {{ totalUsers }} 位）。</div>

          <footer class="bottom-actions">
            <button type="button" :disabled="bulkSaving || dirtyCount === 0" @click="saveAllDirty">
              {{ bulkSaving ? '提交中...' : '保存全部变更' }}
            </button>
            <button type="button" class="ghost" :disabled="bulkSaving || dirtyCount === 0" @click="rollbackAll">
              撤销未提交更改
            </button>
          </footer>
        </article>
      </section>
    </section>
  </main>
</template>
