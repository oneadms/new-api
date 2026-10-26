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
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import type { z } from 'zod'

import { Button } from '@/components/ui/button'

import { reviewRun, setBaseline } from '../api'
import { useQualityMutation } from '../hooks/use-quality-mutation'
import { reviewSchema } from '../lib/schemas'
import type { QualityRun } from '../types'
import { QualityField } from './QualityField'
import { QueryFeedback } from './QueryFeedback'
import { ResultBadge } from './ResultBadge'

export function RunReview(props: { run: QualityRun }) {
  const { t } = useTranslation()
  const form = useForm<z.infer<typeof reviewSchema>>({
    resolver: zodResolver(reviewSchema),
    defaultValues: {
      result: props.run.review_status === 'rejected' ? 'rejected' : 'accepted',
      note: props.run.review_note,
    },
  })
  const review = useQualityMutation((values: z.infer<typeof reviewSchema>) =>
    reviewRun(props.run.id, values)
  )
  const baseline = useQualityMutation(() => setBaseline(props.run.id))
  const canReview = props.run.status === 'completed'
  return (
    <section className='space-y-4 border-t pt-5'>
      <h3 className='font-medium'>{t('Human review')}</h3>
      {props.run.review_status && (
        <ResultBadge value={props.run.review_status} />
      )}
      <p className='text-muted-foreground text-sm'>{props.run.review_note}</p>
      <form
        onSubmit={form.handleSubmit((values) => review.mutate(values))}
        className='space-y-4'
      >
        <QualityField
          form={form}
          name='result'
          label={t('Review result')}
          disabled={!canReview}
          options={[
            { value: 'accepted', label: t('Accepted') },
            { value: 'rejected', label: t('Rejected') },
          ]}
        />
        <QualityField
          form={form}
          name='note'
          label={t('Review note')}
          type='textarea'
          disabled={!canReview}
        />
        <QueryFeedback error={review.error} />
        <Button disabled={!canReview || review.isPending} type='submit'>
          {t('Save review')}
        </Button>
      </form>
      <QueryFeedback error={baseline.error} />
      <Button
        variant='outline'
        disabled={
          !canReview ||
          props.run.review_status !== 'accepted' ||
          baseline.isPending
        }
        onClick={() => baseline.mutate()}
      >
        {t('Set as baseline')}
      </Button>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Accept the completed sample before setting a baseline. Original results remain unchanged.'
        )}
      </p>
    </section>
  )
}
