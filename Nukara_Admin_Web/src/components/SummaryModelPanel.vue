<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import {
  getSelfCognitionSummaryModelConfig,
  listProviders,
  updateSelfCognitionSummaryModelConfig,
} from '../api/admin.js'

const loading = ref(false)
const saving = ref(false)
const isOpen = ref(true)
const statusMessage = ref('')
const errorMessage = ref('')
const providers = ref([])
const form = reactive({
  provider_id: '',
  model: '',
})

const selectedProvider = computed(() =>
  providers.value.find((item) => item.id === form.provider_id) || null,
)
const selectedProviderModels = computed(() =>
  Array.isArray(selectedProvider.value?.models) ? selectedProvider.value.models.filter(Boolean) : [],
)

function applyConfig(payload = {}) {
  form.provider_id = payload.provider_id || ''
  form.model = payload.model || ''
}

async function refreshPanel() {
  loading.value = true
  errorMessage.value = ''
  try {
    const [providerPayload, configPayload] = await Promise.all([
      listProviders().catch(() => []),
      getSelfCognitionSummaryModelConfig(),
    ])
    providers.value = Array.isArray(providerPayload) ? providerPayload : []
    applyConfig(configPayload)
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loading.value = false
  }
}

function onProviderChange() {
  if (!form.provider_id) {
    form.model = ''
    return
  }
  if (!selectedProviderModels.value.includes(form.model)) {
    form.model = selectedProviderModels.value[0] || ''
  }
}

async function saveConfig() {
  if (saving.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    const payload = await updateSelfCognitionSummaryModelConfig(form)
    applyConfig(payload)
    statusMessage.value = '自我认知总结模型配置已保存。'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  refreshPanel()
})
</script>

<template>
  <article class="summary-model-card">
    <header class="panel-header">
      <div>
        <p class="panel-eyebrow">Self-Cognition</p>
        <h2>自我认知总结模型</h2>
      </div>
      <div class="panel-header-actions">
        <button type="button" class="ghost" :disabled="loading" @click="refreshPanel">
          {{ loading ? '刷新中...' : '刷新' }}
        </button>
        <button type="button" class="ghost" :aria-expanded="isOpen" @click="isOpen = !isOpen">
          {{ isOpen ? '收起' : '展开' }}
        </button>
      </div>
    </header>

    <template v-if="isOpen">
      <p class="panel-desc">
        配置“自我认知”总结专用模型；未设置时会自动回退到默认聊天路由。
      </p>

      <div class="panel-grid">
      <label class="mini-form-field">
        <span>Provider</span>
        <select v-model="form.provider_id" @change="onProviderChange">
          <option value="">使用默认路由</option>
          <option v-for="provider in providers" :key="provider.id" :value="provider.id">
            {{ provider.name }}
          </option>
        </select>
      </label>

      <label class="mini-form-field">
        <span>Model</span>
        <input
          v-model.trim="form.model"
          :placeholder="selectedProviderModels[0] || '留空则使用 provider 默认模型'"
          list="self-cognition-model-options"
        />
        <datalist id="self-cognition-model-options">
          <option v-for="model in selectedProviderModels" :key="model" :value="model" />
        </datalist>
      </label>
      </div>

      <div class="panel-actions">
        <button type="button" :disabled="saving" @click="saveConfig">
          {{ saving ? '保存中...' : '保存总结模型' }}
        </button>
      </div>

      <p v-if="statusMessage" class="status-inline">{{ statusMessage }}</p>
      <p v-if="errorMessage" class="error-inline">{{ errorMessage }}</p>
    </template>
  </article>
</template>

<style scoped>
.summary-model-card {
  display: grid;
  gap: 16px;
  padding: 20px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(148, 163, 184, 0.24);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.08);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.panel-header-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.panel-header h2 {
  margin: 4px 0 0;
}

.panel-eyebrow {
  margin: 0;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #6366f1;
}

.panel-desc {
  margin: 0;
  color: #475569;
  line-height: 1.7;
}

.panel-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.mini-form-field {
  display: grid;
  gap: 8px;
}

.mini-form-field span {
  font-size: 13px;
  color: #334155;
}

.mini-form-field input,
.mini-form-field select {
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 14px;
  padding: 12px 14px;
  font-size: 14px;
}

.panel-actions {
  display: flex;
  gap: 12px;
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

@media (max-width: 920px) {
  .panel-grid {
    grid-template-columns: 1fr;
  }
}
</style>
