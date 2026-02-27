import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'

export const useBotsStore = defineStore('bots', () => {
  const api = useApi()
  const list = ref([])
  const isLoading = ref(false)
  const error = ref('')

  async function fetchList() {
    isLoading.value = true
    try {
      const data = await api.get('/api/v1/bots')
      list.value = Array.isArray(data) ? data : []
    } catch (_) {}
    isLoading.value = false
  }

  async function createBot(form) {
    isLoading.value = true
    error.value = ''
    try {
      const bot = await api.post('/api/v1/bots', form)
      if (bot.id) {
        list.value.push(bot)
        return bot
      }
      error.value = bot.error || '创建失败'
    } catch (e) {
      error.value = e.message
    } finally {
      isLoading.value = false
    }
    return null
  }

  async function getBot(id) {
    return api.get(`/api/v1/bots/${id}`)
  }

  async function updateBot(id, updates) {
    return api.put(`/api/v1/bots/${id}`, updates)
  }

  return { list, isLoading, error, fetchList, createBot, getBot, updateBot }
})
