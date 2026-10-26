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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { getOutput } from '../api'
import type { QualityRun } from '../types'
import { QueryFeedback } from './QueryFeedback'

export function RunOutput(props: { run: QualityRun }) {
  const { t } = useTranslation()
  const [kind, setKind] = useState<'raw' | 'html' | null>(null)
  const query = useQuery({
    queryKey: ['quality-detection', 'output', props.run.id, kind],
    queryFn: ({ signal }) => getOutput(props.run.id, kind ?? 'raw', signal),
    enabled: kind !== null,
    retry: false,
    gcTime: 0,
  })
  return (
    <section className='space-y-3'>
      <div className='flex flex-wrap gap-2'>
        <Button
          variant='outline'
          disabled={!props.run.has_output}
          aria-pressed={kind === 'raw'}
          onClick={() => setKind('raw')}
        >
          {t('View raw text')}
        </Button>
        <Button
          variant='outline'
          disabled={!props.run.has_html}
          aria-pressed={kind === 'html'}
          onClick={() => setKind('html')}
        >
          {t('View HTML as text')}
        </Button>
      </div>
      <QueryFeedback
        pending={kind !== null && query.isPending}
        error={query.error}
        retry={() => void query.refetch()}
      />
      {query.data !== undefined && !query.isError && (
        <pre
          tabIndex={0}
          aria-label={t('Saved output as plain text')}
          className='bg-muted max-h-96 overflow-auto rounded-xl p-4 font-mono text-xs break-words whitespace-pre-wrap'
        >
          {query.data}
        </pre>
      )}
    </section>
  )
}
