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
import { useTranslation } from 'react-i18next'

import { ControlledThumbnail } from '@/features/quality-detection/components/ControlledThumbnail'
import { RenderStatus } from '@/features/quality-detection/components/RenderStatus'
import { ResultBadge } from '@/features/quality-detection/components/ResultBadge'
import { SavedTime } from '@/features/quality-detection/components/SavedTime'
import { publicThumbnailPath } from '@/features/quality-detection/lib/artifacts'

import type { StatusSample } from '../types'

export function PublicSample(props: {
  sample: StatusSample
  generatedAt: number
  large?: boolean
}) {
  const { t } = useTranslation()
  const sample = props.sample
  const path = publicThumbnailPath(sample.public_id)
  return (
    <article className='min-w-0 space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-medium break-all'>{sample.channel_name}</span>
        <ResultBadge value={sample.quality_result} />
        {sample.stale_after > 0 && props.generatedAt >= sample.stale_after && (
          <ResultBadge value='stale' />
        )}
      </div>
      <p className='text-muted-foreground text-xs break-all'>
        {sample.model} · <SavedTime value={sample.completed_at} />
      </p>
      <ControlledThumbnail
        path={sample.thumbnail_url === path ? path : null}
        label={t('Static generated sample')}
        large={props.large}
      />
      {props.large && (
        <>
          <p className='text-muted-foreground text-xs'>
            {t(
              'A shared channel sample may appear in several groups. A sample does not prove model identity or overall ability.'
            )}
          </p>
          <div className='flex flex-wrap gap-3 text-xs'>
            <span>
              {t('Execution')}: <ResultBadge value={sample.status} />
            </span>
            <span>
              {t('First visible text')}:{' '}
              {sample.first_text_ms === null
                ? t('No sample')
                : `${sample.first_text_ms} ms`}
            </span>
            <span>
              {t('Generation time')}:{' '}
              {sample.generation_ms === null
                ? t('No sample')
                : `${sample.generation_ms} ms`}
            </span>
            <span>
              {t('Rendering')}: <RenderStatus value={sample.render_status} />
            </span>
          </div>
        </>
      )}
    </article>
  )
}
