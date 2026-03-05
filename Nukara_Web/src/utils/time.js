const ISO_NO_TZ_RE = /^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?$/

function fromUnix(value) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  const millis = Math.abs(value) > 1e12 ? value : value * 1000
  const date = new Date(millis)
  if (Number.isNaN(date.getTime())) return null
  return date
}

function fromString(value) {
  const trimmed = String(value || '').trim()
  if (!trimmed) return null

  if (/^-?\d+(?:\.\d+)?$/.test(trimmed)) {
    return fromUnix(Number(trimmed))
  }

  const normalized = ISO_NO_TZ_RE.test(trimmed)
    ? `${trimmed.replace(' ', 'T')}Z`
    : trimmed
  const date = new Date(normalized)
  if (Number.isNaN(date.getTime())) return null
  return date
}

function toDate(input) {
  if (input instanceof Date) {
    return Number.isNaN(input.getTime()) ? null : input
  }
  if (typeof input === 'number') {
    return fromUnix(input)
  }
  if (typeof input === 'string') {
    return fromString(input)
  }
  return null
}

export function normalizeServerTimestamp(input) {
  const date = toDate(input)
  return date ? date.toISOString() : ''
}

export function resolveMessageTimestamp(message, fallback = null) {
  const created = normalizeServerTimestamp(message?.created_at ?? message?.createdAt)
  if (created) return created
  const fromEpoch = normalizeServerTimestamp(message?.timestamp)
  if (fromEpoch) return fromEpoch
  const fallbackDate = toDate(fallback)
  return fallbackDate ? fallbackDate.toISOString() : ''
}

export function formatClockTime(input, locale = 'zh-CN') {
  const normalized = normalizeServerTimestamp(input)
  if (!normalized) return ''
  return new Date(normalized).toLocaleTimeString(locale, {
    hour: '2-digit',
    minute: '2-digit',
  })
}
