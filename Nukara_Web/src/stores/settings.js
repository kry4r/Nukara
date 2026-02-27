import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'

export const useSettingsStore = defineStore('settings', () => {
  const api = useApi()
  const notifications = ref({
    proactive_enabled: true,
    dnd_start: '23:00',
    dnd_end: '08:00',
    frequency: 'normal',
  })
  const userStatus = ref({ emoji: '', text: '' })
  const isLoading = ref(false)

  async function fetchNotifications() {
    try {
      const data = await api.get('/api/v1/users/notification-settings')
      if (data.user_id || data.proactive_enabled !== undefined) {
        notifications.value = data
      }
    } catch (_) {}
  }

  async function saveNotifications(settings) {
    isLoading.value = true
    try {
      const data = await api.put('/api/v1/users/notification-settings', settings)
      notifications.value = data
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
    notifications, userStatus, isLoading,
    fetchNotifications, saveNotifications,
    fetchUserStatus, saveUserStatus,
  }
})
