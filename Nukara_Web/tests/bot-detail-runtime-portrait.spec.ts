import { expect, test } from '@playwright/test'

const WEB_URL = 'http://127.0.0.1:5173'

test('bot detail renders runtime portrait and auto-applied persona changes', async ({ page }) => {
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
      identity: '会认真接住你情绪的人；不是纯金融，更偏研究型',
      personality: ['细腻', '敏锐'],
      expression_style: '口语化，短句',
      life_context: '住在东京，偶尔夜班',
      taboos_and_preferences: '不喜欢被敷衍',
      self_cognition: ['我最近会在夜班后的慢走里，把自己慢慢安静下来。'],
    },
    bot_state: {
      status_emoji: '🌙',
      status_text: '刚下班',
    },
    conversation_id: 'conv-1',
    runtime_state: {
      activity_text: '刚下晚班，在回去路上',
      basis_tags: ['self_fact'],
    },
    recent_impressions: [
      { id: 'imp-1', kind: 'impression', content: '你给我的感觉是理性又温柔。' },
    ],
    key_memories: [
      { id: 'mem-1', kind: 'promise', content: '答应这周给你整理歌单', status: 'active' },
      { id: 'mem-2', kind: 'event', content: '昨天去看了摄影展', status: 'active' },
    ],
    recent_changes: [
      { id: 'chg-accepted-1', field: 'identity', proposed_value: '不是纯金融，更偏研究型', summary_text: '我更明确了自己的身份：不是纯金融，更偏研究型', status: 'accepted' },
      { id: 'chg-skipped-1', field: 'life_context', proposed_value: '最近换到夜班节奏', summary_text: '这条生活背景是短期状态，因此没有写入稳定人设', status: 'skipped' },
    ],
  }

  await page.route('**/api/v1/bots/bot-1/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(profile),
    })
  })

  await page.goto(`${WEB_URL}/bots/bot-1`)

  await expect(page.getByTestId('bot-detail-page')).toBeVisible()
  await expect(page.getByTestId('bot-detail-impression')).toContainText('理性又温柔')
  await expect(page.getByRole('button', { name: '运行迭代' })).toHaveCount(0)
  await expect(page.getByText('行为指令')).toHaveCount(0)
  await expect(page.getByText('刚下晚班，在回去路上')).toBeVisible()
  await expect(page.getByText('答应这周给你整理歌单')).toBeVisible()
  await expect(page.getByTestId('bot-detail-self-cognition').getByText('夜班后的慢走')).toBeVisible()
  await expect(page.getByTestId('bot-detail-recent-changes').getByText('我更明确了自己的身份：不是纯金融，更偏研究型')).toBeVisible()
  await expect(page.getByTestId('bot-detail-recent-changes').getByText('这条生活背景是短期状态，因此没有写入稳定人设')).toBeVisible()
  await expect(page.getByRole('button', { name: '接受' })).toHaveCount(0)
  await expect(page.getByText('待确认的人设变更')).toHaveCount(0)
})
