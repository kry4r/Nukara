import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useApi } from '../composables/useApi'
import { resolveMessageTimestamp } from '../utils/time'
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
  const errorBanner = ref('')
  const activeReplyGroups = ref({})
  const streamDraftByReply = ref({})
  const personaUpdate = ref({ summary: '', timestamp: 0 })

  let wsSend = null

  function setWsSend(fn) {
    wsSend = typeof fn === 'function' ? fn : null
  }

  function sanitizeDisplayText(input) {
    if (typeof input !== 'string') return ''
    let text = input
      .replace(/<think>[\s\S]*?<\/think>/gi, '')
      .replace(/```(?:thinking|analysis)[\s\S]*?```/gi, '')
      .replace(/^\s*(?:thinking|analysis|思考|推理)[:：].*$/gim, '')
      .replaceAll('<<<MSG>>>', '')
    while (text.includes('\n\n\n')) {
      text = text.replaceAll('\n\n\n', '\n\n')
    }
    return text.trim()
  }

  function sanitizeIncomingMessage(raw) {
    const content = raw?.content || {}
    const text = sanitizeDisplayText(content.text || '')
    const createdAt = resolveMessageTimestamp(raw, Date.now())
    return {
      ...raw,
      created_at: createdAt,
      content: {
        ...content,
        text,
      },
    }
  }

  function isActiveConversation(convId) {
    return Boolean(convId) && convId === conversationId.value
  }

  async function markConversationRead(targetConversationId = conversationId.value) {
    const convId = String(targetConversationId || '').trim()
    if (!convId) {
      return
    }
    const convStore = useConversationsStore()
    convStore.markConversationReadLocal(convId)
    try {
      await api.post(`/api/v1/conversations/${convId}/mark-read`)
    } catch (_) {}
  }

  async function loadMessages(convId) {
    conversationId.value = convId
    isLoading.value = true
    errorBanner.value = ''
    try {
      const data = await api.get(
        `/api/v1/conversations/${convId}/messages?limit=50`
      )
      messages.value = Array.isArray(data) ? data.map(sanitizeIncomingMessage) : []
      await markConversationRead(convId)
    } catch (_) {}
    isLoading.value = false
  }

  async function sendMessage(text) {
    if (!text.trim() || !conversationId.value) return
    errorBanner.value = ''
    const clientMsgId = 'c_' + Date.now() + '_' + Math.random().toString(36).slice(2, 8)

    const draft = {
      id: clientMsgId,
      conversation_id: conversationId.value,
      sender_type: 'user',
      content: { type: 'text', text },
      created_at: new Date().toISOString(),
      status: 'sending',
    }
    messages.value.push(draft)

    const sent = wsSend && wsSend({
      type: 'message',
      conversation_id: conversationId.value,
      client_msg_id: clientMsgId,
      content: { type: 'text', text },
    })

    if (!sent) {
      try {
        const data = await api.post(
          `/api/v1/conversations/${conversationId.value}/send`,
          { client_msg_id: clientMsgId, content: { type: 'text', text } }
        )
        if (data.ack) handleAck(data.ack)
        if (data.bot_message) handleMessage(data.bot_message)
        if (data.bot_status_update) handleBotStatusUpdate(data.bot_status_update)
      } catch (error) {
        const msg = messages.value.find((m) => m.id === clientMsgId)
        if (msg) msg.status = 'failed'
        errorBanner.value = error?.message || '消息发送失败，请稍后重试。'
      }
    }

    const convStore = useConversationsStore()
    convStore.updateConversation(conversationId.value, {
      last_message: text,
      last_message_at: draft.created_at,
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
      (m) => m.id === data.client_msg_id || m.id === data.client_message_id
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

  function handleStreamStart(data) {
    if (data.conversation_id !== conversationId.value) return
    const replyId = data.reply_id
    if (!replyId) return
    isRemoteTyping.value = true
    const draftId = `stream-${replyId}`
    streamDraftByReply.value[replyId] = draftId
    if (!messages.value.some((m) => m.id === draftId)) {
      messages.value.push({
        id: draftId,
        conversation_id: data.conversation_id,
        sender_type: 'bot',
        content: { type: 'text', text: '' },
        created_at: new Date().toISOString(),
        is_streaming: true,
      })
    }
  }

  function handleStreamChunk(data) {
    if (data.conversation_id !== conversationId.value) return
    const replyId = data.reply_id
    const draftId = streamDraftByReply.value[replyId]
    if (!draftId) return
    const draft = messages.value.find((m) => m.id === draftId)
    if (!draft) return
    const delta = sanitizeDisplayText(data.delta || '')
    if (!delta) return
    draft.content = draft.content || { type: 'text', text: '' }
    draft.content.text = (draft.content.text || '') + delta
  }

  function handleStreamEnd(data) {
    if (data.conversation_id !== conversationId.value) return
    isRemoteTyping.value = false
    const replyId = data.reply_id
    const draftId = streamDraftByReply.value[replyId]
    if (!draftId) return
    const draft = messages.value.find((m) => m.id === draftId)
    if (draft) {
      draft.is_streaming = false
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

  function syncConversationPreview(normalized) {
    const targetConversationId = normalized.conversation_id || conversationId.value
    const isBotMessage = normalized.sender_type === 'bot' || !normalized.sender_type
    if (!targetConversationId || !isBotMessage) {
      return
    }
    const convStore = useConversationsStore()
    const isActive = isActiveConversation(targetConversationId)
    void convStore.applyIncomingPreview(
      {
        ...normalized,
        conversation_id: targetConversationId,
      },
      {
        incrementUnread: !isActive,
        markRead: isActive,
      }
    )
    if (isActive) {
      void markConversationRead(targetConversationId)
    }
  }

  function handleMessage(data) {
    const normalized = sanitizeIncomingMessage(data)
    const targetConversationId = normalized.conversation_id || conversationId.value
    const isBotMessage = normalized.sender_type === 'bot' || !normalized.sender_type
    if (isBotMessage && !normalized.content?.text) {
      return
    }

    syncConversationPreview({
      ...normalized,
      conversation_id: targetConversationId,
    })

    if (!isActiveConversation(targetConversationId)) {
      return
    }

    const replyId = normalized.reply_id
    const draftId = replyId ? streamDraftByReply.value[replyId] : ''
    if (draftId) {
      const idx = messages.value.findIndex((m) => m.id === draftId)
      if (idx >= 0) {
        messages.value.splice(idx, 1)
      }
      delete streamDraftByReply.value[replyId]
    }

    const msgId = normalized.msg_id || normalized.id
    if (messages.value.some((m) => m.id === msgId)) return

    messages.value.push({
      id: msgId,
      conversation_id: targetConversationId,
      sender_type: normalized.sender_type || 'bot',
      content: normalized.content,
      emotion_tag: normalized.emotion_tag,
      is_proactive: normalized.is_proactive,
      reply_group_id: normalized.reply_group_id,
      sequence: normalized.sequence,
      created_at: normalized.created_at,
    })

    if (normalized.reply_group_id && activeReplyGroups.value[normalized.reply_group_id]) {
      activeReplyGroups.value[normalized.reply_group_id].received++
    }
  }

  function handleMultiReplyEnd(data) {
    if (!isActiveConversation(data.conversation_id)) {
      return
    }
    isRemoteTyping.value = false
    delete activeReplyGroups.value[data.reply_group_id]
    void markConversationRead(data.conversation_id)
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
  }

  function handleError(data) {
    const message = data?.message || '对话服务暂时不可用，请稍后重试。'
    if (message === 'session invalidated' || message === 'Unauthorized') {
      localStorage.removeItem('nukara_token')
      localStorage.removeItem('nukara_user')
      window.location.href = '/auth'
      return
    }
    errorBanner.value = message
    isRemoteTyping.value = false
  }

  function handleBotPersonaUpdated(data) {
    personaUpdate.value = {
      summary: data.summary || '人设有更新',
      timestamp: data.timestamp || Math.floor(Date.now() / 1000),
    }
  }

  function clear() {
    conversationId.value = ''
    messages.value = []
    botName.value = ''
    botStatus.value = { emoji: '', text: '' }
    isRemoteTyping.value = false
    errorBanner.value = ''
    activeReplyGroups.value = {}
    streamDraftByReply.value = {}
    personaUpdate.value = { summary: '', timestamp: 0 }
  }

  return {
    conversationId,
    botName,
    botStatus,
    messages,
    inputText,
    isRemoteTyping,
    isLoading,
    errorBanner,
    activeReplyGroups,
    personaUpdate,
    setWsSend,
    loadMessages,
    sendMessage,
    sendTyping,
    clear,
    markConversationRead,
    handleAck,
    handleTyping,
    handleStreamStart,
    handleStreamChunk,
    handleStreamEnd,
    handleMultiReplyStart,
    handleMessage,
    handleMultiReplyEnd,
    handleBotStatusUpdate,
    handleProactiveMessage,
    handleError,
    handleBotPersonaUpdated,
  }
})
