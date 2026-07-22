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
import { TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'

interface Props {
  groups?: string[]
  className?: string
}

export function RestrictedGroupsNotice(props: Props) {
  const { t } = useTranslation()

  if (!props.groups?.length) return null

  return (
    <Alert
      className={cn(
        'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-100',
        props.className
      )}
    >
      <TriangleAlert aria-hidden='true' />
      <AlertDescription className='text-current'>
        <span className='font-medium'>{t('Restricted Groups')}:</span>{' '}
        {props.groups.join(', ')}
      </AlertDescription>
    </Alert>
  )
}
