<script setup>
defineProps({
  msg: { type: Object, required: true },
})

function formatTime(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div :class="['bubble-row', msg.sender_type === 'user' ? 'right' : 'left']" role="listitem">
    <div :class="['bubble', msg.sender_type === 'user' ? 'user' : 'bot']">
      <span v-if="msg.is_proactive" class="proactive-tag">主动消息</span>
      <p class="bubble-text">{{ msg.content?.text || '' }}</p>
      <span class="bubble-time">{{ formatTime(msg.created_at) }}</span>
    </div>
  </div>
</template>

<style scoped>
.bubble-row {
  display: flex;
  margin: var(--spacing-sm) var(--spacing-lg);
  animation: slideUp var(--transition-base);
}

.bubble-row.right {
  justify-content: flex-end;
}

.bubble-row.left {
  justify-content: flex-start;
}

.bubble {
  max-width: 75%;
  padding: 10px 14px 9px;
  border-radius: var(--radius-lg);
  word-break: break-word;
  position: relative;
  transition: all var(--transition-base);
  box-shadow: 0 8px 20px rgba(101, 120, 81, 0.09);
}

.bubble.user {
  background: linear-gradient(150deg, #81a864 0%, #5f7f46 100%);
  color: var(--text-on-accent);
  border-bottom-right-radius: 4px;
}

.bubble.bot {
  background: linear-gradient(145deg, #ffffff 0%, #fffcf2 100%);
  color: var(--text-primary);
  border-bottom-left-radius: 4px;
  border: 1px solid #e2e9d4;
}

.bubble-text {
  margin: 0;
  font-size: var(--font-size-base);
  line-height: var(--line-height-relaxed);
}

.bubble-time {
  display: block;
  font-size: var(--font-size-xs);
  margin-top: 4px;
  opacity: 0.7;
  text-align: right;
}

.proactive-tag {
  display: inline-block;
  font-size: var(--font-size-xs);
  background: var(--accent-light);
  color: var(--accent-primary);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  margin-bottom: 4px;
  font-weight: 600;
}
</style>
