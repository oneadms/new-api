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

import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getBatches, getChannels, getPrompts } from './api'
import { ChannelsPanel } from './components/ChannelsPanel'
import { HistoryPanel } from './components/HistoryPanel'
import { PromptsPanel } from './components/PromptsPanel'
import { QueryFeedback } from './components/QueryFeedback'
import { RunBatch } from './components/RunBatch'
import { SettingsPanel } from './components/SettingsPanel'
import { isBatchActive } from './lib/batches'

export function QualityDetection() {
  const { t } = useTranslation()
  const [tab, setTab] = useState('history')
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
    <main className='mx-auto w-full max-w-[1440px] min-w-0 space-y-7 p-4 sm:p-8'>
      <header className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
        <div className='space-y-2'>
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('Quality detection')}
          </h1>
          <p className='text-muted-foreground max-w-3xl text-sm'>
            {t(
              'Inspect representative channel outputs without changing production routing or charging user wallets.'
            )}
          </p>
        </div>
        <Button
          variant='outline'
          render={<Link to='/status' />}
          nativeButton={false}
        >
          {t('Service Status')}
        </Button>
      </header>
      <QueryFeedback
        pending={channels.isPending || prompts.isPending || latest.isPending}
        error={channels.error || prompts.error || latest.error}
        retry={() => {
          void channels.refetch()
          void prompts.refetch()
          void latest.refetch()
        }}
      />
      {active && (
        <div className='flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4'>
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
      {channels.data && !channels.isError && (
        <RunBatch
          channels={channels.data}
          blocked={Boolean(active) || !latest.isSuccess}
          onCreated={(id) => {
            setSelectedBatch(id)
            setTab('history')
          }}
        />
      )}
      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className='flex h-auto max-w-full flex-wrap justify-start'>
          <TabsTrigger value='history'>{t('Run history')}</TabsTrigger>
          <TabsTrigger value='channels'>{t('Channels')}</TabsTrigger>
          <TabsTrigger value='prompts'>{t('Prompt versions')}</TabsTrigger>
          <TabsTrigger value='settings'>{t('Settings')}</TabsTrigger>
        </TabsList>
        <TabsContent value='history' className='pt-5'>
          <HistoryPanel selected={selectedBatch} onSelect={setSelectedBatch} />
        </TabsContent>
        <TabsContent value='channels' className='pt-5'>
          {channels.data &&
            prompts.data &&
            !channels.isError &&
            !prompts.isError && (
              <ChannelsPanel channels={channels.data} prompts={prompts.data} />
            )}
        </TabsContent>
        <TabsContent value='prompts' className='pt-5'>
          {prompts.data && !prompts.isError && (
            <PromptsPanel prompts={prompts.data} />
          )}
        </TabsContent>
        <TabsContent value='settings' className='pt-5'>
          <SettingsPanel channels={channels.data ?? []} />
        </TabsContent>
      </Tabs>
    </main>
  )
}
