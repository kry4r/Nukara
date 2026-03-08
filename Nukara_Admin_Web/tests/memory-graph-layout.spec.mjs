import assert from 'node:assert/strict'
import { buildMemoryGraphLayout, buildGraphNeighborSet } from '../src/utils/memory-graph-layout.js'

const nodes = [
  { id: 'topic-routine', type: 'topic', label: 'routine' },
  { id: 'topic-work', type: 'topic', label: 'work' },
  { id: 'memory-walk', type: 'memory', label: '凌晨散步', content: '最近开始把凌晨散步当作下班后的固定习惯', importance: 4 },
  { id: 'memory-shift', type: 'memory', label: '值晚班', content: '经常值晚班，回家路上会慢走', importance: 3 },
  { id: 'memory-coffee', type: 'memory', label: '便利店咖啡', content: '通勤时会买便利店咖啡', importance: 1 },
]

const edges = [
  { id: 'edge-1', source: 'memory-walk', target: 'topic-routine', type: 'topic' },
  { id: 'edge-2', source: 'memory-shift', target: 'topic-work', type: 'topic' },
  { id: 'edge-3', source: 'memory-shift', target: 'topic-routine', type: 'topic' },
]

const layout = buildMemoryGraphLayout(nodes, edges, {
  width: 920,
  height: 620,
})

assert.equal(layout.nodes.length, nodes.length)
assert.equal(layout.edges.length, edges.length)

const byId = new Map(layout.nodes.map((node) => [node.id, node]))
for (const id of ['topic-routine', 'topic-work', 'memory-walk', 'memory-shift', 'memory-coffee']) {
  assert.ok(byId.has(id), `missing positioned node ${id}`)
  const node = byId.get(id)
  assert.ok(Number.isFinite(node.x), `${id} x should be finite`)
  assert.ok(Number.isFinite(node.y), `${id} y should be finite`)
  assert.ok(node.x >= 48 && node.x <= 872, `${id} x out of bounds: ${node.x}`)
  assert.ok(node.y >= 48 && node.y <= 572, `${id} y out of bounds: ${node.y}`)
}

assert.ok(byId.get('memory-walk').radius > byId.get('memory-coffee').radius, 'importance should increase node radius')
assert.ok(byId.get('topic-routine').shape === 'diamond', 'topic nodes should render as diamond')
assert.ok(byId.get('memory-walk').shape === 'circle', 'memory nodes should render as circle')

const related = buildGraphNeighborSet(edges, 'memory-shift')
assert.deepEqual([...related].sort(), ['memory-shift', 'topic-routine', 'topic-work'])
