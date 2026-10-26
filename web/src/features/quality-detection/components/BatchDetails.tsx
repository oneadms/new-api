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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { cancelBatch, getBatch } from '../api'
import { useQualityMutation } from '../hooks/use-quality-mutation'
import { isBatchActive } from '../lib/batches'
import { QueryFeedback } from './QueryFeedback'
import { ResultBadge } from './ResultBadge'
import { RunDetails } from './RunDetails'

export function BatchDetails(props: { id: number }) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState('')
  const [state, setState] = useState('')
  const query = useQuery({
    queryKey: ['quality-detection', 'batch', props.id],
    queryFn: ({ signal }) => getBatch(props.id, signal),
    retry: false,
    refetchInterval: (query) =>
      query.state.data && isBatchActive(query.state.data.batch) ? 2000 : false,
    refetchIntervalInBackground: false,
  })
  const cancel = useQualityMutation(() => cancelBatch(props.id))
  const data = query.isError ? undefined : query.data
  const runs = (data?.runs ?? []).filter(
    (run) =>
      (!state || run.status === state) &&
      `${run.channel_name} ${run.model} ${run.prompt_name}`
        .toLocaleLowerCase()
        .includes(filter.toLocaleLowerCase())
  )
  return (
    <section className='space-y-4'>
      <QueryFeedback
        pending={query.isPending}
        error={query.error}
        retry={() => void query.refetch()}
      />
      {data && (
        <>
          <div className='flex flex-wrap items-center justify-between gap-4'>
            <div className='flex flex-wrap items-center gap-3'>
              <h2 className='font-medium'>
                {t('Batch')} #{data.batch.id}
              </h2>
              <ResultBadge value={data.batch.status} />
              <span className='text-sm tabular-nums'>
                {data.batch.completed} / {data.batch.total} · {t('Failed')}:{' '}
                {data.batch.failed}
              </span>
            </div>
            {isBatchActive(data.batch) && (
              <Button
                variant='destructive'
                disabled={cancel.isPending || data.batch.cancel_requested}
                onClick={() => cancel.mutate()}
              >
                {data.batch.cancel_requested
                  ? t('Stop requested')
                  : t('Stop batch')}
              </Button>
            )}
          </div>
          {data.batch.summary && data.batch.summary !== data.batch.status && (
            <p className='text-muted-foreground text-sm'>
              {data.batch.summary}
            </p>
          )}
          <QueryFeedback error={cancel.error} />
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label htmlFor='run-filter'>
                {t('Filter this batch by channel, model or prompt')}
              </Label>
              <Input
                id='run-filter'
                value={filter}
                onChange={(event) => setFilter(event.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='run-state'>{t('Execution')}</Label>
              <NativeSelect
                id='run-state'
                value={state}
                onChange={(event) => setState(event.target.value)}
              >
                <NativeSelectOption value=''>
                  {t('All statuses')}
                </NativeSelectOption>
                {[
                  ['queued', t('Queued')],
                  ['running', t('Running')],
                  ['completed', t('Completed')],
                  ['failed', t('Failed')],
                  ['cancelled', t('Cancelled')],
                  ['interrupted', t('Interrupted')],
                  ['skipped', t('Skipped')],
                ].map(([value, label]) => (
                  <NativeSelectOption key={value} value={value}>
                    {label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
          </div>
          <QueryFeedback empty={!runs.length} />
          <div className='divide-y rounded-xl border'>
            {runs.map((run) => (
              <article
                key={run.id}
                className='flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between'
              >
                <div className='min-w-0 space-y-2'>
                  <h3 className='font-medium break-all'>{run.channel_name}</h3>
                  <p className='text-muted-foreground text-xs break-all'>
                    {run.model} · {run.prompt_name} · #{run.prompt_version_id}
                  </p>
                  <div className='flex flex-wrap gap-2'>
                    <ResultBadge value={run.status} />
                    <ResultBadge value={run.quality_result} />
                  </div>
                </div>
                <RunDetails run={run} />
              </article>
            ))}
          </div>
        </>
      )}
    </section>
  )
}
