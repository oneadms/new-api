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
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPanel } from '../components/SettingsPanel'
import { channel, renderQuality, settings } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('quality settings', () => {
  it('rejects invalid intervals and sends a versioned configuration without renderer availability', async () => {
    const writes: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      if (config.method === 'put') writes.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data: settings },
      }
    }
    const user = userEvent.setup()
    renderQuality(<SettingsPanel channels={[channel]} />)
    expect(
      await screen.findByText('Renderer configuration is missing or invalid.')
    ).toBeInTheDocument()
    const interval = screen.getByRole('spinbutton', {
      name: 'Interval (minutes)',
    })
    await user.clear(interval)
    await user.type(interval, '4')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(writes).toHaveLength(0)
    await user.clear(interval)
    await user.type(interval, '5')
    await user.click(
      screen.getByRole('switch', { name: 'Scheduled detection' })
    )
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(JSON.parse(writes[0].data)).toMatchObject({
      version: 2,
      enabled: true,
      interval_minutes: 5,
    })
    expect(JSON.parse(writes[0].data)).not.toHaveProperty('renderer_available')
  })

  it('requires a public alias before saving a published group', async () => {
    const writes: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      if (config.method === 'put') writes.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data: settings },
      }
    }
    const user = userEvent.setup()
    renderQuality(<SettingsPanel channels={[channel]} />)
    await user.click(
      await screen.findByRole('button', { name: 'Add public group' })
    )
    await user.click(screen.getByRole('combobox', { name: 'Group' }))
    await user.click(await screen.findByRole('option', { name: 'premium' }))
    await user.click(screen.getByRole('button', { name: 'Save changes' }))
    expect(
      await screen.findByText('Invalid configuration value')
    ).toBeInTheDocument()
    expect(
      screen.getByRole('textbox', { name: 'Public alias' })
    ).toHaveAttribute('aria-invalid', 'true')
    expect(writes).toHaveLength(0)
  })
})
