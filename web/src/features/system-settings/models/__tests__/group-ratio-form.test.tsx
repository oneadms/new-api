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
import { test } from 'node:test'

import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'

import { GroupRatioForm } from '../group-ratio-form'

function GroupRatioFormFixture() {
  const form = useForm({
    defaultValues: {
      GroupRatio: '{"default":1}',
      TopupGroupRatio: '{}',
      UserUsableGroups: '{"default":"Default"}',
      SubscriptionDisabledGroups: 'null',
      GroupGroupRatio: '{}',
      AutoGroups: '["default"]',
      DefaultUseAutoGroup: false,
      GroupSpecialUsableGroup: '{}',
    },
  })

  return <GroupRatioForm form={form} onSave={async () => {}} isSaving={false} />
}

test('renders an empty subscription-excluded group selector for legacy null values', async () => {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

  const markup = renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <GroupRatioFormFixture />
    </I18nextProvider>
  )

  assert.match(markup, /Global subscription-excluded groups/)
  assert.match(markup, /Select global subscription-excluded groups/)
})
