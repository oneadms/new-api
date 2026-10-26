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
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { SavedTime } from '@/features/quality-detection/components/SavedTime'

import type { StatusGroup } from '../types'

export function StatusTrend(props: {
  series: StatusGroup['series']
  start: number
  end: number
  detail?: boolean
}) {
  const { t } = useTranslation()
  const titleId = useId()
  let path = ''
  let connected = false
  const points: { ts: number; x: number; y: number }[] = []
  for (const sample of props.series) {
    if (sample.value === null || !Number.isFinite(sample.value)) {
      connected = false
      continue
    }
    const x =
      4 +
      ((sample.ts - props.start) / Math.max(1, props.end - props.start)) * 272
    const y = 48 - sample.value * 0.44
    path += `${connected ? 'L' : 'M'}${x.toFixed(2)},${y.toFixed(2)} `
    points.push({ ts: sample.ts, x, y })
    connected = true
  }
  return (
    <figure className='min-w-0'>
      <figcaption id={titleId} className='text-muted-foreground mb-1 text-xs'>
        {t('Business success rate trend')}
      </figcaption>
      <p id={`${titleId}-range`} className='sr-only'>
        {t('Success rate')} (%): <SavedTime value={props.start} /> –{' '}
        <SavedTime value={props.end} />
      </p>
      {points.length === 0 ? (
        <p className='text-muted-foreground py-4 text-xs'>{t('No sample')}</p>
      ) : (
        <svg
          viewBox='0 0 280 52'
          role='img'
          aria-labelledby={titleId}
          aria-describedby={`${titleId}-range`}
          className='text-foreground h-14 w-full'
          preserveAspectRatio='none'
        >
          <path
            d={path}
            fill='none'
            stroke='currentColor'
            strokeWidth='1.5'
            vectorEffect='non-scaling-stroke'
          />
          {points.map((point) => (
            <circle
              key={point.ts}
              cx={point.x}
              cy={point.y}
              r='1.5'
              fill='currentColor'
            />
          ))}
        </svg>
      )}
      {props.detail && (
        <details className='mt-3'>
          <summary className='cursor-pointer text-sm'>
            {t('View recorded data points')}
          </summary>
          <p className='text-muted-foreground my-2 text-xs'>
            {t('Missing data creates gaps, not zero values.')}
          </p>
          <div className='max-h-64 overflow-y-auto'>
            <table className='w-full text-left text-xs'>
              <thead>
                <tr>
                  <th>{t('Time')}</th>
                  <th>{t('Success rate')}</th>
                </tr>
              </thead>
              <tbody>
                {props.series.map((point) => (
                  <tr key={point.ts} className='border-b'>
                    <td className='py-2'>
                      <SavedTime value={point.ts} />
                    </td>
                    <td>
                      {point.value === null
                        ? t('No sample')
                        : `${point.value}%`}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      )}
    </figure>
  )
}
