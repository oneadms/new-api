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

import { RunReview } from '../components/RunReview'
import { renderQuality, run } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('human review', () => {
  it('requires a review note before submitting an acceptance', async () => {
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
    renderQuality(<RunReview run={run} />)
    await user.click(screen.getByRole('button', { name: 'Save review' }))
    expect(
      await screen.findByText('Invalid configuration value')
    ).toBeInTheDocument()
    expect(requests).toHaveLength(0)
    await user.type(
      screen.getByRole('textbox', { name: 'Review note' }),
      'Compared with expected output.'
    )
    await user.click(screen.getByRole('button', { name: 'Save review' }))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(JSON.parse(requests[0].data)).toEqual({
      result: 'accepted',
      note: 'Compared with expected output.',
    })
    expect(requests[0].url).toBe('/api/quality-detection/runs/4/review')
  })

  it('sets an accepted completed sample as baseline only on explicit action', async () => {
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
    renderQuality(<RunReview run={{ ...run, review_status: 'accepted' }} />)
    expect(requests).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: 'Set as baseline' }))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].url).toBe('/api/quality-detection/runs/4/baseline')
    expect(requests[0].method).toBe('post')
  })
})
