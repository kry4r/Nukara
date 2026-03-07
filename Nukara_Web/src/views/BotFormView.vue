<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useBotsStore } from '../stores/bots'

const route = useRoute()
const router = useRouter()
const bots = useBotsStore()

const isEdit = !!route.params.id
const form = ref({
  name: '',
  identity: '',
  personalityText: '',
  expression_style: '',
  life_context: '',
  taboos_and_preferences: '',
})
const saving = ref(false)

function joinPersonality(personality) {
  return Array.isArray(personality) ? personality.join('、') : ''
}

function splitPersonality(value) {
  return String(value || '')
    .split(/[、，,\n]/)
    .map(item => item.trim())
    .filter(Boolean)
}

function buildPayload() {
  return {
    name: form.value.name.trim(),
    identity: form.value.identity.trim(),
    personality: splitPersonality(form.value.personalityText),
    expression_style: form.value.expression_style.trim(),
    life_context: form.value.life_context.trim(),
    taboos_and_preferences: form.value.taboos_and_preferences.trim(),
  }
}

onMounted(async () => {
  if (!isEdit) return
  try {
    const bot = await bots.getBot(route.params.id)
    form.value.name = bot.name || ''
    form.value.identity = bot.identity || ''
    form.value.personalityText = joinPersonality(bot.personality)
    form.value.expression_style = bot.expression_style || ''
    form.value.life_context = bot.life_context || ''
    form.value.taboos_and_preferences = bot.taboos_and_preferences || ''
  } catch (_) {}
})

async function handleSubmit() {
  if (!form.value.name.trim()) return
  saving.value = true
  try {
    const payload = buildPayload()
    if (isEdit) {
      await bots.updateBot(route.params.id, payload)
    } else {
      const bot = await bots.createBot(payload)
      if (!bot) {
        saving.value = false
        return
      }
    }
    router.push('/bots')
  } catch (_) {}
  saving.value = false
}
</script>

<template>
  <div class="form-page">
    <header class="form-header">
      <button class="back-btn" @click="router.push('/bots')">←</button>
      <h2>{{ isEdit ? '编辑 Bot' : '创建 Bot' }}</h2>
    </header>

    <form class="form-body" @submit.prevent="handleSubmit">
      <label class="field">
        <span class="label">名称</span>
        <input v-model="form.name" type="text" placeholder="给 Bot 起个名字" />
      </label>
      <label class="field">
        <span class="label">身份设定</span>
        <textarea v-model="form.identity" rows="3" placeholder="例如：你的恋人，也是会认真接住你情绪的人"></textarea>
      </label>
      <label class="field">
        <span class="label">性格特征</span>
        <textarea v-model="form.personalityText" rows="2" placeholder="用顿号、逗号或换行分隔，例如：细腻、敏锐、会观察"></textarea>
      </label>
      <label class="field">
        <span class="label">表达风格</span>
        <textarea v-model="form.expression_style" rows="2" placeholder="例如：口语化，短句，会接梗"></textarea>
      </label>
      <label class="field">
        <span class="label">生活环境</span>
        <textarea v-model="form.life_context" rows="3" placeholder="例如：现在住在东京，平时摄影、通勤、喝便利店咖啡"></textarea>
      </label>
      <label class="field">
        <span class="label">禁忌与偏好</span>
        <textarea v-model="form.taboos_and_preferences" rows="3" placeholder="例如：不喜欢被命令式对待，更喜欢被温柔回应"></textarea>
      </label>

      <div v-if="bots.error" class="error">{{ bots.error }}</div>

      <button type="submit" class="submit-btn" :disabled="saving || !form.name.trim()">
        {{ saving ? '保存中...' : (isEdit ? '保存' : '创建') }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.form-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #f8faef 0%, #f2f6ea 68%, #edf2e2 100%);
}

.form-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px;
  border-bottom: 1px solid var(--border-default);
  background: rgba(255, 255, 255, 0.78);
}

.back-btn {
  background: none;
  border: none;
  font-size: 20px;
  padding: 4px 8px;
  cursor: pointer;
}

.form-header h2 {
  font-size: 18px;
}

.form-body {
  padding: 20px 16px calc(20px + 76px);
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.label {
  font-size: 14px;
  color: #61724e;
  font-weight: 600;
}

.field input,
.field textarea {
  padding: 12px 14px;
  border: 1px solid #dbe5cb;
  border-radius: 14px;
  font-size: 15px;
  outline: none;
  font-family: inherit;
  resize: none;
  background: #ffffffd9;
  color: var(--text-primary);
}

.field input:focus,
.field textarea:focus {
  border-color: #7ba05b;
}

.error {
  color: #ff3b30;
  font-size: 13px;
  text-align: center;
}

.submit-btn {
  margin-top: 8px;
  padding: 14px;
  background: #7ba05b;
  color: #fff;
  border: none;
  border-radius: 14px;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
}

.submit-btn:disabled {
  background: #b7c4aa;
}
</style>
