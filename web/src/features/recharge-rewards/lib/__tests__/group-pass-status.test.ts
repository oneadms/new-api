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

import type { UserGroupPass } from '../../types'
import { getGroupPassDisplayStatus } from '../group-pass-status'

const basePass: UserGroupPass = {
  id: 1,
  user_id: 10,
  template_id: 'speed-hour',
  name: 'Speed hour',
  group_name: 'speed',
  duration_minutes: 60,
  status: 'unused',
  expires_at: 1_000,
  activated_at: 0,
  active_until: 0,
  source_type: 'test',
  created_at: 100,
}

describe('speed pass display status', () => {
  test('expires an unused pass exactly at its activation deadline', () => {
    assert.equal(getGroupPassDisplayStatus(basePass, 999), 'unused')
    assert.equal(getGroupPassDisplayStatus(basePass, 1_000), 'expired')
  })

  test('expires an active pass exactly when temporary access ends', () => {
    const activePass: UserGroupPass = {
      ...basePass,
      status: 'active',
      activated_at: 500,
      active_until: 1_000,
    }

    assert.equal(getGroupPassDisplayStatus(activePass, 999), 'active')
    assert.equal(getGroupPassDisplayStatus(activePass, 1_000), 'expired')
  })
})
