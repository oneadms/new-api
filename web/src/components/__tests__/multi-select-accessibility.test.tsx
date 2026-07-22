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
import { useForm } from 'react-hook-form'
import { I18nextProvider } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'

function MultiSelectFormFixture() {
  const form = useForm<{ groups: string[] }>({
    defaultValues: { groups: [] },
  })

  return (
    <Form {...form}>
      <FormField
        control={form.control}
        name='groups'
        render={({ field }) => (
          <FormItem>
            <FormLabel>Restricted Groups</FormLabel>
            <FormControl>
              <MultiSelect
                options={[]}
                selected={field.value}
                onChange={field.onChange}
                placeholder='Pick groups'
              />
            </FormControl>
            <FormDescription>Unavailable while subscribed.</FormDescription>
          </FormItem>
        )}
      />
    </Form>
  )
}

describe('multi-select accessibility', () => {
  test('connects the form label and description to the combobox input', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

    const markup = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <MultiSelectFormFixture />
      </I18nextProvider>
    )
    const labelId = markup.match(/<label[^>]*for="([^"]+)"/)?.[1]
    const inputTag = markup.match(
      /<input[^>]*aria-label="Pick groups"[^>]*>/
    )?.[0]
    const inputId = inputTag?.match(/id="([^"]+)"/)?.[1]
    const descriptionId = markup.match(
      /<p[^>]*data-slot="form-description"[^>]*id="([^"]+)"/
    )?.[1]

    assert.ok(labelId)
    assert.equal(inputId, labelId)
    assert.ok(descriptionId)
    assert.match(
      inputTag ?? '',
      new RegExp(`aria-describedby="${descriptionId}"`)
    )
  })

  test('forwards form label, description, and validation attributes to its input', async () => {
    const i18n = createInstance()
    await i18n.init({ lng: 'en', resources: { en: { translation: {} } } })

    const markup = renderToStaticMarkup(
      <I18nextProvider i18n={i18n}>
        <MultiSelect
          options={[]}
          selected={[]}
          onChange={() => {}}
          placeholder='Pick groups'
          id='restricted-groups-control'
          aria-describedby='restricted-groups-description restricted-groups-message'
          aria-invalid='true'
          data-slot='form-control'
          data-form-root='subscription-form'
        />
      </I18nextProvider>
    )

    assert.match(markup, /id="restricted-groups-control"/)
    assert.match(
      markup,
      /aria-describedby="restricted-groups-description restricted-groups-message"/
    )
    assert.match(markup, /aria-invalid="true"/)
    assert.match(markup, /data-slot="form-control"/)
    assert.match(markup, /data-form-root="subscription-form"/)
  })
})
