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
import { api } from '@/lib/api'

import type {
  ApiEnvelope,
  GroupPassGrantRequest,
  RechargeLotteryDrawResult,
  RechargeRewardSelf,
  RechargeRewardSettings,
  UserGroupPass,
} from './types'

export async function getRechargeRewardSelf(): Promise<RechargeRewardSelf> {
  const response = await api.get<ApiEnvelope<RechargeRewardSelf>>(
    '/api/recharge-reward/self'
  )
  return response.data.data
}

export async function activateGroupPass(
  passId: number
): Promise<UserGroupPass> {
  const response = await api.post<ApiEnvelope<UserGroupPass>>(
    `/api/recharge-reward/group-passes/${passId}/activate`
  )
  return response.data.data
}

export async function drawRechargeLottery(): Promise<RechargeLotteryDrawResult> {
  const response = await api.post<ApiEnvelope<RechargeLotteryDrawResult>>(
    '/api/recharge-reward/lottery/draw'
  )
  return response.data.data
}

export async function getRechargeRewardSettings(): Promise<RechargeRewardSettings> {
  const response = await api.get<ApiEnvelope<RechargeRewardSettings>>(
    '/api/recharge-reward/admin/settings'
  )
  return response.data.data
}

export async function saveRechargeRewardSettings(
  settings: RechargeRewardSettings
): Promise<RechargeRewardSettings> {
  const response = await api.put<ApiEnvelope<RechargeRewardSettings>>(
    '/api/recharge-reward/admin/settings',
    settings
  )
  return response.data.data
}

export async function grantGroupPasses(
  request: GroupPassGrantRequest
): Promise<UserGroupPass[]> {
  const response = await api.post<ApiEnvelope<UserGroupPass[]>>(
    '/api/recharge-reward/admin/group-passes/grant',
    request
  )
  return response.data.data
}
