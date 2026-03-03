import { test, expect } from '@playwright/test'

test('chat page has pencil-like input bar', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('nukara_token', 'test-token')
  })
  await page.goto('http://127.0.0.1:18081/')
  await expect(page.locator('.input-bar')).toBeVisible()
  await expect(page.locator('.conv-list')).toBeVisible()
})
