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

import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { createBatch } from '../api'
import { eligibleChannels } from '../lib/batches'
import type { QualityChannel } from '../types'
import { QueryFeedback } from './QueryFeedback'

type Confirmation = { ids: number[]; groups: number; key: string }
export function RunBatch(props: {
  channels: QualityChannel[]
  blocked: boolean
  onCreated: (id: number) => void
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [group, setGroup] = useState('')
  const [selected, setSelected] = useState<number[]>([])
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)
  const available = eligibleChannels(props.channels, group)
  const chosen = eligibleChannels(props.channels).filter((channel) =>
    selected.includes(channel.id)
  )
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
    <section className='space-y-4 rounded-xl border p-5'>
      <div>
        <h2 className='font-medium'>{t('Run detection')}</h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t(
            'One request per selected channel. Shared groups never multiply requests.'
          )}
        </p>
      </div>
      <div className='flex flex-wrap items-end gap-3'>
        <div className='space-y-2'>
          <Label htmlFor='batch-group'>{t('Filter channels by group')}</Label>
          <NativeSelect
            id='batch-group'
            value={group}
            onChange={(event) => setGroup(event.target.value)}
          >
            <NativeSelectOption value=''>{t('All groups')}</NativeSelectOption>
            {groups.map((name) => (
              <NativeSelectOption key={name} value={name}>
                {name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
        <Button
          variant='outline'
          onClick={() =>
            setSelected([
              ...new Set([
                ...selected,
                ...available.map((channel) => channel.id),
              ]),
            ])
          }
        >
          {t('Select filtered channels')}
        </Button>
        <Button variant='ghost' onClick={() => setSelected([])}>
          {t('Clear selection')}
        </Button>
      </div>
      <fieldset className='max-h-64 space-y-2 overflow-y-auto'>
        <legend className='sr-only'>{t('Channels')}</legend>
        {available.map((channel) => (
          <Label
            key={channel.id}
            className='flex min-h-10 cursor-pointer items-center gap-3 rounded-lg border p-3'
          >
            <Checkbox
              checked={selected.includes(channel.id)}
              onCheckedChange={(checked) =>
                setSelected((previous) =>
                  checked
                    ? [...new Set([...previous, channel.id])]
                    : previous.filter((id) => id !== channel.id)
                )
              }
            />
            <span className='min-w-0 break-all'>{channel.name}</span>
            <span className='text-muted-foreground text-xs'>
              {channel.profile.model}
            </span>
          </Label>
        ))}
      </fieldset>
      <QueryFeedback empty={!available.length} />
      <p className='text-muted-foreground text-sm' aria-live='polite'>
        {t('{{count}} unique channels selected', { count: chosen.length })}
      </p>
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
