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

import { ResultBadge } from '@/features/quality-detection/components/ResultBadge'

import type { StatusPageData } from '../types'
import { BusinessMetrics } from './BusinessMetrics'
import { GroupDetails } from './GroupDetails'
import { PublicSample } from './PublicSample'
import { StatusTrend } from './StatusTrend'

export function GroupList(props: {
  data: StatusPageData
  hours: number
  model: string
}) {
  const { t } = useTranslation()
  return (
    <section
      aria-label={t('Published groups')}
      className='bg-card divide-y overflow-hidden rounded-2xl border'
    >
      {props.data.groups.map((group) => (
        <article
          key={group.public_id}
          className='hover:bg-muted/20 grid min-w-0 gap-6 p-5 motion-safe:transition-colors motion-safe:duration-150 sm:p-6 xl:grid-cols-[minmax(10rem,0.8fr)_minmax(17rem,1.4fr)_minmax(12rem,1fr)_auto] xl:items-center'
        >
          <div className='min-w-0 space-y-2'>
            <h2 className='truncate text-lg font-semibold' title={group.name}>
              {group.name}
            </h2>
            <ResultBadge value={props.data.paused ? 'paused' : group.state} />
            <p className='text-muted-foreground text-xs'>
              {t('{{count}} public channels', { count: group.channel_count })}
            </p>
            <p className='text-muted-foreground text-xs'>
              {t(
                '{{fresh}} / {{configured}} configured channels have recent samples',
                { fresh: group.fresh_count, configured: group.configured_count }
              )}
            </p>
          </div>
          <div className='min-w-0 space-y-4'>
            <BusinessMetrics group={group} />
            <StatusTrend
              series={group.series}
              start={props.data.window_start}
              end={props.data.window_end}
            />
          </div>
          <div className='min-w-0 space-y-2'>
            <p className='text-muted-foreground text-xs'>
              {t('Latest quality sample')}
            </p>
            {group.latest_sample ? (
              <PublicSample
                sample={group.latest_sample}
                generatedAt={props.data.generated_at}
              />
            ) : (
              <p className='text-muted-foreground text-sm'>
                {props.model
                  ? t('This model has no detection sample')
                  : t('No sample')}
              </p>
            )}
          </div>
          <GroupDetails
            group={group}
            hours={props.hours}
            model={props.model}
            start={props.data.window_start}
            end={props.data.window_end}
            generatedAt={props.data.generated_at}
          />
        </article>
      ))}
    </section>
  )
}
