import assert from 'node:assert/strict'
import {
  normalizeProviderPayload,
  normalizeEmailAuthSettingsPayload,
  buildListAdminUsersPath,
  buildListUserBotsPath,
  buildMemoryGraphPath,
} from '../src/api/admin.js'

const payload = normalizeProviderPayload({
  name: 'astron',
  api_key: 'k',
  base_url: 'https://example.com/v2',
  models: 'xminimaxm25,xopglm5',
  priority: '1',
})

assert.deepEqual(payload.models, ['xminimaxm25', 'xopglm5'])
assert.equal(payload.priority, 1)

const emailPayload = normalizeEmailAuthSettingsPayload({
  smtp_host: ' smtp.qq.com ',
  smtp_port: ' 465 ',
  smtp_username: ' qq@example.com ',
  smtp_password: ' secret ',
  from_email: ' qq@example.com ',
  from_name: ' Nukara ',
  code_ttl_seconds: 0,
})

assert.equal(emailPayload.smtp_host, 'smtp.qq.com')
assert.equal(emailPayload.smtp_port, '465')
assert.equal(emailPayload.code_ttl_seconds, 900)
assert.equal(buildListAdminUsersPath({ q: 'alice', limit: 20, offset: 40 }), '/users?q=alice&limit=20&offset=40')
assert.equal(buildListUserBotsPath('user-1'), '/users/user-1/bots')
assert.equal(buildMemoryGraphPath('user-1', 'bot-1'), '/users/user-1/bots/bot-1/memory-graph')
