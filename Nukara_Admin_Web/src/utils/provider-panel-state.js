export function pickRuntimeDefaultProviderId(providers = [], defaultProviderId = '') {
  const list = Array.isArray(providers) ? providers : []
  return list.find((provider) => provider?.is_active)?.id || defaultProviderId || list[0]?.id || ''
}

export function resolveExpandedProviderId({
  expandedProviderId = '',
  providers = [],
  runtimeDefaultProviderId = '',
  hasAutoExpandedRuntimeDefault = false,
} = {}) {
  const ids = new Set((Array.isArray(providers) ? providers : []).map((provider) => provider?.id).filter(Boolean))

  if (expandedProviderId && ids.has(expandedProviderId)) {
    return {
      expandedProviderId,
      hasAutoExpandedRuntimeDefault,
    }
  }

  if (runtimeDefaultProviderId && (expandedProviderId || !hasAutoExpandedRuntimeDefault)) {
    return {
      expandedProviderId: runtimeDefaultProviderId,
      hasAutoExpandedRuntimeDefault: true,
    }
  }

  return {
    expandedProviderId: '',
    hasAutoExpandedRuntimeDefault,
  }
}
