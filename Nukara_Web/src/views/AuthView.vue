<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { canSendEmailCode } from '../utils/auth-email'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const mode = ref('login')
const email = ref('')
const code = ref('')
const nickname = ref('')
const countdown = ref(0)
const canSendCode = computed(() => canSendEmailCode({
  email: email.value,
  countdown: countdown.value,
  isLoading: auth.isLoading,
}))
let countdownTimer = null

function stopCountdown() {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
}

async function sendEmailCode() {
  if (!canSendCode.value) {
    return
  }
  const purpose = mode.value === 'login' ? 'login' : 'register'
  const ok = await auth.requestEmailCode(email.value, purpose)
  if (!ok) {
    return
  }
  stopCountdown()
  countdown.value = 60
  countdownTimer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      countdown.value = 0
      stopCountdown()
    }
  }, 1000)
}

onBeforeUnmount(() => {
  stopCountdown()
})

async function submit() {
  let ok = false
  if (mode.value === 'login') {
    ok = await auth.login(email.value, code.value)
  } else {
    ok = await auth.register(email.value, code.value, nickname.value)
  }
  if (ok) router.push('/')
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-header">
      <h1>Nukara</h1>
      <p class="subtitle">你的情感陪伴 AI</p>
    </div>

    <div class="auth-tabs">
      <button
        :class="['tab', { active: mode === 'login' }]"
        @click="mode = 'login'"
      >登录</button>
      <button
        :class="['tab', { active: mode === 'register' }]"
        @click="mode = 'register'"
      >注册</button>
    </div>

    <div class="auth-form">
      <div class="field">
        <input
          v-model="email"
          type="email"
          placeholder="邮箱"
        />
      </div>

      <div class="field sms-row">
        <input
          v-model="code"
          type="text"
          placeholder="验证码"
          maxlength="6"
        />
        <button
          class="sms-btn"
          :disabled="!canSendCode"
          @click="sendEmailCode"
        >
          {{ auth.isLoading ? '发送中...' : (countdown > 0 ? `${countdown}s` : '发送邮箱验证码') }}
        </button>
      </div>

      <div v-if="mode === 'register'" class="field">
        <input
          v-model="nickname"
          type="text"
          placeholder="昵称"
          maxlength="20"
        />
      </div>

      <p v-if="auth.error" class="error">{{ auth.error }}</p>

      <button
        class="submit-btn"
        :disabled="auth.isLoading || !email || !code"
        @click="submit"
      >
        {{ auth.isLoading ? '请稍候...' : (mode === 'login' ? '登录' : '注册') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 40px 24px;
  background: #fff;
}
.auth-header { text-align: center; margin-bottom: 40px; }
.auth-header h1 { font-size: 32px; color: #007aff; }
.subtitle { color: #999; margin-top: 8px; }
.auth-tabs {
  display: flex;
  gap: 0;
  margin-bottom: 24px;
  border-bottom: 1px solid #e5e5e5;
}
.tab {
  flex: 1;
  padding: 12px;
  border: none;
  background: none;
  font-size: 16px;
  color: #999;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
}
.tab.active { color: #007aff; border-bottom-color: #007aff; }
.field { margin-bottom: 16px; }
.field input {
  width: 100%;
  padding: 14px 16px;
  border: 1px solid #e5e5e5;
  border-radius: 12px;
  font-size: 16px;
  outline: none;
  transition: border-color 0.2s;
}
.field input:focus { border-color: #007aff; }
.sms-row { display: flex; gap: 10px; }
.sms-row input { flex: 1; }
.sms-btn {
  padding: 0 16px;
  border: none;
  background: #007aff;
  color: #fff;
  border-radius: 12px;
  font-size: 14px;
  white-space: nowrap;
}
.sms-btn:disabled { background: #ccc; }
.error { color: #ff3b30; font-size: 14px; margin-bottom: 12px; }
.submit-btn {
  width: 100%;
  padding: 14px;
  border: none;
  background: #007aff;
  color: #fff;
  border-radius: 12px;
  font-size: 16px;
  font-weight: 600;
}
.submit-btn:disabled { background: #ccc; }
</style>
