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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { getSettings, saveSettings } from '../api'
import { useQualityMutation } from '../hooks/use-quality-mutation'
import { settingsSchema, type SettingsValues } from '../lib/schemas'
import type { QualityChannel, QualitySettings } from '../types'
import { QualityField, QualityToggle } from './QualityField'
import { QueryFeedback } from './QueryFeedback'
import { RendererSetupNotice } from './RendererSetupNotice'

function SettingsForm(props: {
  settings: QualitySettings
  channels: QualityChannel[]
}) {
  const { t } = useTranslation()
  const form = useForm<SettingsValues>({
    resolver: zodResolver(settingsSchema),
    defaultValues: props.settings,
  })
  const groups = useFieldArray({ control: form.control, name: 'public_groups' })
  const mutation = useQualityMutation(saveSettings)
  const groupNames = [
    ...new Set(props.channels.flatMap((channel) => channel.groups)),
  ]
    .filter((group) => group !== 'auto')
    .sort()
  return (
    <form
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      className='space-y-6'
    >
      <RendererSetupNotice configured={props.settings.renderer_available} />
      <div className='grid gap-4 md:grid-cols-2'>
        <QualityToggle
          form={form}
          name='enabled'
          label={t('Scheduled detection')}
          hint={t(
            'Model requests can incur upstream costs. Refreshing this page is read-only.'
          )}
        />
        <QualityToggle
          form={form}
          name='status_page_enabled'
          label={t('Publish status page')}
          hint={t(
            'Only explicitly published groups and channel aliases are public.'
          )}
        />
      </div>
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
        <QualityField
          form={form}
          name='interval_minutes'
          label={t('Interval (minutes)')}
          type='number'
          min={5}
          max={10080}
          hint={t('Minimum delay after the previous batch finishes.')}
        />
        <QualityField
          form={form}
          name='concurrency'
          label={t('Concurrency')}
          type='number'
          min={1}
          max={10}
        />
        <QualityField
          form={form}
          name='timeout_seconds'
          label={t('Timeout (seconds)')}
          type='number'
          min={60}
          max={1800}
        />
        <QualityField
          form={form}
          name='max_output_tokens'
          label={t('Maximum output tokens')}
          type='number'
          min={1}
        />
        <QualityField
          form={form}
          name='daily_call_limit'
          label={t('Daily call limit (UTC)')}
          type='number'
          min={1}
        />
        <QualityField
          form={form}
          name='daily_token_limit'
          label={t('Daily token limit (UTC)')}
          type='number'
          min={1}
        />
        <QualityField
          form={form}
          name='retention_days'
          label={t('Artifact retention (days)')}
          type='number'
          min={1}
        />
      </div>
      <section className='space-y-4'>
        <h3 className='font-medium'>{t('Published groups')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Publishing does not enable detection. A channel shared by groups is tested once per batch.'
          )}
        </p>
        {groups.fields.map((field, index) => (
          <div
            key={field.id}
            className='grid items-start gap-3 rounded-xl border p-4 sm:grid-cols-[1fr_1fr_auto]'
          >
            <QualityField
              form={form}
              name={`public_groups.${index}.name`}
              label={t('Group')}
              options={[
                { value: '', label: t('Select a group') },
                ...[
                  ...new Set([...groupNames, field.name].filter(Boolean)),
                ].map((name) => ({ value: name, label: name })),
              ]}
            />
            <QualityField
              form={form}
              name={`public_groups.${index}.alias`}
              label={t('Public alias')}
            />
            <Button
              type='button'
              variant='outline'
              aria-label={`${t('Remove')}: ${field.name || index + 1}`}
              onClick={() => groups.remove(index)}
            >
              {t('Remove')}
            </Button>
          </div>
        ))}
        {form.formState.errors.public_groups?.root?.message && (
          <p role='alert'>
            {t(form.formState.errors.public_groups.root.message)}
          </p>
        )}
        <Button
          type='button'
          variant='outline'
          onClick={() => groups.append({ name: '', alias: '' })}
        >
          {t('Add public group')}
        </Button>
      </section>
      <QueryFeedback error={mutation.error} />
      <Button type='submit' disabled={mutation.isPending}>
        {mutation.isPending ? t('Saving...') : t('Save changes')}
      </Button>
    </form>
  )
}

export function SettingsPanel(props: { channels: QualityChannel[] }) {
  const query = useQuery({
    queryKey: ['quality-detection', 'settings'],
    queryFn: ({ signal }) => getSettings(signal),
    retry: false,
  })
  return (
    <>
      <QueryFeedback
        pending={query.isPending}
        error={query.error}
        retry={() => void query.refetch()}
      />
      {query.data && !query.isError && (
        <SettingsForm
          key={query.data.version}
          settings={query.data}
          channels={props.channels}
        />
      )}
    </>
  )
}
