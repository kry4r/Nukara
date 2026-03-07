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

  function findConversation(convId) {
    return list.value.find((item) => item.id === convId) || null
  }

  function handleBotStatus(data) {
    const conv = findConversation(data.conversation_id)
    if (conv) {
      conv.bot_status_emoji = data.emoji || data.status?.emoji || ''
      conv.bot_status_text = data.text || data.status?.text || ''
    }
  }

  function updateConversation(convId, updates) {
    const conv = findConversation(convId)
    if (conv) Object.assign(conv, updates)
  }

  function markConversationReadLocal(convId) {
    const conv = findConversation(convId)
    if (!conv) {
      return
    }
    conv.unread_count = 0
    conv.is_proactive_message = false
  }

  async function applyIncomingPreview(message, options = {}) {
    const convId = message?.conversation_id
    if (!convId) {
      return
    }
    const conv = findConversation(convId)
    if (!conv) {
      await fetchList()
      return
    }
    const nextUnread = options.incrementUnread
      ? Math.max(0, Number(conv.unread_count || 0) + 1)
      : (options.markRead ? 0 : Number(conv.unread_count || 0))
    Object.assign(conv, {
      last_message: message?.content?.text || conv.last_message || '',
      last_message_at: message?.created_at || conv.last_message_at,
      unread_count: nextUnread,
      is_proactive_message: Boolean(message?.is_proactive),
    })
  }

  return {
    list,
    isLoading,
    fetchList,
    findConversation,
    handleBotStatus,
    updateConversation,
    markConversationReadLocal,
    applyIncomingPreview,
  }
})
