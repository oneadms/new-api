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
import {
  Controller,
  type FieldPath,
  type FieldValues,
  type UseFormReturn,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

type FieldProps<T extends FieldValues> = {
  form: UseFormReturn<T>
  name: FieldPath<T>
  label: string
  hint?: string
  disabled?: boolean
}
export function QualityField<T extends FieldValues>(
  props: FieldProps<T> & {
    type?: 'text' | 'number' | 'textarea'
    options?: { value: string | number; label: string }[]
    min?: number
    max?: number
  }
) {
  const { t } = useTranslation()
  const id = useId()
  const error = props.form.getFieldState(props.name, props.form.formState).error
  const registration = props.form.register(props.name, {
    valueAsNumber: props.type === 'number',
  })
  const attrs = {
    id,
    disabled: props.disabled,
    'aria-invalid': Boolean(error),
    'aria-describedby': `${id}-hint ${id}-error`,
  }
  let control = (
    <Input
      {...attrs}
      {...registration}
      type={props.type ?? 'text'}
      min={props.min}
      max={props.max}
    />
  )
  if (props.type === 'textarea') {
    control = <Textarea {...attrs} {...registration} rows={8} />
  }
  if (props.options) {
    control = (
      <NativeSelect {...attrs} {...registration} className='w-full'>
        {props.options.map((option) => (
          <NativeSelectOption key={option.value} value={option.value}>
            {option.label}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    )
  }
  return (
    <div className='min-w-0 space-y-2'>
      <Label htmlFor={id}>{props.label}</Label>
      {control}
      <p id={`${id}-hint`} className='text-muted-foreground text-xs'>
        {props.hint}
      </p>
      <p
        id={`${id}-error`}
        role={error ? 'alert' : undefined}
        className='text-destructive text-xs'
      >
        {error?.message && t(error.message)}
      </p>
    </div>
  )
}
export function QualityToggle<T extends FieldValues>(props: FieldProps<T>) {
  const { t } = useTranslation()
  const id = useId()
  return (
    <Controller
      control={props.form.control}
      name={props.name}
      render={({ field, fieldState }) => (
        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div>
            <Label htmlFor={id}>{props.label}</Label>
            {props.hint && (
              <p
                id={`${id}-hint`}
                className='text-muted-foreground mt-1 text-xs'
              >
                {props.hint}
              </p>
            )}
            {fieldState.error?.message && (
              <p role='alert'>{t(fieldState.error.message)}</p>
            )}
          </div>
          <Switch
            id={id}
            checked={Boolean(field.value)}
            onCheckedChange={field.onChange}
            disabled={props.disabled}
            aria-describedby={props.hint ? `${id}-hint` : undefined}
            aria-invalid={Boolean(fieldState.error)}
          />
        </div>
      )}
    />
  )
}
