<script setup>
import { computed, onMounted, ref } from 'vue'
import ProviderForm from './components/ProviderForm.vue'
import ProviderTable from './components/ProviderTable.vue'
import {
  chatTestProvider,
  createProvider,
  deleteProvider,
  getAdminCredentials,
  listProviders,
  restartNanobot,
  setAdminCredentials,
  switchProvider,
  testProvider,
  updateProvider,
} from './api/admin.js'

const providers = ref([])
const loading = ref(false)
const submitting = ref(false)
const editingProvider = ref(null)
const statusMessage = ref('')
const errorMessage = ref('')
const credentials = ref(getAdminCredentials())
const selectedProviderId = ref('')
const chatInput = ref('你好，请回复 provider 测试已连接。')
const chatSending = ref(false)
const chatMessages = ref([])
const nanobotRestarting = ref(false)

const selectedProvider = computed(() =>
  providers.value.find((provider) => provider.id === selectedProviderId.value) || null,
)

async function refreshProviders() {
  loading.value = true
  errorMessage.value = ''
  try {
    providers.value = await listProviders()
    if (providers.value.length === 0) {
      selectedProviderId.value = ''
      return
    }
    const active = providers.value.find((provider) => provider.is_active)
    const existing = providers.value.find((provider) => provider.id === selectedProviderId.value)
    selectedProviderId.value = existing?.id || active?.id || providers.value[0].id
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    loading.value = false
  }
}

function saveCredentials() {
  setAdminCredentials(credentials.value)
  statusMessage.value = 'Credentials saved locally.'
}

function onEdit(provider) {
  editingProvider.value = provider
}

function onCancelForm() {
  editingProvider.value = null
}

async function onSubmitProvider(payload) {
  submitting.value = true
  errorMessage.value = ''
  statusMessage.value = ''
  try {
    if (editingProvider.value?.id) {
      await updateProvider(editingProvider.value.id, payload)
      statusMessage.value = 'Provider updated.'
    } else {
      await createProvider(payload)
      statusMessage.value = 'Provider created.'
    }
    editingProvider.value = null
    await refreshProviders()
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    submitting.value = false
  }
}

async function onDelete(provider) {
  if (!provider?.id) {
    return
  }
  const ok = window.confirm(`Delete provider "${provider.name}"?`)
  if (!ok) {
    return
  }

  errorMessage.value = ''
  statusMessage.value = ''
  try {
    await deleteProvider(provider.id)
    statusMessage.value = 'Provider deleted.'
    await refreshProviders()
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function onTest(provider) {
  if (!provider?.id) {
    return
  }
  errorMessage.value = ''
  statusMessage.value = ''
  try {
    const result = await testProvider(provider.id)
    statusMessage.value = `Test result: ${result.status || 'ok'}`
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function onSwitch(provider) {
  if (!provider?.id) {
    return
  }
  errorMessage.value = ''
  statusMessage.value = ''
  try {
    await switchProvider(provider.id)
    selectedProviderId.value = provider.id
    statusMessage.value = 'Provider switched.'
    await refreshProviders()
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function onSendChatTest() {
  if (!selectedProviderId.value || chatSending.value) {
    return
  }
  const content = chatInput.value.trim()
  if (!content) {
    errorMessage.value = 'Message cannot be empty.'
    return
  }

  const providerModel = Array.isArray(selectedProvider.value?.models)
    ? selectedProvider.value.models[0] || ''
    : ''

  const userMsg = {
    id: `${Date.now()}-user`,
    role: 'user',
    text: content,
    providerId: selectedProviderId.value,
    latency: null,
    failed: false,
  }
  chatMessages.value.push(userMsg)
  errorMessage.value = ''
  statusMessage.value = ''
  chatSending.value = true

  try {
    const result = await chatTestProvider(selectedProviderId.value, content, providerModel)
    if (result.status !== 'ok') {
      throw new Error(result.error_message || 'Provider chat test failed')
    }
    chatMessages.value.push({
      id: `${Date.now()}-assistant`,
      role: 'assistant',
      text: result.reply || '(empty response)',
      providerId: result.provider_id || selectedProviderId.value,
      latency: result.latency_ms ?? null,
      failed: false,
    })
    statusMessage.value = `Chat test OK (${result.latency_ms ?? '-'} ms)`
  } catch (error) {
    chatMessages.value.push({
      id: `${Date.now()}-error`,
      role: 'assistant',
      text: error.message,
      providerId: selectedProviderId.value,
      latency: null,
      failed: true,
    })
    errorMessage.value = error.message
  } finally {
    chatSending.value = false
  }
}

async function onRestartNanobot() {
  if (nanobotRestarting.value) {
    return
  }
  errorMessage.value = ''
  statusMessage.value = ''
  nanobotRestarting.value = true

  try {
    const result = await restartNanobot()
    statusMessage.value = `Nanobot restarted (${result.latency_ms ?? '-'} ms)`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    nanobotRestarting.value = false
  }
}

onMounted(() => {
  refreshProviders()
})
</script>

<template>
  <main class="admin-app">
    <header class="admin-header">
      <h1>Nukara Admin Portal</h1>
      <button type="button" class="ghost" :disabled="loading" @click="refreshProviders">Refresh</button>
    </header>

    <section class="credentials-card">
      <h2>Admin Credentials</h2>
      <div class="credentials-row">
        <label>
          Username
          <input v-model.trim="credentials.username" autocomplete="username" />
        </label>
        <label>
          Password
          <input v-model="credentials.password" type="password" autocomplete="current-password" />
        </label>
        <button type="button" @click="saveCredentials">Save</button>
      </div>
    </section>

    <p v-if="statusMessage" class="status-message">{{ statusMessage }}</p>
    <p v-if="errorMessage" class="error-message">{{ errorMessage }}</p>

    <section class="content-grid">
      <ProviderForm :provider="editingProvider" :disabled="submitting" @submit="onSubmitProvider" @cancel="onCancelForm" />
      <ProviderTable
        :providers="providers"
        :loading="loading"
        @edit="onEdit"
        @delete="onDelete"
        @test="onTest"
        @switch="onSwitch"
      />
    </section>

    <section class="chat-test-card">
      <header class="chat-test-header">
        <div>
          <h2>Nanobot Provider Chat Test</h2>
          <p>Directly writes selected provider into Nanobot config, then sends real chat request.</p>
        </div>
        <button type="button" class="ghost" :disabled="nanobotRestarting" @click="onRestartNanobot">
          {{ nanobotRestarting ? 'Restarting...' : 'Restart Nanobot' }}
        </button>
      </header>

      <div class="chat-controls">
        <label>
          Provider
          <select v-model="selectedProviderId" :disabled="chatSending || loading">
            <option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.name }}{{ provider.is_active ? ' (active)' : '' }}
            </option>
          </select>
        </label>
        <button
          type="button"
          :disabled="chatSending || !selectedProviderId || !chatInput.trim()"
          @click="onSendChatTest"
        >
          {{ chatSending ? 'Sending...' : 'Send Test Message' }}
        </button>
      </div>

      <label class="chat-input-label">
        Message
        <textarea
          v-model="chatInput"
          rows="3"
          :disabled="chatSending"
          placeholder="Type test message for provider validation"
        />
      </label>

      <div class="chat-thread">
        <p v-if="chatMessages.length === 0" class="chat-empty">No messages yet.</p>
        <article
          v-for="message in chatMessages"
          :key="message.id"
          class="chat-bubble"
          :class="{ 'is-user': message.role === 'user', 'is-error': message.failed }"
        >
          <header>
            <strong>{{ message.role === 'user' ? 'You' : 'Nanobot' }}</strong>
            <span>{{ message.providerId }}</span>
            <span v-if="message.latency !== null">{{ message.latency }} ms</span>
          </header>
          <p>{{ message.text }}</p>
        </article>
      </div>
    </section>
  </main>
</template>
