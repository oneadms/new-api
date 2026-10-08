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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { CheckinCalendarCard } from '../checkin-calendar-card'

async function renderCheckin(
  regular: boolean,
  lucky: boolean,
  checkedToday = false
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: {} })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        enabled: regular,
        lucky: {
          enabled: lucky,
          min_stake_quota: 1000,
          max_stake_quota: 10000,
          min_failure_bps: 2500,
          max_failure_bps: 7500,
        },
        stats: {
          checked_in_today: checkedToday,
          total_checkins: 0,
          total_quota: 0,
          checkin_count: 0,
          records: [],
        },
      },
    },
  })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <CheckinCalendarCard
          checkinEnabled={regular || lucky}
          turnstileEnabled={false}
          turnstileSiteKey=''
        />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

describe('available check-in modes', () => {
  it('opens lucky check-in without offering regular rewards when only lucky mode is enabled', async () => {
    const user = userEvent.setup()
    await renderCheckin(false, true)

    const luckyButton = await screen.findByRole('button', {
      name: 'Try my luck',
    })
    expect(
      screen.queryByRole('button', { name: 'Check in now' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Lucky check-in' })
    ).toBeVisible()
    expect(
      screen.queryByText('Check in daily to receive random balance rewards')
    ).not.toBeInTheDocument()
    await user.click(luckyButton)
    expect(
      await screen.findByRole('dialog', { name: 'Lucky check-in' })
    ).toBeVisible()
  })

  it('offers only the regular action when lucky check-in is disabled', async () => {
    await renderCheckin(true, false)

    expect(
      await screen.findByRole('button', { name: 'Check in now' })
    ).toBeEnabled()
    expect(
      screen.queryByRole('button', { name: 'Try my luck' })
    ).not.toBeInTheDocument()
  })

  it('disables lucky check-in after today’s shared check-in opportunity is used', async () => {
    await renderCheckin(false, true, true)

    expect(
      await screen.findByRole('button', { name: 'Try my luck' })
    ).toBeDisabled()
    expect(
      screen.queryByRole('button', { name: 'Checked in' })
    ).not.toBeInTheDocument()
  })
})
