import test from 'node:test'
import assert from 'node:assert/strict'

import {
  normalizeServerTimestamp,
  resolveMessageTimestamp,
} from '../src/utils/time.js'

test('normalizes timestamp without timezone as UTC', () => {
  assert.equal(
    normalizeServerTimestamp('2026-03-05T08:15:30'),
    '2026-03-05T08:15:30.000Z'
  )
})

test('keeps conversation list and message bubble timestamp source consistent', () => {
  const payload = { created_at: '2026-03-05T08:15:30' }
  const listTime = resolveMessageTimestamp(payload)
  const bubbleTime = resolveMessageTimestamp(payload)
  assert.equal(listTime, bubbleTime)
  assert.equal(listTime, '2026-03-05T08:15:30.000Z')
})

test('falls back to unix timestamp seconds', () => {
  const seconds = 1741162530
  assert.equal(
    resolveMessageTimestamp({ timestamp: seconds }),
    new Date(seconds * 1000).toISOString()
  )
})
