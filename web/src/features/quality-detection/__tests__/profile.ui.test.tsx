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
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { ProfileForm } from '../components/ProfileForm'
import { channel, prompt, renderQuality } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('profile configuration', () => {
  it('preserves the channel version and zero timeout without copying settings by group', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data: channel.profile },
      }
    }
    const user = userEvent.setup()
    renderQuality(
      <ProfileForm channel={channel} prompts={[prompt]} onSaved={vi.fn()} />
    )
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].url).toBe('/api/quality-detection/channels/1')
    expect(JSON.parse(requests[0].data)).toMatchObject({
      channel_id: 1,
      version: 1,
      model: 'model-a',
      timeout_seconds: 0,
      prompt_version_id: 1,
    })
    expect(JSON.parse(requests[0].data)).not.toHaveProperty('groups')
  })

  it('allows an existing unsupported profile to be disabled but never enabled again', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data: null },
      }
    }
    const user = userEvent.setup()
    renderQuality(
      <ProfileForm
        channel={{ ...channel, supported: false }}
        prompts={[prompt]}
        onSaved={vi.fn()}
      />
    )
    const toggle = screen.getByRole('switch', {
      name: 'Enable detection for this channel',
    })
    expect(toggle).not.toHaveAttribute('aria-disabled', 'true')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    expect(toggle).toHaveAttribute('aria-disabled', 'true')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(JSON.parse(requests[0].data).enabled).toBe(false)
    )
  })
})
