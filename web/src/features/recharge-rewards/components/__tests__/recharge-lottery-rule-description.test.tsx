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

import { RechargeLotteryRuleDescription } from '../recharge-lottery-rule-description'

describe('recharge lottery rule description', () => {
  test('shows the configured per-recharge threshold and fixed draw count', async () => {
    const i18n = createInstance()
    await i18n.init({
      lng: 'en',
      resources: {
        en: {
          translation: {
            'Each single recharge of {{amount}} or more grants {{count}} draw chance(s).':
              'Threshold={{amount}}; draws={{count}}',
          },
        },
      },
    })

    const markup = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <RechargeLotteryRuleDescription
          enabled
          minRechargeQuota={2_500_000}
          drawsPerRecharge={3}
        />
      </I18nextProvider>
    )

    assert.match(markup, /Threshold=\$5; draws=3/)
  })

  test('explains that existing chances remain when recharge draws are paused', async () => {
    const i18n = createInstance()
    await i18n.init({
      lng: 'en',
      resources: {
        en: {
          translation: {
            'Recharge draw is currently paused. Existing draw chances remain stored.':
              'Paused; existing chances stay available.',
          },
        },
      },
    })

    const markup = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <RechargeLotteryRuleDescription
          enabled={false}
          minRechargeQuota={2_500_000}
          drawsPerRecharge={3}
        />
      </I18nextProvider>
    )

    assert.match(markup, /Paused; existing chances stay available\./)
  })
})
