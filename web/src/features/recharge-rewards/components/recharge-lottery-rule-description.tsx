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
import { useTranslation } from 'react-i18next'

import { formatQuota } from '@/lib/format'

export function RechargeLotteryRuleDescription(props: {
  enabled: boolean
  minRechargeQuota: number
  drawsPerRecharge: number
}) {
  const { t } = useTranslation()

  return (
    <p className='text-muted-foreground mt-1 text-sm'>
      {props.enabled
        ? t(
            'Each single recharge of {{amount}} or more grants {{count}} draw chance(s).',
            {
              amount: formatQuota(props.minRechargeQuota),
              count: props.drawsPerRecharge,
            }
          )
        : t(
            'Recharge draw is currently paused. Existing draw chances remain stored.'
          )}
    </p>
  )
}
