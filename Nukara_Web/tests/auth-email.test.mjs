import assert from 'node:assert/strict'
import { canSendEmailCode } from '../src/utils/auth-email.js'

assert.equal(canSendEmailCode({ email: '', countdown: 0, isLoading: false }), false)
assert.equal(canSendEmailCode({ email: 'tester@example.com', countdown: 12, isLoading: false }), false)
assert.equal(canSendEmailCode({ email: 'tester@example.com', countdown: 0, isLoading: true }), false)
assert.equal(canSendEmailCode({ email: ' tester@example.com ', countdown: 0, isLoading: false }), true)
