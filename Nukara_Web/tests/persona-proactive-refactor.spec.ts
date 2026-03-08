import { test, expect } from '@playwright/test'

const WEB_URL = 'http://127.0.0.1:5173'

test('persona proactive refactor flow stays consistent end-to-end', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
    class MockWebSocket {
      static OPEN = 1
      static CLOSED = 3
      readyState = MockWebSocket.OPEN
      constructor() {
        setTimeout(() => this.onopen?.(), 0)
      }
      send() {}
      close() {
        this.readyState = MockWebSocket.CLOSED
        this.onclose?.()
      }
    }
    window.WebSocket = MockWebSocket
  })

  const profile = {
    bot: {
      id: 'bot-1',
      name: '苏子衿',
      identity: '你的恋人，也是会认真接住你情绪的人',
      personality: ['细腻', '敏锐'],
      expression_style: '口语化，短句，会接梗',
      life_context: '现在住在东京，平时摄影、通勤、喝便利店咖啡',
      taboos_and_preferences: '不喜欢被命令式对待，更喜欢被温柔回应',
    },
    bot_state: {
      status_emoji: '💭',
      status_text: '在想你',
    },
    conversation_id: 'conv-1',
    recent_impressions: [
      { id: 'imp-1', kind: 'impression', content: '你给我的感觉是理性又温柔。' },
    ],
  }

  let notifications = {
    user_id: 'user-1',
    proactive_enabled: true,
    proactive_interval_minutes: 60,
    dnd_start: '23:00',
    dnd_end: '08:00',
  }

  let userStatus = { emoji: '💭', text: '在想你' }

  await page.route('**/api/v1/bots/bot-1/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(profile),
    })
  })


  await page.route('**/api/v1/users/notification-settings', async (route, request) => {
    if (request.method() === 'PUT') {
      const body = request.postDataJSON()
      notifications = {
        ...notifications,
        proactive_enabled: body.proactive_enabled,
        proactive_interval_minutes: body.proactive_interval_minutes,
        dnd_start: body.dnd_start,
        dnd_end: body.dnd_end,
      }
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(notifications),
    })
  })

  await page.route('**/api/v1/users/status', async (route, request) => {
    if (request.method() === 'PUT') {
      const body = request.postDataJSON()
      userStatus = { emoji: body.emoji, text: body.text }
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(userStatus),
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
          sender_type: 'user',
          content: { type: 'text', text: '你现在在干嘛' },
          created_at: '2026-03-07T12:00:00Z',
        },
        {
          id: 'm-2',
          conversation_id: 'conv-1',
          sender_type: 'bot',
          content: { type: 'text', text: '我刚下班。' },
          reply_group_id: 'rg-1',
          sequence: 1,
          created_at: '2026-03-07T12:01:00Z',
        },
        {
          id: 'm-3',
          conversation_id: 'conv-1',
          sender_type: 'bot',
          content: { type: 'text', text: '好累，不过想到你又好一点。' },
          reply_group_id: 'rg-1',
          sequence: 2,
          created_at: '2026-03-07T12:01:10Z',
        },
      ]),
    })
  })

  await page.goto(`${WEB_URL}/bots/bot-1`)
  await expect(page.getByTestId('bot-detail-page')).toBeVisible()
  await expect(page.getByTestId('bot-detail-impression')).toContainText('理性又温柔')
  await expect(page.getByRole('button', { name: '运行迭代' })).toHaveCount(0)
  await expect(page.getByText('行为指令')).toHaveCount(0)
  await expect(page.locator('body')).not.toContainText('bot not found')

  await page.goto(`${WEB_URL}/settings`)
  await page.locator('select').selectOption('10')
  await page.locator('input[type="time"]').nth(0).fill('00:30')
  await page.locator('input[type="time"]').nth(1).fill('07:45')
  await page.reload()
  await expect(page.locator('select')).toHaveValue('10')
  await expect(page.locator('input[type="time"]').nth(0)).toHaveValue('00:30')
  await expect(page.locator('input[type="time"]').nth(1)).toHaveValue('07:45')

  await page.goto(`${WEB_URL}/chat/conv-1`)
  await expect(page.locator('.phone-shell')).toBeVisible()
  await expect(page.getByText('我刚下班。')).toBeVisible()
  await expect(page.getByText('好累，不过想到你又好一点。')).toBeVisible()

  const shell = page.locator('.phone-shell')
  const box = await shell.boundingBox()
  expect(box).not.toBeNull()
  const ratio = (box?.width ?? 0) / (box?.height ?? 1)
  expect(ratio).toBeGreaterThan(0.53)
  expect(ratio).toBeLessThan(0.59)
})
