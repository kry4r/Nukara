import { API_BASE } from '../utils/constants'

export function useApi() {
  const getToken = () => localStorage.getItem('nukara_token')

  async function request(path, opts = {}) {
    const token = getToken()
    const headers = {
      'Content-Type': 'application/json',
      ...opts.headers,
    }
    if (token) headers['Authorization'] = `Bearer ${token}`

    const res = await fetch(API_BASE + path, {
      ...opts,
      headers,
    })

    const rawText = await res.text()
    let payload = null
    if (rawText) {
      try {
        payload = JSON.parse(rawText)
      } catch {
        payload = rawText
      }
    }

    if (res.status === 401) {
      localStorage.removeItem('nukara_token')
      localStorage.removeItem('nukara_user')
      window.location.href = '/auth'
      throw new Error('Unauthorized')
    }

    if (!res.ok) {
      const message = typeof payload === 'string'
        ? payload
        : payload?.message || payload?.error || `Request failed: ${res.status}`
      throw new Error(message)
    }

    return payload
  }

  const get = (path) => request(path)
  const post = (path, body) => request(path, {
    method: 'POST',
    body: JSON.stringify(body),
  })
  const put = (path, body) => request(path, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
  const patch = (path, body) => request(path, {
    method: 'PATCH',
    body: JSON.stringify(body),
  })
  const del = (path) => request(path, { method: 'DELETE' })

  return { request, get, post, put, patch, del }
}
