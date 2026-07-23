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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  activateGroupPass,
  drawRechargeLottery,
  getRechargeRewardSelf,
  getRechargeRewardSettings,
  grantGroupPasses,
  saveRechargeRewardSettings,
} from './api'

export const rechargeRewardKeys = {
  self: ['recharge-rewards', 'self'] as const,
  settings: ['recharge-rewards', 'settings'] as const,
}

export function useRechargeRewardSelf() {
  return useQuery({
    queryKey: rechargeRewardKeys.self,
    queryFn: getRechargeRewardSelf,
  })
}

export function useActivateGroupPass() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: activateGroupPass,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: rechargeRewardKeys.self })
      void queryClient.invalidateQueries({ queryKey: ['user-groups'] })
      void queryClient.invalidateQueries({ queryKey: ['playground-groups'] })
    },
  })
}

export function useDrawRechargeLottery() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: drawRechargeLottery,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: rechargeRewardKeys.self })
      void queryClient.invalidateQueries({ queryKey: ['status'] })
    },
  })
}

export function useRechargeRewardSettings() {
  return useQuery({
    queryKey: rechargeRewardKeys.settings,
    queryFn: getRechargeRewardSettings,
  })
}

export function useSaveRechargeRewardSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: saveRechargeRewardSettings,
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: rechargeRewardKeys.settings,
      })
      void queryClient.invalidateQueries({ queryKey: rechargeRewardKeys.self })
    },
  })
}

export function useGrantGroupPasses() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: grantGroupPasses,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: rechargeRewardKeys.self })
    },
  })
}
