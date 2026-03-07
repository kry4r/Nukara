import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'

function normalizeNotifications(data = {}) {
  const interval = Number(data.proactive_interval_minutes)
  return {
    proactive_enabled: data.proactive_enabled !== false,
    proactive_interval_minutes: Number.isFinite(interval) && interval > 0 ? interval : 240,
    dnd_start: data.dnd_start || '23:00',
    dnd_end: data.dnd_end || '08:00',
  }
}

export const useSettingsStore = defineStore('settings', () => {
  const api = useApi()
  const notifications = ref(normalizeNotifications())
  const userStatus = ref({ emoji: '', text: '' })
  const isLoading = ref(false)

  async function fetchNotifications() {
    try {
      const data = await api.get('/api/v1/users/notification-settings')
      if (data?.user_id || data?.proactive_enabled !== undefined) {
        notifications.value = normalizeNotifications(data)
      }
    } catch (_) {}
  }

  async function saveNotifications(settings) {
    isLoading.value = true
    try {
      const data = await api.put('/api/v1/users/notification-settings', settings)
      notifications.value = normalizeNotifications(data)
    } catch (_) {}
    isLoading.value = false
  }

  async function fetchUserStatus() {
    try {
      const data = await api.get('/api/v1/users/status')
      userStatus.value = data
    } catch (_) {}
  }

  async function saveUserStatus(status) {
    try {
      await api.put('/api/v1/users/status', status)
      userStatus.value = status
    } catch (_) {}
  }

  return {
    notifications,
    userStatus,
    isLoading,
    fetchNotifications,
    saveNotifications,
    fetchUserStatus,
    saveUserStatus,
  }
})
