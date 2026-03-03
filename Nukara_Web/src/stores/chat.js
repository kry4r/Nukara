import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'
import { useConversationsStore } from './conversations'

export const useChatStore = defineStore('chat', () => {
  const api = useApi()

  const conversationId = ref('')
  const botName = ref('')
  const botStatus = ref({ emoji: '', text: '' })
  const messages = ref([])
  const inputText = ref('')
  const isRemoteTyping = ref(false)
  const isLoading = ref(false)
  const activeReplyGroups = ref({})

  let wsSend = null

  function setWsSend(fn) { wsSend = fn }

  function sanitizeDisplayText(input) {
    if (typeof input !== 'string') return ''
    let text = input
      .replace(/<think>[\s\S]*?<\/think>/gi, '')
      .replace(/```(?:thinking|analysis)[\s\S]*?```/gi, '')
      .replace(/^\s*(?:thinking|analysis|思考|推理)[:：].*$/gim, '')
    while (text.includes('\n\n\n')) {
      text = text.replaceAll('\n\n\n', '\n\n')
    }
    return text.trim()
  }

  function sanitizeIncomingMessage(raw) {
    const content = raw?.content || {}
    const text = sanitizeDisplayText(content.text || '')
    return {
      ...raw,
      content: {
        ...content,
        text,
      },
    }
  }

  async function loadMessages(convId) {
    conversationId.value = convId
    isLoading.value = true
    try {
      const data = await api.get(
        `/api/v1/conversations/${convId}/messages?limit=50`
      )
      messages.value = Array.isArray(data) ? data.map(sanitizeIncomingMessage) : []
    } catch (_) {}
    isLoading.value = false
  }

  async function sendMessage(text) {
    if (!text.trim() || !conversationId.value) return
    const clientMsgId = 'c_' + Date.now() + '_' + Math.random().toString(36).slice(2, 8)

    // Optimistic update
    const draft = {
      id: clientMsgId,
      conversation_id: conversationId.value,
      sender_type: 'user',
      content: { type: 'text', text },
      created_at: new Date().toISOString(),
      status: 'sending',
    }
    messages.value.push(draft)

    // Try WebSocket first
    const sent = wsSend && wsSend({
      type: 'message',
      conversation_id: conversationId.value,
      client_msg_id: clientMsgId,
      content: { type: 'text', text },
    })

    // Fallback to HTTP
    if (!sent) {
      try {
        const data = await api.post(
          `/api/v1/conversations/${conversationId.value}/send`,
          { client_msg_id: clientMsgId, content: { type: 'text', text } }
        )
        if (data.ack) handleAck(data.ack)
        if (data.bot_message) handleMessage(data.bot_message)
        if (data.bot_status_update) handleBotStatusUpdate(data.bot_status_update)
      } catch (_) {
        const msg = messages.value.find(m => m.id === clientMsgId)
        if (msg) msg.status = 'failed'
      }
    }

    // Sync conversation list
    const convStore = useConversationsStore()
    convStore.updateConversation(conversationId.value, {
      last_message: text,
      last_message_at: new Date().toISOString(),
    })
  }

  function sendTyping(isTyping) {
    if (!conversationId.value || !wsSend) return false
    return wsSend({
      type: isTyping ? 'typing_start' : 'typing_stop',
      conversation_id: conversationId.value,
    })
  }

  function handleAck(data) {
    const msg = messages.value.find(
      m => m.id === data.client_msg_id || m.id === data.client_message_id
    )
    if (msg) {
      msg.id = data.server_msg_id || data.server_message_id || msg.id
      msg.status = 'sent'
    }
  }

  function handleTyping(data) {
    if (data.conversation_id === conversationId.value) {
      isRemoteTyping.value = data.is_typing !== false
    }
  }

  function handleMultiReplyStart(data) {
    if (data.conversation_id === conversationId.value) {
      isRemoteTyping.value = true
      activeReplyGroups.value[data.reply_group_id] = {
        count: data.count || 0,
        received: 0,
      }
    }
  }

  function handleMessage(data) {
    // Deduplicate
    const msgId = data.msg_id || data.id
    if (messages.value.some(m => m.id === msgId)) return

    const normalized = sanitizeIncomingMessage(data)
    if ((normalized.sender_type === 'bot' || !normalized.sender_type) && !normalized.content?.text) {
      return
    }

    messages.value.push({
      id: msgId,
      conversation_id: normalized.conversation_id || conversationId.value,
      sender_type: normalized.sender_type || 'bot',
      content: normalized.content,
      emotion_tag: normalized.emotion_tag,
      is_proactive: normalized.is_proactive,
      reply_group_id: normalized.reply_group_id,
      sequence: normalized.sequence,
      created_at: normalized.created_at || new Date(normalized.timestamp * 1000).toISOString(),
    })

    // Track reply group progress
    if (normalized.reply_group_id && activeReplyGroups.value[normalized.reply_group_id]) {
      activeReplyGroups.value[normalized.reply_group_id].received++
    }

    // Sync conversation list
    if (normalized.sender_type === 'bot' || !normalized.sender_type) {
      const convStore = useConversationsStore()
      convStore.updateConversation(
        normalized.conversation_id || conversationId.value,
        {
          last_message: normalized.content?.text || '',
          last_message_at: new Date().toISOString(),
        }
      )
    }
  }

  function handleMultiReplyEnd(data) {
    if (data.conversation_id === conversationId.value) {
      isRemoteTyping.value = false
      delete activeReplyGroups.value[data.reply_group_id]
    }
    // Mark read
    api.post(`/api/v1/conversations/${conversationId.value}/mark-read`).catch(() => {})
  }

  function handleBotStatusUpdate(data) {
    if (data.conversation_id === conversationId.value) {
      const s = data.status || data
      botStatus.value = { emoji: s.emoji || '', text: s.text || '' }
    }
    const convStore = useConversationsStore()
    convStore.handleBotStatus(data)
  }

  function handleProactiveMessage(data) {
    handleMessage({
      ...data,
      sender_type: 'bot',
      is_proactive: true,
    })
    const convStore = useConversationsStore()
    convStore.updateConversation(data.conversation_id, {
      last_message: data.content?.text || '',
      last_message_at: new Date().toISOString(),
      is_proactive_message: true,
    })
  }

  function clear() {
    conversationId.value = ''
    messages.value = []
    botName.value = ''
    botStatus.value = { emoji: '', text: '' }
    isRemoteTyping.value = false
    activeReplyGroups.value = {}
  }

  return {
    conversationId, botName, botStatus,
    messages, inputText, isRemoteTyping,
    isLoading, activeReplyGroups,
    setWsSend, loadMessages, sendMessage, sendTyping, clear,
    handleAck, handleTyping,
    handleMultiReplyStart, handleMessage,
    handleMultiReplyEnd, handleBotStatusUpdate,
    handleProactiveMessage,
  }
})
