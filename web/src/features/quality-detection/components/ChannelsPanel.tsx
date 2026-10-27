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

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import type { QualityChannel, QualityPrompt } from '../types'
import { ChannelReadiness } from './ChannelReadiness'
import { ProfileForm } from './ProfileForm'

function ChannelProfile(props: {
  channel: QualityChannel
  prompts: QualityPrompt[]
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={<Button variant='outline' size='sm' />}
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
        <div className='min-h-0 flex-1 overflow-y-auto p-6'>
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
  selected: number[]
  eligible: Set<number>
  onToggle: (id: number, checked: boolean) => void
}) {
  const { t } = useTranslation()
  if (!props.channels.length) {
    return (
      <Empty className='border py-10'>
        <EmptyHeader>
          <EmptyTitle>{t('No channels found')}</EmptyTitle>
          <EmptyDescription>
            {t(
              'Try another group or search term. Configure channels in channel management if none exist.'
            )}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <div className='overflow-hidden rounded-xl border'>
      <Table aria-label={t('Channels')} className='block table-fixed sm:table'>
        <TableHeader className='bg-muted/40 hidden sm:table-header-group'>
          <TableRow>
            <TableHead className='w-12'>
              <span className='sr-only'>{t('Select')}</span>
            </TableHead>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead className='hidden w-[24%] lg:table-cell'>
              {t('Detection model')}
            </TableHead>
            <TableHead className='w-[25%]'>{t('Detection status')}</TableHead>
            <TableHead className='w-28'>
              <span className='sr-only'>{t('Configure')}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className='block sm:table-row-group [&>tr]:h-auto'>
          {props.channels.map((channel) => {
            const ready = props.eligible.has(channel.id)
            const selected = ready && props.selected.includes(channel.id)
            return (
              <TableRow
                key={channel.id}
                data-state={selected ? 'selected' : undefined}
                className='grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-x-2 px-3 py-4 sm:table-row sm:p-0'
              >
                <TableCell className='row-span-2 p-0 sm:py-5 sm:pl-4'>
                  <Checkbox
                    aria-label={channel.name}
                    aria-describedby={`quality-channel-${channel.id}-readiness`}
                    checked={selected}
                    disabled={!ready}
                    onCheckedChange={(checked) =>
                      props.onToggle(channel.id, checked)
                    }
                  />
                </TableCell>
                <TableCell className='min-w-0 p-0 sm:py-5 sm:pr-4'>
                  <p className='truncate font-medium' title={channel.name}>
                    {channel.name}
                  </p>
                  <p
                    className='text-muted-foreground mt-1 truncate text-xs'
                    title={channel.groups.join(' · ')}
                  >
                    {channel.groups.join(' · ')}
                  </p>
                  <p
                    className='text-muted-foreground mt-1 truncate text-xs lg:hidden'
                    title={channel.profile.model}
                  >
                    {channel.profile.model || t('No model configured')}
                  </p>
                </TableCell>
                <TableCell className='hidden min-w-0 lg:table-cell'>
                  <p className='truncate' title={channel.profile.model}>
                    {channel.profile.model || t('No model configured')}
                  </p>
                </TableCell>
                <TableCell
                  id={`quality-channel-${channel.id}-readiness`}
                  className='col-start-2 row-start-2 min-w-0 p-0 pt-2 whitespace-normal sm:p-2'
                >
                  {ready ? (
                    <Badge
                      variant='outline'
                      className='text-emerald-700 dark:text-emerald-400'
                    >
                      {t('Ready')}
                    </Badge>
                  ) : (
                    <ChannelReadiness channel={channel} />
                  )}
                </TableCell>
                <TableCell className='col-start-3 row-span-2 row-start-1 p-0 text-right sm:pr-4'>
                  <ChannelProfile channel={channel} prompts={props.prompts} />
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
