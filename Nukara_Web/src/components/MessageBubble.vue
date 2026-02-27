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
  <div :class="['bubble-row', msg.sender_type === 'user' ? 'right' : 'left']">
    <div :class="['bubble', msg.sender_type === 'user' ? 'user' : 'bot']">
      <span v-if="msg.is_proactive" class="proactive-tag">主动消息</span>
      <p class="bubble-text">{{ msg.content?.text || '' }}</p>
      <span class="bubble-time">{{ formatTime(msg.created_at) }}</span>
    </div>
  </div>
</template>

<style scoped>
.bubble-row { display: flex; margin: 4px 16px; }
.bubble-row.right { justify-content: flex-end; }
.bubble-row.left { justify-content: flex-start; }
.bubble {
  max-width: 75%;
  padding: 10px 14px;
  border-radius: 16px;
  word-break: break-word;
  position: relative;
}
.bubble.user {
  background: #007aff;
  color: #fff;
  border-bottom-right-radius: 4px;
}
.bubble.bot {
  background: #f0f0f0;
  color: #333;
  border-bottom-left-radius: 4px;
}
.bubble-text { margin: 0; font-size: 15px; line-height: 1.5; }
.bubble-time {
  display: block;
  font-size: 11px;
  margin-top: 4px;
  opacity: 0.6;
  text-align: right;
}
.proactive-tag {
  display: inline-block;
  font-size: 10px;
  background: rgba(0,122,255,0.15);
  color: #007aff;
  padding: 1px 6px;
  border-radius: 4px;
  margin-bottom: 4px;
}
</style>
