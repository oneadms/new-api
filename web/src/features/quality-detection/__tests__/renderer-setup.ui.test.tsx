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
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPanel } from '../components/SettingsPanel'
import { renderQuality, settings } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
let rendererConfigured: boolean
let requests: InternalAxiosRequestConfig[]
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
  rendererConfigured = false
  requests = []
  api.defaults.adapter = async (config) => {
    requests.push(config)
    return {
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: {
        success: true,
        data: { ...settings, renderer_available: rendererConfigured },
      },
    }
  }
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('quality renderer setup', () => {
  it('explains missing or invalid configuration while preserving generation and settings', async () => {
    renderQuality(<SettingsPanel channels={[]} />)

    const notice = await screen.findByRole('status', { name: 'Renderer setup' })
    expect(notice).toHaveTextContent(
      'Renderer configuration is missing or invalid.'
    )
    expect(notice).toHaveTextContent(
      'Text and extracted HTML are saved without the renderer. Screenshots require the separate renderer service.'
    )
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  })

  it('describes valid configuration as health unverified rather than a working renderer', async () => {
    rendererConfigured = true
    renderQuality(<SettingsPanel channels={[]} />)

    const notice = await screen.findByRole('status', { name: 'Renderer setup' })
    expect(notice).toHaveTextContent(
      'Renderer is configured; health has not been checked.'
    )
    expect(notice).toHaveTextContent(
      'Text and extracted HTML are saved without the renderer. Screenshots require the separate renderer service.'
    )
    expect(notice).not.toHaveTextContent('Renderer available')
    expect(notice).not.toHaveTextContent('missing or invalid')
  })

  it('expands deployment-only instructions by keyboard without saving or making health requests', async () => {
    const user = userEvent.setup()
    renderQuality(<SettingsPanel channels={[]} />)

    const trigger = await screen.findByRole('button', {
      name: 'Renderer setup',
    })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(
      screen.queryByRole('region', { name: 'Renderer setup' })
    ).not.toBeInTheDocument()
    await user.tab()
    expect(trigger).toHaveFocus()
    await user.keyboard('{Enter}')

    const instructions = await screen.findByRole('region', {
      name: 'Renderer setup',
    })
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(instructions).toHaveTextContent('tools/quality-renderer/README.md')
    expect(instructions).toHaveTextContent('new-api-quality-renderer-ingress')
    expect(instructions).toHaveTextContent('QUALITY_RENDERER_URL')
    expect(instructions).toHaveTextContent('http://quality-renderer:3000')
    expect(instructions).toHaveTextContent('base URL without /render')
    expect(instructions).toHaveTextContent('QUALITY_RENDERER_TOKEN')
    expect(instructions).toHaveTextContent(
      'Keep it out of the browser worker and frontend.'
    )
    expect(instructions).toHaveTextContent('do not publish host ports')
    expect(instructions).toHaveTextContent('authenticated test screenshot')
    expect(within(instructions).queryByRole('textbox')).not.toBeInTheDocument()

    await user.keyboard(' ')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(
      screen.queryByRole('region', { name: 'Renderer setup' })
    ).not.toBeInTheDocument()
    expect(requests).toHaveLength(1)
    expect(requests[0].method).toBe('get')
    expect(requests[0].url).toBe('/api/quality-detection/settings')
  })
})
