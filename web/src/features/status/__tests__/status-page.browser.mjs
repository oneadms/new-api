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
// Run against the local frontend: node src/features/status/__tests__/status-page.browser.mjs
// Uses the renderer package's existing Playwright install. All API calls are intercepted.
import assert from 'node:assert/strict'
import { tmpdir } from 'node:os'
import path from 'node:path'

import { chromium } from '../../../../../tools/quality-renderer/node_modules/playwright/index.mjs'

const baseURL = process.env.QUALITY_UI_URL ?? 'http://127.0.0.1:5173'
assert.ok(['localhost', '127.0.0.1'].includes(new URL(baseURL).hostname))
const browser = await chromium.launch({
  channel:
    process.env.QUALITY_UI_BROWSER ??
    (process.platform === 'win32' ? 'msedge' : undefined),
  headless: true,
})
const longModel =
  'provider/model-with-a-long-name-and-explicit-capability-suffix-2026-10-27'
const group = {
  public_id: 'standard',
  name: 'Standard · Shared channels with a deliberately long public group name',
  channel_count: 2,
  configured_count: 2,
  fresh_count: 0,
  state: 'unknown',
  success_rate: null,
  avg_latency_ms: null,
  tokens_per_second: null,
  series: [],
  latest_sample: null,
}
const data = {
  enabled: true,
  paused: false,
  generated_at: 1793062800,
  metrics_data_until: 1793062500,
  last_probe_completed_at: 0,
  window_start: 1792976400,
  window_end: 1793062800,
  bucket_seconds: 300,
  total: 2,
  page: 1,
  page_size: 20,
  models: ['model-a', longModel],
  summary: { groups: 2, attention: 0, channels: 2 },
  groups: [group, { ...group, public_id: 'premium', name: 'Premium' }],
}
const samples = [700, 600, 500, 400, 300, 200, 100].map((offset) => ({
  public_id: `history-${offset}`,
  channel_name:
    'Shared public channel with a deliberately long historical label',
  model: longModel,
  status: 'completed',
  quality_result: 'needs_review',
  completed_at: data.generated_at - 86400 + offset,
  stale_after: data.generated_at - 43200,
  first_text_ms: 24,
  generation_ms: 2400,
  thumbnail_url: `/api/status-page/artifacts/history-${offset}/thumbnail`,
  render_status: 'completed',
}))
const errors = []
const unexpected = []
const requests = []
try {
  const fixtureContext = await browser.newContext({
    viewport: { width: 1200, height: 800 },
  })
  const fixture = await fixtureContext.newPage()
  await fixture.setContent(
    '<html><body style="margin:0;background:#f4f5f7;color:#202936;font:48px sans-serif;padding:72px"><h1>Saved detection</h1><p>Historical HTML screenshot</p><svg width="900" height="280"><rect x="0" y="120" width="180" height="160" rx="16" fill="#8594ad"/><rect x="220" y="70" width="180" height="210" rx="16" fill="#60758f"/><rect x="440" y="0" width="180" height="280" rx="16" fill="#415973"/></svg></body></html>'
  )
  const samplePNG = await fixture.screenshot({ type: 'png' })
  await fixtureContext.close()
  for (const scenario of [
    { width: 1440, height: 900, theme: 'dark' },
    { width: 390, height: 844, theme: 'light' },
  ]) {
    const context = await browser.newContext({
      viewport: { width: scenario.width, height: scenario.height },
      colorScheme: scenario.theme,
      reducedMotion: 'reduce',
    })
    await context.addCookies([
      { name: 'vite-ui-theme', value: scenario.theme, url: baseURL },
      { name: 'theme_preset', value: 'simple-large', url: baseURL },
      { name: 'theme_font', value: 'sans', url: baseURL },
    ])
    await context.addInitScript(() => {
      localStorage.setItem('i18nextLng', 'en')
      localStorage.setItem('setup_status_checked', 'true')
    })
    await context.route('**/*', async (route) => {
      const request = route.request()
      const url = new URL(request.url())
      if (url.origin !== new URL(baseURL).origin) return route.abort()
      if (!url.pathname.startsWith('/api/')) return route.continue()
      if (url.pathname === '/api/user/auth/refresh') {
        return route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: '{"success":false}',
        })
      }
      requests.push({
        path: url.pathname,
        method: request.method(),
        query: url.search,
      })
      if (
        url.pathname.startsWith('/api/status-page/artifacts/') &&
        url.pathname.endsWith('/thumbnail')
      ) {
        return route.fulfill({
          status: 200,
          contentType: 'image/png',
          body: samplePNG,
        })
      }
      let response
      switch (url.pathname) {
        case '/api/status':
          response = {
            system_name: 'New API',
            setup: true,
            status_page_enabled: true,
          }
          break
        case '/api/notice':
          response = ''
          break
        case '/api/setup':
          response = { status: true }
          break
        case '/api/status-page':
          response = data
          break
        case '/api/status-page/groups/standard': {
          const historyPage = Number(url.searchParams.get('page') ?? '1')
          assert.equal(url.searchParams.get('page_size'), '6')
          response = {
            group,
            samples: samples.slice((historyPage - 1) * 6, historyPage * 6),
            total: samples.length,
            page: historyPage,
            page_size: 6,
          }
          break
        }
        default:
          unexpected.push(url.pathname)
          return route.abort()
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: response }),
      })
    })
    const page = await context.newPage()
    const consoleErrors = []
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text())
    })
    page.on('pageerror', (error) => errors.push(error.message))
    try {
      await page.goto(`${baseURL}/status`)
      await page
        .getByRole('heading', { name: group.name, exact: true })
        .waitFor()
      await page.getByRole('combobox', { name: 'Model', exact: true }).click()
      const search = page.getByRole('combobox', {
        name: 'Search...',
        exact: true,
      })
      await search.fill('not-an-available-model')
      await page.getByText('No results found', { exact: true }).waitFor()
      await search.fill('provider/')
      const option = page.getByRole('option', { name: longModel, exact: true })
      await option.waitFor()
      const popup = page.locator('[data-slot="combobox-content"]')
      const bounds = await popup.boundingBox()
      assert.ok(
        bounds && bounds.x >= 0 && bounds.x + bounds.width <= scenario.width,
        'Long model menus stay inside the viewport'
      )
      assert.equal(
        await popup.evaluate(
          (element) => element.scrollWidth > element.clientWidth
        ),
        false
      )
      if (scenario.theme === 'dark') {
        assert.notEqual(
          await popup.evaluate(
            (element) => getComputedStyle(element).backgroundColor
          ),
          'rgb(255, 255, 255)'
        )
      }
      await option.click()
      await page
        .getByRole('heading', { name: group.name, exact: true })
        .waitFor()
      await page
        .getByRole('combobox', { name: 'Time window', exact: true })
        .click()
      await page
        .getByRole('option', { name: 'Last 6 hours', exact: true })
        .click()
      await page
        .getByRole('heading', { name: group.name, exact: true })
        .waitFor()
      assert.ok(
        requests.some(
          (request) =>
            new URLSearchParams(request.query).get('hours') === '6' &&
            new URLSearchParams(request.query).get('model') === longModel
        )
      )
      const refresh = page.getByRole('button', {
        name: 'Refresh status',
        exact: true,
      })
      await refresh.waitFor()
      await Promise.all([
        page.waitForResponse(
          (response) => new URL(response.url()).pathname === '/api/status-page'
        ),
        refresh.click(),
      ])
      await page
        .getByRole('heading', { name: group.name, exact: true })
        .waitFor()
      const detailsTrigger = page.getByRole('button', {
        name: `View details: ${group.name}`,
        exact: true,
      })
      await detailsTrigger.click()
      const dialog = page.getByRole('dialog', { name: group.name, exact: true })
      const history = dialog.getByRole('list', {
        name: 'Detection history',
        exact: true,
      })
      await history.getByRole('img').nth(5).waitFor()
      assert.equal(await history.getByRole('img').count(), 6)
      assert.deepEqual(
        await history
          .locator('time')
          .evaluateAll((elements) =>
            elements.map((element) => element.dateTime)
          ),
        samples
          .slice(0, 6)
          .map((sample) => new Date(sample.completed_at * 1000).toISOString())
      )
      await dialog
        .getByRole('heading', { name: 'Detection history', exact: true })
        .scrollIntoViewIfNeeded()
      const firstImage = await history.getByRole('img').first().boundingBox()
      assert.ok(
        firstImage &&
          firstImage.y < scenario.height &&
          firstImage.y + firstImage.height > 0,
        'Opening details shows historical screenshots before business metrics'
      )
      await page.screenshot({
        animations: 'disabled',
        path: path.join(
          tmpdir(),
          `new-api-status-history-${scenario.width}.png`
        ),
      })
      const historyPagination = dialog.getByRole('navigation', {
        name: 'Detection history pagination',
        exact: true,
      })
      const older = historyPagination.getByRole('button', {
        name: 'Next',
        exact: true,
      })
      await older.scrollIntoViewIfNeeded()
      const historyLayout = await older.evaluate((element) => ({
        bottom: element.getBoundingClientRect().bottom,
        height: window.innerHeight,
        scroll: element.closest('[role="region"]').scrollTop,
        overflow:
          element.closest('[role="dialog"]').scrollWidth >
          element.closest('[role="dialog"]').clientWidth,
      }))
      assert.ok(
        historyLayout.bottom <= historyLayout.height,
        'History pagination remains reachable inside the detail panel'
      )
      assert.ok(
        historyLayout.scroll > 0,
        'Historical images scroll within the detail panel'
      )
      assert.equal(
        historyLayout.overflow,
        false,
        'Historical samples and long names never widen the detail panel'
      )
      await older.click()
      await dialog.getByText('Page 2 of 2', { exact: true }).waitFor()
      await history.getByRole('img').first().waitFor()
      assert.equal(await history.getByRole('img').count(), 1)
      assert.equal(
        await history.locator('time').first().getAttribute('datetime'),
        new Date(samples[6].completed_at * 1000).toISOString()
      )
      assert.equal(await older.isDisabled(), true)
      assert.ok(
        await dialog
          .getByRole('heading', { name: 'Detection history', exact: true })
          .evaluate((element) => element === document.activeElement)
      )
      await page.keyboard.press('Escape')
      await dialog.waitFor({ state: 'hidden' })
      assert.ok(
        await detailsTrigger.evaluate(
          (element) => element === document.activeElement
        )
      )
      await page
        .getByRole('heading', { name: 'Service Status', exact: true })
        .scrollIntoViewIfNeeded()
      await page.screenshot({
        animations: 'disabled',
        path: path.join(tmpdir(), `new-api-status-${scenario.width}.png`),
      })
      const next = page.getByRole('button', { name: 'Next', exact: true })
      await next.scrollIntoViewIfNeeded()
      assert.equal(await next.isDisabled(), true)
      const layout = await next.evaluate((element) => ({
        bottom: element.getBoundingClientRect().bottom,
        height: window.innerHeight,
        scroll: window.scrollY,
        overflow: document.documentElement.scrollWidth > window.innerWidth,
      }))
      assert.ok(layout.bottom <= layout.height, 'Pagination remains reachable')
      assert.equal(
        layout.overflow,
        false,
        'Long labels never widen the public page'
      )
      if (scenario.width === 390) {
        assert.ok(layout.scroll > 0, 'Mobile page scrolls to pagination')
      }
    } catch (error) {
      console.error({
        consoleErrors,
        unexpected,
        historyRequests: requests.filter((request) =>
          request.path.startsWith('/api/status-page/')
        ),
        images: await page
          .locator('[role="dialog"] img')
          .evaluateAll((images) =>
            images.map((image) => ({
              src: image.src,
              complete: image.complete,
              width: image.naturalWidth,
              height: image.getBoundingClientRect().height,
            }))
          ),
      })
      await page.screenshot({
        path: path.join(tmpdir(), 'new-api-status-error.png'),
      })
      throw error
    } finally {
      await context.close()
    }
  }
  assert.deepEqual(errors, [])
  assert.deepEqual(unexpected, [])
  assert.ok(
    requests.length > 0 &&
      requests.every((request) => request.method === 'GET'),
    'Public filtering and refresh never create a model task'
  )
  console.log(
    'PASS: public group display, themed menus, model/time filters, historical PNG pagination and chronological order, mobile panel scrolling, focus restoration, read-only requests.'
  )
  console.log(`Screenshots: ${tmpdir()}/new-api-status-*.png`)
} finally {
  await browser.close()
}
