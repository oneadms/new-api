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

import { OptionSelect } from '@/components/option-select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import type { StatusFilters } from '../types'

export function StatusFilterBar(props: {
  filters: StatusFilters
  models: string[]
  onChange: (filters: StatusFilters) => void
}) {
  const { t } = useTranslation()
  const states = [
    { value: '', label: t('All statuses') },
    { value: 'passed', label: t('Rules passed') },
    { value: 'review', label: t('Needs review') },
    { value: 'partial', label: t('Some probe requests failed') },
    { value: 'failed', label: t('Failed') },
    { value: 'unknown', label: t('Insufficient data') },
    { value: 'stale', label: t('Stale sample') },
    { value: 'paused', label: t('Detection paused') },
  ]
  return (
    <div className='grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-5'>
      <div className='flex min-w-0 flex-col gap-2'>
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
      <div className='flex min-w-0 flex-col gap-2'>
        <Label htmlFor='status-model'>{t('Model')}</Label>
        <OptionSelect
          id='status-model'
          searchable
          className='w-full min-w-0'
          value={props.filters.model}
          onValueChange={(model) =>
            props.onChange({ ...props.filters, model, page: 1 })
          }
          options={[
            { value: '', label: t('All models') },
            ...[
              ...new Set(
                [...props.models, props.filters.model].filter(Boolean)
              ),
            ].map((model) => ({ value: model, label: model })),
          ]}
        />
      </div>
      <div className='flex min-w-0 flex-col gap-2'>
        <Label htmlFor='status-state'>{t('Detection status')}</Label>
        <OptionSelect
          id='status-state'
          className='w-full min-w-0'
          value={props.filters.state}
          onValueChange={(state) =>
            props.onChange({ ...props.filters, state, page: 1 })
          }
          options={states}
        />
      </div>
      <div className='flex min-w-0 flex-col gap-2'>
        <Label htmlFor='status-sort'>{t('Sort groups')}</Label>
        <OptionSelect
          id='status-sort'
          className='w-full min-w-0'
          value={props.filters.sort}
          onValueChange={(sort) =>
            props.onChange({ ...props.filters, sort, page: 1 })
          }
          options={[
            { value: 'name', label: t('Name') },
            { value: 'state', label: t('Detection status') },
          ]}
        />
      </div>
      <div className='flex min-w-0 flex-col gap-2'>
        <Label htmlFor='status-window'>{t('Time window')}</Label>
        <OptionSelect
          id='status-window'
          className='w-full min-w-0'
          value={props.filters.hours}
          onValueChange={(hours) =>
            props.onChange({ ...props.filters, hours, page: 1 })
          }
          options={[1, 6, 24, 168].map((hours) => ({
            value: hours,
            label: t('Last {{hours}} hours', { hours }),
          }))}
        />
      </div>
    </div>
  )
}
