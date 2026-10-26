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

import type { StatusGroup } from '../types'

export function BusinessMetrics(props: { group: StatusGroup }) {
  const { t } = useTranslation()
  const metrics = [
    { label: t('Success rate'), value: props.group.success_rate, unit: '%' },
    {
      label: t('Average latency'),
      value: props.group.avg_latency_ms,
      unit: 'ms',
    },
    {
      label: t('Output speed'),
      value: props.group.tokens_per_second,
      unit: 'token/s',
    },
  ]
  return (
    <div>
      <p className='text-muted-foreground mb-2 text-xs'>
        {t('Business request metrics')}
      </p>
      <dl className='grid grid-cols-3 gap-3'>
        {metrics.map((metric) => (
          <div key={metric.label} className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{metric.label}</dt>
            <dd className='mt-1 text-sm font-medium tabular-nums'>
              {metric.value === null
                ? t('No sample')
                : `${metric.value.toLocaleString(undefined, { maximumFractionDigits: 2 })} ${metric.unit}`}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  )
}
