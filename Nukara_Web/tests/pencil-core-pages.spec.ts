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

async function mockChatMessages(page) {
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

  await mockChatMessages(page)

  await page.goto(`${WEB_URL}/chat/conv-1`)

  await expect(page.locator('.phone-shell')).toBeVisible()
  const inputBar = page.locator('.input-bar')
  await expect(inputBar).toBeVisible()
  await expect(inputBar.getByPlaceholder('输入消息...')).toBeVisible()

  const box = await inputBar.boundingBox()
  expect(box).not.toBeNull()
  expect((box?.y ?? 0) + (box?.height ?? 0)).toBeLessThanOrEqual(844)
})

test('chat input uses a non-zooming font size on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await mockChatMessages(page)
  await page.goto(`${WEB_URL}/chat/conv-1`)

  const input = page.getByPlaceholder('输入消息...')
  await expect(input).toBeVisible()

  const fontSize = await input.evaluate((el) => Number.parseFloat(getComputedStyle(el).fontSize))
  expect(fontSize).toBeGreaterThanOrEqual(16)
})

test('chat input shell does not add a focus halo on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await mockChatMessages(page)
  await page.goto(`${WEB_URL}/chat/conv-1`)

  const shell = page.locator('.input-shell')
  const input = page.getByPlaceholder('输入消息...')
  await expect(shell).toBeVisible()
  await expect(input).toBeVisible()

  const before = await shell.evaluate((el) => {
    const styles = getComputedStyle(el)
    return {
      borderColor: styles.borderColor,
      boxShadow: styles.boxShadow,
      backgroundColor: styles.backgroundColor,
    }
  })

  await input.click()

  const after = await shell.evaluate((el) => {
    const styles = getComputedStyle(el)
    return {
      borderColor: styles.borderColor,
      boxShadow: styles.boxShadow,
      backgroundColor: styles.backgroundColor,
    }
  })

  expect(after.borderColor).toBe(before.borderColor)
  expect(after.boxShadow).toBe(before.boxShadow)
  expect(after.backgroundColor).toBe(before.backgroundColor)
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

test('settings page keeps bottom nav visible on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })

  await page.route('**/api/v1/users/notification-settings', async (route) => {
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

  await page.route('**/api/v1/users/status', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ emoji: '💭', text: '在想你' }),
    })
  })

  await page.goto(`${WEB_URL}/settings`)

  const nav = page.locator('.nav-shell')
  await expect(nav).toBeVisible()

  const navBox = await nav.boundingBox()
  expect(navBox).not.toBeNull()
  expect((navBox?.y ?? 0) + (navBox?.height ?? 0)).toBeLessThanOrEqual(844)
})

test('bot detail page keeps a scrollable content area on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
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
          identity: '你的恋人，也是会认真接住你情绪的人。'.repeat(10),
          personality: ['细腻', '敏锐', '会观察', '有分寸感'],
          expression_style: '口语化，短句，会接梗。'.repeat(8),
          life_context: '现在住在东京，平时摄影、通勤、喝便利店咖啡。'.repeat(12),
          taboos_and_preferences: '不喜欢被命令式对待，更喜欢被温柔回应。'.repeat(10),
        },
        bot_state: {
          status_emoji: '💭',
          status_text: '在想你',
        },
        directives: Array.from({ length: 12 }, (_, index) => ({
          id: `d-${index}`,
          content: `请记得第 ${index + 1} 条长期偏好和互动方式。`.repeat(3),
          category: 'behavior',
          source: 'conversation',
        })),
        conversation_id: 'conv-1',
      }),
    })
  })

  await page.route('**/api/v1/bots/bot-1/impression', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ impression: '你给我的感觉是理性又温柔。'.repeat(6) }),
    })
  })

  await page.goto(`${WEB_URL}/bots/bot-1`)

  const detailMain = page.locator('.detail-main')
  await expect(detailMain).toBeVisible()

  const before = await detailMain.evaluate((el) => ({
    scrollHeight: el.scrollHeight,
    clientHeight: el.clientHeight,
    scrollTop: el.scrollTop,
  }))
  expect(before.scrollHeight).toBeGreaterThan(before.clientHeight)

  const after = await detailMain.evaluate((el) => {
    el.scrollTop = 240
    return el.scrollTop
  })
  expect(after).toBeGreaterThan(0)
})
