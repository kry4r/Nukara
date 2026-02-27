import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'

export const useConversationsStore = defineStore('conversations', () => {
  const api = useApi()
  const list = ref([])
  const isLoading = ref(false)

  async function fetchList() {
    isLoading.value = true
    try {
      const data = await api.get('/api/v1/conversations')
      list.value = Array.isArray(data) ? data : []
    } catch (_) {}
    isLoading.value = false
  }

  function handleBotStatus(data) {
    const conv = list.value.find(c => c.id === data.conversation_id)
    if (conv) {
      conv.bot_status_emoji = data.emoji || data.status?.emoji || ''
      conv.bot_status_text = data.text || data.status?.text || ''
    }
  }

  function updateConversation(convId, updates) {
    const conv = list.value.find(c => c.id === convId)
    if (conv) Object.assign(conv, updates)
  }

  return { list, isLoading, fetchList, handleBotStatus, updateConversation }
})
