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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { QueryFeedback } from '@/features/quality-detection/components/QueryFeedback'
import { ResultBadge } from '@/features/quality-detection/components/ResultBadge'
import { SavedTime } from '@/features/quality-detection/components/SavedTime'
import { isAccessError } from '@/features/quality-detection/lib/request'
import { statusPageOptions } from '@/features/status/api'

import { PanelWrapper } from '../ui/panel-wrapper'

export function UptimePanel() {
  const { t } = useTranslation()
  const query = useQuery(statusPageOptions())
  const data = isAccessError(query.error) ? undefined : query.data
  return (
    <PanelWrapper
      title={t('Service Status')}
      description={t('Native group status and saved quality samples')}
      loading={query.isPending}
      empty={false}
      height='h-80'
      contentClassName='p-0'
      headerActions={
        <Button
          variant='ghost'
          size='sm'
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          {t('Refresh')}
        </Button>
      }
    >
      <ScrollArea className='h-80'>
        <div className='space-y-4 p-4'>
          <QueryFeedback
            error={query.error}
            empty={data?.enabled && !data.groups.length}
          />
          {data && !data.enabled && (
            <p>{t('The public status page is not enabled.')}</p>
          )}
          {data?.enabled && (
            <>
              <div className='text-muted-foreground text-xs'>
                {t('Unique detected channels')}: {data.summary.channels} ·{' '}
                {t('Metrics data until')}:{' '}
                <SavedTime value={data.metrics_data_until} />
              </div>
              {data.paused && <ResultBadge value='paused' />}
              {data.groups.map((group) => (
                <div
                  key={group.public_id}
                  className='flex min-w-0 items-center justify-between gap-3 border-b py-3'
                >
                  <span className='truncate' title={group.name}>
                    {group.name}
                  </span>
                  <ResultBadge value={data.paused ? 'paused' : group.state} />
                </div>
              ))}
              <Button
                variant='outline'
                nativeButton={false}
                render={<Link to='/status' />}
              >
                {t('View details')}
              </Button>
            </>
          )}
        </div>
      </ScrollArea>
    </PanelWrapper>
  )
}
