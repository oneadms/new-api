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

import { currencyAmountToQuota, quotaToCurrencyAmount } from '@/lib/currency'
import type { CurrencyConfig } from '@/stores/system-config-store'

import type { RechargeRewardSettings } from '../types'

type Translate = (key: string) => string

export const createRechargeRewardFormSchema = (t: Translate) =>
  z
    .object({
      groupPassEnabled: z.boolean(),
      lotteryEnabled: z.boolean(),
      lotteryMinRechargeAmount: z.coerce.number().min(0),
      lotteryDrawsPerRecharge: z.coerce.number().int().min(0).max(100),
      templates: z
        .array(
          z.object({
            id: z.string().min(1).max(64),
            name: z.string().trim().min(1).max(100),
            groupName: z.string().trim().min(1).max(64),
            durationMinutes: z.coerce.number().int().min(1).max(1440),
            validDays: z.coerce.number().int().min(1).max(3650),
            enabled: z.boolean(),
          })
        )
        .max(100),
      rules: z
        .array(
          z.object({
            id: z.string().min(1).max(64),
            name: z.string().trim().min(1).max(100),
            minRechargeAmount: z.coerce.number().positive(),
            templateId: z.string().min(1),
            quantity: z.coerce.number().int().min(1).max(100),
            enabled: z.boolean(),
          })
        )
        .max(100),
      prizes: z
        .array(
          z.object({
            id: z.string().min(1).max(64),
            name: z.string().trim().min(1).max(100),
            type: z.enum(['quota', 'group_pass']),
            probabilityPercent: z.coerce.number().min(0).max(100),
            minAmount: z.coerce.number().min(0),
            maxAmount: z.coerce.number().min(0),
            templateId: z.string(),
            quantity: z.coerce.number().int().min(0).max(100),
            enabled: z.boolean(),
          })
        )
        .max(100),
    })
    .superRefine((values, context) => {
      const templatesById = new Map(
        values.templates.map((template) => [template.id, template])
      )
      values.rules.forEach((rule, index) => {
        const template = templatesById.get(rule.templateId)
        if (!template) {
          context.addIssue({
            code: 'custom',
            path: ['rules', index, 'templateId'],
            message: t('Select an existing speed pass template'),
          })
        } else if (rule.enabled && !template.enabled) {
          context.addIssue({
            code: 'custom',
            path: ['rules', index, 'templateId'],
            message: t('Select an enabled speed pass template'),
          })
        }
      })

      let totalProbability = 0
      values.prizes.forEach((prize, index) => {
        if (prize.enabled) totalProbability += prize.probabilityPercent
        if (prize.enabled && prize.probabilityPercent <= 0) {
          context.addIssue({
            code: 'custom',
            path: ['prizes', index, 'probabilityPercent'],
            message: t('Enabled prizes must have a probability above 0%'),
          })
        }
        if (
          prize.type === 'quota' &&
          (prize.minAmount <= 0 || prize.maxAmount < prize.minAmount)
        ) {
          context.addIssue({
            code: 'custom',
            path: ['prizes', index, 'maxAmount'],
            message: t('Maximum must not be lower than minimum'),
          })
        }
        if (
          prize.type === 'group_pass' &&
          (!templatesById.has(prize.templateId) || prize.quantity < 1)
        ) {
          context.addIssue({
            code: 'custom',
            path: ['prizes', index, 'templateId'],
            message: t('Select an existing speed pass template'),
          })
        } else if (
          prize.type === 'group_pass' &&
          prize.enabled &&
          !templatesById.get(prize.templateId)?.enabled
        ) {
          context.addIssue({
            code: 'custom',
            path: ['prizes', index, 'templateId'],
            message: t('Select an enabled speed pass template'),
          })
        }
      })
      if (totalProbability > 100) {
        context.addIssue({
          code: 'custom',
          path: ['prizes'],
          message: t('Enabled prize probabilities cannot exceed 100%'),
        })
      }
      if (
        values.lotteryEnabled &&
        (values.lotteryMinRechargeAmount <= 0 ||
          values.lotteryDrawsPerRecharge < 1 ||
          !values.prizes.some(
            (prize) => prize.enabled && prize.probabilityPercent > 0
          ))
      ) {
        context.addIssue({
          code: 'custom',
          path: ['lotteryEnabled'],
          message: t(
            'Enabled lottery requires a recharge threshold, draw count, and at least one prize.'
          ),
        })
      }
    })

export type RechargeRewardFormValues = z.infer<
  ReturnType<typeof createRechargeRewardFormSchema>
>

export function rechargeRewardSettingsToForm(
  settings: RechargeRewardSettings,
  currencyConfig: CurrencyConfig
): RechargeRewardFormValues {
  return {
    groupPassEnabled: settings.group_pass_enabled,
    lotteryEnabled: settings.lottery_enabled,
    lotteryMinRechargeAmount: quotaToCurrencyAmount(
      settings.lottery_min_recharge_quota,
      currencyConfig
    ),
    lotteryDrawsPerRecharge: settings.lottery_draws_per_recharge,
    templates: settings.group_pass_templates.map((template) => ({
      id: template.id,
      name: template.name,
      groupName: template.group_name,
      durationMinutes: template.duration_minutes,
      validDays: template.valid_days,
      enabled: template.enabled,
    })),
    rules: settings.recharge_reward_rules.map((rule) => ({
      id: rule.id,
      name: rule.name,
      minRechargeAmount: quotaToCurrencyAmount(
        rule.min_recharge_quota,
        currencyConfig
      ),
      templateId: rule.template_id,
      quantity: rule.quantity,
      enabled: rule.enabled,
    })),
    prizes: settings.lottery_prizes.map((prize) => ({
      id: prize.id,
      name: prize.name,
      type: prize.type,
      probabilityPercent: prize.probability_bps / 100,
      minAmount: quotaToCurrencyAmount(prize.min_quota, currencyConfig),
      maxAmount: quotaToCurrencyAmount(prize.max_quota, currencyConfig),
      templateId: prize.template_id,
      quantity: prize.quantity,
      enabled: prize.enabled,
    })),
  }
}

export function rechargeRewardFormToSettings(
  values: RechargeRewardFormValues,
  current: RechargeRewardSettings,
  currencyConfig: CurrencyConfig
): RechargeRewardSettings {
  return {
    ...current,
    group_pass_enabled: values.groupPassEnabled,
    lottery_enabled: values.lotteryEnabled,
    lottery_min_recharge_quota: Math.round(
      currencyAmountToQuota(values.lotteryMinRechargeAmount, currencyConfig)
    ),
    lottery_draws_per_recharge: values.lotteryDrawsPerRecharge,
    group_pass_templates: values.templates.map((template) => ({
      id: template.id,
      name: template.name.trim(),
      group_name: template.groupName.trim(),
      duration_minutes: template.durationMinutes,
      valid_days: template.validDays,
      enabled: template.enabled,
    })),
    recharge_reward_rules: values.rules.map((rule) => ({
      id: rule.id,
      name: rule.name.trim(),
      min_recharge_quota: Math.round(
        currencyAmountToQuota(rule.minRechargeAmount, currencyConfig)
      ),
      template_id: rule.templateId,
      quantity: rule.quantity,
      enabled: rule.enabled,
    })),
    lottery_prizes: values.prizes.map((prize) => ({
      id: prize.id,
      name: prize.name.trim(),
      type: prize.type,
      probability_bps: Math.round(prize.probabilityPercent * 100),
      min_quota:
        prize.type === 'quota'
          ? Math.round(currencyAmountToQuota(prize.minAmount, currencyConfig))
          : 0,
      max_quota:
        prize.type === 'quota'
          ? Math.round(currencyAmountToQuota(prize.maxAmount, currencyConfig))
          : 0,
      template_id: prize.type === 'group_pass' ? prize.templateId : '',
      quantity: prize.type === 'group_pass' ? prize.quantity : 0,
      enabled: prize.enabled,
    })),
  }
}
