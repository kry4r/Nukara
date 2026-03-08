import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'

export const useAuthStore = defineStore('auth', () => {
  const api = useApi()
  const user = ref(null)
  const token = ref(localStorage.getItem('nukara_token') || '')
  const isLoading = ref(false)
  const error = ref('')

  function restoreSession() {
    const saved = localStorage.getItem('nukara_user')
    if (saved) user.value = JSON.parse(saved)
    token.value = localStorage.getItem('nukara_token') || ''
  }

  async function requestEmailCode(email, purpose) {
    isLoading.value = true
    error.value = ''
    try {
      await api.post('/api/v1/auth/email/send', { email, purpose })
      return true
    } catch (e) {
      error.value = e.message
      return false
    } finally {
      isLoading.value = false
    }
  }

  async function login(email, emailCode) {
    isLoading.value = true
    error.value = ''
    try {
      const data = await api.post('/api/v1/auth/login', {
        email, email_code: emailCode,
      })
      if (data.access_token) {
        setSession(data)
        return true
      }
      error.value = data.error || '登录失败'
    } catch (e) {
      error.value = e.message
    } finally {
      isLoading.value = false
    }
    return false
  }

  async function register(email, emailCode, nickname) {
    isLoading.value = true
    error.value = ''
    try {
      const data = await api.post('/api/v1/auth/register', {
        email, email_code: emailCode, nickname,
      })
      if (data.access_token) {
        setSession(data)
        return true
      }
      error.value = data.error || '注册失败'
    } catch (e) {
      error.value = e.message
    } finally {
      isLoading.value = false
    }
    return false
  }

  function setSession(data) {
    token.value = data.access_token
    user.value = data.user
    localStorage.setItem('nukara_token', data.access_token)
    localStorage.setItem('nukara_user', JSON.stringify(data.user))
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('nukara_token')
    localStorage.removeItem('nukara_user')
  }

  return {
    user, token, isLoading, error,
    restoreSession, requestEmailCode, login, register, logout,
  }
})
