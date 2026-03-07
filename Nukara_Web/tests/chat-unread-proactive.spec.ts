import { test, expect } from '@playwright/test'

const WEB_URL = 'http://127.0.0.1:5173'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')

    const sockets: any[] = []
    class MockWebSocket {
      static OPEN = 1
      static CLOSED = 3
      readyState = MockWebSocket.OPEN
      onopen?: () => void
      onmessage?: (event: { data: string }) => void
      onclose?: () => void
      constructor() {
        sockets.push(this)
        setTimeout(() => this.onopen?.(), 0)
      }
      send() {}
      close() {
        this.readyState = MockWebSocket.CLOSED
        this.onclose?.()
      }
    }

    ;(window as any).__mockSockets = sockets
    ;(window as any).__emitWS = (payload: unknown) => {
      const raw = JSON.stringify(payload)
      sockets.forEach((socket) => {
        if (socket.readyState === MockWebSocket.OPEN) {
          socket.onmessage?.({ data: raw })
        }
      })
    }
    ;(window as any).WebSocket = MockWebSocket
  })
})

test('entering chat clears unread and leaving chat still receives proactive updates', async ({ page }) => {
  let conversations = [
    {
      id: 'conv-1',
      bot_id: 'bot-1',
      bot_name: '苏子衿',
      last_message: '昨晚睡得好吗',
      last_message_at: '2026-03-07T12:00:00Z',
      unread_count: 2,
      is_proactive_message: false,
    },
    {
      id: 'conv-2',
      bot_id: 'bot-2',
      bot_name: '林见夏',
      last_message: '暂无消息',
      last_message_at: '2026-03-07T11:00:00Z',
      unread_count: 0,
      is_proactive_message: false,
    },
  ]

  await page.route('**/api/v1/conversations', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(conversations),
    })
  })

  await page.route('**/api/v1/conversations/conv-1/messages?limit=50', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'm-1',
          conversation_id: 'conv-1',
          sender_type: 'bot',
          content: { type: 'text', text: '昨晚睡得好吗' },
          created_at: '2026-03-07T12:00:00Z',
        },
      ]),
    })
  })

  await page.route('**/api/v1/conversations/conv-1/mark-read', async (route) => {
    conversations = conversations.map((conv) => (
      conv.id === 'conv-1' ? { ...conv, unread_count: 0 } : conv
    ))
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true }),
    })
  })

  await page.goto(`${WEB_URL}/`)
  await expect(page.locator('.conv-item').filter({ hasText: '苏子衿' }).locator('.conv-badge')).toHaveText('2')

  await page.getByRole('link', { name: '打开会话 苏子衿' }).click()
  await expect(page.locator('.chat-page')).toBeVisible()

  await page.getByRole('button', { name: '返回会话列表' }).click()
  await expect(page).toHaveURL(`${WEB_URL}/`)
  await expect(page.locator('.conv-item').filter({ hasText: '苏子衿' }).locator('.conv-badge')).toHaveCount(0)

  conversations = conversations.map((conv) => (
    conv.id === 'conv-2'
      ? {
          ...conv,
          last_message: '刚刚想到你了。',
          last_message_at: '2026-03-07T12:20:00Z',
          unread_count: 1,
          is_proactive_message: true,
        }
      : conv
  ))

  await page.evaluate(() => {
    ;(window as any).__emitWS({
      type: 'proactive_message',
      conversation_id: 'conv-2',
      msg_id: 'pm-1',
      content: { type: 'text', text: '刚刚想到你了。' },
      timestamp: 1772870400,
    })
  })

  await expect(page.locator('.conv-item').filter({ hasText: '林见夏' })).toContainText('刚刚想到你了。')
  await expect(page.locator('.conv-item').filter({ hasText: '林见夏' }).locator('.conv-badge')).toHaveText('1')
})

test('first proactive message can surface a conversation that was not previously opened', async ({ page }) => {
  let conversations = [
    {
      id: 'conv-1',
      bot_id: 'bot-1',
      bot_name: '苏子衿',
      last_message: '你好呀',
      last_message_at: '2026-03-07T12:00:00Z',
      unread_count: 0,
      is_proactive_message: false,
    },
  ]

  await page.route('**/api/v1/conversations', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(conversations),
    })
  })

  await page.goto(`${WEB_URL}/`)
  await expect(page.locator('.conv-item')).toHaveCount(1)

  conversations = [
    ...conversations,
    {
      id: 'conv-2',
      bot_id: 'bot-2',
      bot_name: '林见夏',
      last_message: '第一次主动找你。',
      last_message_at: '2026-03-07T12:10:00Z',
      unread_count: 1,
      is_proactive_message: true,
    },
  ]

  await page.evaluate(() => {
    ;(window as any).__emitWS({
      type: 'proactive_message',
      conversation_id: 'conv-2',
      msg_id: 'pm-first',
      content: { type: 'text', text: '第一次主动找你。' },
      timestamp: 1772871000,
    })
  })

  await expect(page.locator('.conv-item').filter({ hasText: '林见夏' })).toContainText('第一次主动找你。')
  await expect(page.locator('.conv-item').filter({ hasText: '林见夏' }).locator('.conv-badge')).toHaveText('1')
})

test('proactive message for another conversation does not appear inside current thread', async ({ page }) => {
  await page.route('**/api/v1/conversations', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'conv-1',
          bot_id: 'bot-1',
          bot_name: '苏子衿',
          last_message: '你好呀',
          last_message_at: '2026-03-07T12:00:00Z',
          unread_count: 0,
          is_proactive_message: false,
        },
        {
          id: 'conv-2',
          bot_id: 'bot-2',
          bot_name: '林见夏',
          last_message: '晚点聊',
          last_message_at: '2026-03-07T11:00:00Z',
          unread_count: 0,
          is_proactive_message: false,
        },
      ]),
    })
  })

  await page.route('**/api/v1/conversations/conv-1/messages?limit=50', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'm-1',
          conversation_id: 'conv-1',
          sender_type: 'bot',
          content: { type: 'text', text: '你好呀' },
          created_at: '2026-03-07T12:00:00Z',
        },
      ]),
    })
  })

  await page.route('**/api/v1/conversations/conv-1/mark-read', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true }),
    })
  })

  await page.goto(`${WEB_URL}/chat/conv-1`)
  await expect(page.locator('.chat-page')).toBeVisible()
  await expect(page.getByText('你好呀')).toBeVisible()

  await page.evaluate(() => {
    ;(window as any).__emitWS({
      type: 'proactive_message',
      conversation_id: 'conv-2',
      msg_id: 'pm-2',
      content: { type: 'text', text: '这条不该插进当前会话。' },
      timestamp: 1772870400,
    })
  })

  await expect(page.getByText('这条不该插进当前会话。')).toHaveCount(0)
})
