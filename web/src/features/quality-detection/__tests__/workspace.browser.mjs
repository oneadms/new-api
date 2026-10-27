/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/* eslint-disable no-console -- This standalone browser test reports its results to the terminal. */
// Run against the local frontend: node src/features/quality-detection/__tests__/workspace.browser.mjs
// Uses the renderer package's existing Playwright install. All API calls are intercepted.
import assert from 'node:assert/strict'
import { tmpdir } from 'node:os'
import path from 'node:path'

import { chromium } from '../../../../../tools/quality-renderer/node_modules/playwright/index.mjs'

const baseURL = process.env.QUALITY_UI_URL ?? 'http://127.0.0.1:5173'
assert.ok(
  ['localhost', '127.0.0.1'].includes(new URL(baseURL).hostname),
  'Only local frontend testing is allowed'
)
const browser = await chromium.launch({
  channel:
    process.env.QUALITY_UI_BROWSER ??
    (process.platform === 'win32' ? 'msedge' : undefined),
  headless: true,
})
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
  colorScheme: 'dark',
  reducedMotion: 'reduce',
})
await context.addCookies([
  { name: 'vite-ui-theme', value: 'dark', url: baseURL },
  { name: 'theme_preset', value: 'simple-large', url: baseURL },
  { name: 'theme_font', value: 'sans', url: baseURL },
])
await context.addInitScript(() => {
  localStorage.setItem('i18nextLng', 'en')
  localStorage.setItem('setup_status_checked', 'true')
})
let channels = [
  {
    id: 1,
    name: 'OpenAI · Primary',
    groups: ['Standard', 'Premium'],
    models: ['gpt-5'],
    status: 1,
    supported: true,
    unsupported_reason: '',
    profile: {
      channel_id: 1,
      version: 0,
      enabled: false,
      model: '',
      protocol: 'chat',
      prompt_version_id: 0,
      reasoning_effort: '',
      timeout_seconds: 0,
      public_alias: '',
      publish_sample: false,
    },
  },
  {
    id: 2,
    name: 'Claude · Direct',
    groups: ['Claude', 'Premium'],
    models: ['claude-sonnet-4'],
    status: 1,
    supported: true,
    unsupported_reason: '',
    profile: {
      channel_id: 2,
      version: 1,
      enabled: true,
      model: 'claude-sonnet-4',
      protocol: 'claude',
      prompt_version_id: 1,
      reasoning_effort: '',
      timeout_seconds: 0,
      public_alias: '',
      publish_sample: false,
    },
  },
  {
    id: 3,
    name: 'Unavailable protocol · channel remains visible',
    groups: ['Experimental'],
    models: ['text-model'],
    status: 1,
    supported: false,
    unsupported_reason: 'unsupported_protocol',
    profile: {
      channel_id: 3,
      version: 0,
      enabled: false,
      model: '',
      protocol: 'chat',
      prompt_version_id: 0,
      reasoning_effort: '',
      timeout_seconds: 0,
      public_alias: '',
      publish_sample: false,
    },
  },
]
const prompts = [
  {
    id: 1,
    key: 'builtin-visual',
    name: 'Visual task',
    content: 'Create a self-contained HTML page.',
    rule: 'visual',
    archived: false,
    version: 1,
    created_at: 1,
  },
]
const settings = {
  version: 1,
  enabled: false,
  interval_minutes: 60,
  concurrency: 3,
  timeout_seconds: 600,
  max_output_tokens: 8192,
  daily_call_limit: 500,
  daily_token_limit: 5000000,
  retention_days: 14,
  status_page_enabled: false,
  public_groups: [],
  renderer_available: false,
}
const mutations = []
const unknown = []
const errors = []
await context.route('**/*', async (route) => {
  const request = route.request()
  const url = new URL(request.url())
  if (url.origin !== new URL(baseURL).origin) return route.abort()
  if (!url.pathname.startsWith('/api/')) return route.continue()
  let data
  switch (url.pathname) {
    case '/api/user/auth/refresh':
      data = {
        access_token: 'local-browser-fixture',
        token_type: 'Bearer',
        access_expires_at: 4102444800,
        user: {
          id: 1,
          username: 'admin',
          role: 100,
          status: 1,
          language: 'en',
        },
        session: {
          sid: 'fixture',
          current: true,
          login_method: 'password',
          ip: '127.0.0.1',
          user_agent: 'fixture',
          created_at: 1,
          last_active_at: 1,
          expires_at: 4102444800,
        },
      }
      break
    case '/api/status':
      data = {
        system_name: 'New API',
        setup: true,
        version: 'ui-regression',
        status_page_enabled: true,
      }
      break
    case '/api/notice':
      data = ''
      break
    case '/api/setup':
      data = { status: true }
      break
    case '/api/user/self':
      data = { id: 1, username: 'admin', role: 100, status: 1, language: 'en' }
      break
    case '/api/quality-detection/channels':
      data = channels
      break
    case '/api/quality-detection/channels/1': {
      assert.equal(request.method(), 'PUT')
      const profile = request.postDataJSON()
      mutations.push({ path: url.pathname, profile })
      channels = channels.map((channel) =>
        channel.id === 1
          ? { ...channel, profile: { ...profile, version: 1 } }
          : channel
      )
      data = channels[0].profile
      break
    }
    case '/api/quality-detection/prompts':
      data = prompts
      break
    case '/api/quality-detection/batches':
      assert.equal(
        request.method(),
        'GET',
        'No model batch should start during UI inspection'
      )
      data = { items: [], total: 0, page: 1, page_size: 20 }
      break
    case '/api/quality-detection/settings':
      assert.equal(
        request.method(),
        'GET',
        'Settings should not be saved during scroll inspection'
      )
      data = settings
      break
    default:
      unknown.push(url.pathname)
      data = []
  }
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ success: true, data }),
  })
})
const page = await context.newPage()
page.on('pageerror', (error) => errors.push(error.message))
page.on('console', (message) => {
  if (message.type() === 'error') {
    console.error('Browser console:', message.text())
  }
})
page.on('requestfailed', (request) =>
  console.error('Failed request:', request.url(), request.failure()?.errorText)
)
try {
  await page.goto(`${baseURL}/quality-detection`)
  await page
    .getByRole('checkbox', { name: 'OpenAI · Primary', exact: true })
    .waitFor()
  assert.equal(
    await page
      .getByRole('checkbox', { name: 'OpenAI · Primary', exact: true })
      .getAttribute('aria-disabled'),
    'true'
  )
  await page.screenshot({
    animations: 'disabled',
    path: path.join(tmpdir(), 'new-api-quality-desktop.png'),
  })
  await page.getByRole('combobox', { name: 'Filter channels by group' }).click()
  await page.getByRole('option', { name: 'Premium', exact: true }).waitFor()
  const popup = page.locator('[data-slot="combobox-content"]')
  assert.notEqual(
    await popup.evaluate(
      (element) => getComputedStyle(element).backgroundColor
    ),
    'rgb(255, 255, 255)'
  )
  await page.screenshot({
    animations: 'disabled',
    path: path.join(tmpdir(), 'new-api-quality-dropdown.png'),
  })
  await page.getByRole('combobox', { name: 'Search...' }).fill('Premium')
  await page.getByRole('option', { name: 'Premium', exact: true }).click()
  assert.equal(await page.getByRole('checkbox').count(), 2)
  await page
    .getByRole('button', { name: 'Configure: OpenAI · Primary', exact: true })
    .click()
  const sheet = page.getByRole('dialog')
  await sheet.getByRole('combobox', { name: 'Detection model' }).click()
  await page.getByRole('option', { name: 'gpt-5', exact: true }).click()
  await sheet.getByRole('combobox', { name: 'Prompt version' }).click()
  await page
    .getByRole('option', { name: 'Visual task · v1', exact: true })
    .click()
  await sheet
    .getByRole('switch', { name: 'Enable detection for this channel' })
    .click()
  await sheet.getByRole('button', { name: 'Save changes' }).click()
  await sheet.waitFor({ state: 'hidden' })
  await page
    .getByRole('checkbox', { name: 'OpenAI · Primary', exact: true })
    .click()
  assert.equal(
    await page
      .getByRole('button', { name: 'Run detection', exact: true })
      .isEnabled(),
    true
  )
  assert.equal(mutations.length, 1)
  assert.equal(mutations[0].profile.prompt_version_id, 1)
  assert.equal(mutations[0].profile.model, 'gpt-5')
  for (const viewport of [
    { width: 1440, height: 700 },
    { width: 390, height: 844 },
  ]) {
    await page.setViewportSize(viewport)
    await page.getByRole('tab', { name: 'Settings', exact: true }).click()
    const save = page.getByRole('button', { name: 'Save changes', exact: true })
    await save.waitFor()
    await page
      .getByRole('button', { name: 'Renderer setup', exact: true })
      .click()
    for (let i = 0; i < 4; i++) {
      await page.getByRole('button', { name: 'Add public group' }).click()
    }
    await save.scrollIntoViewIfNeeded()
    const scroll = await save.evaluate((element) => {
      const region = element.closest('.workspace-page-content')
      const r = element.getBoundingClientRect()
      return {
        top: region.scrollTop,
        height: region.clientHeight,
        contentHeight: region.scrollHeight,
        bottom: r.bottom,
        viewport: window.innerHeight,
        overflow: region.scrollWidth > region.clientWidth,
      }
    })
    assert.ok(
      scroll.contentHeight > scroll.height,
      'Long settings must have scrollable content'
    )
    assert.ok(scroll.top > 0, 'Settings scroll must move')
    assert.ok(
      scroll.bottom <= scroll.viewport,
      'Save button must be reachable in viewport'
    )
    assert.equal(scroll.overflow, false, 'No horizontal page overflow')
    await save.focus()
    assert.equal(
      await save.evaluate((element) => document.activeElement === element),
      true
    )
    await page.screenshot({
      animations: 'disabled',
      path: path.join(
        tmpdir(),
        `new-api-quality-settings-${viewport.width}.png`
      ),
    })
    await page.getByRole('tab', { name: 'Channels', exact: true }).click()
    await page
      .getByRole('heading', { name: 'Channels', exact: true })
      .scrollIntoViewIfNeeded()
    assert.equal(
      await page
        .locator('.workspace-page-content')
        .evaluate((element) => element.scrollWidth > element.clientWidth),
      false
    )
    await page.screenshot({
      animations: 'disabled',
      path: path.join(
        tmpdir(),
        `new-api-quality-channels-${viewport.width}.png`
      ),
    })
    await page
      .getByRole('button', { name: 'Configure: OpenAI · Primary', exact: true })
      .click()
    await sheet.getByRole('combobox', { name: 'Prompt version' }).click()
    const option = page.getByRole('option', {
      name: 'Visual task · v1',
      exact: true,
    })
    await option.waitFor()
    const optionBounds = await option.boundingBox()
    assert.ok(
      optionBounds &&
        optionBounds.y >= 0 &&
        optionBounds.y + optionBounds.height <= viewport.height,
      'Prompt menu stays inside the viewport'
    )
    await option.click()
    await page.keyboard.press('Escape')
    await sheet.waitFor({ state: 'hidden' })
  }
  assert.deepEqual(errors, [])
  assert.deepEqual(unknown, [])
  console.log(
    'PASS: channel setup, numeric prompt selection, themed searchable menu, selection without generation, desktop/mobile settings scroll, keyboard focus, no horizontal overflow.'
  )
  console.log(`Screenshots: ${tmpdir()}/new-api-quality-*.png`)
} catch (error) {
  await page.screenshot({
    animations: 'disabled',
    path: path.join(tmpdir(), 'new-api-quality-error.png'),
  })
  console.error('Page errors:', errors)
  throw error
} finally {
  await browser.close()
}
