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
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { saveProfile } from '../api'
import { useQualityMutation } from '../hooks/use-quality-mutation'
import { profileSchema } from '../lib/schemas'
import type { QualityChannel, QualityProfile, QualityPrompt } from '../types'
import { ChannelReadiness } from './ChannelReadiness'
import { QualityField, QualityToggle } from './QualityField'
import { QueryFeedback } from './QueryFeedback'

export function ProfileForm(props: {
  channel: QualityChannel
  prompts: QualityPrompt[]
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const form = useForm<QualityProfile>({
    resolver: zodResolver(profileSchema),
    defaultValues: { ...props.channel.profile, channel_id: props.channel.id },
  })
  const enabled = useWatch({ control: form.control, name: 'enabled' })
  const model = useWatch({ control: form.control, name: 'model' })
  const protocol = useWatch({ control: form.control, name: 'protocol' })
  const correctingTarget =
    ['model_unavailable', 'unsupported_model', 'unsupported_protocol'].includes(
      props.channel.unsupported_reason
    ) &&
    (model !== props.channel.profile.model ||
      protocol !== props.channel.profile.protocol)
  const canEnable =
    props.channel.status === 1 && (props.channel.supported || correctingTarget)
  const mutation = useQualityMutation(saveProfile, props.onSaved)
  return (
    <form
      className='space-y-5'
      onSubmit={form.handleSubmit((values) => {
        if (values.enabled && !props.channel.models.includes(values.model)) {
          form.setError('model', {
            message: 'Choose a model supported by this channel',
          })
          return
        }
        mutation.mutate(values)
      })}
    >
      <p className='text-muted-foreground text-sm'>
        {t(
          'One profile per channel, shared across all its groups. No automatic model fallback.'
        )}
      </p>
      <QualityToggle
        form={form}
        name='enabled'
        label={t('Enable detection for this channel')}
        disabled={!canEnable && !enabled}
      />
      {!props.channel.supported && !correctingTarget && (
        <ChannelReadiness channel={props.channel} />
      )}
      <div className='grid gap-4 sm:grid-cols-2'>
        <QualityField
          form={form}
          name='model'
          label={t('Detection model')}
          searchable
          options={[
            { value: '', label: t('Select model') },
            ...[
              ...new Set(
                [...props.channel.models, props.channel.profile.model].filter(
                  Boolean
                )
              ),
            ].map((model) => ({ value: model, label: model })),
          ]}
        />
        <QualityField
          form={form}
          name='protocol'
          label={t('Protocol')}
          options={[
            { value: 'chat', label: 'Chat Completions' },
            { value: 'responses', label: 'Responses' },
            { value: 'claude', label: 'Claude Messages' },
          ]}
        />
        <QualityField
          form={form}
          name='prompt_version_id'
          type='number'
          label={t('Prompt version')}
          options={[
            { value: 0, label: t('Select prompt version') },
            ...props.prompts
              .filter(
                (prompt) =>
                  !prompt.archived ||
                  prompt.id === props.channel.profile.prompt_version_id
              )
              .map((prompt) => ({
                value: prompt.id,
                label: `${prompt.name} · v${prompt.version}${prompt.archived ? ` · ${t('Archived')}` : ''}`,
              })),
          ]}
        />
        <QualityField
          form={form}
          name='reasoning_effort'
          label={t('Reasoning effort')}
          options={[
            { value: '', label: t('Default') },
            ...['none', 'minimal', 'low', 'medium', 'high', 'xhigh'].map(
              (value) => ({ value, label: value })
            ),
          ]}
          hint={t(
            'Optional; only use values supported by the selected protocol and model.'
          )}
        />
        <QualityField
          form={form}
          name='timeout_seconds'
          label={t('Timeout override (seconds)')}
          type='number'
          min={0}
          max={1800}
          hint={t(
            'Zero inherits the global timeout. Otherwise use 60–1800 seconds.'
          )}
        />
        <QualityField
          form={form}
          name='public_alias'
          label={t('Public alias')}
        />
      </div>
      <QualityToggle
        form={form}
        name='publish_sample'
        label={t('Publish channel samples')}
      />
      <QueryFeedback error={mutation.error} />
      <Button type='submit' disabled={mutation.isPending}>
        {mutation.isPending ? t('Saving...') : t('Save changes')}
      </Button>
    </form>
  )
}
