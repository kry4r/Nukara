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
    <header class="page-header"><h2>设置</h2></header>

    <div class="settings-body">
      <section class="section">
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

      <section class="section" v-if="proactiveEnabled">
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

      <section class="section">
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
.settings-page { flex: 1; display: flex; flex-direction: column; background: #fff; }
.page-header { padding: 16px; border-bottom: 0.5px solid #e5e5e5; }
.page-header h2 { font-size: 20px; }
.settings-body { flex: 1; overflow-y: auto; padding: 16px; }
.section { margin-bottom: 24px; }
.section h3 { font-size: 15px; color: #666; margin-bottom: 10px; }
.row {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 0; border-bottom: 0.5px solid #f0f0f0;
}
.row span { font-size: 15px; }
.row select, .num-input {
  padding: 6px 10px; border: 1px solid #e5e5e5;
  border-radius: 8px; font-size: 14px; outline: none;
}
.num-input { width: 60px; text-align: center; }
.toggle-row input[type="checkbox"] { width: 20px; height: 20px; }
.dnd-row {
  display: flex; gap: 16px;
}
.dnd-row label { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.dnd-row span { font-size: 13px; color: #999; }
.dnd-row input[type="time"] {
  padding: 8px 10px; border: 1px solid #e5e5e5;
  border-radius: 8px; font-size: 14px; outline: none;
}
.status-grid { display: flex; flex-wrap: wrap; gap: 8px; }
.status-chip {
  padding: 6px 12px; border: 1px solid #e5e5e5;
  border-radius: 16px; background: #fff; font-size: 13px; cursor: pointer;
}
.status-chip.active { border-color: #007aff; background: rgba(0,122,255,0.08); color: #007aff; }
.toast {
  text-align: center; padding: 8px; background: #34c759;
  color: #fff; border-radius: 8px; font-size: 14px; margin: 12px 0;
}
.logout-btn {
  width: 100%; padding: 12px; background: none;
  border: 1px solid #ff3b30; border-radius: 12px;
  color: #ff3b30; font-size: 15px; cursor: pointer; margin-top: 20px;
}
</style>
