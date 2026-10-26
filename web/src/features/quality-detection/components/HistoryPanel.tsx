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

import { getBatches } from '../api'
import { isBatchActive } from '../lib/batches'
import { BatchDetails } from './BatchDetails'
import { QueryFeedback } from './QueryFeedback'
import { ResultBadge } from './ResultBadge'
import { SavedTime } from './SavedTime'

export function HistoryPanel(props: {
  selected: number | null
  onSelect: (id: number) => void
}) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: ['quality-detection', 'batches', page],
    queryFn: ({ signal }) => getBatches(page, signal),
    retry: false,
    refetchInterval: (state) =>
      state.state.data?.items.some(isBatchActive) ? 2000 : 30000,
    refetchIntervalInBackground: false,
  })
  return (
    <section className='space-y-6'>
      <div className='flex justify-end'>
        <Button
          variant='outline'
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          {t('Refresh')}
        </Button>
      </div>
      <QueryFeedback
        pending={query.isPending}
        error={query.error}
        empty={query.isSuccess && !query.data.items.length}
        retry={() => void query.refetch()}
      />
      {query.data && !query.isError && (
        <>
          <div className='divide-y rounded-xl border'>
            {query.data.items.map((batch) => (
              <article
                key={batch.id}
                className='flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between'
              >
                <div className='space-y-2'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='font-medium'>
                      {t('Batch')} #{batch.id}
                    </span>
                    <ResultBadge value={batch.status} />
                    <span className='text-muted-foreground text-xs'>
                      {batch.trigger === 'manual'
                        ? t('Manual')
                        : t('Scheduled')}
                    </span>
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    <SavedTime value={batch.created_at} /> · {batch.completed} /{' '}
                    {batch.total}
                  </p>
                </div>
                <Button
                  variant={
                    props.selected === batch.id ? 'secondary' : 'outline'
                  }
                  aria-pressed={props.selected === batch.id}
                  onClick={() => props.onSelect(batch.id)}
                >
                  {t('View batch')} #{batch.id}
                </Button>
              </article>
            ))}
          </div>
          <nav aria-label={t('Pagination')} className='flex justify-end gap-3'>
            <Button
              variant='outline'
              disabled={page === 1 || query.isFetching}
              onClick={() => setPage(page - 1)}
            >
              {t('Previous')}
            </Button>
            <Button
              variant='outline'
              disabled={
                query.data.page * query.data.page_size >= query.data.total ||
                query.isFetching
              }
              onClick={() => setPage(page + 1)}
            >
              {t('Next')}
            </Button>
          </nav>
        </>
      )}
      {props.selected !== null && (
        <BatchDetails key={props.selected} id={props.selected} />
      )}
    </section>
  )
}
