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
import { useId, useRef, useState, type ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { QueryFeedback } from '@/features/quality-detection/components/QueryFeedback'
import { ResultBadge } from '@/features/quality-detection/components/ResultBadge'
import { isAccessError } from '@/features/quality-detection/lib/request'

import { statusGroupOptions } from '../api'
import type { StatusGroup } from '../types'
import { BusinessMetrics } from './BusinessMetrics'
import { PublicSample } from './PublicSample'
import { StatusTrend } from './StatusTrend'

export type GroupDetailsContentProps = {
  group: StatusGroup
  hours: number
  model: string
  start: number
  end: number
  generatedAt: number
}

export function GroupDetailsContent(
  props: GroupDetailsContentProps
): ReactElement {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const heading = useRef<HTMLHeadingElement>(null)
  const headingId = useId()
  const query = useQuery(
    statusGroupOptions(props.group.public_id, props.hours, props.model, page)
  )
  const data = query.data
  const group = data?.group ?? props.group

  if (isAccessError(query.error)) {
    return (
      <QueryFeedback error={query.error} retry={() => void query.refetch()} />
    )
  }

  return (
    <>
      <section
        aria-labelledby={headingId}
        className='flex min-w-0 flex-col gap-4'
      >
        <header className='flex flex-col gap-2'>
          <h3
            id={headingId}
            ref={heading}
            tabIndex={-1}
            className='focus-visible:outline-ring text-lg font-medium focus-visible:outline-2 focus-visible:outline-offset-4'
          >
            {t('Detection history')}
          </h3>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Saved detection images, newest first. Viewing history never starts a model request.'
            )}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t(
              'A shared channel sample may appear in several groups. A sample does not prove model identity or overall ability.'
            )}
          </p>
        </header>
        <QueryFeedback
          pending={query.isPending}
          error={query.error}
          empty={Boolean(data && !data.samples.length)}
          retry={() => void query.refetch()}
        />
        {query.isError && data && (
          <p role='status' className='text-muted-foreground text-sm'>
            {t('Refresh failed. Showing previously saved data.')}
          </p>
        )}
        <ol
          aria-label={t('Detection history')}
          aria-busy={query.isFetching}
          className='flex min-w-0 flex-col gap-4'
        >
          {data?.samples.map((sample) => (
            <li
              key={sample.public_id}
              className='bg-card min-w-0 rounded-xl border p-4'
            >
              <PublicSample
                sample={sample}
                generatedAt={props.generatedAt}
                large
                historical
              />
            </li>
          ))}
        </ol>
        <nav
          aria-label={t('Detection history pagination')}
          className='flex flex-wrap items-center justify-between gap-3 border-t pt-4'
        >
          <Button
            variant='outline'
            disabled={page === 1 || query.isFetching}
            onClick={() => {
              setPage(page - 1)
              heading.current?.focus()
            }}
          >
            {t('Previous')}
          </Button>
          <span
            className='text-muted-foreground text-sm tabular-nums'
            aria-live='polite'
          >
            {data
              ? t('Page {{page}} of {{pages}}', {
                  page: data.page,
                  pages: Math.max(1, Math.ceil(data.total / data.page_size)),
                })
              : t('Page {{page}}', { page })}
          </span>
          <Button
            variant='outline'
            disabled={
              !data ||
              query.isFetching ||
              query.isError ||
              page * data.page_size >= data.total
            }
            onClick={() => {
              setPage(page + 1)
              heading.current?.focus()
            }}
          >
            {t('Next')}
          </Button>
        </nav>
      </section>
      <div className='flex flex-col gap-4 border-t pt-6'>
        <div>
          <ResultBadge value={group.state} />
        </div>
        <BusinessMetrics group={group} />
        <StatusTrend
          series={group.series}
          start={props.start}
          end={props.end}
          detail
        />
        <p className='text-muted-foreground text-sm'>
          {t(
            '{{fresh}} / {{configured}} configured channels have recent samples',
            {
              fresh: group.fresh_count,
              configured: group.configured_count,
            }
          )}
        </p>
      </div>
    </>
  )
}
