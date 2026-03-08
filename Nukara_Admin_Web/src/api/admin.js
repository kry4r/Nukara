const ADMIN_API_BASE = '/api/admin'
const USERNAME_KEY = 'nukara_admin_username'
const PASSWORD_KEY = 'nukara_admin_password'

export function normalizeProviderPayload(input) {
  return {
    name: input.name?.trim() || '',
    api_key: input.api_key || '',
    base_url: input.base_url?.trim() || '',
    api_mode: input.api_mode || 'chat_completions',
    models: String(input.models || '')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean),
    is_active: Boolean(input.is_active),
    priority: Number.parseInt(input.priority ?? 0, 10) || 0,
  }
}

export function getAdminCredentials() {
  return {
    username: localStorage.getItem(USERNAME_KEY) || '',
    password: localStorage.getItem(PASSWORD_KEY) || '',
  }
}

export function setAdminCredentials({ username, password }) {
  localStorage.setItem(USERNAME_KEY, username || '')
  localStorage.setItem(PASSWORD_KEY, password || '')
}

function withAuthHeaders(headers = {}) {
  const { username, password } = getAdminCredentials()
  if (!username || !password) {
    return headers
  }

  const auth = btoa(`${username}:${password}`)
  return {
    ...headers,
    Authorization: `Basic ${auth}`,
  }
}

async function request(path, options = {}) {
  const method = options.method || 'GET'
  const headers = withAuthHeaders(options.headers || {})
  const requestOptions = {
    ...options,
    method,
    headers,
  }

  if (options.body !== undefined) {
    requestOptions.body = JSON.stringify(options.body)
    requestOptions.headers = {
      'Content-Type': 'application/json',
      ...headers,
    }
  }

  const response = await fetch(`${ADMIN_API_BASE}${path}`, requestOptions)
  const text = await response.text()
  let payload = {}
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { message: text }
    }
  }

  if (!response.ok) {
    const message = payload.message || text || `Request failed: ${response.status}`
    throw new Error(message)
  }

  return payload
}

export function listProviders() {
  return request('/providers')
}

export function createProvider(input) {
  return request('/providers', {
    method: 'POST',
    body: normalizeProviderPayload(input),
  })
}

export function updateProvider(id, input) {
  return request(`/providers/${id}`, {
    method: 'PUT',
    body: normalizeProviderPayload(input),
  })
}

export function deleteProvider(id) {
  return request(`/providers/${id}`, {
    method: 'DELETE',
  })
}

export function testProvider(id) {
  return request(`/providers/${id}/test`, {
    method: 'POST',
  })
}

export function switchProvider(id, model = '') {
  return request(`/providers/${id}/switch`, {
    method: 'POST',
    body: model ? { model } : {},
  })
}

export function chatTestProvider(id, message, model = '') {
  return request(`/providers/${id}/chat-test`, {
    method: 'POST',
    body: {
      message,
      model,
    },
  })
}

export function restartRuntime() {
  return request('/runtime/restart-agent-runtime', {
    method: 'POST',
  })
}

export function listUserProviderSettings({ q = '', limit = 50, offset = 0 } = {}) {
  const search = new URLSearchParams()
  if (q) search.set('q', q)
  if (limit !== undefined) search.set('limit', String(limit))
  if (offset !== undefined) search.set('offset', String(offset))
  const query = search.toString()
  return request(`/users/provider-settings${query ? `?${query}` : ''}`)
}

export function buildListAdminUsersPath({ q = '', limit = 50, offset = 0 } = {}) {
  const search = new URLSearchParams()
  if (q) search.set('q', q)
  if (limit !== undefined) search.set('limit', String(limit))
  if (offset !== undefined) search.set('offset', String(offset))
  const query = search.toString()
  return `/users${query ? `?${query}` : ''}`
}

export function buildListUserBotsPath(userId) {
  return `/users/${userId}/bots`
}

export function buildMemoryGraphPath(userId, botId, filters = {}) {
  const search = new URLSearchParams()
  if (filters.kind) search.set('kind', String(filters.kind).trim())
  if (filters.status) search.set('status', String(filters.status).trim())
  const query = search.toString()
  return `/users/${userId}/bots/${botId}/memory-graph${query ? `?${query}` : ''}`
}

export function buildPostTurnModelPath() {
  return '/settings/post-turn-model'
}

export function buildSelfCognitionSummaryModelPath() {
  return '/settings/self-cognition-summary-model'
}

export function normalizePostTurnModelPayload(input = {}) {
  return {
    provider_id: input.provider_id?.trim() || '',
    model: input.model?.trim() || '',
  }
}

export function normalizeSelfCognitionSummaryModelPayload(input = {}) {
  return {
    provider_id: input.provider_id?.trim() || '',
    model: input.model?.trim() || '',
  }
}

export function normalizeEmailAuthSettingsPayload(input = {}) {
  return {
    smtp_host: input.smtp_host?.trim() || '',
    smtp_port: String(input.smtp_port || '465').trim() || '465',
    smtp_username: input.smtp_username?.trim() || '',
    smtp_password: input.smtp_password || '',
    from_email: input.from_email?.trim() || '',
    from_name: input.from_name?.trim() || 'Nukara',
    code_ttl_seconds: Number.parseInt(input.code_ttl_seconds ?? 900, 10) || 900,
  }
}

export function listAdminUsers(options = {}) {
  return request(buildListAdminUsersPath(options))
}

export function listUserBots(userId) {
  return request(buildListUserBotsPath(userId))
}

export function getMemoryGraph(userId, botId, filters = {}) {
  return request(buildMemoryGraphPath(userId, botId, filters))
}

export function getPostTurnModelConfig() {
  return request(buildPostTurnModelPath())
}

export function updatePostTurnModelConfig(payload) {
  return request(buildPostTurnModelPath(), {
    method: 'PUT',
    body: normalizePostTurnModelPayload(payload),
  })
}

export function getSelfCognitionSummaryModelConfig() {
  return request(buildSelfCognitionSummaryModelPath())
}

export function updateSelfCognitionSummaryModelConfig(payload) {
  return request(buildSelfCognitionSummaryModelPath(), {
    method: 'PUT',
    body: normalizeSelfCognitionSummaryModelPayload(payload),
  })
}

export function getEmbeddingConfig() {
  return request('/settings/embedding-config')
}

export function updateEmbeddingConfig(payload) {
  return request('/settings/embedding-config', {
    method: 'PUT',
    body: {
      base_url: payload.base_url || '',
      api_key: payload.api_key || '',
      model: payload.model || '',
      provider_id: payload.provider_id || '',
    },
  })
}

export function getEmailAuthSettings() {
  return request('/settings/email-auth')
}

export function updateEmailAuthSettings(payload) {
  return request('/settings/email-auth', {
    method: 'PUT',
    body: normalizeEmailAuthSettingsPayload(payload),
  })
}

export function sendEmailAuthTest(to_email) {
  return request('/settings/email-auth/test', {
    method: 'POST',
    body: { to_email: to_email?.trim() || '' },
  })
}

export function updateUserProviderSetting(userId, payload) {
  return request(`/users/provider-settings/${userId}`, {
    method: 'PUT',
    body: {
      provider_id: payload.provider_id,
      model: payload.model || '',
    },
  })
}

export function clearUserProviderSetting(userId) {
  return request(`/users/provider-settings/${userId}`, {
    method: 'DELETE',
  })
}
