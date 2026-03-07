<script setup>
import { ref, onBeforeUnmount } from 'vue'

const emit = defineEmits(['send', 'typing'])
const text = ref('')
const isTyping = ref(false)
let typingStopTimer = null

function handleSend() {
  if (!text.value.trim()) return
  emit('send', text.value.trim())
  text.value = ''
  emitTyping(false)
}

function emitTyping(next) {
  if (isTyping.value === next) return
  isTyping.value = next
  emit('typing', next)
}

function scheduleTypingStop() {
  if (typingStopTimer) clearTimeout(typingStopTimer)
  typingStopTimer = setTimeout(() => {
    emitTyping(false)
    typingStopTimer = null
  }, 900)
}

function handleInput() {
  if (text.value.trim()) {
    emitTyping(true)
    scheduleTypingStop()
    return
  }
  if (typingStopTimer) {
    clearTimeout(typingStopTimer)
    typingStopTimer = null
  }
  emitTyping(false)
}

onBeforeUnmount(() => {
  if (typingStopTimer) clearTimeout(typingStopTimer)
  emitTyping(false)
})
</script>

<template>
  <div class="input-bar">
    <button type="button" class="attach-btn" aria-label="添加附件">+</button>
    <div class="input-shell">
      <input
        v-model="text"
        type="text"
        placeholder="输入消息..."
        aria-label="消息输入框"
        @input="handleInput"
        @keydown.enter="handleSend"
      />
      <button type="button" class="send-btn" :disabled="!text.trim()" aria-label="发送消息" @click="handleSend">发送</button>
    </div>
  </div>
</template>

<style scoped>
.input-bar {
  position: sticky;
  bottom: 0;
  z-index: 3;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: 10px var(--spacing-lg);
  border-top: 1px solid #dce6cb;
  background: rgba(255, 255, 255, 0.94);
  backdrop-filter: blur(10px);
  box-shadow: 0 -10px 24px rgba(69, 91, 48, 0.08);
  padding-bottom: calc(10px + env(safe-area-inset-bottom, 0));
}

.attach-btn {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  border: 1px solid #d5dec5;
  background: #ffffff;
  color: #597946;
  font-size: 21px;
  line-height: 1;
}

.attach-btn:focus-visible,
.send-btn:focus-visible {
  outline: 2px solid #5d8046;
  outline-offset: 2px;
}

.input-bar input:focus-visible {
  outline: none;
}

.input-shell {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid #d9e2cb;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.82);
  backdrop-filter: blur(4px);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.85);
  padding: 4px 6px 4px 14px;
}

.input-shell:focus-within {
  border-color: #d9e2cb;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.85);
}

.input-bar input {
  flex: 1;
  min-width: 0;
  padding: 8px 0;
  border: 0;
  font-size: 16px;
  color: var(--text-primary);
  background: transparent;
  outline: none;
}

.input-bar input::placeholder {
  color: #8c9b7d;
}

.send-btn {
  min-width: 58px;
  padding: 7px 14px;
  background: linear-gradient(145deg, #7fa860 0%, #628347 100%);
  color: #fff;
  border: 0;
  border-radius: 9999px;
  font-size: 14px;
  font-weight: 600;
}

.send-btn:disabled {
  background: #b9c7aa;
}
</style>
