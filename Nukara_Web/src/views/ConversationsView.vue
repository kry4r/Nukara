<script setup>
import { onMounted } from 'vue'
import { useConversationsStore } from '../stores/conversations'
import { useAuthStore } from '../stores/auth'
import ConversationItem from '../components/ConversationItem.vue'

const convStore = useConversationsStore()
const auth = useAuthStore()

onMounted(() => {
  auth.restoreSession()
  convStore.fetchList()
})
</script>

<template>
  <div class="conv-page">
    <header class="page-header">
      <h2>消息</h2>
    </header>
    <div class="conv-list">
      <div v-if="convStore.isLoading" class="empty">加载中...</div>
      <div v-else-if="!convStore.list.length" class="empty">
        <p>暂无会话</p>
        <router-link to="/bots" class="link">去创建一个 Bot</router-link>
      </div>
      <ConversationItem
        v-for="conv in convStore.list"
        :key="conv.id"
        :conv="conv"
      />
    </div>
  </div>
</template>

<style scoped>
.conv-page { flex: 1; display: flex; flex-direction: column; background: #fff; }
.page-header {
  padding: 16px;
  border-bottom: 0.5px solid #e5e5e5;
}
.page-header h2 { font-size: 20px; }
.conv-list { flex: 1; overflow-y: auto; }
.empty {
  text-align: center; padding: 60px 20px;
  color: #999; font-size: 15px;
}
.link { color: #007aff; text-decoration: none; margin-top: 12px; display: inline-block; }
</style>
