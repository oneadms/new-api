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

import { PublicLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { QueryFeedback } from '@/features/quality-detection/components/QueryFeedback'
import { SavedTime } from '@/features/quality-detection/components/SavedTime'
import { isAccessError } from '@/features/quality-detection/lib/request'

import { defaultStatusFilters, statusPageOptions } from './api'
import { GroupList } from './components/GroupList'
import { StatusFilterBar } from './components/StatusFilterBar'

export function StatusContent() {
  const { t } = useTranslation()
  const [filters, setFilters] = useState(defaultStatusFilters)
  const query = useQuery(statusPageOptions(filters))
  const data = isAccessError(query.error) ? undefined : query.data
  return (
    <main className='mx-auto flex min-h-svh w-full max-w-7xl min-w-0 flex-col gap-8 px-4 pt-28 pb-12 sm:px-8 sm:pt-32'>
      <header className='flex flex-col gap-5 border-b pb-6 sm:flex-row sm:items-start sm:justify-between'>
        <div className='flex min-w-0 flex-col gap-3'>
          <h1 className='public-page-title text-3xl font-semibold tracking-tight'>
            {t('Service Status')}
          </h1>
          <p className='text-muted-foreground max-w-2xl text-sm'>
            {t(
              'Business traffic and representative channel samples are measured separately.'
            )}
          </p>
          <div className='text-muted-foreground flex flex-wrap gap-x-6 gap-y-2 text-xs'>
            <span>
              {t('Metrics data until')}:{' '}
              <SavedTime value={data?.metrics_data_until ?? 0} />
            </span>
            <span>
              {t('Last detection completed')}:{' '}
              <SavedTime value={data?.last_probe_completed_at ?? 0} />
            </span>
          </div>
        </div>
        <div className='flex flex-col items-start gap-3 sm:max-w-64 sm:items-end'>
          <Button
            variant='outline'
            onClick={() => void query.refetch()}
            disabled={query.isFetching}
          >
            {query.isFetching ? t('Refreshing...') : t('Refresh status')}
          </Button>
          <p className='text-muted-foreground text-xs leading-relaxed sm:text-right'>
            {t(
              'Refreshing reads saved results only. It never starts a model request.'
            )}
          </p>
        </div>
      </header>
      <QueryFeedback
        pending={query.isPending}
        error={query.error}
        retry={() => void query.refetch()}
      />
      {data && !data.enabled && (
        <Alert>
          <AlertDescription>
            {t('The public status page is not enabled.')}
          </AlertDescription>
        </Alert>
      )}
      {data?.enabled && (
        <>
          {data.bucket_seconds > 0 && (
            <p className='text-muted-foreground text-xs'>
              {t('Business metrics use completed {{minutes}}-minute buckets.', {
                minutes: data.bucket_seconds / 60,
              })}
            </p>
          )}
          {data.paused && (
            <Alert>
              <AlertDescription>
                {t('Detection paused. Historical samples remain available.')}
              </AlertDescription>
            </Alert>
          )}
          {query.isError && (
            <p role='status' className='text-muted-foreground text-sm'>
              {t('Refresh failed. Showing previously saved data.')}
            </p>
          )}
          <dl className='grid grid-cols-1 gap-6 rounded-xl border p-5 sm:grid-cols-3 sm:gap-8 sm:px-6'>
            <div>
              <dt className='text-muted-foreground text-xs'>
                {t('Published groups')}
              </dt>
              <dd className='mt-2 text-2xl font-medium tabular-nums'>
                {data.summary.groups}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground text-xs'>
                {t('Groups needing attention')}
              </dt>
              <dd className='mt-2 text-2xl font-medium tabular-nums'>
                {data.summary.attention}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground text-xs'>
                {t('Unique detected channels')}
              </dt>
              <dd className='mt-2 text-2xl font-medium tabular-nums'>
                {data.summary.channels}
              </dd>
            </div>
          </dl>
        </>
      )}
      <StatusFilterBar
        filters={filters}
        models={data?.models ?? []}
        onChange={setFilters}
      />
      {!filters.model && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Business metrics aggregate across models. Select one model for comparison.'
          )}
        </p>
      )}
      {data?.enabled && (
        <>
          <QueryFeedback empty={!data.groups.length} />
          {data.groups.length > 0 && (
            <GroupList
              data={data}
              hours={filters.hours}
              model={filters.model}
            />
          )}
          <nav
            aria-label={t('Pagination')}
            className='flex flex-wrap items-center justify-end gap-3'
          >
            <Button
              variant='outline'
              disabled={filters.page === 1 || query.isFetching}
              onClick={() => setFilters({ ...filters, page: filters.page - 1 })}
            >
              {t('Previous')}
            </Button>
            <span className='text-muted-foreground text-sm'>
              {t('Page {{page}}', { page: data.page })}
            </span>
            <Button
              variant='outline'
              disabled={
                data.page * data.page_size >= data.total || query.isFetching
              }
              onClick={() => setFilters({ ...filters, page: filters.page + 1 })}
            >
              {t('Next')}
            </Button>
          </nav>
        </>
      )}
    </main>
  )
}

export function Status() {
  return (
    <PublicLayout showMainContainer={false}>
      <StatusContent />
    </PublicLayout>
  )
}
