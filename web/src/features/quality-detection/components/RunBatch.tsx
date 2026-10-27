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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { OptionSelect } from '@/components/option-select'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { createBatch } from '../api'
import { eligibleChannels } from '../lib/batches'
import type { QualityChannel, QualityPrompt } from '../types'
import { ChannelsPanel } from './ChannelsPanel'
import { QueryFeedback } from './QueryFeedback'

type Confirmation = { ids: number[]; groups: number; key: string }
export function RunBatch(props: {
  channels: QualityChannel[]
  prompts: QualityPrompt[]
  blocked: boolean
  onCreated: (id: number) => void
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [group, setGroup] = useState('')
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<number[]>([])
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)
  const uniqueChannels = [
    ...new Map(props.channels.map((channel) => [channel.id, channel])).values(),
  ]
  const visible = uniqueChannels.filter(
    (channel) =>
      (!group || channel.groups.includes(group)) &&
      `${channel.name} ${channel.id} ${channel.profile.model}`
        .toLocaleLowerCase()
        .includes(search.trim().toLocaleLowerCase())
  )
  const activePrompts = new Set(
    props.prompts
      .filter((prompt) => !prompt.archived)
      .map((prompt) => prompt.id)
  )
  const eligible = eligibleChannels(uniqueChannels).filter((channel) =>
    activePrompts.has(channel.profile.prompt_version_id)
  )
  const eligibleIDs = new Set(eligible.map((channel) => channel.id))
  const available = visible.filter((channel) => eligibleIDs.has(channel.id))
  const chosen = eligible.filter((channel) => selected.includes(channel.id))
  const groups = [
    ...new Set(props.channels.flatMap((channel) => channel.groups)),
  ].sort()
  const mutation = useMutation({
    mutationFn: (selection: Confirmation) =>
      createBatch({ channel_ids: selection.ids }, selection.key),
    retry: false,
    onError: () => {
      /* qualityRequest reports through handleServerError. */
    },
    onSuccess: async (batch) => {
      await client.invalidateQueries({ queryKey: ['quality-detection'] })
      setConfirmation(null)
      props.onCreated(batch.id)
    },
  })
  return (
    <section className='space-y-5'>
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div className='space-y-1'>
          <h2 className='text-base font-semibold'>{t('Channels')}</h2>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Configure a model and prompt for each channel, then select it to run detection.'
            )}
          </p>
        </div>
        <Button
          disabled={props.blocked || chosen.length === 0 || mutation.isPending}
          onClick={() => {
            mutation.reset()
            setConfirmation({
              ids: chosen.map((channel) => channel.id).sort((a, b) => a - b),
              groups: new Set(chosen.flatMap((channel) => channel.groups)).size,
              key: crypto.randomUUID(),
            })
          }}
        >
          {t('Run detection')}
        </Button>
      </div>
      <FieldGroup className='grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,20rem)]'>
        <Field>
          <FieldLabel htmlFor='quality-channel-search'>
            {t('Search channels')}
          </FieldLabel>
          <Input
            id='quality-channel-search'
            className='h-9'
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Search channels')}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='batch-group'>
            {t('Filter channels by group')}
          </FieldLabel>
          <OptionSelect
            id='batch-group'
            searchable
            value={group}
            onValueChange={setGroup}
            options={[
              { value: '', label: t('All groups') },
              ...groups.map((name) => ({ value: name, label: name })),
            ]}
          />
        </Field>
      </FieldGroup>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={!available.length}
          onClick={() =>
            setSelected((previous) => [
              ...new Set([
                ...previous,
                ...available.map((channel) => channel.id),
              ]),
            ])
          }
        >
          {t('Select filtered channels')}
        </Button>
        <Button
          variant='ghost'
          size='sm'
          disabled={!selected.length}
          onClick={() => setSelected([])}
        >
          {t('Clear selection')}
        </Button>
        <p
          className='text-muted-foreground text-sm tabular-nums sm:ml-auto'
          aria-live='polite'
        >
          {t('{{count}} unique channels selected', { count: chosen.length })}
        </p>
      </div>
      <ChannelsPanel
        channels={visible}
        prompts={props.prompts}
        selected={selected}
        eligible={eligibleIDs}
        onToggle={(id, checked) =>
          setSelected((previous) =>
            checked
              ? [...new Set([...previous, id])]
              : previous.filter((value) => value !== id)
          )
        }
      />
      <p className='text-muted-foreground text-xs leading-relaxed'>
        {t(
          'One request per selected channel. Shared groups never multiply requests.'
        )}
      </p>
      <AlertDialog
        open={Boolean(confirmation)}
        onOpenChange={(open) => {
          if (!open && !mutation.isPending) setConfirmation(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm paid detection')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                '{{groups}} groups reference {{channels}} unique channels. At most {{channels}} model requests will be started. Upstream costs cannot be undone.',
                {
                  groups: confirmation?.groups ?? 0,
                  channels: confirmation?.ids.length ?? 0,
                }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <QueryFeedback error={mutation.error} />
          <AlertDialogFooter>
            <Button
              variant='outline'
              disabled={mutation.isPending}
              onClick={() => setConfirmation(null)}
            >
              {t('Cancel')}
            </Button>
            <Button
              disabled={mutation.isPending || props.blocked}
              onClick={() => {
                if (confirmation) mutation.mutate(confirmation)
              }}
            >
              {mutation.isPending ? t('Starting...') : t('Confirm and run')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  )
}
