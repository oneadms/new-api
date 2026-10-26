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
import { useQuery } from '@tanstack/react-query'
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
import { QueryFeedback } from '@/features/quality-detection/components/QueryFeedback'
import { ResultBadge } from '@/features/quality-detection/components/ResultBadge'
import { isAccessError } from '@/features/quality-detection/lib/request'

import { statusGroupOptions } from '../api'
import type { StatusGroup } from '../types'
import { BusinessMetrics } from './BusinessMetrics'
import { PublicSample } from './PublicSample'
import { StatusTrend } from './StatusTrend'

export function GroupDetails(props: {
  group: StatusGroup
  hours: number
  model: string
  start: number
  end: number
  generatedAt: number
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const query = useQuery({
    ...statusGroupOptions(props.group.public_id, props.hours, props.model),
    enabled: open,
  })
  const data = isAccessError(query.error) ? undefined : query.data
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
          <SheetTitle className='text-xl break-all'>
            {props.group.name}
          </SheetTitle>
          <SheetDescription>
            {t(
              'Business traffic and representative channel samples are measured separately.'
            )}
          </SheetDescription>
        </SheetHeader>
        <div className='flex-1 space-y-6 overflow-y-auto p-6'>
          <QueryFeedback
            pending={query.isPending}
            error={query.error}
            retry={() => void query.refetch()}
          />
          {data && (
            <>
              <ResultBadge value={data.group.state} />
              <BusinessMetrics group={data.group} />
              <StatusTrend
                series={data.group.series}
                start={props.start}
                end={props.end}
                detail
              />
              <h3 className='text-lg font-medium'>{t('Channel samples')}</h3>
              <p className='text-muted-foreground text-sm'>
                {t(
                  '{{fresh}} / {{configured}} configured channels have recent samples',
                  {
                    fresh: data.group.fresh_count,
                    configured: data.group.configured_count,
                  }
                )}
              </p>
              <QueryFeedback empty={!data.samples.length} />
              {data.samples.map((sample) => (
                <section
                  key={sample.public_id}
                  className='rounded-xl border p-4'
                >
                  <PublicSample
                    sample={sample}
                    generatedAt={props.generatedAt}
                    large
                  />
                </section>
              ))}
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
