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
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { getCookie, removeCookie, setCookie } from '@/lib/cookies'
import { THEME_COOKIE_KEYS } from '@/lib/theme-customization'

import {
  ThemeCustomizationProvider,
  useThemeCustomization,
} from '../theme-customization-provider'

function ThemeControls() {
  const { customization, setPreset, resetCustomization } =
    useThemeCustomization()

  return (
    <>
      <output aria-label='Active preset'>{customization.preset}</output>
      <button type='button' onClick={() => setPreset('default')}>
        Use base palette
      </button>
      <button type='button' onClick={resetCustomization}>
        Reset customization
      </button>
    </>
  )
}

function renderThemeControls() {
  return render(
    <ThemeCustomizationProvider>
      <ThemeControls />
    </ThemeCustomizationProvider>
  )
}

beforeEach(() => {
  for (const name of Object.values(THEME_COOKIE_KEYS)) removeCookie(name)
})

afterEach(() => {
  for (const name of Object.values(THEME_COOKIE_KEYS)) removeCookie(name)
  for (const attribute of document.body.getAttributeNames()) {
    if (attribute.startsWith('data-theme-')) {
      document.body.removeAttribute(attribute)
    }
  }
})

describe('theme preset preferences', () => {
  it.each([undefined, 'removed-preset'])(
    'applies Anthropic colors and typography when the saved preset is %s',
    (savedPreset) => {
      if (savedPreset) setCookie(THEME_COOKIE_KEYS.preset, savedPreset)

      renderThemeControls()

      expect(
        screen.getByRole('status', { name: 'Active preset' })
      ).toHaveTextContent('anthropic')
      expect(document.body).toHaveAttribute('data-theme-preset', 'anthropic')
      expect(document.body).toHaveAttribute('data-theme-font', 'serif')
    }
  )

  it('keeps an existing color preset and explicit font preference on load', () => {
    setCookie(THEME_COOKIE_KEYS.preset, 'forest-whisper')
    setCookie(THEME_COOKIE_KEYS.font, 'sans')

    renderThemeControls()

    expect(document.body).toHaveAttribute('data-theme-preset', 'forest-whisper')
    expect(document.body).toHaveAttribute('data-theme-font', 'sans')
    expect(getCookie(THEME_COOKIE_KEYS.preset)).toBe('forest-whisper')
  })

  it('restores Anthropic after resetting customization and reloading', async () => {
    const user = userEvent.setup()
    setCookie(THEME_COOKIE_KEYS.preset, 'forest-whisper')
    setCookie(THEME_COOKIE_KEYS.font, 'sans')
    const view = renderThemeControls()

    await user.click(
      screen.getByRole('button', { name: 'Reset customization' })
    )

    expect(document.body).toHaveAttribute('data-theme-preset', 'anthropic')
    expect(document.body).toHaveAttribute('data-theme-font', 'serif')
    expect(getCookie(THEME_COOKIE_KEYS.preset)).toBeUndefined()
    view.unmount()

    renderThemeControls()

    expect(document.body).toHaveAttribute('data-theme-preset', 'anthropic')
  })

  it('preserves an explicit choice of the base palette after reloading', async () => {
    const user = userEvent.setup()
    setCookie(THEME_COOKIE_KEYS.preset, 'anthropic')
    const view = renderThemeControls()

    await user.click(screen.getByRole('button', { name: 'Use base palette' }))
    expect(getCookie(THEME_COOKIE_KEYS.preset)).toBe('default')
    view.unmount()

    renderThemeControls()

    expect(
      screen.getByRole('status', { name: 'Active preset' })
    ).toHaveTextContent('default')
    expect(document.body).not.toHaveAttribute('data-theme-preset')
    expect(document.body).toHaveAttribute('data-theme-font', 'sans')
  })
})
