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
export type LotteryPrizeType = 'quota' | 'group_pass'

export interface GroupPassTemplate {
  id: string
  name: string
  group_name: string
  duration_minutes: number
  valid_days: number
  enabled: boolean
}

export interface RechargeRewardRule {
  id: string
  name: string
  min_recharge_quota: number
  template_id: string
  quantity: number
  enabled: boolean
}

export interface RechargeLotteryPrize {
  id: string
  name: string
  type: LotteryPrizeType
  probability_bps: number
  min_quota: number
  max_quota: number
  template_id: string
  quantity: number
  enabled: boolean
}

export interface RechargeRewardSettings {
  version: number
  group_pass_enabled: boolean
  lottery_enabled: boolean
  lottery_min_recharge_quota: number
  lottery_draws_per_recharge: number
  group_pass_templates: GroupPassTemplate[]
  recharge_reward_rules: RechargeRewardRule[]
  lottery_prizes: RechargeLotteryPrize[]
  updated_at: number
}

export type UserGroupPassStatus = 'unused' | 'active' | 'expired'

export interface UserGroupPass {
  id: number
  user_id: number
  template_id: string
  name: string
  group_name: string
  duration_minutes: number
  status: UserGroupPassStatus
  expires_at: number
  activated_at: number
  active_until: number
  source_type: string
  created_at: number
}

export interface RechargeLotteryDraw {
  id: number
  user_id: number
  reward_event_id: number
  draw_index: number
  prize_id: string
  prize_name: string
  prize_type: LotteryPrizeType | 'none'
  probability_bps: number
  quota_awarded: number
  group_pass_id: number
  group_pass_count: number
  config_version: number
  created_at: number
}

export interface RechargeRewardSelf {
  group_pass_enabled: boolean
  lottery_enabled: boolean
  available_draws: number
  group_passes: UserGroupPass[]
  recent_draws: RechargeLotteryDraw[]
  lottery_prizes: RechargeLotteryPrize[]
}

export interface RechargeLotteryDrawResult {
  draw: RechargeLotteryDraw
  group_pass?: UserGroupPass
}

export interface GroupPassGrantRequest {
  user_id: number
  template_id: string
  quantity: number
  expires_at: number
}

export interface ApiEnvelope<T> {
  success: boolean
  message: string
  data: T
}
