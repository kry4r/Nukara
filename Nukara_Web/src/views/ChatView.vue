<script setup>
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useChatStore } from '../stores/chat'
import { useConversationsStore } from '../stores/conversations'
import { useWebSocket } from '../composables/useWebSocket'
import MessageBubble from '../components/MessageBubble.vue'
import MessageInput from '../components/MessageInput.vue'
import TypingIndicator from '../components/TypingIndicator.vue'
import BotStatusBadge from '../components/BotStatusBadge.vue'

const route = useRoute()
const router = useRouter()
const chat = useChatStore()
const convStore = useConversationsStore()
const ws = useWebSocket()

const listEl = ref(null)
const convId = route.params.convId

// Find conversation info
const conv = convStore.list.find(c => c.id === convId)
if (conv) {
  chat.botName = conv.bot_name || conv.name || 'Bot'
}

// Wire WS events to chat store handlers
ws.on('ack', chat.handleAck)
ws.on('typing', chat.handleTyping)
ws.on('multi_reply_start', chat.handleMultiReplyStart)
ws.on('message', chat.handleMessage)
ws.on('multi_reply_end', chat.handleMultiReplyEnd)
ws.on('bot_status_update', chat.handleBotStatusUpdate)
ws.on('proactive_message', chat.handleProactiveMessage)

chat.setWsSend(ws.send)

onMounted(async () => {
  const token = localStorage.getItem('nukara_token')
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${location.host}/ws/chat?token=${token}&conversation_id=${convId}`
  ws.connect(wsUrl)
  await chat.loadMessages(convId)
  scrollToBottom()
})

onUnmounted(() => {
  ws.disconnect()
  chat.clear()
})

watch(() => chat.messages.length, () => {
  nextTick(scrollToBottom)
})

function scrollToBottom() {
  if (listEl.value) {
    listEl.value.scrollTop = listEl.value.scrollHeight
  }
}

function handleSend(text) {
  chat.sendMessage(text)
}

function goBack() {
  router.push('/')
}
</script>

<template>
  <div class="chat-page">
    <header class="chat-header">
      <button class="back-btn" @click="goBack">←</button>
      <div class="header-info">
        <span class="bot-name">{{ chat.botName }}</span>
        <BotStatusBadge :emoji="chat.botStatus.emoji" :text="chat.botStatus.text" />
      </div>
      <span class="ws-dot" :class="{ online: ws.isConnected.value }"></span>
    </header>

    <div ref="listEl" class="message-list">
      <div v-if="chat.isLoading" class="center-hint">加载中...</div>
      <MessageBubble v-for="msg in chat.messages" :key="msg.id" :msg="msg" />
      <div v-if="chat.isRemoteTyping" class="typing-row">
        <TypingIndicator />
      </div>
    </div>

    <MessageInput @send="handleSend" />
  </div>
</template>

<style scoped>
.chat-page {
  flex: 1; display: flex; flex-direction: column;
  background: #fff; height: 100vh;
}
.chat-header {
  display: flex; align-items: center; gap: 10px;
  padding: 12px 16px;
  border-bottom: 0.5px solid #e5e5e5;
  background: #fff;
  padding-top: calc(12px + env(safe-area-inset-top, 0));
}
.back-btn {
  background: none; border: none; font-size: 20px;
  padding: 4px 8px; cursor: pointer;
}
.header-info { flex: 1; display: flex; align-items: center; gap: 8px; }
.bot-name { font-size: 17px; font-weight: 600; }
.ws-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: #ccc; flex-shrink: 0;
}
.ws-dot.online { background: #34c759; }
.message-list {
  flex: 1; overflow-y: auto; padding: 12px 0;
}
.center-hint {
  text-align: center; padding: 40px 20px;
  color: #999; font-size: 14px;
}
.typing-row { padding: 4px 16px; }
</style>
