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

export const FREQUENCY_OPTIONS = [
  { value: 'high', label: '高频（2小时）' },
  { value: 'normal', label: '正常（4小时）' },
  { value: 'low', label: '低频（8小时）' },
]

export const WS_HEARTBEAT_INTERVAL = 25000
export const WS_MAX_RECONNECT_ATTEMPTS = 8
export const WS_MAX_RECONNECT_DELAY = 30000
