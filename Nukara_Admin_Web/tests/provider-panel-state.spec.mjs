import assert from 'node:assert/strict'
import { pickRuntimeDefaultProviderId, resolveExpandedProviderId } from '../src/utils/provider-panel-state.js'

const providers = [
  { id: 'provider-a', is_active: false },
  { id: 'provider-b', is_active: true },
]

assert.equal(pickRuntimeDefaultProviderId(providers, 'provider-a'), 'provider-b')
assert.equal(pickRuntimeDefaultProviderId([{ id: 'provider-a', is_active: false }], 'provider-a'), 'provider-a')

assert.deepEqual(
  resolveExpandedProviderId({
    expandedProviderId: '',
    providers,
    runtimeDefaultProviderId: 'provider-b',
    hasAutoExpandedRuntimeDefault: false,
  }),
  {
    expandedProviderId: 'provider-b',
    hasAutoExpandedRuntimeDefault: true,
  },
)

assert.deepEqual(
  resolveExpandedProviderId({
    expandedProviderId: '',
    providers,
    runtimeDefaultProviderId: 'provider-b',
    hasAutoExpandedRuntimeDefault: true,
  }),
  {
    expandedProviderId: '',
    hasAutoExpandedRuntimeDefault: true,
  },
)

assert.deepEqual(
  resolveExpandedProviderId({
    expandedProviderId: 'missing',
    providers,
    runtimeDefaultProviderId: 'provider-b',
    hasAutoExpandedRuntimeDefault: true,
  }),
  {
    expandedProviderId: 'provider-b',
    hasAutoExpandedRuntimeDefault: true,
  },
)
