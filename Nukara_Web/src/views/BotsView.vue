<script setup>
import { onMounted } from 'vue'
import { useBotsStore } from '../stores/bots'

const bots = useBotsStore()

function avatarText(name) {
  const cleaned = String(name || '').trim()
  if (!cleaned) return 'AI'
  const chars = Array.from(cleaned)
  return chars[0].toUpperCase()
}

onMounted(() => {
  bots.fetchList()
})
</script>

<template>
  <div class="bots-page">
    <header class="page-header">
      <h2>我的 Bot</h2>
      <router-link to="/bots/new" class="add-btn">+ 创建</router-link>
    </header>

    <div class="bot-list">
      <div v-if="bots.isLoading" class="empty">加载中...</div>
      <div v-else-if="!bots.list.length" class="empty">
        <p>还没有 Bot</p>
        <router-link to="/bots/new" class="link">创建第一个</router-link>
      </div>

      <router-link
        v-for="bot in bots.list"
        :key="bot.id"
        :to="`/bots/${bot.id}/edit`"
        class="bot-card"
      >
        <span class="bot-avatar">{{ avatarText(bot.name) }}</span>
        <div class="bot-info">
          <span class="bot-name">{{ bot.name }}</span>
          <span class="bot-desc">{{ bot.description || bot.summary || '暂无描述' }}</span>
        </div>
        <span class="arrow">›</span>
      </router-link>
    </div>
  </div>
</template>

<style scoped>
.bots-page { flex: 1; display: flex; flex-direction: column; background: var(--bg-page); }
.page-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px; border-bottom: 1px solid var(--border-default);
  background: rgba(255, 255, 255, 0.74);
  backdrop-filter: blur(6px);
}
.page-header h2 { font-size: 20px; color: var(--text-primary); }
.add-btn {
  color: var(--accent-primary); text-decoration: none; font-size: 15px; font-weight: 600;
}
.bot-list { flex: 1; overflow-y: auto; }
.empty {
  text-align: center; padding: 60px 20px; color: var(--text-muted); font-size: 15px;
}
.link { color: var(--accent-primary); text-decoration: none; margin-top: 12px; display: inline-block; }
.bot-card {
  display: flex; align-items: center; gap: 12px;
  margin: 10px 12px;
  padding: 14px 16px;
  border: 1px solid #dce6cd;
  border-radius: 14px;
  background: linear-gradient(145deg, #ffffff 0%, #fbfdf7 100%);
  box-shadow: 0 5px 14px rgba(102, 126, 74, 0.08);
  text-decoration: none; color: inherit;
}
.bot-avatar {
  width: 38px;
  height: 38px;
  flex-shrink: 0;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  color: var(--accent-dark);
  background: radial-gradient(circle at 30% 20%, #f5f8ef 0%, #e6efdc 100%);
  border: 1px solid #d0dcc2;
}
.bot-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
.bot-name { font-size: 16px; font-weight: 600; color: var(--text-primary); }
.bot-desc { font-size: 13px; color: var(--text-muted); }
.arrow { color: #b5beaa; font-size: 20px; }
</style>
