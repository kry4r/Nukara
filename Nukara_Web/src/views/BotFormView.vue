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
  description: '',
  background: '',
  speaking_style: '',
})
const saving = ref(false)

onMounted(async () => {
  if (isEdit) {
    try {
      const bot = await bots.getBot(route.params.id)
      form.value.name = bot.name || ''
      form.value.description = bot.description || bot.summary || ''
      form.value.background = bot.background || ''
      form.value.speaking_style = bot.speaking_style || ''
    } catch (_) {}
  }
})

async function handleSubmit() {
  if (!form.value.name.trim()) return
  saving.value = true
  try {
    if (isEdit) {
      await bots.updateBot(route.params.id, form.value)
    } else {
      const bot = await bots.createBot(form.value)
      if (!bot) { saving.value = false; return }
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
        <span class="label">描述</span>
        <textarea v-model="form.description" rows="2" placeholder="简单描述一下 Bot"></textarea>
      </label>
      <label class="field">
        <span class="label">背景</span>
        <textarea v-model="form.background" rows="2" placeholder="角色的背景故事..."></textarea>
      </label>
      <label class="field">
        <span class="label">说话风格</span>
        <textarea v-model="form.speaking_style" rows="2" placeholder="口语化、文艺、幽默..."></textarea>
      </label>

      <div v-if="bots.error" class="error">{{ bots.error }}</div>

      <button type="submit" class="submit-btn" :disabled="saving || !form.name.trim()">
        {{ saving ? '保存中...' : (isEdit ? '保存' : '创建') }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.form-page { flex: 1; display: flex; flex-direction: column; background: #fff; }
.form-header {
  display: flex; align-items: center; gap: 10px;
  padding: 16px; border-bottom: 0.5px solid #e5e5e5;
}
.back-btn {
  background: none; border: none; font-size: 20px;
  padding: 4px 8px; cursor: pointer;
}
.form-header h2 { font-size: 18px; }
.form-body { padding: 20px 16px; display: flex; flex-direction: column; gap: 16px; }
.field { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 14px; color: #666; font-weight: 500; }
.field input, .field textarea {
  padding: 10px 14px; border: 1px solid #e5e5e5;
  border-radius: 10px; font-size: 15px; outline: none;
  font-family: inherit; resize: none;
}
.field input:focus, .field textarea:focus { border-color: #007aff; }
.error { color: #ff3b30; font-size: 13px; text-align: center; }
.submit-btn {
  margin-top: 8px; padding: 12px; background: #007aff;
  color: #fff; border: none; border-radius: 12px;
  font-size: 16px; font-weight: 500; cursor: pointer;
}
.submit-btn:disabled { background: #ccc; }
</style>
