<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import EmailAuthPanel from './components/EmailAuthPanel.vue'
import MemoryGraphPanel from './components/MemoryGraphPanel.vue'
import PostTurnModelPanel from './components/PostTurnModelPanel.vue'
import SummaryModelPanel from './components/SummaryModelPanel.vue'
import {
  pickRuntimeDefaultProviderId,
  resolveExpandedProviderId,
} from './utils/provider-panel-state.js'
import {
  clearUserProviderSetting,
  createProvider,
  deleteProvider,
  getAdminCredentials,
  getEmbeddingConfig,
  chatTestProvider,
  listProviders,
  listUserProviderSettings,
  switchProvider,
  testProvider,
  updateEmbeddingConfig,
  updateProvider,
  updateUserProviderSetting,
} from './api/admin.js'

const loading = ref(false)
const refreshing = ref(false)
const bulkSaving = ref(false)
const statusMessage = ref('')
const errorMessage = ref('')
const searchQuery = ref('')
const creatingProvider = ref(false)
const embeddingSaving = ref(false)
const showCreateProvider = ref(false)
const createStatus = ref('')
const selectedSourceId = ref('')
const expandedProviderId = ref('')
const embeddingSectionOpen = ref(true)
const hasAutoExpandedRuntimeDefault = ref(false)

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
const savingByProvider = reactive({})
const testingByProvider = reactive({})
const providerDrafts = reactive({})
const providerTestDrafts = reactive({})
const providerTestReplies = reactive({})
const newProvider = reactive({
  name: '',
  base_url: '',
  api_key: '',
  api_mode: 'chat_completions',
  models: '',
})
const embeddingConfig = reactive({
  base_url: '',
  api_key: '',
  model: '',
  provider_id: '',
})

const runtimeDefaultProviderId = computed(() =>
  pickRuntimeDefaultProviderId(providers.value, defaultProviderId.value),
)

const activeProvider = computed(() =>
  providers.value.find((provider) => provider.id === runtimeDefaultProviderId.value) || null,
)

const isRuntimeDefaultExpanded = computed(() =>
  Boolean(runtimeDefaultProviderId.value) && expandedProviderId.value === runtimeDefaultProviderId.value,
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

function applyEmbeddingConfig(payload = {}) {
  embeddingConfig.base_url = payload.base_url || ''
  embeddingConfig.api_key = payload.api_key || ''
  embeddingConfig.model = payload.model || ''
  embeddingConfig.provider_id = payload.provider_id || ''
}

function modelOptionsForProvider(providerId) {
  const provider = providers.value.find((item) => item.id === providerId)
  return Array.isArray(provider?.models) ? provider.models.filter(Boolean) : []
}

function providerUsersCount(providerId) {
  return providerUsageMap.value[providerId] || 0
}

function providerModeLabel(mode) {
  switch (mode) {
    case 'responses':
      return 'Responses'
    case 'auto':
      return 'Auto'
    default:
      return 'Chat Completions'
  }
}

function defaultProviderTestMessage(provider) {
  const model = Array.isArray(provider?.models) ? provider.models.find(Boolean) || '' : ''
  return `你好，请简短回复“${provider?.name || model || 'provider'} 已连接”。`
}

function ensureProviderTestDraft(provider) {
  if (!provider?.id) {
    return ''
  }
  if (!providerTestDrafts[provider.id]) {
    providerTestDrafts[provider.id] = defaultProviderTestMessage(provider)
  }
  return providerTestDrafts[provider.id]
}

function ensureProviderDraft(provider) {
  if (!provider?.id) {
    return {
      name: '',
      base_url: '',
      api_key: '',
      api_mode: 'chat_completions',
      models: '',
      priority: '100',
    }
  }
  if (!providerDrafts[provider.id]) {
    providerDrafts[provider.id] = {
      name: provider.name || '',
      base_url: provider.base_url || '',
      api_key: provider.api_key || '',
      api_mode: provider.api_mode || 'chat_completions',
      models: Array.isArray(provider.models) ? provider.models.join(', ') : '',
      priority: String(provider.priority ?? 100),
    }
  }
  return providerDrafts[provider.id]
}

function resetProviderDrafts() {
  Object.keys(providerDrafts).forEach((key) => {
    delete providerDrafts[key]
  })
  providers.value.forEach((provider) => {
    providerDrafts[provider.id] = {
      name: provider.name || '',
      base_url: provider.base_url || '',
      api_key: provider.api_key || '',
      api_mode: provider.api_mode || 'chat_completions',
      models: Array.isArray(provider.models) ? provider.models.join(', ') : '',
      priority: String(provider.priority ?? 100),
    }
  })
}

function toggleProviderExpand(providerId) {
  expandedProviderId.value = expandedProviderId.value === providerId ? '' : providerId
}

function toggleRuntimeDefaultExpand() {
  if (!runtimeDefaultProviderId.value) {
    return
  }
  toggleProviderExpand(runtimeDefaultProviderId.value)
  hasAutoExpandedRuntimeDefault.value = true
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

    const [settingPayload, providerPayload, embeddingPayload] = await Promise.all([
      listUserProviderSettings({
        q: searchQuery.value,
        limit: 100,
        offset: 0,
      }),
      listProviders().catch(() => []),
      getEmbeddingConfig().catch(() => ({})),
    ])

    const fullProviders = Array.isArray(providerPayload) ? providerPayload : []
    const options = Array.isArray(settingPayload.providers) ? settingPayload.providers : []
    if (fullProviders.length > 0) {
      providers.value = normalizeProviders(fullProviders)
    } else if (options.length > 0) {
      providers.value = normalizeProviders(options)
    } else {
      providers.value = []
    }
    rows.value = Array.isArray(settingPayload.items) ? settingPayload.items : []
    totalUsers.value = Number(settingPayload.total || rows.value.length)
    defaultProviderId.value = settingPayload.default_provider_id || ''
    if (!providers.value.some((provider) => provider.id === defaultProviderId.value)) {
      defaultProviderId.value = providers.value.find((provider) => provider.is_active)?.id || providers.value[0]?.id || ''
    }
    defaultModel.value = settingPayload.default_model || ''
    applyEmbeddingConfig(embeddingPayload)
    if (selectedSourceId.value && !providers.value.some((provider) => provider.id === selectedSourceId.value)) {
      selectedSourceId.value = ''
    }
    const expandedState = resolveExpandedProviderId({
      expandedProviderId: expandedProviderId.value,
      providers: providers.value,
      runtimeDefaultProviderId: pickRuntimeDefaultProviderId(providers.value, defaultProviderId.value),
      hasAutoExpandedRuntimeDefault: hasAutoExpandedRuntimeDefault.value,
    })
    expandedProviderId.value = expandedState.expandedProviderId
    hasAutoExpandedRuntimeDefault.value = expandedState.hasAutoExpandedRuntimeDefault

    resetDraftsFromRows()
    resetProviderDrafts()
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
    statusMessage.value = `已保存 ${item.nickname || item.email} 的配置。`
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

async function changeProviderMode(provider, apiMode) {
  if (!provider?.id || savingByProvider[provider.id]) {
    return
  }
  savingByProvider[provider.id] = true
  errorMessage.value = ''
  try {
    await updateProvider(provider.id, {
      name: provider.name,
      api_key: provider.api_key || '',
      base_url: provider.base_url,
      api_mode: apiMode,
      models: Array.isArray(provider.models) ? provider.models.join(', ') : '',
      priority: provider.priority,
      is_active: provider.is_active,
    })
    statusMessage.value = `${provider.name} 已切换到 ${providerModeLabel(apiMode)}。`
    await refreshAll()
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    savingByProvider[provider.id] = false
  }
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
    statusMessage.value = `${provider.name} 连通测试成功（${providerModeLabel(provider.api_mode)} / ${result.latency_ms ?? '-'} ms）。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    creatingProvider.value = false
  }
}

async function activateProvider(provider) {
  if (!provider?.id || savingByProvider[provider.id]) {
    return
  }
  savingByProvider[provider.id] = true
  errorMessage.value = ''
  try {
    const activeModel = Array.isArray(provider.models) ? provider.models.find(Boolean) || '' : ''
    await switchProvider(provider.id, activeModel)
    statusMessage.value = `${provider.name} 已切换为默认 Provider。`
    await refreshAll()
    expandedProviderId.value = provider.id
    hasAutoExpandedRuntimeDefault.value = true
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    savingByProvider[provider.id] = false
  }
}

async function runProviderMessageTest(provider) {
  if (!provider?.id || testingByProvider[provider.id]) {
    return
  }
  testingByProvider[provider.id] = true
  errorMessage.value = ''
  providerTestReplies[provider.id] = ''
  try {
    const message = (providerTestDrafts[provider.id] || defaultProviderTestMessage(provider)).trim()
    providerTestDrafts[provider.id] = message
    const model = Array.isArray(provider.models) ? provider.models.find(Boolean) || '' : ''
    const result = await chatTestProvider(provider.id, message, model)
    if (result.status !== 'ok') {
      throw new Error(result.error_message || 'Provider 消息测试失败。')
    }
    providerTestReplies[provider.id] = result.reply || ''
    statusMessage.value = `${provider.name} 消息测试成功（${providerModeLabel(provider.api_mode)} / ${result.latency_ms ?? '-'} ms）。`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    testingByProvider[provider.id] = false
  }
}

async function saveProvider(provider) {
  if (!provider?.id || savingByProvider[provider.id]) {
    return
  }
  const draft = ensureProviderDraft(provider)
  savingByProvider[provider.id] = true
  errorMessage.value = ''
  try {
    await updateProvider(provider.id, {
      name: draft.name,
      api_key: draft.api_key,
      base_url: draft.base_url,
      api_mode: draft.api_mode,
      models: draft.models,
      priority: Number(draft.priority || 100),
      is_active: provider.is_active,
    })
    statusMessage.value = `${provider.name} 配置已保存。`
    await refreshAll()
    expandedProviderId.value = provider.id
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    savingByProvider[provider.id] = false
  }
}

async function removeProvider(provider) {
  if (!provider?.id || savingByProvider[provider.id] || provider.is_active) {
    return
  }
  savingByProvider[provider.id] = true
  errorMessage.value = ''
  try {
    await deleteProvider(provider.id)
    if (selectedSourceId.value === provider.id) {
      selectedSourceId.value = ''
    }
    if (expandedProviderId.value === provider.id) {
      expandedProviderId.value = ''
    }
    statusMessage.value = `${provider.name} 已删除。`
    await refreshAll()
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    savingByProvider[provider.id] = false
  }
}

async function saveEmbeddingSettings() {
  if (embeddingSaving.value) {
    return
  }

  embeddingSaving.value = true
  errorMessage.value = ''
  try {
    const payload = await updateEmbeddingConfig(embeddingConfig)
    applyEmbeddingConfig(payload)
    statusMessage.value = 'Embedding 配置已保存。'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    embeddingSaving.value = false
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
      api_key: newProvider.api_key,
      api_mode: newProvider.api_mode,
      models: newProvider.models,
      priority: providers.value.length + 1,
      is_active: false,
    })

    const testResult = await testProvider(created.id)
    if (testResult.status !== 'ok') {
      throw new Error(testResult.error_message || 'Provider 连通测试失败。')
    }

    createStatus.value = `连通测试成功（${testResult.latency_ms ?? '-'} ms）`
    newProvider.name = ''
    newProvider.base_url = ''
    newProvider.api_key = ''
    newProvider.api_mode = 'chat_completions'
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
              :class="['provider-item-card', {
                selected: selectedSourceId === provider.id,
                expanded: expandedProviderId === provider.id,
              }]"
            >
              <button type="button" class="provider-card-toggle" @click="toggleProviderExpand(provider.id)">
                <div class="provider-item-top">
                  <strong>{{ provider.name }}</strong>
                  <span :class="['provider-state', { active: provider.is_active }]">
                    {{ provider.is_active ? 'Active' : 'Standby' }}
                  </span>
                </div>
                <div class="provider-summary-grid">
                  <p class="provider-model">
                    {{ Array.isArray(provider.models) && provider.models.length > 0 ? provider.models[0] : '未配置 model' }}
                  </p>
                  <p class="provider-count">{{ providerModeLabel(provider.api_mode) }}</p>
                  <p class="provider-count">{{ providerUsersCount(provider.id) }} 位用户</p>
                </div>
              </button>

              <div class="provider-inline-actions">
                <button
                  type="button"
                  class="ghost mini"
                  @click="selectedSourceId = selectedSourceId === provider.id ? '' : provider.id"
                >
                  {{ selectedSourceId === provider.id ? '取消筛选' : '筛选用户' }}
                </button>
                <button
                  type="button"
                  class="ghost mini"
                  :disabled="creatingProvider || savingByProvider[provider.id]"
                  @click="quickTestProvider(provider)"
                >
                  连通测试
                </button>
                <button
                  type="button"
                  class="mini"
                  :disabled="creatingProvider || savingByProvider[provider.id] || provider.is_active"
                  @click="activateProvider(provider)"
                >
                  {{ provider.is_active ? 'Active' : '设为默认' }}
                </button>
              </div>

              <div v-if="expandedProviderId === provider.id" class="provider-expander">
                <label class="mini-form-field">
                  <span>Name</span>
                  <input v-model.trim="ensureProviderDraft(provider).name" placeholder="Provider 名称" />
                </label>
                <label class="mini-form-field">
                  <span>Base URL</span>
                  <input v-model.trim="ensureProviderDraft(provider).base_url" placeholder="https://api.example.com/v1" />
                </label>
                <label class="mini-form-field">
                  <span>API Key</span>
                  <input v-model.trim="ensureProviderDraft(provider).api_key" type="password" placeholder="sk-..." />
                </label>
                <div class="provider-expander-grid">
                  <label class="mini-form-field">
                    <span>API Mode</span>
                    <select v-model="ensureProviderDraft(provider).api_mode">
                      <option value="chat_completions">Chat Completions</option>
                      <option value="responses">Responses</option>
                      <option value="auto">Auto</option>
                    </select>
                  </label>
                  <label class="mini-form-field">
                    <span>Priority</span>
                    <input v-model.trim="ensureProviderDraft(provider).priority" inputmode="numeric" placeholder="100" />
                  </label>
                </div>
                <label class="mini-form-field">
                  <span>Models（逗号分隔）</span>
                  <input v-model.trim="ensureProviderDraft(provider).models" placeholder="gpt-4o-mini, gpt-4.1-mini" />
                </label>

                <div class="provider-item-actions">
                  <button
                    type="button"
                    :disabled="savingByProvider[provider.id]"
                    @click="saveProvider(provider)"
                  >
                    {{ savingByProvider[provider.id] ? '保存中...' : '保存配置' }}
                  </button>
                  <button
                    type="button"
                    class="ghost mini"
                    :disabled="testingByProvider[provider.id]"
                    @click="runProviderMessageTest(provider)"
                  >
                    {{ testingByProvider[provider.id] ? '测试中...' : '消息测试' }}
                  </button>
                  <button
                    type="button"
                    class="ghost mini"
                    :disabled="savingByProvider[provider.id] || provider.is_active"
                    @click="removeProvider(provider)"
                  >
                    删除
                  </button>
                </div>

                <div class="provider-test-box">
                  <input
                    :value="ensureProviderTestDraft(provider)"
                    placeholder="输入一条测试消息，验证端点是否可用"
                    @input="providerTestDrafts[provider.id] = $event.target.value"
                  />
                  <p v-if="providerTestReplies[provider.id]" class="provider-test-reply">
                    {{ providerTestReplies[provider.id] }}
                  </p>
                </div>
              </div>
            </article>
          </div>

          <div class="default-summary">
            <div class="section-card-header">
              <p class="panel-eyebrow">Runtime Default</p>
              <button
                type="button"
                class="ghost mini section-toggle"
                :disabled="!runtimeDefaultProviderId"
                :aria-expanded="isRuntimeDefaultExpanded"
                @click="toggleRuntimeDefaultExpand"
              >
                {{ isRuntimeDefaultExpanded ? '收起主 Provider' : '展开主 Provider' }}
              </button>
            </div>
            <div class="section-card-body default-summary-body">
              <div class="kv-row">
                <span>Provider</span>
                <strong>{{ activeProvider?.name || defaultProviderName() }}</strong>
              </div>
              <div class="kv-row">
                <span>Model</span>
                <strong>{{ activeProviderModel }}</strong>
              </div>
            </div>
          </div>

          <div class="embedding-config-card">
            <div class="section-card-header">
              <div>
                <p class="panel-eyebrow">全局 Embedding Config</p>
                <p class="panel-desc">默认所有用户 / Bot 共用这一套 embedding 模型，仅用于记忆检索，不影响聊天回复模型。这里填写到 <strong>/v1</strong> 即可，实际请求会自动走 <strong>/embeddings</strong>。</p>
              </div>
              <button
                type="button"
                class="ghost mini section-toggle"
                :aria-expanded="embeddingSectionOpen"
                @click="embeddingSectionOpen = !embeddingSectionOpen"
              >
                {{ embeddingSectionOpen ? '收起' : '展开' }}
              </button>
            </div>
            <div v-if="embeddingSectionOpen" class="section-card-body">
              <label class="mini-form-field">
                <span>Embedding Base URL</span>
                <input v-model.trim="embeddingConfig.base_url" placeholder="https://router.tumuer.me/v1" />
              </label>
              <label class="mini-form-field">
                <span>Embedding API Key</span>
                <input v-model.trim="embeddingConfig.api_key" type="password" placeholder="sk-..." />
              </label>
              <label class="mini-form-field">
                <span>Embedding Model</span>
                <input v-model.trim="embeddingConfig.model" placeholder="Qwen/Qwen3-Embedding-4B" />
              </label>
              <label class="mini-form-field">
                <span>未配置专用 URL/Key 时的回退 Provider（可选）</span>
                <select v-model="embeddingConfig.provider_id">
                  <option value="">不指定</option>
                  <option v-for="provider in providers" :key="provider.id" :value="provider.id">
                    {{ provider.name }}
                  </option>
                </select>
              </label>
              <div class="create-actions">
                <button type="button" :disabled="embeddingSaving" @click="saveEmbeddingSettings">
                  {{ embeddingSaving ? '保存中...' : '保存全局 Embedding 配置' }}
                </button>
              </div>
            </div>
          </div>

          <EmailAuthPanel />
          <PostTurnModelPanel />
          <SummaryModelPanel />

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
              <span>API Key</span>
              <input v-model.trim="newProvider.api_key" type="password" placeholder="sk-..." />
            </label>
            <label class="mini-form-field">
              <span>API Mode</span>
              <select v-model="newProvider.api_mode">
                <option value="chat_completions">Chat Completions</option>
                <option value="responses">Responses</option>
                <option value="auto">Auto</option>
              </select>
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
                placeholder="按邮箱 / 昵称 / 用户ID 搜索"
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
                <strong>{{ item.email }} · {{ item.nickname || '未命名用户' }}</strong>
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

        <MemoryGraphPanel />
      </section>
    </section>
  </main>
</template>
