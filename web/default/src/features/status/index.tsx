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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  CircleCheck,
  RefreshCw,
  Server,
  WifiOff,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { getUptimeStatus } from './api'
import type { UptimeGroupResult, UptimeMonitor } from './types'

const ALL_MONITORS_VALUE = 'all'

type MonitorStatusMeta = {
  labelKey: string
  variant: StatusVariant
  dotClassName: string
}

const MONITOR_STATUS_META: Record<number, MonitorStatusMeta> = {
  0: {
    labelKey: 'Outage',
    variant: 'danger',
    dotClassName: 'bg-destructive',
  },
  1: {
    labelKey: 'Operational',
    variant: 'success',
    dotClassName: 'bg-success',
  },
  2: {
    labelKey: 'Pending',
    variant: 'warning',
    dotClassName: 'bg-warning',
  },
  3: {
    labelKey: 'Maintenance',
    variant: 'info',
    dotClassName: 'bg-info',
  },
}

const UNKNOWN_STATUS_META: MonitorStatusMeta = {
  labelKey: 'Unknown',
  variant: 'neutral',
  dotClassName: 'bg-muted-foreground/40',
}

function getMonitorStatusMeta(status: number): MonitorStatusMeta {
  return MONITOR_STATUS_META[status] ?? UNKNOWN_STATUS_META
}

function formatUptime(uptime: number | undefined) {
  if (typeof uptime !== 'number' || Number.isNaN(uptime)) return '--'
  return `${(uptime * 100).toFixed(2)}%`
}

function getAverageUptime(monitors: UptimeMonitor[]) {
  if (!monitors.length) return '--'
  const total = monitors.reduce((sum, monitor) => {
    return sum + (typeof monitor.uptime === 'number' ? monitor.uptime : 0)
  }, 0)
  return formatUptime(total / monitors.length)
}

function getMonitorGroups(groups: UptimeGroupResult[]) {
  return groups.filter((group) => group.monitors?.length)
}

function getLastUpdatedLabel(timestamp: number, locale?: string) {
  if (!timestamp) return '--'
  return new Intl.DateTimeFormat(locale, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(timestamp))
}

function SummaryCard(props: {
  title: string
  value: string
  description: string
  icon: React.ReactNode
}) {
  return (
    <Card size='sm'>
      <CardHeader className='grid-cols-[1fr_auto] gap-3'>
        <div className='flex min-w-0 flex-col gap-1'>
          <CardDescription>{props.title}</CardDescription>
          <CardTitle className='truncate text-2xl'>{props.value}</CardTitle>
        </div>
        <div className='bg-muted text-muted-foreground flex size-9 items-center justify-center rounded-lg'>
          {props.icon}
        </div>
      </CardHeader>
      <CardContent>
        <p className='text-muted-foreground text-xs'>{props.description}</p>
      </CardContent>
    </Card>
  )
}

function StatusPageSkeleton() {
  return (
    <PublicLayout showMainContainer={false}>
      <main className='mx-auto flex w-full max-w-6xl flex-col gap-6 px-3 pt-24 pb-10 sm:px-6 lg:px-8'>
        <div className='flex flex-col gap-3'>
          <Skeleton className='h-8 w-48' />
          <Skeleton className='h-4 w-full max-w-md' />
        </div>
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
          {Array.from({ length: 4 }, (_, index) => (
            <Skeleton key={index} className='h-28 w-full rounded-xl' />
          ))}
        </div>
        <Skeleton className='h-[360px] w-full rounded-xl' />
      </main>
    </PublicLayout>
  )
}

function StatusEmptyState(props: {
  title: string
  description: string
  onRefresh?: () => void
  refreshing?: boolean
}) {
  const { t } = useTranslation()

  return (
    <Empty className='min-h-[360px] border'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>
          <WifiOff />
        </EmptyMedia>
        <EmptyTitle>{props.title}</EmptyTitle>
        <EmptyDescription>{props.description}</EmptyDescription>
      </EmptyHeader>
      {props.onRefresh && (
        <EmptyContent>
          <Button
            variant='outline'
            size='sm'
            onClick={props.onRefresh}
            disabled={props.refreshing}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn(props.refreshing && 'animate-spin')}
            />
            {t('Refresh')}
          </Button>
        </EmptyContent>
      )}
    </Empty>
  )
}

function MonitorRow(props: { monitor: UptimeMonitor }) {
  const { t } = useTranslation()
  const meta = getMonitorStatusMeta(props.monitor.status)

  return (
    <div className='hover:bg-muted/40 grid min-h-16 grid-cols-1 gap-3 px-4 py-3 transition-colors md:grid-cols-[minmax(0,1fr)_12rem_9rem] md:items-center'>
      <div className='flex min-w-0 items-center gap-3'>
        <span
          className={cn('size-2.5 shrink-0 rounded-full', meta.dotClassName)}
          aria-hidden='true'
        />
        <div className='flex min-w-0 flex-col gap-1'>
          <span className='truncate font-medium'>{props.monitor.name}</span>
          {props.monitor.group && (
            <span className='text-muted-foreground truncate text-xs'>
              {props.monitor.group}
            </span>
          )}
        </div>
      </div>
      <div className='flex items-center justify-between gap-3 md:justify-start'>
        <span className='text-muted-foreground text-xs md:hidden'>
          {t('Current status')}
        </span>
        <StatusBadge
          label={t(meta.labelKey)}
          variant={meta.variant}
          copyable={false}
        />
      </div>
      <div className='flex items-center justify-between gap-3 md:justify-end'>
        <span className='text-muted-foreground text-xs md:hidden'>
          {t('24h uptime')}
        </span>
        <span className='font-mono text-sm font-semibold tabular-nums'>
          {formatUptime(props.monitor.uptime)}
        </span>
      </div>
    </div>
  )
}

export function Status() {
  const { t, i18n } = useTranslation()
  const [activeGroup, setActiveGroup] = useState(ALL_MONITORS_VALUE)

  const statusQuery = useQuery({
    queryKey: ['uptime-status'],
    queryFn: getUptimeStatus,
    refetchOnWindowFocus: false,
  })

  const groups = useMemo(
    () => getMonitorGroups(statusQuery.data?.data ?? []),
    [statusQuery.data?.data]
  )

  const tabGroups = useMemo(
    () =>
      groups.map((group, index) => ({
        value: `group-${index}`,
        group,
      })),
    [groups]
  )

  const monitors = useMemo(
    () => groups.flatMap((group) => group.monitors ?? []),
    [groups]
  )

  const selectedMonitors = useMemo(() => {
    if (activeGroup === ALL_MONITORS_VALUE) return monitors
    return (
      tabGroups.find((group) => group.value === activeGroup)?.group.monitors ??
      []
    )
  }, [activeGroup, monitors, tabGroups])

  useEffect(() => {
    if (
      activeGroup !== ALL_MONITORS_VALUE &&
      !tabGroups.some((group) => group.value === activeGroup)
    ) {
      setActiveGroup(ALL_MONITORS_VALUE)
    }
  }, [activeGroup, tabGroups])

  const summary = useMemo(() => {
    const total = monitors.length
    const operational = monitors.filter(
      (monitor) => monitor.status === 1
    ).length
    const affected = monitors.filter((monitor) => monitor.status !== 1).length
    const outage = monitors.filter((monitor) => monitor.status === 0).length
    const pending = monitors.filter((monitor) => monitor.status === 2).length
    const maintenance = monitors.filter(
      (monitor) => monitor.status === 3
    ).length

    let headline = 'All systems operational'
    let headlineStatus: StatusVariant = 'success'
    if (!total) {
      headline = 'Unknown'
      headlineStatus = 'neutral'
    } else if (outage > 0) {
      headline = 'Service disruption'
      headlineStatus = 'danger'
    } else if (pending > 0) {
      headline = 'Pending'
      headlineStatus = 'warning'
    } else if (maintenance > 0) {
      headline = 'Maintenance'
      headlineStatus = 'info'
    }

    return {
      total,
      operational,
      affected,
      headline,
      headlineStatus,
      averageUptime: getAverageUptime(monitors),
    }
  }, [monitors])

  if (statusQuery.isLoading) {
    return <StatusPageSkeleton />
  }

  const lastUpdated = getLastUpdatedLabel(
    statusQuery.dataUpdatedAt,
    i18n.language || undefined
  )

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto flex w-full max-w-6xl flex-col gap-6 px-3 pt-24 pb-10 sm:px-6 lg:px-8'>
        <header className='flex flex-col gap-4 md:flex-row md:items-end md:justify-between'>
          <div className='flex min-w-0 flex-col gap-3'>
            <div className='flex items-center gap-3'>
              <div className='bg-muted text-muted-foreground flex size-10 items-center justify-center rounded-lg'>
                <Activity className='size-5' />
              </div>
              <div className='flex min-w-0 flex-col gap-1'>
                <h1 className='text-2xl font-semibold sm:text-3xl'>
                  {t('Service Status')}
                </h1>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Current availability across configured upstream monitors.'
                  )}
                </p>
              </div>
            </div>
            <div className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-2 text-sm'>
              <span>
                {t('Last updated')}: {lastUpdated}
              </span>
              <StatusBadge
                label={t(summary.headline)}
                variant={summary.headlineStatus}
                copyable={false}
              />
            </div>
          </div>
          <Button
            variant='outline'
            onClick={() => statusQuery.refetch()}
            disabled={statusQuery.isFetching}
            aria-label={t('Refresh status')}
          >
            <RefreshCw
              data-icon='inline-start'
              className={cn(statusQuery.isFetching && 'animate-spin')}
            />
            {t('Refresh')}
          </Button>
        </header>

        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
          <SummaryCard
            title={t('Overall health')}
            value={t(summary.headline)}
            description={t('{{count}} operational monitors', {
              count: summary.operational,
            })}
            icon={<CircleCheck className='size-4' />}
          />
          <SummaryCard
            title={t('Configured monitors')}
            value={String(summary.total)}
            description={t('Across {{count}} monitor groups', {
              count: groups.length,
            })}
            icon={<Server className='size-4' />}
          />
          <SummaryCard
            title={t('Affected monitors')}
            value={String(summary.affected)}
            description={t('Non-operational or maintenance state')}
            icon={<AlertTriangle className='size-4' />}
          />
          <SummaryCard
            title={t('Average uptime')}
            value={summary.averageUptime}
            description={t('Reported 24h uptime average')}
            icon={<Activity className='size-4' />}
          />
        </div>

        {statusQuery.isError ? (
          <StatusEmptyState
            title={t('Unable to load service status')}
            description={t('Please refresh and try again.')}
            onRefresh={() => statusQuery.refetch()}
            refreshing={statusQuery.isFetching}
          />
        ) : !groups.length ? (
          <StatusEmptyState
            title={t('No uptime monitoring configured')}
            description={t(
              'Ask an administrator to configure Uptime Kuma groups in system settings.'
            )}
            onRefresh={() => statusQuery.refetch()}
            refreshing={statusQuery.isFetching}
          />
        ) : (
          <section className='flex flex-col gap-4'>
            <Tabs value={activeGroup} onValueChange={setActiveGroup}>
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                <TabsTrigger value={ALL_MONITORS_VALUE}>
                  {t('All monitors')}
                  <span className='text-muted-foreground ms-1 font-mono text-xs'>
                    {monitors.length}
                  </span>
                </TabsTrigger>
                {tabGroups.map(({ value, group }) => (
                  <TabsTrigger key={value} value={value}>
                    <span className='max-w-40 truncate'>
                      {group.categoryName}
                    </span>
                    <span className='text-muted-foreground ms-1 font-mono text-xs'>
                      {group.monitors.length}
                    </span>
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            <Card>
              <CardHeader className='border-b'>
                <CardTitle>{t('Monitor status')}</CardTitle>
                <CardDescription>
                  {t('Current heartbeat and 24h uptime by monitor')}
                </CardDescription>
              </CardHeader>
              <CardContent className='p-0'>
                <div className='text-muted-foreground hidden grid-cols-[minmax(0,1fr)_12rem_9rem] border-b px-4 py-2 text-xs font-medium md:grid'>
                  <span>{t('Monitor')}</span>
                  <span>{t('Current status')}</span>
                  <span className='text-right'>{t('24h uptime')}</span>
                </div>
                <div className='divide-border divide-y'>
                  {selectedMonitors.map((monitor) => (
                    <MonitorRow
                      key={`${monitor.group ?? 'default'}-${monitor.name}`}
                      monitor={monitor}
                    />
                  ))}
                </div>
              </CardContent>
            </Card>
          </section>
        )}
      </PageTransition>
    </PublicLayout>
  )
}
