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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

import type { QualityRun } from '../types'
import { RunContent } from './RunContent'

export function RunDetails(props: { run: QualityRun }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={<Button variant='outline' />}
        aria-label={`${t('View sample')}: ${props.run.channel_name}`}
      >
        {t('View sample')}
      </SheetTrigger>
      <SheetContent className='w-full motion-reduce:transition-none sm:max-w-3xl'>
        <SheetHeader className='border-b p-6 pr-12'>
          <SheetTitle className='break-all'>
            {props.run.channel_name}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Execution, quality results and human reviews are separate records.'
            )}
          </SheetDescription>
        </SheetHeader>
        <div className='flex-1 overflow-y-auto p-6'>
          {open && <RunContent id={props.run.id} />}
        </div>
      </SheetContent>
    </Sheet>
  )
}
