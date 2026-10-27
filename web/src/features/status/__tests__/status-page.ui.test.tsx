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
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  AxiosError,
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { renderQuality } from '@/features/quality-detection/__tests__/fixtures'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'

import { GroupDetails } from '../components/GroupDetails'
import { StatusTrend } from '../components/StatusTrend'
import { StatusContent } from '../index'
import type { StatusGroup, StatusPageData } from '../types'

const group: StatusGroup = {
  public_id: 'group-a',
  name: 'Public group with a deliberately long label',
  channel_count: 2,
  configured_count: 2,
  fresh_count: 1,
  state: 'stale',
  success_rate: null,
  avg_latency_ms: null,
  tokens_per_second: null,
  series: [
    { ts: 100, value: 90 },
    { ts: 200, value: null },
    { ts: 300, value: 95 },
  ],
  latest_sample: null,
}
const data: StatusPageData = {
  enabled: true,
  paused: true,
  generated_at: 500,
  metrics_data_until: 300,
  last_probe_completed_at: 200,
  window_start: 100,
  window_end: 400,
  bucket_seconds: 100,
  total: 1,
  page: 1,
  page_size: 20,
  models: ['model-a', 'model-b'],
  summary: { groups: 1, attention: 1, channels: 1 },
  groups: [group],
}
let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('native status page', () => {
  it('keeps pagination in natural page flow and lets its controls wrap on narrow screens', async () => {
    api.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: { success: true, data },
    })
    renderQuality(<StatusContent />)
    const pagination = await screen.findByRole('navigation', {
      name: 'Pagination',
    })
    const main = screen.getByRole('main')
    expect(main).toHaveClass('min-h-svh', 'min-w-0')
    expect(main).not.toHaveClass('overflow-hidden', 'h-svh')
    expect(main).toContainElement(pagination)
    expect(pagination).toHaveClass('flex-wrap')
    expect(
      within(pagination).getByRole('button', { name: 'Next' })
    ).toBeDisabled()
  })

  it('reads server summaries and saved times; changing filters and refreshing never posts a model task', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data },
      }
    }
    const user = userEvent.setup()
    renderQuality(<StatusContent />)
    expect(await screen.findByText(group.name)).toBeInTheDocument()
    expect(
      screen.getByText('Unique detected channels').nextElementSibling
    ).toHaveTextContent('1')
    expect(
      screen.getByText('Detection paused. Historical samples remain available.')
    ).toBeInTheDocument()
    expect(screen.getAllByText('No sample').length).toBeGreaterThanOrEqual(3)
    expect(screen.queryByText('0%')).not.toBeInTheDocument()
    expect(
      document.querySelector('time[datetime="1970-01-01T00:05:00.000Z"]')
    ).toBeInTheDocument()
    await user.click(screen.getByRole('combobox', { name: 'Model' }))
    await user.click(await screen.findByRole('option', { name: 'model-b' }))
    await screen.findByText('This model has no detection sample')
    await user.click(screen.getByRole('combobox', { name: 'Time window' }))
    await user.click(
      await screen.findByRole('option', { name: 'Last 6 hours' })
    )
    await waitFor(() =>
      expect(requests.at(-1)?.params).toMatchObject({
        model: 'model-b',
        hours: 6,
        page: 1,
      })
    )
    await screen.findByText(group.name)
    await user.click(screen.getByRole('combobox', { name: 'Sort groups' }))
    await user.click(
      await screen.findByRole('option', { name: 'Detection status' })
    )
    await waitFor(() =>
      expect(requests.at(-1)?.params).toMatchObject({ sort: 'state', page: 1 })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Refresh status' })
      ).toBeEnabled()
    )
    await user.click(screen.getByRole('button', { name: 'Refresh status' }))
    expect(
      requests.every(
        (request) =>
          request.method === 'get' && request.url === '/api/status-page'
      )
    ).toBe(true)
  })

  it('shows an honest empty result and updates the interface when the language changes', async () => {
    api.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: {
        success: true,
        data: {
          ...data,
          total: 0,
          groups: [],
          summary: { groups: 0, attention: 0, channels: 0 },
        },
      },
    })
    const view = renderQuality(<StatusContent />)
    expect(
      await screen.findByText('No saved data for this selection')
    ).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    view.i18n.addResourceBundle('zh', 'translation', zh.translation)
    await act(async () => {
      await view.i18n.changeLanguage('zh')
    })
    expect(
      screen.getByRole('heading', { name: zh.translation['Service Status'] })
    ).toBeInTheDocument()
    expect(
      screen.getByText(zh.translation['No saved data for this selection'])
    ).toBeInTheDocument()
  })

  it('retains stale data after a 500 but removes it immediately after a 403', async () => {
    let status = 200
    api.defaults.adapter = async (config) => {
      if (status !== 200) {
        throw new AxiosError('unavailable', undefined, config, undefined, {
          status,
          statusText: 'error',
          config,
          headers: {},
          data: {},
        })
      }
      return {
        status,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data },
      }
    }
    const user = userEvent.setup()
    renderQuality(<StatusContent />)
    await screen.findByText(group.name)
    status = 500
    await user.click(screen.getByRole('button', { name: 'Refresh status' }))
    expect(
      await screen.findByText('Refresh failed. Showing previously saved data.')
    ).toBeInTheDocument()
    expect(screen.getByText(group.name)).toBeInTheDocument()
    status = 403
    await user.click(screen.getByRole('button', { name: 'Refresh status' }))
    await screen.findByText('This content is unavailable or access was revoked')
    await waitFor(() =>
      expect(screen.queryByText(group.name)).not.toBeInTheDocument()
    )
  })

  it('opens a full-width mobile detail sheet with keyboard focus restoration', async () => {
    api.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: {
        success: true,
        data: { group, samples: [], total: 0, page: 1, page_size: 6 },
      },
    })
    const user = userEvent.setup()
    renderQuality(
      <GroupDetails
        group={group}
        hours={24}
        model=''
        start={100}
        end={400}
        generatedAt={500}
      />
    )
    const trigger = screen.getByRole('button', {
      name: `View details: ${group.name}`,
    })
    trigger.focus()
    await user.keyboard('{Enter}')
    const dialog = await screen.findByRole('dialog', { name: group.name })
    expect(dialog).toHaveClass(
      'w-full',
      'sm:max-w-2xl',
      'motion-reduce:transition-none'
    )
    await within(dialog).findByText('Detection history')
    expect(
      within(dialog).getByRole('region', { name: 'Group details' })
    ).toHaveClass('min-h-0', 'overflow-y-auto')
    expect(dialog).toContainElement(document.activeElement as HTMLElement)
    await user.keyboard('{Escape}')
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(trigger).toHaveFocus()
  })

  it('breaks the success-rate line at null buckets and exposes keyboard-readable recorded values', async () => {
    const user = userEvent.setup()
    renderQuality(
      <StatusTrend series={group.series} start={100} end={400} detail />
    )
    const chart = screen.getByRole('img', {
      name: 'Business success rate trend',
    })
    expect(
      chart.querySelector('path')?.getAttribute('d')?.match(/M/g)
    ).toHaveLength(2)
    expect(chart.querySelector('path')?.getAttribute('d')).not.toContain('L')
    const toggle = screen.getByText('View recorded data points')
    await user.click(toggle)
    expect(screen.getByRole('table')).toBeVisible()
    expect(screen.getByRole('cell', { name: 'No sample' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: '95%' })).toBeInTheDocument()
  })
})
