import { ref } from 'vue'
import {
  WS_HEARTBEAT_INTERVAL,
  WS_MAX_RECONNECT_ATTEMPTS,
  WS_MAX_RECONNECT_DELAY,
} from '../utils/constants'

export function useWebSocket() {
  let ws = null
  let heartbeatTimer = null
  let reconnectTimer = null
  let handlers = {}

  const isConnected = ref(false)
  const reconnectAttempts = ref(0)
  let manualClose = false
  let currentUrl = ''

  function connect(url) {
    currentUrl = url
    manualClose = false
    _doConnect()
  }

  function _doConnect() {
    if (ws) {
      try { ws.close() } catch (_) {}
    }

    ws = new WebSocket(currentUrl)

    ws.onopen = () => {
      isConnected.value = true
      reconnectAttempts.value = 0
      _startHeartbeat()
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        _dispatch(data)
      } catch (_) {}
    }

    ws.onclose = () => {
      isConnected.value = false
      _stopHeartbeat()
      if (!manualClose) {
        _dispatch({ type: 'connection_error', message: 'WebSocket 已断开，正在尝试重连。' })
        _scheduleReconnect()
      }
    }

    ws.onerror = () => {
      isConnected.value = false
      _dispatch({ type: 'error', message: 'WebSocket 连接失败，请检查网络或服务状态。' })
    }
  }

  function _dispatch(data) {
    if (data.type === 'pong') return
    if (data.type === 'bot_memory_saved' || data.type === 'bot_persona_updated') {
      console.log('[ws-debug]', data.type, data)
    }
    const handler = handlers[data.type]
    if (handler) handler(data)
  }

  function on(type, handler) {
    handlers[type] = handler
  }

  function send(obj) {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(obj))
      return true
    }
    return false
  }

  function _startHeartbeat() {
    _stopHeartbeat()
    heartbeatTimer = setInterval(() => {
      send({ type: 'ping' })
    }, WS_HEARTBEAT_INTERVAL)
  }

  function _stopHeartbeat() {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = null
    }
  }

  function _scheduleReconnect() {
    if (reconnectAttempts.value >= WS_MAX_RECONNECT_ATTEMPTS) return
    const delay = Math.min(
      Math.pow(2, reconnectAttempts.value) * 1000,
      WS_MAX_RECONNECT_DELAY,
    )
    reconnectAttempts.value++
    reconnectTimer = setTimeout(_doConnect, delay)
  }

  function disconnect() {
    manualClose = true
    _stopHeartbeat()
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.close()
      ws = null
    }
    isConnected.value = false
  }

  return {
    isConnected,
    reconnectAttempts,
    connect, disconnect, send, on,
  }
}
