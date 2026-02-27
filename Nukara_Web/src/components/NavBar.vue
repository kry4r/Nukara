<script setup>
import { useRoute } from 'vue-router'

const route = useRoute()
const tabs = [
  { path: '/', label: '消息', icon: '💬' },
  { path: '/bots', label: 'Bot', icon: '🤖' },
  { path: '/settings', label: '设置', icon: '⚙️' },
]
</script>

<template>
  <nav class="nav-bar">
    <router-link
      v-for="tab in tabs"
      :key="tab.path"
      :to="tab.path"
      :class="['nav-item', { active: route.path === tab.path }]"
    >
      <span class="nav-icon">{{ tab.icon }}</span>
      <span class="nav-label">{{ tab.label }}</span>
    </router-link>
  </nav>
</template>

<style scoped>
.nav-bar {
  display: flex;
  border-top: 1px solid #e5e5e5;
  background: #fff;
  padding: 6px 0 env(safe-area-inset-bottom, 0);
}
.nav-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 0;
  text-decoration: none;
  color: #999;
  font-size: 11px;
  transition: color 0.2s;
}
.nav-item.active { color: #007aff; }
.nav-icon { font-size: 22px; margin-bottom: 2px; }
</style>
