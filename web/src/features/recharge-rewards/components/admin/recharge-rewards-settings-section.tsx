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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { getGroups } from '@/features/users/api'
import type { CurrencyConfig } from '@/stores/system-config-store'

import { useRechargeRewardSettings } from '../../hooks'
import { RechargeRewardConfigForm } from './recharge-reward-config-form'

export function RechargeRewardsSettingsSection(props: {
  currencyConfig: CurrencyConfig
}) {
  const { t } = useTranslation()
  const settingsQuery = useRechargeRewardSettings()
  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })

  if (settingsQuery.isLoading || groupsQuery.isLoading) {
    return (
      <div className='space-y-4'>
        <Skeleton className='h-52 w-full' />
        <Skeleton className='h-52 w-full' />
      </div>
    )
  }

  if (!settingsQuery.data || !groupsQuery.data?.data) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load recharge reward settings')}</AlertTitle>
        <AlertDescription>
          {t('Refresh the page and try again.')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <RechargeRewardConfigForm
      key={settingsQuery.data.version}
      settings={settingsQuery.data}
      groups={groupsQuery.data.data}
      currencyConfig={props.currencyConfig}
    />
  )
}
