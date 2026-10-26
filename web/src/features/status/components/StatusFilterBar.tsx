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

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import type { StatusFilters } from '../types'

export function StatusFilterBar(props: {
  filters: StatusFilters
  models: string[]
  onChange: (filters: StatusFilters) => void
}) {
  const { t } = useTranslation()
  const states = [
    ['', t('All statuses')],
    ['passed', t('Rules passed')],
    ['review', t('Needs review')],
    ['partial', t('Some probe requests failed')],
    ['failed', t('Failed')],
    ['unknown', t('Insufficient data')],
    ['stale', t('Stale sample')],
    ['paused', t('Detection paused')],
  ]
  return (
    <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-5'>
      <div className='space-y-2'>
        <Label htmlFor='status-group'>{t('Search groups')}</Label>
        <Input
          id='status-group'
          value={props.filters.group}
          onChange={(event) =>
            props.onChange({
              ...props.filters,
              group: event.target.value,
              page: 1,
            })
          }
        />
      </div>
      <div className='min-w-0 space-y-2'>
        <Label htmlFor='status-model'>{t('Model')}</Label>
        <NativeSelect
          id='status-model'
          className='w-full'
          value={props.filters.model}
          onChange={(event) =>
            props.onChange({
              ...props.filters,
              model: event.target.value,
              page: 1,
            })
          }
        >
          <NativeSelectOption value=''>{t('All models')}</NativeSelectOption>
          {[
            ...new Set([...props.models, props.filters.model].filter(Boolean)),
          ].map((model) => (
            <NativeSelectOption key={model} value={model}>
              {model}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <div className='space-y-2'>
        <Label htmlFor='status-state'>{t('Detection status')}</Label>
        <NativeSelect
          id='status-state'
          className='w-full'
          value={props.filters.state}
          onChange={(event) =>
            props.onChange({
              ...props.filters,
              state: event.target.value,
              page: 1,
            })
          }
        >
          {states.map(([value, label]) => (
            <NativeSelectOption key={value} value={value}>
              {label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <div className='space-y-2'>
        <Label htmlFor='status-sort'>{t('Sort groups')}</Label>
        <NativeSelect
          id='status-sort'
          className='w-full'
          value={props.filters.sort}
          onChange={(event) =>
            props.onChange({
              ...props.filters,
              sort: event.target.value,
              page: 1,
            })
          }
        >
          <NativeSelectOption value='name'>{t('Name')}</NativeSelectOption>
          <NativeSelectOption value='state'>
            {t('Detection status')}
          </NativeSelectOption>
        </NativeSelect>
      </div>
      <div className='space-y-2'>
        <Label htmlFor='status-window'>{t('Time window')}</Label>
        <NativeSelect
          id='status-window'
          className='w-full'
          value={props.filters.hours}
          onChange={(event) =>
            props.onChange({
              ...props.filters,
              hours: Number(event.target.value),
              page: 1,
            })
          }
        >
          {[1, 6, 24, 168].map((hours) => (
            <NativeSelectOption key={hours} value={hours}>
              {t('Last {{hours}} hours', { hours })}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
    </div>
  )
}
