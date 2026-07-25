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

import {
  formValuesToPlanPayload,
  planToFormValues,
} from '@/features/subscriptions/lib/plan-form'
import type { SubscriptionPlan } from '@/features/subscriptions/types'

const plan: SubscriptionPlan = {
  id: 1,
  title: 'Restricted plan',
  subtitle: '',
  price_amount: 10,
  currency: 'USD',
  duration_unit: 'month',
  duration_value: 1,
  custom_seconds: 0,
  quota_reset_period: 'never',
  quota_reset_custom_seconds: 0,
  enabled: true,
  sort_order: 0,
  allow_balance_pay: true,
  allow_wallet_overflow: true,
  max_purchase_per_user: 0,
  total_amount: 1_000_000,
  upgrade_group: '',
  downgrade_group: '',
  restricted_groups: ['auto', 'vip'],
  subscription_disabled_groups: ['default'],
  stripe_price_id: '',
  creem_product_id: '',
  waffo_pancake_product_id: '',
}

describe('subscription plan restricted groups', () => {
  test('preserves restricted groups through edit and submit conversion', () => {
    const values = planToFormValues(plan)

    const payload = formValuesToPlanPayload(values)

    assert.deepEqual(values.restricted_groups, ['auto', 'vip'])
    assert.deepEqual(payload.plan.restricted_groups, ['auto', 'vip'])
    assert.deepEqual(values.subscription_disabled_groups, ['default'])
    assert.deepEqual(payload.plan.subscription_disabled_groups, ['default'])
  })

  test('defaults legacy plan responses to no restricted groups', () => {
    const {
      restricted_groups: _restrictedGroups,
      subscription_disabled_groups: _subscriptionDisabledGroups,
      ...legacyPlan
    } = plan

    const values = planToFormValues(legacyPlan as SubscriptionPlan)

    assert.deepEqual(values.restricted_groups, [])
    assert.deepEqual(values.subscription_disabled_groups, [])
  })
})
