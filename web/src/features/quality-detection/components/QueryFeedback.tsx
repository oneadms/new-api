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
import { Button } from '@/components/ui/button'
import { Empty, EmptyHeader, EmptyTitle } from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'

import { QualityRequestError, isAccessError } from '../lib/request'

export function QueryFeedback(props: {
  pending?: boolean
  error?: unknown
  empty?: boolean
  retry?: () => void
}) {
  const { t } = useTranslation()
  if (props.pending) {
    return (
      <div
        role='status'
        aria-label={t('Loading...')}
        className='space-y-3 py-6'
      >
        <Skeleton className='h-16 w-full' />
        <Skeleton className='h-16 w-full' />
      </div>
    )
  }
  if (props.error) {
    let title = t('Unable to load saved data')
    if (isAccessError(props.error)) {
      title = t('This content is unavailable or access was revoked')
    }
    if (
      props.error instanceof QualityRequestError &&
      props.error.status === 409
    ) {
      title = t(
        'Configuration changed or a batch is already running. Reload before trying again.'
      )
    }
    return (
      <Alert variant='destructive'>
        <AlertTitle>{title}</AlertTitle>
        <AlertDescription>
          {props.error instanceof QualityRequestError &&
          props.error.status === 400 ? (
            <span>{props.error.message}</span>
          ) : (
            <span>{t('Please refresh and try again.')}</span>
          )}
          {props.retry && (
            <Button variant='outline' onClick={props.retry}>
              {t('Refresh')}
            </Button>
          )}
        </AlertDescription>
      </Alert>
    )
  }
  if (props.empty) {
    return (
      <Empty className='min-h-40 border border-dashed'>
        <EmptyHeader>
          <EmptyTitle>{t('No saved data for this selection')}</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  }
  return null
}
