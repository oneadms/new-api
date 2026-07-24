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

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

export function SpeedPassTutorial() {
  const { t } = useTranslation()

  return (
    <Alert>
      <AlertTitle>{t('How to use a speed pass')}</AlertTitle>
      <AlertDescription>
        <ol className='mt-1 flex list-decimal flex-col gap-1 pl-4 text-xs'>
          <li>
            {t(
              'Earn a speed pass from an eligible recharge, promotion, or recharge draw.'
            )}
          </li>
          <li>
            {t(
              'Activate it before the activation deadline. The timer starts immediately and cannot be paused.'
            )}
          </li>
          <li>
            {t(
              "While it is active, requests automatically use the group shown on the pass. When access expires, they automatically return to the API key's original group."
            )}
          </li>
          <li>{t('Only one speed pass can be active at a time.')}</li>
        </ol>
      </AlertDescription>
    </Alert>
  )
}
