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
import { Plus, Trash2 } from 'lucide-react'
import { useFieldArray, useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { SettingsPageFormActions } from '@/features/system-settings/components/settings-page-context'
import { getCurrencyAmountLabel } from '@/lib/currency'
import type { CurrencyConfig } from '@/stores/system-config-store'

import { useSaveRechargeRewardSettings } from '../../hooks'
import {
  createRechargeRewardFormSchema,
  rechargeRewardFormToSettings,
  rechargeRewardSettingsToForm,
  type RechargeRewardFormValues,
} from '../../lib/admin-form'
import type { RechargeRewardSettings } from '../../types'
import { ManualGroupPassGrant } from './manual-group-pass-grant'

function createConfigId(prefix: string) {
  return `${prefix}-${crypto.randomUUID().replaceAll('-', '').slice(0, 16)}`
}

function RemoveButton(props: { label: string; onClick: () => void }) {
  return (
    <Button
      type='button'
      size='icon-sm'
      variant='ghost'
      aria-label={props.label}
      onClick={props.onClick}
    >
      <Trash2 aria-hidden='true' />
    </Button>
  )
}

export function RechargeRewardConfigForm(props: {
  settings: RechargeRewardSettings
  groups: string[]
  currencyConfig: CurrencyConfig
}) {
  const { t } = useTranslation()
  const schema = createRechargeRewardFormSchema(t)
  const defaultValues = rechargeRewardSettingsToForm(
    props.settings,
    props.currencyConfig
  )
  const form = useForm<RechargeRewardFormValues>({
    resolver: zodResolver(schema) as Resolver<RechargeRewardFormValues>,
    defaultValues,
  })
  const templates = useFieldArray({ control: form.control, name: 'templates' })
  const rules = useFieldArray({ control: form.control, name: 'rules' })
  const prizes = useFieldArray({ control: form.control, name: 'prizes' })
  const saveMutation = useSaveRechargeRewardSettings()
  const watchedTemplates = form.watch('templates')
  const watchedPrizes = form.watch('prizes')
  const groupPassEnabled = form.watch('groupPassEnabled')
  const lotteryEnabled = form.watch('lotteryEnabled')
  const totalProbability = watchedPrizes.reduce(
    (total, prize) =>
      prize.enabled ? total + Number(prize.probabilityPercent || 0) : total,
    0
  )
  const amountLabel = getCurrencyAmountLabel(props.currencyConfig)

  async function onSubmit(values: RechargeRewardFormValues) {
    const saved = await saveMutation.mutateAsync(
      rechargeRewardFormToSettings(values, props.settings, props.currencyConfig)
    )
    form.reset(rechargeRewardSettingsToForm(saved, props.currencyConfig))
    toast.success(t('Recharge reward settings saved'))
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-5'>
        <Card data-card-hover='false'>
          <CardHeader>
            <CardTitle>{t('Speed pass templates')}</CardTitle>
            <CardDescription>
              {t(
                'A speed pass grants temporary access to one configured billing group. The activation deadline and access duration are separate.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <Field orientation='horizontal'>
              <div className='flex-1'>
                <FieldLabel htmlFor='group-pass-enabled'>
                  {t('Enable speed passes')}
                </FieldLabel>
                <FieldDescription>
                  {t(
                    'Turning this off immediately pauses all temporary group access.'
                  )}
                </FieldDescription>
              </div>
              <Switch
                id='group-pass-enabled'
                checked={groupPassEnabled}
                onCheckedChange={(checked) =>
                  form.setValue('groupPassEnabled', checked, {
                    shouldDirty: true,
                  })
                }
              />
            </Field>
            <Separator />
            <FieldGroup>
              {templates.fields.map((field, index) => (
                <Card key={field.id} size='sm' data-card-hover='false'>
                  <CardHeader className='flex-row items-center justify-between'>
                    <CardTitle className='text-sm'>
                      {watchedTemplates[index]?.name || t('New speed pass')}
                    </CardTitle>
                    <RemoveButton
                      label={t('Remove speed pass template')}
                      onClick={() => templates.remove(index)}
                    />
                  </CardHeader>
                  <CardContent>
                    <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-5'>
                      <Field>
                        <FieldLabel htmlFor={`template-name-${index}`}>
                          {t('Template name')}
                        </FieldLabel>
                        <Input
                          id={`template-name-${index}`}
                          {...form.register(`templates.${index}.name`)}
                        />
                      </Field>
                      <Field>
                        <FieldLabel htmlFor={`template-group-${index}`}>
                          {t('Target group')}
                        </FieldLabel>
                        <NativeSelect
                          id={`template-group-${index}`}
                          className='w-full'
                          {...form.register(`templates.${index}.groupName`)}
                        >
                          <NativeSelectOption value=''>
                            {t('Select a group')}
                          </NativeSelectOption>
                          {props.groups
                            .filter((group) => group !== 'auto')
                            .map((group) => (
                              <NativeSelectOption key={group} value={group}>
                                {group}
                              </NativeSelectOption>
                            ))}
                        </NativeSelect>
                      </Field>
                      <Field>
                        <FieldLabel htmlFor={`template-duration-${index}`}>
                          {t('Access minutes')}
                        </FieldLabel>
                        <Input
                          id={`template-duration-${index}`}
                          type='number'
                          min={1}
                          max={1440}
                          {...form.register(
                            `templates.${index}.durationMinutes`,
                            { valueAsNumber: true }
                          )}
                        />
                      </Field>
                      <Field>
                        <FieldLabel htmlFor={`template-validity-${index}`}>
                          {t('Valid days before activation')}
                        </FieldLabel>
                        <Input
                          id={`template-validity-${index}`}
                          type='number'
                          min={1}
                          max={3650}
                          {...form.register(`templates.${index}.validDays`, {
                            valueAsNumber: true,
                          })}
                        />
                      </Field>
                      <Field orientation='horizontal' className='self-end py-2'>
                        <FieldLabel htmlFor={`template-enabled-${index}`}>
                          {t('Enabled')}
                        </FieldLabel>
                        <Switch
                          id={`template-enabled-${index}`}
                          checked={watchedTemplates[index]?.enabled ?? false}
                          onCheckedChange={(checked) =>
                            form.setValue(
                              `templates.${index}.enabled`,
                              checked,
                              { shouldDirty: true }
                            )
                          }
                        />
                      </Field>
                    </FieldGroup>
                  </CardContent>
                </Card>
              ))}
            </FieldGroup>
            <Button
              type='button'
              variant='outline'
              onClick={() =>
                templates.append({
                  id: createConfigId('pass'),
                  name: '',
                  groupName:
                    props.groups.find((group) => group !== 'auto') ?? '',
                  durationMinutes: 60,
                  validDays: 30,
                  enabled: true,
                })
              }
            >
              <Plus data-icon='inline-start' />
              {t('Add speed pass template')}
            </Button>
          </CardContent>
        </Card>

        <Card data-card-hover='false'>
          <CardHeader>
            <CardTitle>{t('Recharge trigger rules')}</CardTitle>
            <CardDescription>
              {t(
                'Every successful qualifying recharge grants the configured cards exactly once.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <FieldGroup>
              {rules.fields.map((field, index) => (
                <Card key={field.id} size='sm' data-card-hover='false'>
                  <CardHeader className='flex-row items-center justify-between'>
                    <CardTitle className='text-sm'>
                      {form.watch(`rules.${index}.name`) ||
                        t('New recharge rule')}
                    </CardTitle>
                    <RemoveButton
                      label={t('Remove recharge rule')}
                      onClick={() => rules.remove(index)}
                    />
                  </CardHeader>
                  <CardContent>
                    <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-5'>
                      <Field>
                        <FieldLabel htmlFor={`rule-name-${index}`}>
                          {t('Rule name')}
                        </FieldLabel>
                        <Input
                          id={`rule-name-${index}`}
                          {...form.register(`rules.${index}.name`)}
                        />
                      </Field>
                      <Field>
                        <FieldLabel htmlFor={`rule-threshold-${index}`}>
                          {t('Minimum recharge')} ({amountLabel})
                        </FieldLabel>
                        <Input
                          id={`rule-threshold-${index}`}
                          type='number'
                          min={0}
                          step='any'
                          {...form.register(
                            `rules.${index}.minRechargeAmount`,
                            { valueAsNumber: true }
                          )}
                        />
                      </Field>
                      <Field>
                        <FieldLabel htmlFor={`rule-template-${index}`}>
                          {t('Speed pass template')}
                        </FieldLabel>
                        <NativeSelect
                          id={`rule-template-${index}`}
                          className='w-full'
                          {...form.register(`rules.${index}.templateId`)}
                        >
                          <NativeSelectOption value=''>
                            {t('Select a template')}
                          </NativeSelectOption>
                          {watchedTemplates.map((template) => (
                            <NativeSelectOption
                              key={template.id}
                              value={template.id}
                            >
                              {template.name || template.id}
                            </NativeSelectOption>
                          ))}
                        </NativeSelect>
                      </Field>
                      <Field>
                        <FieldLabel htmlFor={`rule-quantity-${index}`}>
                          {t('Quantity')}
                        </FieldLabel>
                        <Input
                          id={`rule-quantity-${index}`}
                          type='number'
                          min={1}
                          max={100}
                          {...form.register(`rules.${index}.quantity`, {
                            valueAsNumber: true,
                          })}
                        />
                      </Field>
                      <Field orientation='horizontal' className='self-end py-2'>
                        <FieldLabel htmlFor={`rule-enabled-${index}`}>
                          {t('Enabled')}
                        </FieldLabel>
                        <Switch
                          id={`rule-enabled-${index}`}
                          checked={form.watch(`rules.${index}.enabled`)}
                          onCheckedChange={(checked) =>
                            form.setValue(`rules.${index}.enabled`, checked, {
                              shouldDirty: true,
                            })
                          }
                        />
                      </Field>
                    </FieldGroup>
                  </CardContent>
                </Card>
              ))}
            </FieldGroup>
            <Button
              type='button'
              variant='outline'
              disabled={watchedTemplates.length === 0}
              onClick={() =>
                rules.append({
                  id: createConfigId('rule'),
                  name: '',
                  minRechargeAmount: 1,
                  templateId: watchedTemplates[0]?.id ?? '',
                  quantity: 1,
                  enabled: true,
                })
              }
            >
              <Plus data-icon='inline-start' />
              {t('Add recharge rule')}
            </Button>
          </CardContent>
        </Card>

        <Card data-card-hover='false'>
          <CardHeader>
            <CardTitle className='flex items-center justify-between gap-3'>
              <span>{t('Recharge draw')}</span>
              <Badge
                variant={totalProbability > 100 ? 'destructive' : 'outline'}
              >
                {t('Total probability: {{percent}}%', {
                  percent: totalProbability.toFixed(2),
                })}
              </Badge>
            </CardTitle>
            <CardDescription>
              {t(
                'Any probability left below 100% is the no-prize probability.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='space-y-4'>
            <Field orientation='horizontal'>
              <div className='flex-1'>
                <FieldLabel htmlFor='lottery-enabled'>
                  {t('Enable recharge draw')}
                </FieldLabel>
                <FieldDescription>
                  {t(
                    'Existing draw chances remain stored while the feature is paused.'
                  )}
                </FieldDescription>
              </div>
              <Switch
                id='lottery-enabled'
                checked={lotteryEnabled}
                onCheckedChange={(checked) =>
                  form.setValue('lotteryEnabled', checked, {
                    shouldDirty: true,
                  })
                }
              />
            </Field>
            <FieldError errors={[form.formState.errors.lotteryEnabled]} />
            <FieldGroup className='grid gap-4 md:grid-cols-2'>
              <Field>
                <FieldLabel htmlFor='lottery-threshold'>
                  {t('Minimum recharge')} ({amountLabel})
                </FieldLabel>
                <Input
                  id='lottery-threshold'
                  type='number'
                  min={0}
                  step='any'
                  {...form.register('lotteryMinRechargeAmount', {
                    valueAsNumber: true,
                  })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor='lottery-draw-count'>
                  {t('Draws per qualifying recharge')}
                </FieldLabel>
                <Input
                  id='lottery-draw-count'
                  type='number'
                  min={0}
                  max={100}
                  {...form.register('lotteryDrawsPerRecharge', {
                    valueAsNumber: true,
                  })}
                />
              </Field>
            </FieldGroup>
            <Separator />
            <FieldGroup>
              {prizes.fields.map((field, index) => {
                const prize = watchedPrizes[index]
                return (
                  <Card key={field.id} size='sm' data-card-hover='false'>
                    <CardHeader className='flex-row items-center justify-between'>
                      <CardTitle className='text-sm'>
                        {prize?.name || t('New prize')}
                      </CardTitle>
                      <RemoveButton
                        label={t('Remove prize')}
                        onClick={() => prizes.remove(index)}
                      />
                    </CardHeader>
                    <CardContent>
                      <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-6'>
                        <Field>
                          <FieldLabel htmlFor={`prize-name-${index}`}>
                            {t('Prize name')}
                          </FieldLabel>
                          <Input
                            id={`prize-name-${index}`}
                            {...form.register(`prizes.${index}.name`)}
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor={`prize-type-${index}`}>
                            {t('Prize type')}
                          </FieldLabel>
                          <NativeSelect
                            id={`prize-type-${index}`}
                            className='w-full'
                            {...form.register(`prizes.${index}.type`)}
                          >
                            <NativeSelectOption value='quota'>
                              {t('Quota')}
                            </NativeSelectOption>
                            <NativeSelectOption value='group_pass'>
                              {t('Speed pass')}
                            </NativeSelectOption>
                          </NativeSelect>
                        </Field>
                        <Field>
                          <FieldLabel htmlFor={`prize-probability-${index}`}>
                            {t('Probability (%)')}
                          </FieldLabel>
                          <Input
                            id={`prize-probability-${index}`}
                            type='number'
                            min={0}
                            max={100}
                            step={0.01}
                            {...form.register(
                              `prizes.${index}.probabilityPercent`,
                              { valueAsNumber: true }
                            )}
                          />
                        </Field>
                        {prize?.type === 'quota' ? (
                          <>
                            <Field>
                              <FieldLabel htmlFor={`prize-min-${index}`}>
                                {t('Minimum reward')} ({amountLabel})
                              </FieldLabel>
                              <Input
                                id={`prize-min-${index}`}
                                type='number'
                                min={0}
                                step='any'
                                {...form.register(`prizes.${index}.minAmount`, {
                                  valueAsNumber: true,
                                })}
                              />
                            </Field>
                            <Field>
                              <FieldLabel htmlFor={`prize-max-${index}`}>
                                {t('Maximum reward')} ({amountLabel})
                              </FieldLabel>
                              <Input
                                id={`prize-max-${index}`}
                                type='number'
                                min={0}
                                step='any'
                                {...form.register(`prizes.${index}.maxAmount`, {
                                  valueAsNumber: true,
                                })}
                              />
                            </Field>
                          </>
                        ) : (
                          <>
                            <Field>
                              <FieldLabel htmlFor={`prize-template-${index}`}>
                                {t('Speed pass template')}
                              </FieldLabel>
                              <NativeSelect
                                id={`prize-template-${index}`}
                                className='w-full'
                                {...form.register(`prizes.${index}.templateId`)}
                              >
                                <NativeSelectOption value=''>
                                  {t('Select a template')}
                                </NativeSelectOption>
                                {watchedTemplates.map((template) => (
                                  <NativeSelectOption
                                    key={template.id}
                                    value={template.id}
                                  >
                                    {template.name || template.id}
                                  </NativeSelectOption>
                                ))}
                              </NativeSelect>
                            </Field>
                            <Field>
                              <FieldLabel htmlFor={`prize-quantity-${index}`}>
                                {t('Quantity')}
                              </FieldLabel>
                              <Input
                                id={`prize-quantity-${index}`}
                                type='number'
                                min={1}
                                max={100}
                                {...form.register(`prizes.${index}.quantity`, {
                                  valueAsNumber: true,
                                })}
                              />
                            </Field>
                          </>
                        )}
                        <Field
                          orientation='horizontal'
                          className='self-end py-2'
                        >
                          <FieldLabel htmlFor={`prize-enabled-${index}`}>
                            {t('Enabled')}
                          </FieldLabel>
                          <Switch
                            id={`prize-enabled-${index}`}
                            checked={prize?.enabled ?? false}
                            onCheckedChange={(checked) =>
                              form.setValue(
                                `prizes.${index}.enabled`,
                                checked,
                                { shouldDirty: true }
                              )
                            }
                          />
                        </Field>
                      </FieldGroup>
                    </CardContent>
                  </Card>
                )
              })}
            </FieldGroup>
            <FieldError errors={[form.formState.errors.prizes?.root]} />
            <Button
              type='button'
              variant='outline'
              onClick={() =>
                prizes.append({
                  id: createConfigId('prize'),
                  name: '',
                  type: 'quota',
                  probabilityPercent: 0,
                  minAmount: 1,
                  maxAmount: 1,
                  templateId: watchedTemplates[0]?.id ?? '',
                  quantity: 1,
                  enabled: true,
                })
              }
            >
              <Plus data-icon='inline-start' />
              {t('Add prize')}
            </Button>
          </CardContent>
        </Card>

        <Alert>
          <AlertTitle>{t('Financial safety')}</AlertTitle>
          <AlertDescription>
            {t(
              'Probabilities use 10,000 basis points on the server. Recharge completion, draw consumption, quota awards, and card grants commit in one transaction.'
            )}
          </AlertDescription>
        </Alert>

        <SettingsPageFormActions
          onSave={form.handleSubmit(onSubmit)}
          onReset={() => form.reset(defaultValues)}
          isSaving={saveMutation.isPending}
          isSaveDisabled={!form.formState.isDirty}
          isResetDisabled={!form.formState.isDirty}
          saveLabel='Save reward settings'
        />
      </form>

      <div className='mt-5'>
        <ManualGroupPassGrant templates={props.settings.group_pass_templates} />
      </div>
    </Form>
  )
}
