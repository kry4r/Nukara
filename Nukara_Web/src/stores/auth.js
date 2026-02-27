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

  async function requestSMS(phone, purpose) {
    isLoading.value = true
    error.value = ''
    try {
      await api.post('/api/v1/auth/sms/send', { phone, purpose })
    } catch (e) {
      error.value = e.message
    } finally {
      isLoading.value = false
    }
  }

  async function login(phone, smsCode) {
    isLoading.value = true
    error.value = ''
    try {
      const data = await api.post('/api/v1/auth/login', {
        phone, sms_code: smsCode,
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

  async function register(phone, smsCode, nickname) {
    isLoading.value = true
    error.value = ''
    try {
      const data = await api.post('/api/v1/auth/register', {
        phone, sms_code: smsCode, nickname,
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
    restoreSession, requestSMS, login, register, logout,
  }
})
