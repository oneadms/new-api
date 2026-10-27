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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { renderQuality } from '@/features/quality-detection/__tests__/fixtures'
import { api } from '@/lib/api'

import { GroupDetails } from '../components/GroupDetails'
import type { StatusGroup, StatusSample } from '../types'

const samples: StatusSample[] = [700, 600, 500, 400, 300, 200, 100].map(
  (time) => ({
    public_id: `sample-${time}`,
    channel_name: 'Shared public channel',
    model: 'model-a',
    status: 'completed',
    quality_result: 'needs_review',
    completed_at: time,
    stale_after: time + 60,
    first_text_ms: 10,
    generation_ms: 2000,
    thumbnail_url: `/api/status-page/artifacts/sample-${time}/thumbnail`,
    render_status: 'completed',
  })
)
const group: StatusGroup = {
  public_id: 'group-a',
  name: 'Standard',
  channel_count: 1,
  configured_count: 1,
  fresh_count: 1,
  state: 'review',
  success_rate: null,
  avg_latency_ms: null,
  tokens_per_second: null,
  series: [],
  latest_sample: samples[0],
}
let previous: AxiosAdapter | AxiosAdapter[] | undefined
let requests: InternalAxiosRequestConfig[]

function historyResponse(config: InternalAxiosRequestConfig, values = samples) {
  const page = Number(config.params?.page ?? 1)
  return {
    status: 200,
    statusText: 'OK',
    config,
    headers: {},
    data: {
      success: true,
      data: {
        group,
        samples: values.slice((page - 1) * 6, page * 6),
        total: values.length,
        page,
        page_size: 6,
      },
    },
  }
}

function thumbnailResponse(config: InternalAxiosRequestConfig) {
  return {
    status: 200,
    statusText: 'OK',
    config,
    headers: {},
    data: new Blob([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])], {
      type: 'image/png',
    }),
  }
}

function renderDetails() {
  return renderQuality(
    <GroupDetails
      group={group}
      hours={24}
      model='model-a'
      start={100}
      end={800}
      generatedAt={800}
    />
  )
}

beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
  requests = []
  let imageNumber = 0
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL() {
        return `blob:history-${++imageNumber}`
      }
      static revokeObjectURL() {}
    }
  )
  api.defaults.adapter = async (config) => {
    requests.push(config)
    return config.responseType === 'blob'
      ? thumbnailResponse(config)
      : historyResponse(config)
  }
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('public detection history', () => {
  it('loads on opening and paginates several saved images from the same channel without creating a task', async () => {
    const user = userEvent.setup()
    renderDetails()
    expect(requests).toHaveLength(0)
    await user.click(
      screen.getByRole('button', { name: 'View details: Standard' })
    )
    const history = await screen.findByRole('list', {
      name: 'Detection history',
    })
    await waitFor(() =>
      expect(within(history).getAllByRole('img')).toHaveLength(6)
    )
    expect(
      within(history)
        .getAllByRole('listitem')
        .map((item) => item.querySelector('time')?.dateTime)
    ).toEqual(
      samples
        .slice(0, 6)
        .map((sample) => new Date(sample.completed_at * 1000).toISOString())
    )
    expect(
      requests.find((request) => request.url?.includes('/groups/'))?.params
    ).toMatchObject({
      hours: 24,
      model: 'model-a',
      page: 1,
      page_size: 6,
    })
    const pagination = screen.getByRole('navigation', {
      name: 'Detection history pagination',
    })
    expect(
      within(pagination).getByRole('button', { name: 'Previous' })
    ).toBeDisabled()
    await user.click(within(pagination).getByRole('button', { name: 'Next' }))
    expect(await screen.findByText('Page 2 of 2')).toBeInTheDocument()
    await waitFor(() =>
      expect(within(history).getAllByRole('img')).toHaveLength(1)
    )
    expect(
      within(history).getByRole('listitem').querySelector('time')
    ).toHaveAttribute('datetime', '1970-01-01T00:01:40.000Z')
    expect(
      within(pagination).getByRole('button', { name: 'Next' })
    ).toBeDisabled()
    expect(
      screen.getByRole('heading', { name: 'Detection history' })
    ).toHaveFocus()
    await user.click(
      within(pagination).getByRole('button', { name: 'Previous' })
    )
    await screen.findByText('Page 1 of 2')
    expect(
      requests.some((request) => request.url?.endsWith('/sample-100/thumbnail'))
    ).toBe(true)
    expect(
      requests.every(
        (request) =>
          request.method === 'get' &&
          request.url?.startsWith('/api/status-page/')
      )
    ).toBe(true)
  })

  it('hides the previous page images and disables pagination while older history is loading', async () => {
    let resolvePage:
      | ((value: ReturnType<typeof historyResponse>) => void)
      | undefined
    let pendingConfig: InternalAxiosRequestConfig | undefined
    api.defaults.adapter = async (config) => {
      if (config.responseType === 'blob') return thumbnailResponse(config)
      if (config.params?.page === 2) {
        pendingConfig = config
        return new Promise((resolve) => {
          resolvePage = resolve
        })
      }
      return historyResponse(config)
    }
    const user = userEvent.setup()
    renderDetails()
    await user.click(
      screen.getByRole('button', { name: 'View details: Standard' })
    )
    await screen.findByRole('list', { name: 'Detection history' })
    await user.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(resolvePage).toBeDefined())
    expect(
      screen.getByRole('status', { name: 'Loading...' })
    ).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    if (!pendingConfig || !resolvePage) {
      throw new Error('History page request was not started')
    }
    const response = historyResponse(pendingConfig)
    const finishRequest = resolvePage
    await act(async () => {
      finishRequest(response)
    })
    await screen.findByText('Page 2 of 2')
    expect(screen.getByRole('button', { name: 'Previous' })).toBeEnabled()
  })

  it('returns to the newest history after closing and reopening with keyboard focus preserved', async () => {
    const user = userEvent.setup()
    renderDetails()
    const trigger = screen.getByRole('button', {
      name: 'View details: Standard',
    })
    trigger.focus()
    await user.keyboard('{Enter}')
    await screen.findByRole('list', { name: 'Detection history' })
    await user.click(screen.getByRole('button', { name: 'Next' }))
    await screen.findByText('Page 2 of 2')
    await user.keyboard('{Escape}')
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(trigger).toHaveFocus()
    await user.keyboard('{Enter}')
    await screen.findByText('Page 1 of 2')
    const region = screen.getByRole('region', { name: 'Group details' })
    expect(region).toHaveClass('min-h-0', 'overflow-y-auto')
    expect(
      screen.getByRole('navigation', { name: 'Detection history pagination' })
    ).toHaveClass('flex-wrap')
  })

  it('shows empty history without inventing an image or allowing a next page', async () => {
    api.defaults.adapter = async (config) => historyResponse(config, [])
    const user = userEvent.setup()
    renderDetails()
    await user.click(
      screen.getByRole('button', { name: 'View details: Standard' })
    )
    expect(
      await screen.findByText('No saved data for this selection')
    ).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
  })

  it('keeps a historical detection visible when its screenshot was never generated', async () => {
    api.defaults.adapter = async (config) =>
      historyResponse(config, [
        {
          ...samples[0],
          thumbnail_url: null,
          render_status: 'unavailable',
        },
      ])
    const user = userEvent.setup()
    renderDetails()
    await user.click(
      screen.getByRole('button', { name: 'View details: Standard' })
    )
    expect(await screen.findByText('Thumbnail unavailable')).toBeInTheDocument()
    expect(screen.getByText('Shared public channel')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  it('removes loaded historical images when the group publication is revoked', async () => {
    let revoked = false
    api.defaults.adapter = async (config) => {
      if (revoked) {
        throw new AxiosError('revoked', undefined, config, undefined, {
          status: 404,
          statusText: 'Not Found',
          config,
          headers: {},
          data: {},
        })
      }
      return config.responseType === 'blob'
        ? thumbnailResponse(config)
        : historyResponse(config)
    }
    const user = userEvent.setup()
    const view = renderDetails()
    await user.click(
      screen.getByRole('button', { name: 'View details: Standard' })
    )
    await waitFor(() => expect(screen.getAllByRole('img')).toHaveLength(6))
    revoked = true
    await act(async () => {
      await view.client.invalidateQueries({
        queryKey: ['status-page', 'group'],
      })
    })
    expect(
      await screen.findByText(
        'This content is unavailable or access was revoked'
      )
    ).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.queryByText('Shared public channel')).not.toBeInTheDocument()
  })

  it('allows returning to the previous page after an older-history request fails', async () => {
    api.defaults.adapter = async (config) => {
      if (config.responseType === 'blob') return thumbnailResponse(config)
      if (config.params?.page === 2) {
        throw new AxiosError('unavailable', undefined, config, undefined, {
          status: 500,
          statusText: 'Server Error',
          config,
          headers: {},
          data: {},
        })
      }
      return historyResponse(config)
    }
    const user = userEvent.setup()
    renderDetails()
    await user.click(
      screen.getByRole('button', { name: 'View details: Standard' })
    )
    await screen.findByRole('list', { name: 'Detection history' })
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(
      await screen.findByText('Unable to load saved data')
    ).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Previous' }))
    await screen.findByText('Page 1 of 2')
  })
})
