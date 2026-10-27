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
import { useState, type ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

import {
  GroupDetailsContent,
  type GroupDetailsContentProps,
} from './GroupDetailsContent'

export function GroupDetails(props: GroupDetailsContentProps): ReactElement {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={<Button variant='outline' />}
        aria-label={`${t('View details')}: ${props.group.name}`}
      >
        {t('View details')}
      </SheetTrigger>
      <SheetContent className='w-full motion-reduce:transition-none sm:max-w-2xl'>
        <SheetHeader className='border-b p-6 pr-12'>
          <SheetTitle className='text-xl wrap-anywhere'>
            {props.group.name}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Business traffic and representative channel samples are measured separately.'
            )}
          </SheetDescription>
        </SheetHeader>
        <div
          role='region'
          aria-label={t('Group details')}
          className='flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto overscroll-contain p-4 sm:p-6'
        >
          {open && (
            <GroupDetailsContent
              key={`${props.group.public_id}:${props.hours}:${props.model}`}
              {...props}
            />
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
