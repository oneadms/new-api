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

import { Badge } from '@/components/ui/badge'

export function ResultBadge(props: { value: string }) {
  const { t } = useTranslation()
  const labels: Record<string, string> = {
    passed: t('Rules passed'),
    rules_passed: t('Rules passed'),
    review: t('Needs review'),
    needs_review: t('Needs review'),
    suspected_degradation: t('Suspected quality degradation'),
    not_evaluated: t('Not evaluated'),
    partial: t('Some probe requests failed'),
    failed: t('Failed'),
    unknown: t('Insufficient data'),
    stale: t('Stale sample'),
    paused: t('Detection paused'),
    queued: t('Queued'),
    pending: t('Pending'),
    running: t('Running'),
    completed: t('Completed'),
    cancelled: t('Cancelled'),
    interrupted: t('Interrupted'),
    skipped: t('Skipped'),
    accepted: t('Accepted'),
    rejected: t('Rejected'),
    budget_blocked: t('Budget limit reached'),
  }
  const destructive = ['failed', 'suspected_degradation', 'rejected'].includes(
    props.value
  )
  return (
    <Badge
      variant={destructive ? 'destructive' : 'secondary'}
      className='max-w-full whitespace-normal'
    >
      {labels[props.value] ?? props.value}
    </Badge>
  )
}
