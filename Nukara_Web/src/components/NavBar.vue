<script setup>
import { useRoute } from 'vue-router'

const route = useRoute()
const tabs = [
  { path: '/', label: '消息', icon: 'message-circle' },
  { path: '/bots', label: '通讯录', icon: 'users' },
  { path: '/settings', label: '我的', icon: 'user' },
]

const iconPaths = {
  'message-circle': 'M21 11.5a8.5 8.5 0 0 1-8.5 8.5H8l-5 3 1.5-4.5A8.5 8.5 0 1 1 21 11.5Z',
  users: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2m20 0v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75M12 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z',
  user: 'M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2m12-14a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z',
}

function isActive(path) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <nav class="nav-shell" aria-label="主导航">
    <div class="nav-pill">
      <router-link
        v-for="tab in tabs"
        :key="tab.path"
        :to="tab.path"
        :class="['nav-item', { active: isActive(tab.path) }]"
        :aria-label="tab.label"
      >
        <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path :d="iconPaths[tab.icon]" />
        </svg>
        <span class="nav-label">{{ tab.label }}</span>
      </router-link>
    </div>
  </nav>
</template>

<style scoped>
.nav-shell {
  padding: 12px 21px calc(21px + env(safe-area-inset-bottom, 0));
  background: transparent;
}

.nav-pill {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: center;
  gap: 0;
  height: 62px;
  padding: 4px;
  border-radius: 36px;
  border: 1px solid #e5e8de;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(80, 101, 58, 0.12);
}

.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  height: 100%;
  border-radius: 26px;
  padding: 0;
  text-decoration: none;
  color: #9a9a9a;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.nav-item.active {
  color: #ffffff;
  background: #7ba05b;
}

.nav-item:focus-visible {
  outline: 2px solid #7ba05b;
  outline-offset: 2px;
}

.nav-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.nav-label {
  font-size: 10px;
  line-height: 1;
  font-weight: 500;
}

@media (max-width: 640px) {
  .nav-shell {
    padding-left: 16px;
    padding-right: 16px;
  }
}
</style>
