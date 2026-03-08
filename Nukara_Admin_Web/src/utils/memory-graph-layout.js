const DEFAULT_WIDTH = 920
const DEFAULT_HEIGHT = 620
const DEFAULT_PADDING = 52

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value))
}

function truncateText(value, maxLength) {
  const text = String(value || '').trim()
  if (!text || text.length <= maxLength) {
    return text
  }
  return `${text.slice(0, maxLength)}…`
}

function normalizeImportance(value) {
  const num = Number(value)
  if (!Number.isFinite(num)) {
    return 1
  }
  return clamp(num, 1, 5)
}

function buildTopicAdjacency(edges, nodeMap) {
  const adjacency = new Map()
  const topicLinks = new Map()

  for (const edge of edges || []) {
    const source = nodeMap.get(edge.source)
    const target = nodeMap.get(edge.target)
    if (!source || !target) {
      continue
    }

    if (!adjacency.has(source.id)) adjacency.set(source.id, new Set())
    if (!adjacency.has(target.id)) adjacency.set(target.id, new Set())
    adjacency.get(source.id).add(target.id)
    adjacency.get(target.id).add(source.id)

    const memoryNode = source.type === 'memory' ? source : target.type === 'memory' ? target : null
    const topicNode = source.type === 'topic' ? source : target.type === 'topic' ? target : null
    if (!memoryNode || !topicNode) {
      continue
    }
    if (!topicLinks.has(memoryNode.id)) {
      topicLinks.set(memoryNode.id, [])
    }
    topicLinks.get(memoryNode.id).push(topicNode.id)
  }

  return { adjacency, topicLinks }
}

function buildTopicPositions(topicNodes, width, height) {
  const centerX = width / 2
  const centerY = height / 2
  const radiusX = clamp(width * 0.21, 130, 210)
  const radiusY = clamp(height * 0.17, 96, 165)

  return topicNodes.map((node, index) => {
    const angle = (-Math.PI / 2) + (Math.PI * 2 * index) / Math.max(topicNodes.length, 1)
    return {
      ...node,
      shape: 'diamond',
      radius: 24,
      shortLabel: truncateText(node.label || node.content || node.id, 16),
      x: centerX + Math.cos(angle) * radiusX,
      y: centerY + Math.sin(angle) * radiusY,
      anchorX: centerX,
      anchorY: centerY,
      mobility: 0.18,
    }
  })
}

function buildMemoryPositions(memoryNodes, topicLookup, topicLinks, width, height) {
  const centerX = width / 2
  const centerY = height / 2
  const outerRadiusX = clamp(width * 0.34, 220, 340)
  const outerRadiusY = clamp(height * 0.31, 180, 290)
  const clusterCounts = new Map()
  const orphanNodes = []
  const positioned = []

  for (const node of memoryNodes) {
    const linkedTopicIds = (topicLinks.get(node.id) || []).filter((topicID) => topicLookup.has(topicID))
    const importance = normalizeImportance(node.importance)
    const radius = 14 + importance * 3
    const baseNode = {
      ...node,
      shape: 'circle',
      radius,
      shortLabel: truncateText(node.label || node.content || node.id, 14),
      metaLabel: node.kind ? truncateText(node.kind, 12) : `${importance} 分`,
      mobility: 0.96,
    }

    if (linkedTopicIds.length === 0) {
      orphanNodes.push(baseNode)
      continue
    }

    const topicPoints = linkedTopicIds
      .map((topicID) => topicLookup.get(topicID))
      .filter(Boolean)
    const anchorX = topicPoints.reduce((sum, item) => sum + item.x, 0) / topicPoints.length
    const anchorY = topicPoints.reduce((sum, item) => sum + item.y, 0) / topicPoints.length
    const clusterKey = linkedTopicIds.slice().sort().join('|')
    const clusterIndex = clusterCounts.get(clusterKey) || 0
    clusterCounts.set(clusterKey, clusterIndex + 1)
    const orbit = 68 + (clusterIndex % 4) * 24
    const angle = (-Math.PI / 2) + clusterIndex * 0.88

    positioned.push({
      ...baseNode,
      anchorX,
      anchorY,
      x: anchorX + Math.cos(angle) * orbit,
      y: anchorY + Math.sin(angle) * orbit,
    })
  }

  orphanNodes.forEach((node, index) => {
    const angle = (-Math.PI / 2) + (Math.PI * 2 * index) / Math.max(orphanNodes.length, 1)
    positioned.push({
      ...node,
      anchorX: centerX,
      anchorY: centerY,
      x: centerX + Math.cos(angle) * outerRadiusX,
      y: centerY + Math.sin(angle) * outerRadiusY,
    })
  })

  return positioned
}

function relaxNodePositions(positionedNodes, width, height, padding) {
  const iterations = 28
  for (let iteration = 0; iteration < iterations; iteration += 1) {
    for (let index = 0; index < positionedNodes.length; index += 1) {
      const node = positionedNodes[index]
      let pushX = 0
      let pushY = 0

      for (let otherIndex = 0; otherIndex < positionedNodes.length; otherIndex += 1) {
        if (index === otherIndex) {
          continue
        }
        const other = positionedNodes[otherIndex]
        const dx = node.x - other.x
        const dy = node.y - other.y
        const distance = Math.hypot(dx, dy) || 0.001
        const minDistance = node.radius + other.radius + 26
        if (distance >= minDistance) {
          continue
        }
        const overlap = (minDistance - distance) / minDistance
        pushX += (dx / distance) * overlap * 18
        pushY += (dy / distance) * overlap * 18
      }

      const spring = 0.08 * (node.mobility || 1)
      pushX += (node.anchorX - node.x) * spring
      pushY += (node.anchorY - node.y) * spring

      node.x = clamp(node.x + pushX, padding + node.radius, width - padding - node.radius)
      node.y = clamp(node.y + pushY, padding + node.radius, height - padding - node.radius)
    }
  }

  return positionedNodes
}

export function buildMemoryGraphLayout(nodes = [], edges = [], options = {}) {
  const width = Number(options.width) || DEFAULT_WIDTH
  const height = Number(options.height) || DEFAULT_HEIGHT
  const padding = Number(options.padding) || DEFAULT_PADDING
  const sourceNodes = Array.isArray(nodes) ? nodes : []
  const sourceEdges = Array.isArray(edges) ? edges : []
  const nodeMap = new Map(sourceNodes.map((node) => [node.id, node]))
  const { topicLinks } = buildTopicAdjacency(sourceEdges, nodeMap)

  const topicNodes = sourceNodes
    .filter((node) => node.type === 'topic')
    .sort((left, right) => String(left.label || left.id).localeCompare(String(right.label || right.id)))
  const memoryNodes = sourceNodes
    .filter((node) => node.type !== 'topic')
    .sort((left, right) => normalizeImportance(right.importance) - normalizeImportance(left.importance))

  const positionedTopics = buildTopicPositions(topicNodes, width, height)
  const topicLookup = new Map(positionedTopics.map((node) => [node.id, node]))
  const positionedMemories = buildMemoryPositions(memoryNodes, topicLookup, topicLinks, width, height)
  const positionedNodes = relaxNodePositions([...positionedTopics, ...positionedMemories], width, height, padding)
  const positionedLookup = new Map(positionedNodes.map((node) => [node.id, node]))

  const positionedEdges = sourceEdges
    .map((edge) => ({
      ...edge,
      source: positionedLookup.get(edge.source),
      target: positionedLookup.get(edge.target),
    }))
    .filter((edge) => edge.source && edge.target)

  return {
    width,
    height,
    nodes: positionedNodes,
    edges: positionedEdges,
  }
}

export function buildGraphNeighborSet(edges = [], selectedNodeId = '') {
  const selectedID = String(selectedNodeId || '').trim()
  if (!selectedID) {
    return new Set()
  }
  const related = new Set([selectedID])
  for (const edge of edges || []) {
    if (edge.source === selectedID) {
      related.add(edge.target)
    }
    if (edge.target === selectedID) {
      related.add(edge.source)
    }
  }
  return related
}
