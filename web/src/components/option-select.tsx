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
import type { ComponentProps, Ref } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from '@/components/ui/combobox'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'

type Option<Value> = { value: Value; label: string; disabled?: boolean }
type OptionSelectProps<Value extends string | number> = Pick<
  ComponentProps<'button'>,
  'id' | 'name' | 'onBlur' | 'aria-label' | 'aria-invalid' | 'aria-describedby'
> & {
  options: Option<Value>[]
  value: Value
  onValueChange: (value: Value) => void
  searchable?: boolean
  disabled?: boolean
  placeholder?: string
  className?: string
  ref?: Ref<HTMLButtonElement>
}

/** A themed option picker with the same value contract for forms and filters. */
export function OptionSelect<Value extends string | number>(
  props: OptionSelectProps<Value>
) {
  const { t } = useTranslation()
  const selected = props.options.find((option) => option.value === props.value)
  const triggerProps = {
    id: props.id,
    ref: props.ref,
    onBlur: props.onBlur,
    'aria-label': props['aria-label'],
    'aria-invalid': props['aria-invalid'],
    'aria-describedby': props['aria-describedby'],
    className: cn(
      'h-9 w-full min-w-0 justify-between font-normal',
      props.className
    ),
  }
  const label = selected?.label ?? props.placeholder ?? String(props.value)

  if (props.searchable) {
    return (
      <Combobox<Option<Value>>
        items={props.options}
        value={selected ?? null}
        onValueChange={(option) => {
          if (option) props.onValueChange(option.value)
        }}
        isItemEqualToValue={(a, b) => a.value === b.value}
        disabled={props.disabled}
        name={props.name}
      >
        <ComboboxTrigger
          {...triggerProps}
          render={<Button variant='outline' />}
        >
          <span className='min-w-0 flex-1 truncate text-left' title={label}>
            {label}
          </span>
        </ComboboxTrigger>
        <ComboboxContent className='min-w-(--anchor-width) motion-reduce:animate-none'>
          <ComboboxInput
            aria-label={t('Search...')}
            placeholder={t('Search...')}
            showTrigger={false}
          />
          <ComboboxEmpty>{t('No results found')}</ComboboxEmpty>
          <ComboboxList>
            {(option: Option<Value>) => (
              <ComboboxItem
                key={option.value}
                value={option}
                disabled={option.disabled}
                className='min-h-9 py-2'
              >
                <span className='min-w-0 truncate' title={option.label}>
                  {option.label}
                </span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    )
  }

  return (
    <Select
      items={props.options}
      value={props.value}
      onValueChange={(value) => {
        if (value !== null) props.onValueChange(value)
      }}
      disabled={props.disabled}
      name={props.name}
    >
      <SelectTrigger {...triggerProps}>
        <SelectValue className='min-w-0 truncate' title={label}>
          {label}
        </SelectValue>
      </SelectTrigger>
      <SelectContent
        align='start'
        alignItemWithTrigger={false}
        className='max-h-64 max-w-(--available-width) motion-reduce:animate-none'
      >
        <SelectGroup>
          {props.options.map((option) => (
            <SelectItem
              key={option.value}
              value={option.value}
              disabled={option.disabled}
              className='min-h-9 py-2 *:data-[slot=select-item-text]:min-w-0 *:data-[slot=select-item-text]:shrink'
            >
              <span className='truncate' title={option.label}>
                {option.label}
              </span>
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
