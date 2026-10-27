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
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  AxiosError,
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { RunBatch } from '../components/RunBatch'
import { channel, prompt, renderQuality } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('manual batches', () => {
  it('deduplicates shared channels, confirms cost and reuses the key only for retries', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      if (requests.length === 1) {
        throw new AxiosError('offline', 'ERR_NETWORK', config)
      }
      return {
        status: 202,
        statusText: 'Accepted',
        headers: {},
        config,
        data: { success: true, data: { id: requests.length } },
      }
    }
    const created = vi.fn()
    const user = userEvent.setup()
    const second = {
      ...channel,
      id: 2,
      name: 'Channel B',
      groups: ['premium'],
      profile: { ...channel.profile, channel_id: 2 },
    }
    renderQuality(
      <RunBatch
        prompts={[prompt]}
        channels={[channel, second, channel]}
        blocked={false}
        onCreated={created}
      />
    )
    await user.click(
      screen.getByRole('button', { name: 'Select filtered channels' })
    )
    expect(screen.getByText('2 unique channels selected')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Run detection' }))
    expect(requests).toHaveLength(0)
    const dialog = await screen.findByRole('alertdialog')
    expect(
      within(dialog).getByText(/2 groups reference 2 unique channels/)
    ).toBeInTheDocument()
    await user.click(
      within(dialog).getByRole('button', { name: 'Confirm and run' })
    )
    await screen.findByText('Unable to load saved data')
    await user.click(
      within(dialog).getByRole('button', { name: 'Confirm and run' })
    )
    await waitFor(() => expect(created).toHaveBeenCalledWith(2))
    expect(JSON.parse(requests[0].data)).toEqual({ channel_ids: [1, 2] })
    expect(requests[0].headers.get('Idempotency-Key')).toBe(
      requests[1].headers.get('Idempotency-Key')
    )
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    await user.click(screen.getByRole('button', { name: 'Run detection' }))
    await user.click(screen.getByRole('button', { name: 'Confirm and run' }))
    await waitFor(() => expect(created).toHaveBeenCalledWith(3))
    expect(requests[2].headers.get('Idempotency-Key')).not.toBe(
      requests[0].headers.get('Idempotency-Key')
    )
  })

  it('keeps empty, unsupported and running-batch submissions disabled', async () => {
    const user = userEvent.setup()
    const view = renderQuality(
      <RunBatch
        prompts={[prompt]}
        channels={[{ ...channel, supported: false }]}
        blocked={false}
        onCreated={vi.fn()}
      />
    )
    expect(screen.getByRole('button', { name: 'Run detection' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Channel A' })).toHaveAttribute(
      'aria-disabled',
      'true'
    )
    view.unmount()
    renderQuality(
      <RunBatch
        channels={[channel]}
        prompts={[prompt]}
        blocked
        onCreated={vi.fn()}
      />
    )
    await user.click(
      screen.getByRole('button', { name: 'Select filtered channels' })
    )
    expect(screen.getByRole('button', { name: 'Run detection' })).toBeDisabled()
  })
})
