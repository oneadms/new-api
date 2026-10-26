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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

import type { QualityChannel, QualityPrompt } from '../types'
import { ProfileForm } from './ProfileForm'
import { QueryFeedback } from './QueryFeedback'

function ChannelProfile(props: {
  channel: QualityChannel
  prompts: QualityPrompt[]
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={<Button variant='outline' />}
        aria-label={`${t('Configure')}: ${props.channel.name}`}
      >
        {t('Configure')}
      </SheetTrigger>
      <SheetContent className='w-full motion-reduce:transition-none sm:max-w-2xl'>
        <SheetHeader className='p-6 pr-12'>
          <SheetTitle className='break-all'>{props.channel.name}</SheetTitle>
          <SheetDescription>
            {props.channel.groups.join(' · ')}
          </SheetDescription>
        </SheetHeader>
        <div className='flex-1 overflow-y-auto p-6'>
          {open && (
            <ProfileForm
              key={`${props.channel.id}-${props.channel.profile.version}`}
              channel={props.channel}
              prompts={props.prompts}
              onSaved={() => setOpen(false)}
            />
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

export function ChannelsPanel(props: {
  channels: QualityChannel[]
  prompts: QualityPrompt[]
}) {
  const { t } = useTranslation()
  const [group, setGroup] = useState('')
  const [search, setSearch] = useState('')
  const groups = [
    ...new Set(props.channels.flatMap((channel) => channel.groups)),
  ].sort()
  const channels = props.channels.filter(
    (channel) =>
      (!group || channel.groups.includes(group)) &&
      channel.name.toLocaleLowerCase().includes(search.toLocaleLowerCase())
  )
  return (
    <section className='space-y-5'>
      <div className='grid gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label htmlFor='quality-channel-search'>{t('Search channels')}</Label>
          <Input
            id='quality-channel-search'
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='quality-channel-group'>{t('Group')}</Label>
          <NativeSelect
            id='quality-channel-group'
            className='w-full'
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
      </div>
      <QueryFeedback empty={!channels.length} />
      <div className='divide-y rounded-xl border'>
        {channels.map((channel) => (
          <article
            key={channel.id}
            className='flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between'
          >
            <div className='min-w-0 space-y-1'>
              <h3 className='truncate font-medium' title={channel.name}>
                {channel.name}
              </h3>
              <p className='text-muted-foreground text-xs break-all'>
                {channel.groups.join(' · ')}
              </p>
              <p className='text-muted-foreground text-xs break-all'>
                {channel.profile.model || t('No model configured')} ·{' '}
                {channel.profile.enabled ? t('Enabled') : t('Disabled')}
              </p>
              {!channel.supported && (
                <p className='text-muted-foreground text-sm'>
                  {t('Unsupported detection target')}:{' '}
                  {channel.unsupported_reason}
                </p>
              )}
            </div>
            <ChannelProfile channel={channel} prompts={props.prompts} />
          </article>
        ))}
      </div>
    </section>
  )
}
