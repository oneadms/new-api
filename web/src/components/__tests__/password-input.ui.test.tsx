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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { describe, expect, it, vi } from 'vitest'

import { PasswordInput } from '../password-input'

async function renderPassword(disabled = false) {
  const i18n = createInstance()
  await i18n.init({
    lng: 'zh',
    resources: {
      zh: {
        translation: {
          'Show password': '显示密码',
          'Hide password': '隐藏密码',
        },
      },
    },
  })
  const onSubmit = vi.fn((event) => event.preventDefault())
  render(
    <I18nextProvider i18n={i18n}>
      <form onSubmit={onSubmit}>
        <PasswordInput
          aria-label='密码'
          defaultValue='preview-password'
          disabled={disabled}
        />
      </form>
    </I18nextProvider>
  )
  return onSubmit
}

describe('password visibility', () => {
  it('announces the localized action and preserves the value without submitting the form', async () => {
    const user = userEvent.setup()
    const onSubmit = await renderPassword()
    expect(screen.getByLabelText('密码')).toHaveAttribute('type', 'password')
    await user.click(screen.getByRole('button', { name: '显示密码' }))
    expect(screen.getByLabelText('密码')).toHaveAttribute('type', 'text')
    expect(screen.getByLabelText('密码')).toHaveValue('preview-password')
    await user.click(screen.getByRole('button', { name: '隐藏密码' }))
    expect(screen.getByLabelText('密码')).toHaveAttribute('type', 'password')
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('keeps the visibility control disabled with the input', async () => {
    const user = userEvent.setup()
    await renderPassword(true)
    const toggle = screen.getByRole('button', { name: '显示密码' })
    expect(toggle).toBeDisabled()
    await user.click(toggle)
    expect(screen.getByLabelText('密码')).toHaveAttribute('type', 'password')
  })
})
