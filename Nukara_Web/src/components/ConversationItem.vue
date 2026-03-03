<script setup>
import { computed } from 'vue'

const props = defineProps({
  conv: { type: Object, required: true },
})

const avatarText = computed(() => {
  const name = String(props.conv.bot_name || '').trim()
  if (!name) return 'AI'
  return Array.from(name)[0].toUpperCase()
})

</script>

<template>
  <router-link :to="`/chat/${conv.id}`" class="conv-item" :aria-label="`打开会话 ${conv.bot_name || '未命名 Bot'}`">
    <div class="conv-avatar">
      {{ avatarText }}
    </div>
    <div class="conv-body">
      <div class="conv-top">
        <span class="conv-name">{{ conv.bot_name }}</span>
        <span class="conv-time">
          {{ conv.last_message_at ? new Date(conv.last_message_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) : '' }}
        </span>
      </div>
      <div class="conv-bottom">
        <span class="conv-msg" :class="{ proactive: conv.is_proactive_message }">
          {{ conv.last_message || '暂无消息' }}
        </span>
        <span v-if="conv.unread_count" class="conv-badge">
          {{ conv.unread_count > 99 ? '99+' : conv.unread_count }}
        </span>
      </div>
    </div>
  </router-link>
</template>

<style scoped>
.conv-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  text-decoration: none;
  color: inherit;
  background: linear-gradient(145deg, #ffffff 0%, #fbfdf7 100%);
  border: 1px solid #dce6cd;
  border-radius: var(--radius-lg);
  transition: all var(--transition-base);
  box-shadow: 0 5px 14px rgba(102, 126, 74, 0.08);
}

.conv-item:hover {
  box-shadow: 0 8px 18px rgba(102, 126, 74, 0.14);
  transform: translateY(-2px);
}

.conv-item:active {
  background: #f2f7e8;
}

.conv-item:focus-visible {
  outline: 2px solid #5f8247;
  outline-offset: 2px;
}

.conv-avatar {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: radial-gradient(circle at 30% 20%, #f5f8ef 0%, #e6efdc 100%);
  border: 1px solid #d0dcc2;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 700;
  color: var(--accent-dark);
  flex-shrink: 0;
}

.conv-body {
  flex: 1;
  min-width: 0;
}

.conv-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.conv-name {
  font-size: var(--font-size-base);
  font-weight: 700;
  color: var(--text-primary);
}

.conv-time {
  font-size: var(--font-size-xs);
  color: var(--text-muted);
}

.conv-bottom {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.conv-msg {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.conv-msg.proactive {
  color: var(--accent-primary);
}

.conv-badge {
  background: var(--status-negative);
  color: var(--text-on-accent);
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 10px;
  margin-left: var(--spacing-sm);
  flex-shrink: 0;
  font-weight: 600;
}
</style>
