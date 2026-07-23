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
import { zodResolver } from '@hookform/resolvers/zod'
import { Send } from 'lucide-react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Form } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'

import { useGrantGroupPasses } from '../../hooks'
import type { GroupPassTemplate } from '../../types'

const schema = z.object({
  userId: z.coerce.number().int().positive(),
  templateId: z.string().min(1),
  quantity: z.coerce.number().int().min(1).max(100),
  expiresAt: z.string(),
})

type FormValues = z.infer<typeof schema>

export function ManualGroupPassGrant(props: {
  templates: GroupPassTemplate[]
}) {
  const { t } = useTranslation()
  const grantMutation = useGrantGroupPasses()
  const enabledTemplates = props.templates.filter(
    (template) => template.enabled
  )
  const form = useForm<FormValues>({
    resolver: zodResolver(schema) as unknown as Resolver<FormValues>,
    defaultValues: {
      userId: 0,
      templateId: enabledTemplates[0]?.id ?? '',
      quantity: 1,
      expiresAt: '',
    },
  })

  async function onSubmit(values: FormValues) {
    const expiresAt = values.expiresAt
      ? Math.floor(new Date(values.expiresAt).getTime() / 1000)
      : 0
    const passes = await grantMutation.mutateAsync({
      user_id: values.userId,
      template_id: values.templateId,
      quantity: values.quantity,
      expires_at: expiresAt,
    })
    toast.success(
      t('{{count}} speed pass(es) granted', { count: passes.length })
    )
    form.reset({
      userId: 0,
      templateId: values.templateId,
      quantity: 1,
      expiresAt: '',
    })
  }

  return (
    <Card data-card-hover='false'>
      <CardHeader>
        <CardTitle>{t('Manual speed pass grant')}</CardTitle>
        <CardDescription>
          {t(
            'Grant cards directly to a user. Leave expiration empty to use the template validity period.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)}>
            <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
              <Field data-invalid={Boolean(form.formState.errors.userId)}>
                <FieldLabel htmlFor='group-pass-user-id'>
                  {t('User ID')}
                </FieldLabel>
                <Input
                  id='group-pass-user-id'
                  type='number'
                  min={1}
                  aria-invalid={Boolean(form.formState.errors.userId)}
                  {...form.register('userId', { valueAsNumber: true })}
                />
                <FieldError errors={[form.formState.errors.userId]} />
              </Field>
              <Field data-invalid={Boolean(form.formState.errors.templateId)}>
                <FieldLabel htmlFor='group-pass-template'>
                  {t('Speed pass template')}
                </FieldLabel>
                <NativeSelect
                  id='group-pass-template'
                  className='w-full'
                  aria-invalid={Boolean(form.formState.errors.templateId)}
                  {...form.register('templateId')}
                >
                  <NativeSelectOption value=''>
                    {t('Select a template')}
                  </NativeSelectOption>
                  {enabledTemplates.map((template) => (
                    <NativeSelectOption key={template.id} value={template.id}>
                      {template.name}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
                <FieldError errors={[form.formState.errors.templateId]} />
              </Field>
              <Field data-invalid={Boolean(form.formState.errors.quantity)}>
                <FieldLabel htmlFor='group-pass-quantity'>
                  {t('Quantity')}
                </FieldLabel>
                <Input
                  id='group-pass-quantity'
                  type='number'
                  min={1}
                  max={100}
                  aria-invalid={Boolean(form.formState.errors.quantity)}
                  {...form.register('quantity', { valueAsNumber: true })}
                />
                <FieldError errors={[form.formState.errors.quantity]} />
              </Field>
              <Field>
                <FieldLabel htmlFor='group-pass-expiration'>
                  {t('Activation deadline')}
                </FieldLabel>
                <Input
                  id='group-pass-expiration'
                  type='datetime-local'
                  {...form.register('expiresAt')}
                />
                <FieldDescription>{t('Optional')}</FieldDescription>
              </Field>
            </FieldGroup>
            <div className='mt-4 flex justify-end'>
              <Button
                type='submit'
                disabled={
                  grantMutation.isPending || enabledTemplates.length === 0
                }
              >
                {grantMutation.isPending ? (
                  <Spinner data-icon='inline-start' />
                ) : (
                  <Send data-icon='inline-start' />
                )}
                {t('Grant speed pass')}
              </Button>
            </div>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}
