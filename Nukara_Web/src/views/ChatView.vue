<script setup>
import { ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useChatStore } from '../stores/chat'
import { useConversationsStore } from '../stores/conversations'
import { useRealtimeStore } from '../stores/realtime'
import MessageBubble from '../components/MessageBubble.vue'
import MessageInput from '../components/MessageInput.vue'
import TypingIndicator from '../components/TypingIndicator.vue'
import BotStatusBadge from '../components/BotStatusBadge.vue'

const route = useRoute()
const router = useRouter()
const chat = useChatStore()
const convStore = useConversationsStore()
const realtime = useRealtimeStore()

const listEl = ref(null)
const convId = route.params.convId
const nearBottomThreshold = 120

const conv = convStore.list.find((item) => item.id === convId)
if (conv) {
  chat.botName = conv.bot_name || conv.name || 'Bot'
}

onMounted(async () => {
  await chat.loadMessages(convId)
  scrollToBottom(true)
})

onUnmounted(() => {
  chat.sendTyping(false)
  chat.clear()
})

watch(() => chat.messages.length, () => {
  const shouldStickBottom = isNearBottom()
  nextTick(() => scrollToBottom(shouldStickBottom))
})

watch(() => chat.isRemoteTyping, (typing) => {
  if (!typing) return
  const shouldStickBottom = isNearBottom()
  nextTick(() => scrollToBottom(shouldStickBottom))
})

function isNearBottom() {
  if (!listEl.value) return true
  const el = listEl.value
  const remain = el.scrollHeight - el.scrollTop - el.clientHeight
  return remain <= nearBottomThreshold
}

function scrollToBottom(force = false) {
  if (listEl.value) {
    if (!force && !isNearBottom()) return
    listEl.value.scrollTop = listEl.value.scrollHeight
  }
}

function handleSend(text) {
  chat.sendMessage(text)
}

function handleTyping(isTyping) {
  chat.sendTyping(isTyping)
}

function goBack() {
  router.push('/')
}
</script>

<template>
  <div class="chat-page">
    <header class="chat-header">
      <button class="back-btn" aria-label="返回会话列表" @click="goBack">←</button>
      <div class="header-info">
        <span class="bot-name">{{ chat.botName }}</span>
        <BotStatusBadge :emoji="chat.botStatus.emoji" :text="chat.botStatus.text" />
      </div>
      <span class="ws-dot" :class="{ online: realtime.isConnected }" aria-hidden="true"></span>
    </header>

    <main ref="listEl" class="message-list" aria-label="聊天消息列表">
      <div v-if="chat.isLoading" class="center-hint">加载中...</div>
      <div v-if="chat.personaUpdate.summary" class="persona-banner">
        {{ chat.personaUpdate.summary }}
      </div>
      <div v-if="chat.errorBanner" class="error-banner">
        {{ chat.errorBanner }}
      </div>
      <MessageBubble v-for="msg in chat.messages" :key="msg.id" :msg="msg" />
      <div v-if="chat.isRemoteTyping" class="typing-row">
        <TypingIndicator />
      </div>
    </main>

    <MessageInput @send="handleSend" @typing="handleTyping" />
  </div>
</template>

<style scoped>
.chat-page {
  flex: 1;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: radial-gradient(circle at 20% -20%, #ffffff 0%, #f6f8f0 45%, #eef3e5 100%);
}

.chat-header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  background: rgba(255, 255, 255, 0.85);
  border-bottom: 1px solid #dbe3ca;
  backdrop-filter: blur(8px);
  padding-top: calc(var(--spacing-md) + env(safe-area-inset-top, 0));
}

.back-btn {
  border: 1px solid #d7dfc8;
  background: #ffffff;
  border-radius: 9999px;
  font-size: 18px;
  width: 36px;
  height: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #4d6640;
  transition: transform var(--transition-base), box-shadow var(--transition-base);
}

.back-btn:hover {
  transform: translateX(-1px);
  box-shadow: 0 4px 10px rgba(93, 120, 65, 0.18);
}

.back-btn:focus-visible {
  outline: 2px solid #5a7d42;
  outline-offset: 2px;
}

.header-info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.bot-name {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-primary);
}

.ws-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--text-muted);
  flex-shrink: 0;
  box-shadow: 0 0 0 5px rgba(149, 164, 126, 0.18);
}

.ws-dot.online {
  background: var(--status-positive);
}

.message-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: var(--spacing-lg) 0;
}

.center-hint {
  text-align: center;
  padding: var(--spacing-3xl) var(--spacing-xl);
  color: var(--text-muted);
  font-size: var(--font-size-sm);
}

.typing-row {
  padding: var(--spacing-xs) var(--spacing-lg);
}

.persona-banner {
  margin: 0 var(--spacing-lg) var(--spacing-sm);
  padding: 8px 12px;
  border-radius: 12px;
  font-size: 12px;
  color: #3d5131;
  background: rgba(143, 170, 116, 0.18);
}

.error-banner {
  margin: 0 var(--spacing-lg) var(--spacing-sm);
  padding: 10px 12px;
  border-radius: 12px;
  font-size: 13px;
  color: #8a2b2b;
  background: rgba(255, 107, 107, 0.14);
  border: 1px solid rgba(255, 107, 107, 0.22);
}
</style>
