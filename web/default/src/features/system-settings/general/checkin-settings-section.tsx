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
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const createSchema = (t: (key: string) => string) =>
  z
    .object({
      enabled: z.boolean(),
      minQuota: z.coerce.number().int().min(0),
      maxQuota: z.coerce.number().int().min(0),
      luckyEnabled: z.boolean(),
      minStakeQuota: z.coerce.number().int().positive(),
      maxStakeQuota: z.coerce.number().int().positive(),
      minFailurePercent: z.coerce.number().min(0).max(100),
      maxFailurePercent: z.coerce.number().min(0).max(100),
      actualMinFailurePercent: z.coerce.number().min(0).max(100),
      actualMaxFailurePercent: z.coerce.number().min(0).max(100),
    })
    .superRefine((values, context) => {
      if (values.maxQuota < values.minQuota) {
        context.addIssue({
          code: 'custom',
          path: ['maxQuota'],
          message: t('Maximum must not be lower than minimum'),
        })
      }
      if (values.maxStakeQuota < values.minStakeQuota) {
        context.addIssue({
          code: 'custom',
          path: ['maxStakeQuota'],
          message: t('Maximum must not be lower than minimum'),
        })
      }
      if (values.maxFailurePercent < values.minFailurePercent) {
        context.addIssue({
          code: 'custom',
          path: ['maxFailurePercent'],
          message: t('Maximum must not be lower than minimum'),
        })
      }
      if (values.actualMaxFailurePercent < values.actualMinFailurePercent) {
        context.addIssue({
          code: 'custom',
          path: ['actualMaxFailurePercent'],
          message: t('Maximum must not be lower than minimum'),
        })
      }
    })

type Values = z.infer<ReturnType<typeof createSchema>>

const luckyNumberFields = [
  {
    name: 'minStakeQuota',
    label: 'Minimum lucky check-in stake',
    min: 1,
    step: 1,
  },
  {
    name: 'maxStakeQuota',
    label: 'Maximum lucky check-in stake',
    min: 1,
    step: 1,
  },
  {
    name: 'minFailurePercent',
    label: 'Displayed minimum failure probability (%)',
    min: 0,
    max: 100,
    step: 0.01,
  },
  {
    name: 'maxFailurePercent',
    label: 'Displayed maximum failure probability (%)',
    min: 0,
    max: 100,
    step: 0.01,
  },
  {
    name: 'actualMinFailurePercent',
    label: 'Actual minimum failure probability (%)',
    min: 0,
    max: 100,
    step: 0.01,
  },
  {
    name: 'actualMaxFailurePercent',
    label: 'Actual maximum failure probability (%)',
    min: 0,
    max: 100,
    step: 0.01,
  },
] as const

export function CheckinSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    minQuota: number
    maxQuota: number
    luckyEnabled: boolean
    minStakeQuota: number
    maxStakeQuota: number
    minFailureBps: number
    maxFailureBps: number
    actualMinFailureBps: number
    actualMaxFailureBps: number
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const schema = createSchema(t)

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      enabled: defaultValues.enabled,
      minQuota: defaultValues.minQuota,
      maxQuota: defaultValues.maxQuota,
      luckyEnabled: defaultValues.luckyEnabled,
      minStakeQuota: defaultValues.minStakeQuota,
      maxStakeQuota: defaultValues.maxStakeQuota,
      minFailurePercent: defaultValues.minFailureBps / 100,
      maxFailurePercent: defaultValues.maxFailureBps / 100,
      actualMinFailurePercent: defaultValues.actualMinFailureBps / 100,
      actualMaxFailurePercent: defaultValues.actualMaxFailureBps / 100,
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const enabled = form.watch('enabled')
  const luckyEnabled = form.watch('luckyEnabled')

  async function onSubmit(values: Values) {
    const updates: Array<{ key: string; value: string }> = []

    if (values.enabled !== defaultValues.enabled) {
      updates.push({
        key: 'checkin_setting.enabled',
        value: String(values.enabled),
      })
    }

    if (values.minQuota !== defaultValues.minQuota) {
      updates.push({
        key: 'checkin_setting.min_quota',
        value: String(values.minQuota),
      })
    }

    if (values.maxQuota !== defaultValues.maxQuota) {
      updates.push({
        key: 'checkin_setting.max_quota',
        value: String(values.maxQuota),
      })
    }

    const luckyUpdates = [
      ['lucky_checkin_setting.enabled', values.luckyEnabled],
      ['lucky_checkin_setting.min_stake_quota', values.minStakeQuota],
      ['lucky_checkin_setting.max_stake_quota', values.maxStakeQuota],
      [
        'lucky_checkin_setting.min_failure_bps',
        Math.round(values.minFailurePercent * 100),
      ],
      [
        'lucky_checkin_setting.max_failure_bps',
        Math.round(values.maxFailurePercent * 100),
      ],
      [
        'lucky_checkin_setting.actual_min_failure_bps',
        Math.round(values.actualMinFailurePercent * 100),
      ],
      [
        'lucky_checkin_setting.actual_max_failure_bps',
        Math.round(values.actualMaxFailurePercent * 100),
      ],
    ] as const
    const luckyDefaults = [
      defaultValues.luckyEnabled,
      defaultValues.minStakeQuota,
      defaultValues.maxStakeQuota,
      defaultValues.minFailureBps,
      defaultValues.maxFailureBps,
      defaultValues.actualMinFailureBps,
      defaultValues.actualMaxFailureBps,
    ]
    luckyUpdates.forEach(([key, value], index) => {
      if (value !== luckyDefaults[index]) {
        updates.push({ key, value: String(value) })
      }
    })

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    form.reset(values)
  }

  return (
    <SettingsSection title={t('Check-in Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save check-in settings'
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable check-in feature')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to check in daily for random quota rewards'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <div className='grid gap-6 border-b pb-6 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='minQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('1000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Minimum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='maxQuota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum check-in quota')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder={t('10000')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Maximum quota amount awarded for check-in')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}

          {enabled && (
            <>
              <FormField
                control={form.control}
                name='luckyEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable lucky check-in')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Allow users to risk quota for a chance to win the same amount'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              {luckyEnabled && (
                <div className='grid gap-6 sm:grid-cols-2'>
                  {luckyNumberFields.map(
                    ({ name, label, min, step, ...fieldProps }) => (
                      <FormField
                        key={name}
                        control={form.control}
                        name={name}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t(label)}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={min}
                                step={step}
                                {...fieldProps}
                                {...field}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    )
                  )}
                </div>
              )}
            </>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
