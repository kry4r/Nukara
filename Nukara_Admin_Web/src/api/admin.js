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
