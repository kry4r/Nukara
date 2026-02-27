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

    if (res.status === 401) {
      localStorage.removeItem('nukara_token')
      localStorage.removeItem('nukara_user')
      window.location.href = '/auth'
      throw new Error('Unauthorized')
    }

    return res.json()
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
