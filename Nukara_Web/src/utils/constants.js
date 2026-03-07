export const API_BASE = ''

export const EMOTION_EMOJI = {
  happy: '😊',
  excited: '🤩',
  love: '💕',
  sad: '🌧',
  angry: '😤',
  anxious: '😟',
  gentle: '☕️',
}

export const EMOTION_STATUS = {
  happy: ['😊 开心', '🎵 哼歌中', '✨ 心情好'],
  excited: ['🤩 超兴奋', '✨ 灵感爆发', '🎉 好开心'],
  love: ['💕 心动中', '🥰 甜蜜', '💭 在想你'],
  sad: ['🌧 有点低落', '💭 在想事情', '🤗 想陪你'],
  angry: ['😤 有点生气', '💭 冷静中', '🍵 喝茶消气'],
  anxious: ['😟 有点担心', '💭 在想你', '🤗 想陪你'],
  gentle: ['☕️ 慢慢聊', '🌸 温柔模式', '📖 静静陪你'],
}

export const DEFAULT_STATUSES = [
  '🙂 在线', '💭 在想你', '☕️ 慢慢聊', '🎵 听歌中',
  '🌙 夜聊', '✨ 灵感中', '📖 读书', '🎨 创作中',
]

export const PROACTIVE_INTERVAL_OPTIONS = [
  { value: 10, label: '10分钟' },
  { value: 30, label: '30分钟' },
  { value: 60, label: '1小时' },
  { value: 120, label: '2小时' },
  { value: 180, label: '3小时' },
  { value: 240, label: '4小时' },
  { value: 300, label: '5小时' },
]

export function formatProactiveIntervalLabel(minutes) {
  const value = Number(minutes)
  if (!Number.isFinite(value) || value <= 0) {
    return '未设置'
  }
  const hours = Math.floor(value / 60)
  const remain = value % 60
  if (hours > 0 && remain === 0) {
    return `${hours}小时`
  }
  if (hours > 0 && remain > 0) {
    return `${hours}小时${remain}分钟`
  }
  return `${value}分钟`
}

export const WS_HEARTBEAT_INTERVAL = 25000
export const WS_MAX_RECONNECT_ATTEMPTS = 8
export const WS_MAX_RECONNECT_DELAY = 30000
