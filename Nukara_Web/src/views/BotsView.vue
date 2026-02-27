<script setup>
import { onMounted } from 'vue'
import { useBotsStore } from '../stores/bots'

const bots = useBotsStore()

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
        <span class="bot-avatar">🤖</span>
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
.bots-page { flex: 1; display: flex; flex-direction: column; background: #fff; }
.page-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 16px; border-bottom: 0.5px solid #e5e5e5;
}
.page-header h2 { font-size: 20px; }
.add-btn {
  color: #007aff; text-decoration: none; font-size: 15px; font-weight: 500;
}
.bot-list { flex: 1; overflow-y: auto; }
.empty {
  text-align: center; padding: 60px 20px; color: #999; font-size: 15px;
}
.link { color: #007aff; text-decoration: none; margin-top: 12px; display: inline-block; }
.bot-card {
  display: flex; align-items: center; gap: 12px;
  padding: 14px 16px; border-bottom: 0.5px solid #f0f0f0;
  text-decoration: none; color: inherit;
}
.bot-avatar { font-size: 28px; }
.bot-info { flex: 1; display: flex; flex-direction: column; gap: 2px; }
.bot-name { font-size: 16px; font-weight: 500; }
.bot-desc { font-size: 13px; color: #999; }
.arrow { color: #ccc; font-size: 20px; }
</style>
