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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { useState } from 'react'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { DEFAULT_CURRENCY_CONFIG } from '@/stores/system-config-store'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { CheckinSettingsSection } from '../checkin-settings-section'

function CheckinSettingsFixture(props: { regular: boolean; lucky: boolean }) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setActions} />
      <SettingsPageProvider actionsContainer={actions}>
        <CheckinSettingsSection
          defaultValues={{
            enabled: props.regular,
            luckyEnabled: props.lucky,
            minQuota: 1000,
            maxQuota: 10000,
            minStakeQuota: 1000,
            maxStakeQuota: 10000,
            minFailureBps: 2500,
            maxFailureBps: 7500,
            actualMinFailureBps: 2500,
            actualMaxFailureBps: 7500,
            currencyConfig: DEFAULT_CURRENCY_CONFIG,
          }}
        />
      </SettingsPageProvider>
    </>
  )
}

async function renderCheckinSettings(regular = false, lucky = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: {} })
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data: {} } })
  const save = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <CheckinSettingsFixture regular={regular} lucky={lucky} />
      </I18nextProvider>
    </QueryClientProvider>
  )
  return save
}

describe('independent check-in switches', () => {
  it('allows saving lucky check-in while regular check-in stays disabled', async () => {
    const user = userEvent.setup()
    const save = await renderCheckinSettings()

    const luckySwitch = screen.getByRole('switch', {
      name: 'Enable lucky check-in',
    })
    expect(luckySwitch).not.toBeChecked()
    await user.click(luckySwitch)

    expect(luckySwitch).toBeChecked()
    expect(
      screen.getByRole('switch', { name: 'Enable regular check-in' })
    ).not.toBeChecked()
    expect(
      screen.getByLabelText(/Minimum lucky check-in stake amount/)
    ).toBeVisible()
    expect(
      screen.queryByLabelText(/Minimum check-in reward amount/)
    ).not.toBeInTheDocument()
    await user.click(
      screen.getByRole('button', { name: 'Save check-in settings' })
    )

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith('/api/option/', {
        key: 'lucky_checkin_setting.enabled',
        value: 'true',
      })
    )
    expect(save).toHaveBeenCalledTimes(1)
  })

  it('keeps lucky settings available when regular check-in is switched off', async () => {
    const user = userEvent.setup()
    const save = await renderCheckinSettings(true, true)

    await user.click(
      screen.getByRole('switch', { name: 'Enable regular check-in' })
    )
    expect(
      screen.getByRole('switch', { name: 'Enable lucky check-in' })
    ).toBeChecked()
    expect(
      screen.getByLabelText(/Minimum lucky check-in stake amount/)
    ).toBeVisible()
    await user.click(
      screen.getByRole('button', { name: 'Save check-in settings' })
    )

    await waitFor(() =>
      expect(save).toHaveBeenCalledWith('/api/option/', {
        key: 'checkin_setting.enabled',
        value: 'false',
      })
    )
    expect(save).toHaveBeenCalledTimes(1)
  })
})
