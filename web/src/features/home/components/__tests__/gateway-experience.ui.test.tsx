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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { GatewayExperience } from '../gateway-experience'

async function renderExample() {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  return render(
    <I18nextProvider i18n={i18n}>
      <GatewayExperience />
    </I18nextProvider>
  )
}

beforeEach(() =>
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState())
)

describe('gateway model example', () => {
  it('selecting another provider updates the model and the accessible selected state', async () => {
    const user = userEvent.setup()
    await renderExample()
    await user.click(screen.getByRole('button', { name: 'Claude' }))
    expect(screen.getByRole('button', { name: 'Claude' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: 'OpenAI' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    expect(screen.getByLabelText('Request example')).toHaveTextContent(
      'claude-sonnet-4'
    )
    expect(screen.getByLabelText('Request example')).not.toHaveTextContent(
      'gpt-4.1'
    )
  })

  it('keyboard activation selects a provider without sending an API request', async () => {
    const user = userEvent.setup()
    const fetchSpy = vi.fn()
    vi.stubGlobal('fetch', fetchSpy)
    await renderExample()
    await user.tab()
    await user.tab()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('button', { name: 'Claude' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('copying after a model switch copies the complete request for the selected model', async () => {
    const user = userEvent.setup()
    const writeText = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockResolvedValue()
    await renderExample()
    await user.click(screen.getByRole('button', { name: 'Gemini' }))
    await user.click(screen.getByRole('button', { name: 'Copy example' }))
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith(
        [
          'const response = await client.chat.completions.create({',
          "  model: 'gemini-2.5-pro',",
          "  messages: [{ role: 'user', content: 'Hello!' }],",
          '})',
        ].join('\n')
      )
    )
  })
})
