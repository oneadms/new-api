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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import { SpeedPassTutorial } from '../speed-pass-tutorial'

describe('speed pass tutorial', () => {
  test('explains how to earn, activate, and use a speed pass', async () => {
    const i18n = createInstance()
    await i18n.init({
      lng: 'en',
      resources: {
        en: {
          translation: {
            'How to use a speed pass': 'Speed pass tutorial',
            'Earn a speed pass from an eligible recharge, promotion, or recharge draw.':
              'Earn the pass.',
            'Activate it before the activation deadline. The timer starts immediately and cannot be paused.':
              'Activate before the deadline.',
            "While it is active, requests automatically use the group shown on the pass. When access expires, they automatically return to the API key's original group.":
              'Switch automatically, then restore the original group.',
            'Only one speed pass can be active at a time.':
              'Activate only one pass.',
          },
        },
      },
    })

    const markup = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <SpeedPassTutorial />
      </I18nextProvider>
    )

    assert.match(markup, /role="alert"/)
    assert.match(markup, /Speed pass tutorial/)
    assert.equal(markup.match(/<li>/g)?.length, 4)
    assert.ok(
      markup.indexOf('Earn the pass.') < markup.indexOf('Activate before')
    )
    assert.ok(
      markup.indexOf('Activate before') < markup.indexOf('Switch automatically')
    )
    assert.ok(
      markup.indexOf('Switch automatically') <
        markup.indexOf('Activate only one pass')
    )
  })
})
