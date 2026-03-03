<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useSettingsStore } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import { FREQUENCY_OPTIONS, DEFAULT_STATUSES } from '../utils/constants'

const router = useRouter()
const settings = useSettingsStore()
const auth = useAuthStore()

const proactiveEnabled = ref(true)
const frequency = ref('normal')
const dndStart = ref('23:00')
const dndEnd = ref('08:00')
const selectedStatus = ref('')
const saved = ref(false)

onMounted(async () => {
  await Promise.all([settings.fetchNotifications(), settings.fetchUserStatus()])
  proactiveEnabled.value = settings.notifications.proactive_enabled !== false
  frequency.value = settings.notifications.frequency || 'normal'
  dndStart.value = settings.notifications.dnd_start || '23:00'
  dndEnd.value = settings.notifications.dnd_end || '08:00'
  selectedStatus.value = settings.userStatus.text || ''
})

async function saveSettings() {
  await settings.saveNotifications({
    proactive_enabled: proactiveEnabled.value,
    frequency: frequency.value,
    dnd_start: dndStart.value,
    dnd_end: dndEnd.value,
  })
  saved.value = true
  setTimeout(() => { saved.value = false }, 2000)
}

async function pickStatus(s) {
  selectedStatus.value = s
  const parts = s.split(' ')
  await settings.saveUserStatus({ emoji: parts[0], text: parts.slice(1).join(' ') })
}

function handleLogout() {
  auth.logout()
  router.push('/auth')
}
</script>

<template>
  <div class="settings-page">
    <header class="page-header">
      <p class="eyebrow">Profile</p>
      <h1>我的</h1>
      <p class="subtitle">管理主动消息和你的在线状态</p>
    </header>
    <div class="settings-body">
      <section class="section-card">
        <h3>主动消息</h3>
        <label class="row toggle-row">
          <span>启用主动消息</span>
          <input type="checkbox" v-model="proactiveEnabled" @change="saveSettings" />
        </label>
        <label class="row" v-if="proactiveEnabled">
          <span>频率</span>
          <select v-model="frequency" @change="saveSettings">
            <option v-for="opt in FREQUENCY_OPTIONS" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>
        </label>
      </section>

      <section class="section-card" v-if="proactiveEnabled">
        <h3>免打扰</h3>
        <div class="dnd-row">
          <label>
            <span>开始</span>
            <input type="time" v-model="dndStart" @change="saveSettings" />
          </label>
          <label>
            <span>结束</span>
            <input type="time" v-model="dndEnd" @change="saveSettings" />
          </label>
        </div>
      </section>

      <section class="section-card">
        <h3>我的状态</h3>
        <div class="status-grid">
          <button v-for="s in DEFAULT_STATUSES" :key="s"
            :class="['status-chip', { active: selectedStatus === s }]"
            @click="pickStatus(s)">{{ s }}</button>
        </div>
      </section>

      <div v-if="saved" class="toast">已保存</div>

      <button class="logout-btn" @click="handleLogout">退出登录</button>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
  background: linear-gradient(180deg, #f8faef 0%, #f2f6ea 68%, #edf2e2 100%);
  padding: var(--spacing-xl) var(--spacing-lg) calc(var(--spacing-xl) + 76px);
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

.settings-body {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
  padding-right: 2px;
}

.section-card {
  border: 1px solid #dce6cd;
  border-radius: var(--radius-lg);
  background: linear-gradient(145deg, #ffffff 0%, #fbfdf7 100%);
  box-shadow: 0 6px 16px rgba(102, 126, 74, 0.1);
  padding: var(--spacing-md) var(--spacing-lg);
}

.section-card h3 {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-bottom: 10px;
}

.row {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-default);
}

.row:last-child {
  border-bottom: 0;
}

.row span {
  font-size: var(--font-size-base);
  color: var(--text-primary);
}

.row select, .num-input {
  padding: 6px 10px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  font-size: var(--font-size-sm);
  outline: none;
  background: #fff;
  color: var(--text-primary);
}
.num-input { width: 60px; text-align: center; }

.toggle-row input[type="checkbox"] {
  width: 20px;
  height: 20px;
  accent-color: var(--accent-primary);
}

.dnd-row {
  display: flex; gap: 16px;
}
.dnd-row label { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.dnd-row span { font-size: var(--font-size-xs); color: var(--text-muted); }
.dnd-row input[type="time"] {
  padding: 8px 10px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  font-size: var(--font-size-sm);
  outline: none;
  background: #fff;
  color: var(--text-primary);
}
.status-grid { display: flex; flex-wrap: wrap; gap: 8px; }
.status-chip {
  padding: 6px 12px;
  border: 1px solid var(--border-default);
  border-radius: 16px;
  background: #fff;
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}
.status-chip.active {
  border-color: var(--accent-primary);
  background: rgba(123, 160, 91, 0.14);
  color: var(--accent-dark);
}
.toast {
  text-align: center;
  padding: 8px;
  background: var(--status-positive);
  color: #fff;
  border-radius: 10px;
  font-size: 14px;
  margin: 2px 0 6px;
  box-shadow: 0 8px 16px rgba(77, 120, 84, 0.2);
}
.logout-btn {
  width: 100%;
  padding: 12px;
  background: #fff;
  border: 1px solid #e8b0ac;
  border-radius: 12px;
  color: #a8433b;
  font-size: 15px;
  cursor: pointer;
  margin-top: 8px;
  font-weight: 600;
}
</style>
