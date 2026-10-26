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
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { api } from '@/lib/api'

import { PromptsPanel } from '../components/PromptsPanel'
import { prompt, renderQuality } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('immutable prompts', () => {
  it('creates a new version with the existing key without editing historical content', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data: { ...prompt, id: 2, version: 2 } },
      }
    }
    const user = userEvent.setup()
    renderQuality(<PromptsPanel prompts={[prompt]} />)
    await user.click(screen.getByRole('button', { name: 'Create version' }))
    const dialog = screen.getByRole('dialog', { name: 'Create version' })
    await user.type(
      within(dialog).getByRole('textbox', { name: 'Prompt content' }),
      ' Draw a clock.'
    )
    await user.click(
      within(dialog).getByRole('button', { name: 'Create version' })
    )
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(JSON.parse(requests[0].data)).toEqual({
      key: 'visual',
      name: 'Visual task',
      rule: 'visual',
      content: 'Create a self-contained page. Draw a clock.',
    })
    expect(requests[0].url).toBe('/api/quality-detection/prompts')
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
  })

  it('archives a prompt with an explicit boolean mutation', async () => {
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
    renderQuality(<PromptsPanel prompts={[prompt]} />)
    await user.click(screen.getByRole('button', { name: 'Archive' }))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].url).toBe('/api/quality-detection/prompts/1/archive')
    expect(JSON.parse(requests[0].data)).toEqual({ archived: true })
  })
})
