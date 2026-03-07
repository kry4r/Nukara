import { test, expect } from '@playwright/test'

const WEB_URL = 'http://127.0.0.1:5173'

async function mockConversations(page) {
  await page.route('**/api/v1/conversations', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'conv-1',
          bot_id: 'bot-1',
          bot_name: '苏子衿',
          last_message: '刚下班，在想你。',
          last_message_at: '2026-03-07T12:00:00Z',
          unread_count: 0,
        },
      ]),
    })
  })
}

test('app renders inside a fixed 9:16 phone shell on desktop', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })
  await mockConversations(page)

  await page.goto(`${WEB_URL}/`)
  const shell = page.locator('.phone-shell')
  await expect(shell).toBeVisible()

  const box = await shell.boundingBox()
  expect(box).not.toBeNull()
  const ratio = (box?.width ?? 0) / (box?.height ?? 1)
  expect(ratio).toBeGreaterThan(0.53)
  expect(ratio).toBeLessThan(0.59)
})

test('chat page keeps message input visible inside the phone shell on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await page.route('**/api/v1/conversations/conv-1/messages?limit=50', async (route) => {
    const messages = Array.from({ length: 24 }, (_, index) => ({
      id: `m-${index}`,
      conversation_id: 'conv-1',
      sender_type: index % 2 === 0 ? 'user' : 'bot',
      content: { type: 'text', text: `message ${index} `.repeat(10).trim() },
      created_at: new Date(Date.UTC(2026, 2, 6, 12, index, 0)).toISOString(),
    }))

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(messages),
    })
  })

  await page.goto(`${WEB_URL}/chat/conv-1`)

  await expect(page.locator('.phone-shell')).toBeVisible()
  const inputBar = page.locator('.input-bar')
  await expect(inputBar).toBeVisible()
  await expect(inputBar.getByPlaceholder('输入消息...')).toBeVisible()

  const box = await inputBar.boundingBox()
  expect(box).not.toBeNull()
  expect((box?.y ?? 0) + (box?.height ?? 0)).toBeLessThanOrEqual(844)
})

test('bot form page renders persona v2 labels', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await page.goto(`${WEB_URL}/bots/new`)
  await expect(page.getByText('身份设定')).toBeVisible()
  await expect(page.getByText('性格特征')).toBeVisible()
  await expect(page.getByText('表达风格')).toBeVisible()
  await expect(page.getByText('生活环境')).toBeVisible()
  await expect(page.getByText('禁忌与偏好')).toBeVisible()
})

test('bot form page keeps create action reachable on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  let createCount = 0
  await page.route('**/api/v1/bots', async (route, request) => {
    if (request.method() === 'POST') {
      createCount += 1
      const body = request.postDataJSON()
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'bot-1',
          name: body.name,
          identity: body.identity,
          personality: body.personality,
          expression_style: body.expression_style,
          life_context: body.life_context,
          taboos_and_preferences: body.taboos_and_preferences,
        }),
      })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'bot-1',
          name: '测试 Bot',
          personality: [],
        },
      ]),
    })
  })

  await page.goto(`${WEB_URL}/bots/new`)

  const formBody = page.locator('.form-body')
  await expect(formBody).toBeVisible()

  const overflow = await formBody.evaluate((el) => ({
    scrollHeight: el.scrollHeight,
    clientHeight: el.clientHeight,
  }))
  expect(overflow.scrollHeight).toBeGreaterThan(overflow.clientHeight)

  await page.getByPlaceholder('给 Bot 起个名字').fill('测试 Bot')

  const submitBtn = page.locator('.submit-btn')
  await expect(submitBtn).toBeVisible()
  const box = await submitBtn.boundingBox()
  expect(box).not.toBeNull()
  expect((box?.y ?? 0) + (box?.height ?? 0)).toBeLessThanOrEqual(844)

  await submitBtn.click()
  await page.waitForURL(`${WEB_URL}/bots`)
  expect(createCount).toBe(1)
})

test('bot detail page renders persona v2 sections', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await page.route('**/api/v1/bots/bot-1/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
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
        directives: [
          { id: 'd-1', content: '多问开放问题', category: 'behavior', source: 'conversation' },
        ],
        conversation_id: 'conv-1',
      }),
    })
  })

  await page.route('**/api/v1/bots/bot-1/impression', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ impression: '你给我的感觉是理性又温柔。' }),
    })
  })

  await page.goto(`${WEB_URL}/bots/bot-1`)
  await expect(page.getByTestId('bot-detail-page')).toBeVisible()
  await expect(page.getByText('身份设定')).toBeVisible()
  await expect(page.getByText('性格特征')).toBeVisible()
  await expect(page.getByText('表达风格')).toBeVisible()
  await expect(page.getByText('生活环境')).toBeVisible()
  await expect(page.getByText('禁忌与偏好')).toBeVisible()
  await expect(page.getByTestId('bot-detail-impression')).toContainText('理性又温柔')
})

test('settings page shows explicit interval options from 10 minutes to 5 hours', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await page.route('**/api/v1/users/notification-settings', async (route, request) => {
    const method = request.method()
    if (method === 'PUT') {
      const body = request.postDataJSON()
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          user_id: 'user-1',
          proactive_enabled: body.proactive_enabled,
          proactive_interval_minutes: body.proactive_interval_minutes,
          dnd_start: body.dnd_start,
          dnd_end: body.dnd_end,
        }),
      })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        user_id: 'user-1',
        proactive_enabled: true,
        proactive_interval_minutes: 60,
        dnd_start: '23:00',
        dnd_end: '08:00',
      }),
    })
  })

  await page.route('**/api/v1/users/status', async (route, request) => {
    const method = request.method()
    if (method === 'PUT') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true }),
      })
      return
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ emoji: '💭', text: '在想你' }),
    })
  })

  await page.goto(`${WEB_URL}/settings`)
  const select = page.locator('select')
  await expect(select).toBeVisible()
  await expect(select.locator('option', { hasText: '10分钟' })).toHaveCount(1)
  await expect(select.locator('option', { hasText: '5小时' })).toHaveCount(1)
})
