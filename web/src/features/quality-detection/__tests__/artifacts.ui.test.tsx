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
import { act, configure, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  AxiosError,
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ControlledThumbnail } from '../components/ControlledThumbnail'
import { RunDetails } from '../components/RunDetails'
import { loadThumbnail } from '../lib/artifacts'
import { renderQuality, run } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('saved artifacts', () => {
  it('loads a saved screenshot after StrictMode cancels and remounts its initial request', async () => {
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL() {
          return 'blob:strict-mode-sample'
        }
        static revokeObjectURL() {}
      }
    )
    api.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: new Blob([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])], {
        type: 'image/png',
      }),
    })
    configure({ reactStrictMode: true })
    try {
      renderQuality(
        <ControlledThumbnail
          path='/api/status-page/artifacts/remounted-sample/thumbnail'
          label='Remounted sample'
        />
      )
      expect(
        await screen.findByRole('img', { name: 'Remounted sample' })
      ).toHaveAttribute('src', 'blob:strict-mode-sample')
    } finally {
      configure({ reactStrictMode: false })
    }
  })

  it('renders model HTML as inert text and preserves unknown usage', async () => {
    const output =
      '<script>document.title="changed"</script><h1>Untrusted output</h1>'
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: config.url?.endsWith('/output')
          ? output
          : { success: true, data: run },
      }
    }
    const user = userEvent.setup()
    const view = renderQuality(<RunDetails run={run} />)
    await user.click(
      screen.getByRole('button', { name: 'View sample: Channel A' })
    )
    await user.click(
      await screen.findByRole('button', { name: 'View HTML as text' })
    )
    expect(
      await screen.findByLabelText('Saved output as plain text')
    ).toHaveTextContent(output)
    expect(
      view.baseElement.querySelector('script, iframe')
    ).not.toBeInTheDocument()
    expect(screen.getAllByText('Unknown usage')).toHaveLength(2)
    expect(
      screen.getByRole('button', { name: 'Set as baseline' })
    ).toBeDisabled()
    expect(requests.some((request) => request.params?.kind === 'html')).toBe(
      true
    )
    expect(requests.every((request) => request.method === 'get')).toBe(true)
  })

  it('keeps polling a completed generation until its pending screenshot finishes', async () => {
    let renderStatus = 'pending'
    api.defaults.adapter = async (config) => ({
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: { success: true, data: { ...run, render_status: renderStatus } },
    })
    const user = userEvent.setup()
    const view = renderQuality(
      <RunDetails run={{ ...run, render_status: 'pending' }} />
    )
    await user.click(
      screen.getByRole('button', { name: 'View sample: Channel A' })
    )
    const statusLabel = await screen.findByText('Rendering')
    expect(statusLabel.nextElementSibling).toHaveTextContent(/pending|Pending/)
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] })
    try {
      // Re-fetch once with controlled intervals, then let the scheduled read finish rendering.
      await act(async () => {
        await view.client.invalidateQueries({
          queryKey: ['quality-detection', 'run'],
        })
      })
      renderStatus = 'completed'
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000)
      })
      await waitFor(() =>
        expect(statusLabel.nextElementSibling).toHaveTextContent(
          /completed|Completed/
        )
      )
    } finally {
      view.unmount()
      vi.useRealTimers()
    }
  })

  it('removes a previously loaded PNG immediately when access is revoked', async () => {
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL() {
          return 'blob:controlled-sample'
        }
        static revokeObjectURL() {}
      }
    )
    let revoked = false
    api.defaults.adapter = async (config) => {
      if (revoked) {
        throw new AxiosError('revoked', undefined, config, undefined, {
          status: 403,
          statusText: 'Forbidden',
          config,
          headers: {},
          data: {},
        })
      }
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
    const view = renderQuality(
      <ControlledThumbnail
        path='/api/status-page/artifacts/shared-sample/thumbnail'
        label='Shared sample'
      />
    )
    expect(
      await screen.findByRole('img', { name: 'Shared sample' })
    ).toHaveAttribute('src', 'blob:controlled-sample')
    revoked = true
    await act(async () => {
      await view.client.invalidateQueries({ queryKey: ['quality-thumbnail'] })
    })
    expect(await screen.findByText('Thumbnail unavailable')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('rejects executable thumbnail responses and external artifact paths', async () => {
    const adapter = vi.fn(async (config: InternalAxiosRequestConfig) => ({
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: new Blob(['<svg/>'], { type: 'image/svg+xml' }),
    }))
    api.defaults.adapter = adapter
    await expect(
      loadThumbnail('https://external.invalid/image.png')
    ).rejects.toThrow('Invalid artifact path')
    expect(adapter).not.toHaveBeenCalled()
    renderQuality(
      <ControlledThumbnail
        path='/api/quality-detection/runs/4/thumbnail'
        label='Sample'
      />
    )
    expect(await screen.findByText('Thumbnail unavailable')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    await waitFor(() => expect(adapter).toHaveBeenCalledTimes(1))
  })
})
