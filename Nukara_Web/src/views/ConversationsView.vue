<script setup>
import { computed, onMounted, ref } from 'vue'
import { useConversationsStore } from '../stores/conversations'
import { useAuthStore } from '../stores/auth'
import ConversationItem from '../components/ConversationItem.vue'

const convStore = useConversationsStore()
const auth = useAuthStore()
const keyword = ref('')

const filteredConversations = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  if (!term) return convStore.list
  return convStore.list.filter((conv) => {
    const name = String(conv.bot_name || conv.name || '').toLowerCase()
    const lastMessage = String(conv.last_message || '').toLowerCase()
    return name.includes(term) || lastMessage.includes(term)
  })
})

onMounted(() => {
  auth.restoreSession()
  convStore.fetchList()
})
</script>

<template>
  <div class="conv-page">
    <header class="page-header">
      <p class="eyebrow">Nukara Chat</p>
      <h1>会话</h1>
      <p class="subtitle">选择一个角色继续聊天</p>
    </header>

    <label class="input-bar search-row">
      <span class="search-icon" aria-hidden="true">⌕</span>
      <input
        v-model.trim="keyword"
        type="search"
        class="search-input"
        placeholder="搜索 Bot 或消息"
        aria-label="搜索会话"
      />
    </label>

    <div class="conv-list">
      <div v-if="convStore.isLoading" class="empty">加载中...</div>
      <div v-else-if="!filteredConversations.length" class="empty">
        <p>暂无会话</p>
        <router-link to="/bots" class="link">去创建一个 Bot</router-link>
      </div>
      <ConversationItem
        v-for="conv in filteredConversations"
        :key="conv.id"
        :conv="conv"
      />
    </div>
  </div>
</template>

<style scoped>
.conv-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #f8faef 0%, #f2f6ea 68%, #edf2e2 100%);
  padding: var(--spacing-xl) var(--spacing-lg) calc(var(--spacing-xl) + 76px);
  gap: var(--spacing-lg);
}

.page-header {
  padding-top: calc(env(safe-area-inset-top, 0) + var(--spacing-xs));
  display: grid;
  gap: 4px;
}

.eyebrow {
  font-size: var(--font-size-xs);
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 700;
}

.page-header h1 {
  font-size: var(--font-size-2xl);
  font-weight: 700;
  line-height: var(--line-height-tight);
  color: var(--text-primary);
}

.subtitle {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.input-bar.search-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: 10px 12px;
  border-radius: var(--radius-pill);
  background: #ffffff;
  border: 1px solid #dfe7cf;
  box-shadow: 0 6px 18px rgba(62, 82, 41, 0.08);
}

.search-icon {
  color: var(--text-muted);
  font-size: 14px;
}

.search-input {
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--text-primary);
  font-size: var(--font-size-sm);
  outline: none;
}

.search-input::placeholder {
  color: #a0a88d;
}

.conv-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
  padding-right: 2px;
}

.empty {
  text-align: center;
  padding: var(--spacing-3xl) var(--spacing-xl);
  color: var(--text-muted);
  font-size: var(--font-size-base);
}

.link {
  color: var(--accent-primary);
  text-decoration: none;
  margin-top: var(--spacing-md);
  display: inline-block;
  font-weight: 600;
}

.link:hover {
  text-decoration: underline;
}
</style>
