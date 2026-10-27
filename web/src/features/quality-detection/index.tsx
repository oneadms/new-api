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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getBatches, getChannels, getPrompts } from './api'
import { HistoryPanel } from './components/HistoryPanel'
import { PromptsPanel } from './components/PromptsPanel'
import { QueryFeedback } from './components/QueryFeedback'
import { RunBatch } from './components/RunBatch'
import { SettingsPanel } from './components/SettingsPanel'
import { isBatchActive } from './lib/batches'

export function QualityDetection() {
  const { t } = useTranslation()
  const [tab, setTab] = useState('channels')
  const [selectedBatch, setSelectedBatch] = useState<number | null>(null)
  const channels = useQuery({
    queryKey: ['quality-detection', 'channels'],
    queryFn: ({ signal }) => getChannels(signal),
    retry: false,
  })
  const prompts = useQuery({
    queryKey: ['quality-detection', 'prompts'],
    queryFn: ({ signal }) => getPrompts(signal),
    retry: false,
  })
  const latest = useQuery({
    queryKey: ['quality-detection', 'batches', 1],
    queryFn: ({ signal }) => getBatches(1, signal),
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.items.some(isBatchActive) ? 2000 : 30000,
    refetchIntervalInBackground: false,
  })
  const active = latest.data?.items.find(isBatchActive)
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Quality detection')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Description>
        {t(
          'Inspect representative channel outputs without changing production routing or charging user wallets.'
        )}
      </SectionPageLayout.Description>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          render={<Link to='/status' />}
          nativeButton={false}
        >
          {t('Service Status')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-[1440px] min-w-0 flex-col gap-6'>
          <QueryFeedback
            pending={
              channels.isPending || prompts.isPending || latest.isPending
            }
            error={channels.error || prompts.error || latest.error}
            retry={() => {
              void channels.refetch()
              void prompts.refetch()
              void latest.refetch()
            }}
          />
          {active && (
            <div className='flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4 text-sm'>
              <p>
                {t(
                  'A batch is already running. View or stop it before starting another.'
                )}
              </p>
              <Button
                variant='outline'
                onClick={() => {
                  setSelectedBatch(active.id)
                  setTab('history')
                }}
              >
                {t('View batch')} #{active.id}
              </Button>
            </div>
          )}
          <Tabs value={tab} onValueChange={setTab} className='min-w-0 gap-6'>
            <div className='min-w-0 overflow-x-auto border-b pt-1'>
              <TabsList
                variant='line'
                className='w-max min-w-full justify-start gap-5 p-0 group-data-horizontal/tabs:h-11 sm:gap-6'
              >
                <TabsTrigger
                  value='channels'
                  className='h-11 flex-none px-1 group-data-horizontal/tabs:after:bottom-0'
                >
                  {t('Channels')}
                </TabsTrigger>
                <TabsTrigger
                  value='history'
                  className='h-11 flex-none px-1 group-data-horizontal/tabs:after:bottom-0'
                >
                  {t('Run history')}
                </TabsTrigger>
                <TabsTrigger
                  value='prompts'
                  className='h-11 flex-none px-1 group-data-horizontal/tabs:after:bottom-0'
                >
                  {t('Prompt versions')}
                </TabsTrigger>
                <TabsTrigger
                  value='settings'
                  className='h-11 flex-none px-1 group-data-horizontal/tabs:after:bottom-0'
                >
                  {t('Settings')}
                </TabsTrigger>
              </TabsList>
            </div>
            <TabsContent value='channels' className='min-w-0'>
              {channels.data &&
                prompts.data &&
                !channels.isError &&
                !prompts.isError && (
                  <RunBatch
                    channels={channels.data}
                    prompts={prompts.data}
                    blocked={Boolean(active) || !latest.isSuccess}
                    onCreated={(id) => {
                      setSelectedBatch(id)
                      setTab('history')
                    }}
                  />
                )}
            </TabsContent>
            <TabsContent value='history' className='min-w-0'>
              <HistoryPanel
                selected={selectedBatch}
                onSelect={setSelectedBatch}
              />
            </TabsContent>
            <TabsContent value='prompts' className='min-w-0'>
              {prompts.data && !prompts.isError && (
                <PromptsPanel prompts={prompts.data} />
              )}
            </TabsContent>
            <TabsContent value='settings' className='min-w-0'>
              <SettingsPanel channels={channels.data ?? []} />
            </TabsContent>
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
