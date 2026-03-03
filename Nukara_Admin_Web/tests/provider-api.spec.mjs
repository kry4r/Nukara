import assert from 'node:assert/strict'
import { normalizeProviderPayload } from '../src/api/admin.js'

const payload = normalizeProviderPayload({
  name: 'astron',
  api_key: 'k',
  base_url: 'https://example.com/v2',
  models: 'xminimaxm25,xopglm5',
  priority: '1',
})

assert.deepEqual(payload.models, ['xminimaxm25', 'xopglm5'])
assert.equal(payload.priority, 1)
