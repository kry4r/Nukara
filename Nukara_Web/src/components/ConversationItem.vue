<script setup>
defineProps({
  conv: { type: Object, required: true },
})
</script>

<template>
  <router-link :to="`/chat/${conv.id}`" class="conv-item">
    <div class="conv-avatar">
      {{ conv.bot_status_emoji || '🤖' }}
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
  padding: 14px 16px;
  text-decoration: none;
  color: inherit;
  border-bottom: 0.5px solid #f0f0f0;
  transition: background 0.15s;
}
.conv-item:active { background: #f5f5f5; }
.conv-avatar {
  width: 48px; height: 48px;
  border-radius: 50%;
  background: #f0f0f0;
  display: flex; align-items: center; justify-content: center;
  font-size: 24px;
  margin-right: 12px;
  flex-shrink: 0;
}
.conv-body { flex: 1; min-width: 0; }
.conv-top { display: flex; justify-content: space-between; margin-bottom: 4px; }
.conv-name { font-size: 16px; font-weight: 500; }
.conv-time { font-size: 12px; color: #999; }
.conv-bottom { display: flex; justify-content: space-between; align-items: center; }
.conv-msg {
  font-size: 14px; color: #999;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex: 1;
}
.conv-msg.proactive { color: #007aff; }
.conv-badge {
  background: #ff3b30; color: #fff;
  font-size: 11px; padding: 2px 6px;
  border-radius: 10px; margin-left: 8px;
  flex-shrink: 0;
}
</style>
