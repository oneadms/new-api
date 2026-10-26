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
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { api } from '@/lib/api'

import { BatchDetails } from '../components/BatchDetails'
import { renderQuality, run } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('batch cancellation', () => {
  it('stops a running batch only on an explicit action and keeps cancellation separate from generation', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    let cancelled = false
    api.defaults.adapter = async (config) => {
      requests.push(config)
      if (config.method === 'post') cancelled = true
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: {
          success: true,
          data:
            config.method === 'post'
              ? null
              : {
                  batch: {
                    id: 2,
                    status: 'running',
                    total: 1,
                    completed: 0,
                    failed: 0,
                    cancel_requested: cancelled,
                    summary: '',
                  },
                  runs: [run],
                },
        },
      }
    }
    const user = userEvent.setup()
    renderQuality(<BatchDetails id={2} />)
    await screen.findByRole('button', { name: 'Stop batch' })
    expect(requests.every((request) => request.method === 'get')).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Stop batch' }))
    expect(
      await screen.findByRole('button', { name: 'Stop requested' })
    ).toBeDisabled()
    expect(
      requests
        .filter((request) => request.method === 'post')
        .map((request) => request.url)
    ).toEqual(['/api/quality-detection/batches/2/cancel'])
  })
})
