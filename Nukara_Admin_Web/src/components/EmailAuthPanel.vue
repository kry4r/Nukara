<script setup>
import { onMounted, reactive, ref } from 'vue'
import {
  getEmailAuthSettings,
  updateEmailAuthSettings,
  sendEmailAuthTest,
} from '../api/admin.js'

const saving = ref(false)
const testing = ref(false)
const statusMessage = ref('')
const errorMessage = ref('')
const testRecipient = ref('')
const form = reactive({
  smtp_host: 'smtp.qq.com',
  smtp_port: '465',
  smtp_username: '',
  smtp_password: '',
  smtp_password_configured: false,
  from_email: '',
  from_name: 'Nukara',
  code_ttl_seconds: 900,
})

function applySettings(payload = {}) {
  form.smtp_host = payload.smtp_host || 'smtp.qq.com'
  form.smtp_port = String(payload.smtp_port || '465')
  form.smtp_username = payload.smtp_username || ''
  form.smtp_password = ''
  form.smtp_password_configured = Boolean(payload.smtp_password_configured)
  form.from_email = payload.from_email || ''
  form.from_name = payload.from_name || 'Nukara'
  form.code_ttl_seconds = Number(payload.code_ttl_seconds || 900) || 900
}

async function refreshSettings() {
  errorMessage.value = ''
  try {
    applySettings(await getEmailAuthSettings())
  } catch (error) {
    errorMessage.value = error.message
  }
}

async function saveSettings() {
  if (saving.value) {
    return
  }
  saving.value = true
  errorMessage.value = ''
  try {
    const payload = await updateEmailAuthSettings(form)
    applySettings(payload)
    statusMessage.value = 'SMTP / 邮箱认证配置已保存，授权码不会被回显。'
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    saving.value = false
  }
}

async function sendTest() {
  if (testing.value) {
    return
  }
  testing.value = true
  errorMessage.value = ''
  try {
    const payload = await sendEmailAuthTest(testRecipient.value)
    statusMessage.value = payload.message || `测试邮件已发送到 ${testRecipient.value}`
  } catch (error) {
    errorMessage.value = error.message
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  refreshSettings()
})
</script>

<template>
  <article class="email-auth-card">
    <header class="panel-header">
      <div>
        <p class="panel-eyebrow">邮箱认证 / SMTP</p>
        <h2>QQ 邮箱验证码配置</h2>
      </div>
      <button type="button" class="ghost" @click="refreshSettings">刷新</button>
    </header>

    <p class="panel-desc">
      当前认证链路已切换为 <strong>邮箱验证码</strong>。QQ 邮箱推荐填写 <strong>smtp.qq.com</strong> 与端口 <strong>465</strong>，密码使用 SMTP 授权码而不是邮箱登录密码。
    </p>

    <div class="email-grid">
      <label class="mini-form-field">
        <span>SMTP Host</span>
        <input v-model.trim="form.smtp_host" placeholder="smtp.qq.com" />
      </label>
      <label class="mini-form-field">
        <span>SMTP Port</span>
        <input v-model.trim="form.smtp_port" placeholder="465" />
      </label>
      <label class="mini-form-field">
        <span>SMTP Username</span>
        <input v-model.trim="form.smtp_username" placeholder="your-qq-mail@qq.com" />
      </label>
      <label class="mini-form-field">
        <span>SMTP Password / 授权码</span>
        <input
          v-model="form.smtp_password"
          type="password"
          :placeholder="form.smtp_password_configured ? '已配置，留空则保持不变' : '输入邮箱授权码'"
        />
        <small v-if="form.smtp_password_configured" class="field-tip">已在服务器安全保存；如不需要变更，保持留空即可。</small>
      </label>
      <label class="mini-form-field">
        <span>发件邮箱</span>
        <input v-model.trim="form.from_email" placeholder="your-qq-mail@qq.com" />
      </label>
      <label class="mini-form-field">
        <span>发件人名称</span>
        <input v-model.trim="form.from_name" placeholder="Nukara" />
      </label>
      <label class="mini-form-field full-width">
        <span>验证码有效期（秒）</span>
        <input v-model.number="form.code_ttl_seconds" type="number" min="60" step="60" placeholder="900" />
      </label>
    </div>

    <div class="email-actions">
      <button type="button" :disabled="saving" @click="saveSettings">
        {{ saving ? '保存中...' : '保存 SMTP 配置' }}
      </button>
    </div>

    <div class="test-mail-box">
      <label class="mini-form-field full-width">
        <span>测试收件邮箱</span>
        <input v-model.trim="testRecipient" placeholder="例如：Nidhogxt@outlook.com" />
      </label>
      <button type="button" class="ghost" :disabled="testing || !testRecipient" @click="sendTest">
        {{ testing ? '发送中...' : '发送测试邮件' }}
      </button>
    </div>

    <p v-if="statusMessage" class="status-inline">{{ statusMessage }}</p>
    <p v-if="errorMessage" class="error-inline">{{ errorMessage }}</p>
  </article>
</template>

<style scoped>
.email-auth-card {
  display: grid;
  gap: 16px;
  padding: 20px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(148, 163, 184, 0.24);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.08);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
}

.panel-header h2 {
  margin: 4px 0 0;
}

.panel-eyebrow {
  margin: 0;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #6366f1;
}

.panel-desc {
  margin: 0;
  color: #475569;
  line-height: 1.7;
}

.email-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.full-width {
  grid-column: 1 / -1;
}

.mini-form-field {
  display: grid;
  gap: 8px;
}

.mini-form-field span {
  font-size: 13px;
  color: #334155;
}

.mini-form-field input {
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 14px;
  padding: 12px 14px;
  font-size: 14px;
}

.field-tip {
  color: #64748b;
  font-size: 12px;
}

.email-actions,
.test-mail-box {
  display: flex;
  gap: 12px;
  align-items: end;
}

button {
  border: none;
  border-radius: 14px;
  padding: 12px 16px;
  background: linear-gradient(135deg, #111827, #1f2937);
  color: white;
  font-weight: 600;
  cursor: pointer;
}

button.ghost {
  background: #eef2ff;
  color: #3730a3;
}

.status-inline,
.error-inline {
  margin: 0;
  font-size: 14px;
}

.status-inline {
  color: #0f766e;
}

.error-inline {
  color: #b91c1c;
}

@media (max-width: 920px) {
  .email-grid {
    grid-template-columns: 1fr;
  }

  .test-mail-box {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
