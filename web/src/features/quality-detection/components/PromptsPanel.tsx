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
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'

import { archivePrompt, createPrompt } from '../api'
import { useQualityMutation } from '../hooks/use-quality-mutation'
import { promptSchema } from '../lib/schemas'
import type { PromptInput, QualityPrompt } from '../types'
import { QualityField } from './QualityField'
import { QueryFeedback } from './QueryFeedback'

function PromptForm(props: {
  prompt?: QualityPrompt
  copy?: boolean
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const form = useForm<PromptInput>({
    resolver: zodResolver(promptSchema),
    defaultValues: {
      key: props.copy ? undefined : props.prompt?.key,
      name: props.prompt?.name ?? '',
      content: props.prompt?.content ?? '',
      rule: props.prompt?.rule ?? 'visual',
    },
  })
  const mutation = useQualityMutation(createPrompt, props.onSaved)
  return (
    <form
      onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      className='space-y-4'
    >
      <QualityField form={form} name='name' label={t('Name')} />
      <QualityField
        form={form}
        name='rule'
        label={t('Rule type')}
        options={[
          { value: 'visual', label: t('Visual comparison') },
          { value: 'interactive', label: t('Interactive checks') },
        ]}
      />
      <QualityField
        form={form}
        name='content'
        label={t('Prompt content')}
        type='textarea'
        hint={t(
          'Output requirements are added by the server. No custom evaluation scripts are executed.'
        )}
      />
      <QueryFeedback error={mutation.error} />
      <Button type='submit' disabled={mutation.isPending}>
        {mutation.isPending ? t('Saving...') : t('Create version')}
      </Button>
    </form>
  )
}

function PromptEditor(props: { prompt?: QualityPrompt; copy?: boolean }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  let title = t('Create prompt')
  if (props.prompt) title = props.copy ? t('Copy prompt') : t('Create version')
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant='outline' />}>
        {title}
      </DialogTrigger>
      <DialogContent className='max-h-[90dvh] overflow-y-auto motion-reduce:transition-none sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {t(
              'Every edit creates an immutable version. Historical samples keep their original prompt.'
            )}
          </DialogDescription>
        </DialogHeader>
        {open && (
          <PromptForm
            prompt={props.prompt}
            copy={props.copy}
            onSaved={() => setOpen(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

export function PromptsPanel(props: { prompts: QualityPrompt[] }) {
  const { t } = useTranslation()
  const mutation = useQualityMutation((prompt: QualityPrompt) =>
    archivePrompt(prompt.id, !prompt.archived)
  )
  return (
    <section className='space-y-5'>
      <PromptEditor />
      <QueryFeedback error={mutation.error} empty={!props.prompts.length} />
      <div className='space-y-4'>
        {props.prompts.map((prompt) => (
          <article key={prompt.id} className='space-y-4 rounded-xl border p-5'>
            <div>
              <h3 className='font-medium break-all'>
                {prompt.name} · v{prompt.version}
              </h3>
              <p className='text-muted-foreground text-xs'>
                {prompt.archived ? t('Archived') : t('Available')} ·{' '}
                {prompt.rule === 'visual'
                  ? t('Visual comparison')
                  : t('Interactive checks')}
              </p>
            </div>
            <details>
              <summary className='cursor-pointer text-sm'>
                {t('Prompt content')}
              </summary>
              <pre className='bg-muted mt-3 max-h-64 overflow-auto rounded-lg p-4 text-xs break-words whitespace-pre-wrap'>
                {prompt.content}
              </pre>
            </details>
            <div className='flex flex-wrap gap-2'>
              {!prompt.key.startsWith('builtin-') && (
                <PromptEditor prompt={prompt} />
              )}
              <PromptEditor prompt={prompt} copy />
              <Button
                variant='ghost'
                disabled={mutation.isPending}
                onClick={() => mutation.mutate(prompt)}
              >
                {prompt.archived ? t('Restore') : t('Archive')}
              </Button>
            </div>
          </article>
        ))}
      </div>
    </section>
  )
}
