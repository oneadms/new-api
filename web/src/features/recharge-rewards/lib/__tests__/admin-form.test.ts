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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { CurrencyConfig } from '@/stores/system-config-store'

import type { RechargeRewardSettings } from '../../types'
import {
  createRechargeRewardFormSchema,
  rechargeRewardFormToSettings,
  rechargeRewardSettingsToForm,
  type RechargeRewardFormValues,
} from '../admin-form'

const translate = (key: string) => key
const currencyConfig: CurrencyConfig = {
  displayInCurrency: true,
  quotaDisplayType: 'USD',
  quotaPerUnit: 500_000,
  usdExchangeRate: 1,
  customCurrencySymbol: '¤',
  customCurrencyExchangeRate: 1,
}

const validValues: RechargeRewardFormValues = {
  groupPassEnabled: true,
  lotteryEnabled: true,
  lotteryMinRechargeAmount: 10,
  lotteryDrawsPerRecharge: 1,
  templates: [
    {
      id: 'speed-hour',
      name: 'Speed hour',
      groupName: 'speed',
      durationMinutes: 60,
      validDays: 30,
      enabled: true,
    },
  ],
  rules: [
    {
      id: 'recharge-speed',
      name: 'Recharge speed reward',
      minRechargeAmount: 10,
      templateId: 'speed-hour',
      quantity: 1,
      enabled: true,
    },
  ],
  prizes: [
    {
      id: 'quota-prize',
      name: 'Quota prize',
      type: 'quota',
      probabilityPercent: 60,
      minAmount: 1,
      maxAmount: 2,
      templateId: '',
      quantity: 0,
      enabled: true,
    },
    {
      id: 'pass-prize',
      name: 'Pass prize',
      type: 'group_pass',
      probabilityPercent: 40,
      minAmount: 0,
      maxAmount: 0,
      templateId: 'speed-hour',
      quantity: 1,
      enabled: true,
    },
  ],
}

describe('recharge reward admin form', () => {
  test('rejects enabled prize probability totals above 100 percent', () => {
    const values = structuredClone(validValues)
    values.prizes[1].probabilityPercent = 40.01

    const result = createRechargeRewardFormSchema(translate).safeParse(values)

    assert.equal(result.success, false)
    if (!result.success) {
      assert.ok(
        result.error.issues.some(
          (issue) =>
            issue.path.join('.') === 'prizes' &&
            issue.message === 'Enabled prize probabilities cannot exceed 100%'
        )
      )
    }
  })

  test('rejects inverted quota prize ranges and zero-probability enabled prizes', () => {
    const values = structuredClone(validValues)
    values.prizes[0].minAmount = 3
    values.prizes[0].maxAmount = 2
    values.prizes[0].probabilityPercent = 0

    const result = createRechargeRewardFormSchema(translate).safeParse(values)

    assert.equal(result.success, false)
    if (!result.success) {
      const paths = new Set(
        result.error.issues.map((issue) => issue.path.join('.'))
      )
      assert.ok(paths.has('prizes.0.maxAmount'))
      assert.ok(paths.has('prizes.0.probabilityPercent'))
    }
  })

  test('rejects enabled rules and prizes that reference a disabled template', () => {
    const values = structuredClone(validValues)
    values.templates[0].enabled = false

    const result = createRechargeRewardFormSchema(translate).safeParse(values)

    assert.equal(result.success, false)
    if (!result.success) {
      const paths = new Set(
        result.error.issues
          .filter(
            (issue) => issue.message === 'Select an enabled speed pass template'
          )
          .map((issue) => issue.path.join('.'))
      )
      assert.deepEqual(
        paths,
        new Set(['rules.0.templateId', 'prizes.1.templateId'])
      )
    }
  })

  test('maps currency amounts, basis points, templates, and card quantities without loss', () => {
    const current: RechargeRewardSettings = {
      version: 7,
      group_pass_enabled: false,
      lottery_enabled: false,
      lottery_min_recharge_quota: 0,
      lottery_draws_per_recharge: 0,
      group_pass_templates: [],
      recharge_reward_rules: [],
      lottery_prizes: [],
      updated_at: 123,
    }

    const settings = rechargeRewardFormToSettings(
      validValues,
      current,
      currencyConfig
    )
    const restored = rechargeRewardSettingsToForm(settings, currencyConfig)

    assert.equal(settings.version, 7)
    assert.equal(settings.updated_at, 123)
    assert.equal(settings.lottery_min_recharge_quota, 5_000_000)
    assert.equal(settings.lottery_prizes[0].probability_bps, 6_000)
    assert.equal(settings.lottery_prizes[0].min_quota, 500_000)
    assert.equal(settings.lottery_prizes[0].max_quota, 1_000_000)
    assert.equal(settings.lottery_prizes[1].template_id, 'speed-hour')
    assert.equal(settings.lottery_prizes[1].quantity, 1)
    assert.deepEqual(restored, validValues)
  })
})
