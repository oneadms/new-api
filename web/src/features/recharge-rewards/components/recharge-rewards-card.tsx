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
import dayjs from 'dayjs'
import { Clock3, Gift, Sparkles, TicketCheck } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
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
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { formatQuota } from '@/lib/format'

import {
  useActivateGroupPass,
  useDrawRechargeLottery,
  useRechargeRewardSelf,
} from '../hooks'
import { getGroupPassDisplayStatus } from '../lib/group-pass-status'
import type { RechargeLotteryPrize, UserGroupPass } from '../types'

function formatPrize(
  prize: RechargeLotteryPrize,
  t: (key: string, options?: Record<string, unknown>) => string
) {
  if (prize.type === 'quota') {
    if (prize.min_quota === prize.max_quota) {
      return formatQuota(prize.min_quota)
    }
    return `${formatQuota(prize.min_quota)} – ${formatQuota(prize.max_quota)}`
  }
  return t('{{count}} speed pass(es)', { count: prize.quantity })
}

function PassRow(props: {
  pass: UserGroupPass
  enabled: boolean
  onActivate: (pass: UserGroupPass) => void
}) {
  const { t } = useTranslation()
  const status = getGroupPassDisplayStatus(
    props.pass,
    Math.floor(Date.now() / 1000)
  )
  let statusLabel = t('Unused')
  let statusVariant: 'default' | 'secondary' | 'outline' = 'secondary'
  let timestamp = props.pass.expires_at
  let timeLabel = t('Activate before {{time}}', {
    time: dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm'),
  })
  if (status === 'active') {
    statusLabel = t('Active')
    statusVariant = 'default'
    timestamp = props.pass.active_until
    timeLabel = t('Active until {{time}}', {
      time: dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm'),
    })
  } else if (status === 'expired') {
    statusLabel = t('Expired')
    statusVariant = 'outline'
    timeLabel = t('Expired at {{time}}', {
      time: dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm'),
    })
  }

  return (
    <div className='flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='min-w-0 space-y-1'>
        <div className='flex flex-wrap items-center gap-2'>
          <span className='font-medium'>{props.pass.name}</span>
          <Badge variant={statusVariant}>{statusLabel}</Badge>
          <Badge variant='outline'>{props.pass.group_name}</Badge>
        </div>
        <p className='text-muted-foreground flex items-center gap-1.5 text-xs'>
          <Clock3 className='size-3.5' aria-hidden='true' />
          {timeLabel} ·{' '}
          {t('{{minutes}} minutes of access', {
            minutes: props.pass.duration_minutes,
          })}
        </p>
      </div>
      {status === 'unused' && (
        <Button
          type='button'
          size='sm'
          disabled={!props.enabled}
          onClick={() => props.onActivate(props.pass)}
        >
          {t('Activate')}
        </Button>
      )}
    </div>
  )
}

export function RechargeRewardsCard(props: {
  onQuotaAwarded?: () => Promise<void> | void
}) {
  const { t } = useTranslation()
  const rewardsQuery = useRechargeRewardSelf()
  const activateMutation = useActivateGroupPass()
  const drawMutation = useDrawRechargeLottery()
  const [selectedPass, setSelectedPass] = useState<UserGroupPass | null>(null)

  if (rewardsQuery.isLoading) {
    return (
      <Card data-card-hover='false'>
        <CardHeader>
          <Skeleton className='h-5 w-40' />
          <Skeleton className='h-4 w-72 max-w-full' />
        </CardHeader>
        <CardContent className='grid gap-4 lg:grid-cols-2'>
          <Skeleton className='h-32 w-full' />
          <Skeleton className='h-32 w-full' />
        </CardContent>
      </Card>
    )
  }

  const rewards = rewardsQuery.data
  if (
    !rewards ||
    (!rewards.group_pass_enabled &&
      !rewards.lottery_enabled &&
      rewards.group_passes.length === 0 &&
      rewards.available_draws === 0)
  ) {
    return null
  }

  async function confirmActivation() {
    if (!selectedPass) return
    await activateMutation.mutateAsync(selectedPass.id)
    toast.success(t('Speed pass activated'))
    setSelectedPass(null)
  }

  async function handleDraw() {
    const result = await drawMutation.mutateAsync()
    if (result.draw.prize_type === 'quota') {
      await props.onQuotaAwarded?.()
      toast.success(
        t('You won {{quota}}', {
          quota: formatQuota(result.draw.quota_awarded),
        })
      )
      return
    }
    if (result.draw.prize_type === 'group_pass') {
      toast.success(
        t('You won {{count}} speed pass(es)', {
          count: result.draw.group_pass_count,
        })
      )
      return
    }
    toast(t('Better luck next time'))
  }

  return (
    <>
      <Card data-card-hover='false'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2'>
            <Gift className='size-5' aria-hidden='true' />
            {t('Recharge rewards')}
          </CardTitle>
          <CardDescription>
            {t(
              'Activate a speed pass for temporary access to a discounted group, or use draw chances earned from recharges.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-5 lg:grid-cols-2'>
          <section aria-labelledby='speed-pass-heading' className='space-y-3'>
            <div>
              <h3
                id='speed-pass-heading'
                className='flex items-center gap-2 font-medium'
              >
                <TicketCheck className='size-4' aria-hidden='true' />
                {t('Speed passes')}
              </h3>
              <p className='text-muted-foreground mt-1 text-sm'>
                {rewards.group_pass_enabled
                  ? t('Activation starts the access timer immediately.')
                  : t('Speed pass activation is currently paused.')}
              </p>
            </div>
            {rewards.group_passes.length === 0 ? (
              <Empty className='min-h-32 border'>
                <EmptyHeader>
                  <EmptyMedia variant='icon'>
                    <TicketCheck aria-hidden='true' />
                  </EmptyMedia>
                  <EmptyTitle>{t('No speed passes yet')}</EmptyTitle>
                  <EmptyDescription>
                    {t('Eligible recharges and promotions will appear here.')}
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className='max-h-80 space-y-2 overflow-y-auto pr-1'>
                {rewards.group_passes.map((pass) => (
                  <PassRow
                    key={pass.id}
                    pass={pass}
                    enabled={rewards.group_pass_enabled}
                    onActivate={setSelectedPass}
                  />
                ))}
              </div>
            )}
          </section>

          <section aria-labelledby='lottery-heading' className='space-y-3'>
            <div className='flex items-start justify-between gap-3'>
              <div>
                <h3
                  id='lottery-heading'
                  className='flex items-center gap-2 font-medium'
                >
                  <Sparkles className='size-4' aria-hidden='true' />
                  {t('Recharge draw')}
                </h3>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t('{{count}} draw chance(s) available', {
                    count: rewards.available_draws,
                  })}
                </p>
              </div>
              <Button
                type='button'
                onClick={handleDraw}
                disabled={
                  !rewards.lottery_enabled ||
                  rewards.available_draws < 1 ||
                  drawMutation.isPending
                }
              >
                {drawMutation.isPending && <Spinner data-icon='inline-start' />}
                {t('Draw now')}
              </Button>
            </div>
            <Separator />
            <div className='space-y-2'>
              <p className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
                {t('Available prizes')}
              </p>
              {rewards.lottery_prizes.map((prize) => (
                <div
                  key={prize.id}
                  className='bg-muted/50 flex items-center justify-between gap-3 rounded-md px-3 py-2 text-sm'
                >
                  <div className='min-w-0'>
                    <p className='truncate font-medium'>{prize.name}</p>
                    <p className='text-muted-foreground text-xs'>
                      {formatPrize(prize, t)}
                    </p>
                  </div>
                  <Badge variant='outline'>
                    {(prize.probability_bps / 100).toFixed(2)}%
                  </Badge>
                </div>
              ))}
              {rewards.lottery_prizes.length === 0 && (
                <p className='text-muted-foreground py-6 text-center text-sm'>
                  {t('No prizes are currently available.')}
                </p>
              )}
            </div>
          </section>
        </CardContent>
      </Card>

      <AlertDialog
        open={selectedPass !== null}
        onOpenChange={(open) => {
          if (!open && !activateMutation.isPending) setSelectedPass(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Activate speed pass?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'The timer starts immediately and cannot be paused. Existing API keys for the target group will work only until the timer expires.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={activateMutation.isPending}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmActivation}
              disabled={activateMutation.isPending}
            >
              {activateMutation.isPending && (
                <Spinner data-icon='inline-start' />
              )}
              {t('Activate')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
