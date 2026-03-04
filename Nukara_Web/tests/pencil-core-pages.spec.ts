import { test, expect } from '@playwright/test'

test('chat page has pencil-like input bar', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })
  await page.goto('http://127.0.0.1:18081/')
  await expect(page.locator('.input-bar')).toBeVisible()
  await expect(page.locator('.conv-list')).toBeVisible()
})

test('bot detail page renders key sections', async ({ page }) => {
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
          summary: '温柔|会倾听',
          speaking_style: '口语化',
          background: '江南|摄影',
          traits: ['体贴', '好奇'],
          gender: 'female',
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

  await page.route('**/api/v1/bots/bot-1/iterate', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        speaking_style_adds: ['会接梗'],
        background_adds: ['最近在学摄影'],
        trait_adds: ['观察细致'],
        gender: 'female',
        bot: {
          id: 'bot-1',
          name: '苏子衿',
          summary: '温柔|会倾听',
          speaking_style: '口语化|会接梗',
          background: '江南|摄影|最近在学摄影',
          traits: ['体贴', '好奇', '观察细致'],
          gender: 'female',
        },
      }),
    })
  })

  await page.route('**/api/v1/bots/bot-1/directives/**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true }),
    })
  })

  await page.goto('http://127.0.0.1:18081/bots/bot-1')
  await expect(page.getByTestId('bot-detail-page')).toBeVisible()
  await expect(page.getByText('自我状态')).toBeVisible()
  await expect(page.getByText('原始人设')).toBeVisible()
  await expect(page.getByText('行为指令')).toBeVisible()
  await expect(page.getByTestId('bot-detail-impression')).toContainText('理性又温柔')
})
