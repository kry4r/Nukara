import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useChatStore } from './chat'
import { useWebSocket } from '../composables/useWebSocket'

export const useRealtimeStore = defineStore('realtime', () => {
  const ws = useWebSocket()
  const chat = useChatStore()
  const isBootstrapped = ref(false)
  const currentUrl = ref('')

  function bootstrapHandlers() {
    if (isBootstrapped.value) {
      return
    }
    ws.on('ack', chat.handleAck)
    ws.on('typing', chat.handleTyping)
    ws.on('stream_start', chat.handleStreamStart)
    ws.on('stream_chunk', chat.handleStreamChunk)
    ws.on('stream_end', chat.handleStreamEnd)
    ws.on('multi_reply_start', chat.handleMultiReplyStart)
    ws.on('message', chat.handleMessage)
    ws.on('multi_reply_end', chat.handleMultiReplyEnd)
    ws.on('bot_status_update', chat.handleBotStatusUpdate)
    ws.on('proactive_message', chat.handleProactiveMessage)
    ws.on('error', chat.handleError)
    ws.on('connection_error', chat.handleError)
    ws.on('bot_persona_updated', chat.handleBotPersonaUpdated)
    chat.setWsSend(ws.send)
    isBootstrapped.value = true
  }

  function connectWithToken(token) {
    const safeToken = String(token || '').trim()
    if (!safeToken) {
      disconnect()
      return
    }
    bootstrapHandlers()
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const nextUrl = `${protocol}//${location.host}/ws/chat?token=${safeToken}`
    if (currentUrl.value === nextUrl && ws.isConnected.value) {
      return
    }
    currentUrl.value = nextUrl
    ws.connect(nextUrl)
  }

  function disconnect() {
    currentUrl.value = ''
    ws.disconnect()
    chat.setWsSend(null)
  }

  return {
    isConnected: ws.isConnected,
    reconnectAttempts: ws.reconnectAttempts,
    connectWithToken,
    disconnect,
  }
})
