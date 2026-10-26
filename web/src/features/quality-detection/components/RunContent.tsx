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
import { useTranslation } from 'react-i18next'

import { getRun } from '../api'
import { ControlledThumbnail } from './ControlledThumbnail'
import { QueryFeedback } from './QueryFeedback'
import { RenderStatus } from './RenderStatus'
import { ResultBadge } from './ResultBadge'
import { RunOutput } from './RunOutput'
import { RunReview } from './RunReview'
import { SavedTime } from './SavedTime'

export function RunContent(props: { id: number }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['quality-detection', 'run', props.id],
    queryFn: ({ signal }) => getRun(props.id, signal),
    retry: false,
    refetchInterval: (state) => {
      const run = state.state.data
      return run &&
        (['queued', 'running'].includes(run.status) ||
          run.render_status === 'pending')
        ? 2000
        : false
    },
    refetchIntervalInBackground: false,
    gcTime: 0,
  })
  const run = query.isError ? undefined : query.data
  const baseline = useQuery({
    queryKey: ['quality-detection', 'run', run?.baseline_id],
    queryFn: ({ signal }) => getRun(run?.baseline_id ?? 0, signal),
    enabled: Boolean(run?.baseline_id),
    retry: false,
    gcTime: 0,
  })
  return (
    <div className='space-y-6'>
      <QueryFeedback
        pending={query.isPending}
        error={query.error}
        retry={() => void query.refetch()}
      />
      {run && (
        <>
          <div className='flex flex-wrap gap-2'>
            <ResultBadge value={run.status} />
            <ResultBadge value={run.quality_result} />
          </div>
          <dl className='grid grid-cols-2 gap-4 text-sm'>
            <div>
              <dt className='text-muted-foreground'>{t('Model')}</dt>
              <dd className='break-all'>
                {run.model}
                {run.mapped_model && ` → ${run.mapped_model}`}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>{t('Prompt version')}</dt>
              <dd>
                {run.prompt_name} · #{run.prompt_version_id}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>
                {t('First visible text')}
              </dt>
              <dd>
                {run.first_text_ms === null
                  ? t('No sample')
                  : `${run.first_text_ms} ms`}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>{t('Generation time')}</dt>
              <dd>
                {run.generation_ms === null
                  ? t('No sample')
                  : `${run.generation_ms} ms`}
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>{t('Input tokens')}</dt>
              <dd>{run.input_tokens ?? t('Unknown usage')}</dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>{t('Output tokens')}</dt>
              <dd>{run.output_tokens ?? t('Unknown usage')}</dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>{t('Completed')}</dt>
              <dd>
                <SavedTime value={run.completed_at} />
              </dd>
            </div>
            <div>
              <dt className='text-muted-foreground'>{t('Rendering')}</dt>
              <dd>
                <RenderStatus value={run.render_status} />
              </dd>
            </div>
          </dl>
          {run.error_code && (
            <p role='status' className='text-destructive text-sm break-all'>
              {t('Error code')}: {run.error_code}
            </p>
          )}
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='space-y-2'>
              <h3>{t('Current sample')}</h3>
              <ControlledThumbnail
                path={
                  run.has_thumbnail
                    ? `/api/quality-detection/runs/${run.id}/thumbnail`
                    : null
                }
                label={t('Static generated sample')}
                large
              />
            </div>
            {run.baseline_id > 0 && (
              <div className='space-y-2'>
                <h3>{t('Baseline')}</h3>
                <QueryFeedback
                  pending={baseline.isPending}
                  error={baseline.error}
                />
                {baseline.data && !baseline.isError && (
                  <ControlledThumbnail
                    path={
                      baseline.data.has_thumbnail
                        ? `/api/quality-detection/runs/${baseline.data.id}/thumbnail`
                        : null
                    }
                    label={t('Baseline')}
                    large
                  />
                )}
              </div>
            )}
          </div>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Screenshots are static PNG images. Generated HTML is never executed in this page.'
            )}
          </p>
          <RunOutput run={run} />
          <RunReview
            key={`${run.id}-${run.review_status}-${run.review_note}`}
            run={run}
          />
        </>
      )}
    </div>
  )
}
