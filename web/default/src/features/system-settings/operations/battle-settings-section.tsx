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
import { useMemo, useRef } from 'react'
import { z } from 'zod'
import { useForm, type Resolver } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const createSchema = () =>
  z.object({
    enabled: z.boolean(),
    hideRoomInput: z.boolean(),
    matchModeEnabled: z.boolean(),
    allowNegativeBalance: z.boolean(),
    capQuota: z.coerce.number().int().min(0),
    maxRoundLossQuota: z.coerce.number().int().min(0),
    maxRoundGainQuota: z.coerce.number().int().min(0),
    maxDailyLossQuota: z.coerce.number().int().min(0),
    maxDailyGainQuota: z.coerce.number().int().min(0),
    maxPlayersPerRoom: z.coerce.number().int().min(2).max(32),
    tickRate: z.coerce.number().int().min(10).max(60),
    playerSpeed: z.coerce.number().int().min(80).max(900),
    bulletSpeed: z.coerce.number().int().min(100).max(1800),
    fireCooldownMs: z.coerce.number().int().min(80).max(2000),
    matchEntryQuota: z.coerce.number().int().min(0),
    matchMinPlayers: z.coerce.number().int().min(2).max(32),
    matchDurationSeconds: z.coerce.number().int().min(30).max(86400),
    matchStartAt: z.coerce.number().int().min(0),
  })

type Values = z.infer<ReturnType<typeof createSchema>>
type NumericFieldName = Exclude<
  keyof Values,
  | 'enabled'
  | 'hideRoomInput'
  | 'matchModeEnabled'
  | 'allowNegativeBalance'
  | 'matchStartAt'
>

type BattleSettingsSectionProps = {
  defaultValues: Values
}

const optionKeys: Record<keyof Values, string> = {
  enabled: 'battle_setting.enabled',
  hideRoomInput: 'battle_setting.hide_room_input',
  matchModeEnabled: 'battle_setting.match_mode_enabled',
  allowNegativeBalance: 'battle_setting.allow_negative_balance',
  capQuota: 'battle_setting.cap_quota',
  maxRoundLossQuota: 'battle_setting.max_round_loss_quota',
  maxRoundGainQuota: 'battle_setting.max_round_gain_quota',
  maxDailyLossQuota: 'battle_setting.max_daily_loss_quota',
  maxDailyGainQuota: 'battle_setting.max_daily_gain_quota',
  maxPlayersPerRoom: 'battle_setting.max_players_per_room',
  tickRate: 'battle_setting.tick_rate',
  playerSpeed: 'battle_setting.player_speed',
  bulletSpeed: 'battle_setting.bullet_speed',
  fireCooldownMs: 'battle_setting.fire_cooldown_ms',
  matchEntryQuota: 'battle_setting.match_entry_quota',
  matchMinPlayers: 'battle_setting.match_min_players',
  matchDurationSeconds: 'battle_setting.match_duration_seconds',
  matchStartAt: 'battle_setting.match_start_at',
}

export function BattleSettingsSection(props: BattleSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const quotaPerUnit = useSystemConfigStore(
    (state) =>
      state.config.currency?.quotaPerUnit ||
      DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  )
  const savedValuesRef = useRef<Values>(props.defaultValues)
  const schema = useMemo(() => createSchema(), [])

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: props.defaultValues,
  })
  const startMatchMutation = useMutation({
    mutationFn: async () => {
      const res = await api.post<{
        success: boolean
        message: string
        data?: { started_rooms?: number }
      }>('/api/battle/match/start')
      return res.data
    },
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to start match'))
        return
      }
      toast.success(
        t('Match start requested for {{count}} room(s)', {
          count: data.data?.started_rooms ?? 0,
        })
      )
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to start match'))
    },
  })

  const enabled = form.watch('enabled')
  const matchModeEnabled = form.watch('matchModeEnabled')
  const { isDirty, isSubmitting } = form.formState

  async function onSubmit(values: Values) {
    const savedValues = savedValuesRef.current
    const updates = (Object.keys(optionKeys) as Array<keyof Values>)
      .filter((key) => values[key] !== savedValues[key])
      .map((key) => ({
        key: optionKeys[key],
        value: String(values[key]),
      }))

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }

    savedValuesRef.current = values
    form.reset(values)
    toast.success(t('Forgive Cap Battle settings saved'))
  }

  const numericFields: Array<{
    name: NumericFieldName
    label: string
    description: string
    min: number
    max?: number
    unit?: 'usd'
    matchOnly?: boolean
  }> = [
    {
      name: 'capQuota',
      label: t('Quota per cap'),
      description: t('Quota transferred for each green cap on a player.'),
      min: 0,
      unit: 'usd',
    },
    {
      name: 'maxRoundLossQuota',
      label: t('Round loss cap'),
      description: t('Maximum quota one player can lose in one room session.'),
      min: 0,
      unit: 'usd',
    },
    {
      name: 'maxRoundGainQuota',
      label: t('Round win cap'),
      description: t('Maximum quota one player can gain in one room session.'),
      min: 0,
      unit: 'usd',
    },
    {
      name: 'maxDailyLossQuota',
      label: t('Daily loss cap'),
      description: t('Maximum quota one player can lose per day.'),
      min: 0,
      unit: 'usd',
    },
    {
      name: 'maxDailyGainQuota',
      label: t('Daily win cap'),
      description: t('Maximum quota one player can gain per day.'),
      min: 0,
      unit: 'usd',
    },
    {
      name: 'maxPlayersPerRoom',
      label: t('Room size'),
      description: t('Maximum number of connected players in one room.'),
      min: 2,
      max: 32,
    },
    {
      name: 'tickRate',
      label: t('Tick rate'),
      description: t('Server simulation snapshots per second.'),
      min: 10,
      max: 60,
    },
    {
      name: 'playerSpeed',
      label: t('Player speed'),
      description: t('Movement speed in map units per second.'),
      min: 80,
      max: 900,
    },
    {
      name: 'bulletSpeed',
      label: t('Flying cap speed'),
      description: t('Horizontal speed of a thrown green cap.'),
      min: 100,
      max: 1800,
    },
    {
      name: 'fireCooldownMs',
      label: t('Throw cooldown'),
      description: t('Minimum milliseconds between two cap throws.'),
      min: 80,
      max: 2000,
    },
    {
      name: 'matchEntryQuota',
      label: t('Match entry deposit'),
      description: t(
        'Each player must have this balance to enter a match; in-match losses are capped to this same amount.'
      ),
      min: 0,
      unit: 'usd',
      matchOnly: true,
    },
    {
      name: 'matchMinPlayers',
      label: t('Match start players'),
      description: t(
        'A scheduled match starts when this many players are in the room.'
      ),
      min: 2,
      max: 32,
      matchOnly: true,
    },
    {
      name: 'matchDurationSeconds',
      label: t('Match duration'),
      description: t('Seconds before a match ends and settles all caps.'),
      min: 30,
      max: 86400,
      matchOnly: true,
    },
  ]

  return (
    <SettingsSection title={t('Forgive Cap Battle')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save Forgive Cap Battle settings'
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Forgive Cap Battle')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow authenticated users to join cap battle rooms and settle real quota rewards.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={updateOption.isPending || isSubmitting}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          {enabled && (
            <>
              <FormField
                control={form.control}
                name='hideRoomInput'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Hide room input')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Send all players to the default lobby and hide the room selector.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='matchModeEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable match mode')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Players wait for a scheduled or manually started match; normal match end settles all caps.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='allowNegativeBalance'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Allow negative battle balance')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'When disabled, cap hits require enough balance coverage and invalid hits do not stack.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending || isSubmitting}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </>
          )}

          {enabled && (
            <div className='grid gap-6 sm:grid-cols-2 xl:grid-cols-3'>
              {numericFields
                .filter((item) => !item.matchOnly || matchModeEnabled)
                .map((item) => (
                  <FormField
                    key={item.name}
                    control={form.control}
                    name={item.name}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {item.label}
                          {item.unit === 'usd' ? ' (USD)' : ''}
                        </FormLabel>
                        <FormControl>
                          {item.unit === 'usd' ? (
                            <Input
                              type='number'
                              min={quotaToUsd(item.min, quotaPerUnit)}
                              max={
                                item.max === undefined
                                  ? undefined
                                  : quotaToUsd(item.max, quotaPerUnit)
                              }
                              step='0.0001'
                              name={field.name}
                              ref={field.ref}
                              value={quotaToUsd(field.value, quotaPerUnit)}
                              onBlur={field.onBlur}
                              onChange={(event) =>
                                field.onChange(
                                  usdToQuota(event.target.value, quotaPerUnit)
                                )
                              }
                            />
                          ) : (
                            <Input
                              type='number'
                              min={item.min}
                              max={item.max}
                              {...field}
                            />
                          )}
                        </FormControl>
                        <FormDescription>{item.description}</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ))}
            </div>
          )}

          {enabled && matchModeEnabled && (
            <div className='grid gap-6 sm:grid-cols-2 xl:grid-cols-3'>
              <FormField
                control={form.control}
                name='matchStartAt'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Scheduled match start')}</FormLabel>
                    <FormControl>
                      <Input
                        type='datetime-local'
                        name={field.name}
                        ref={field.ref}
                        value={unixSecondsToLocalInput(field.value)}
                        onBlur={field.onBlur}
                        onChange={(event) =>
                          field.onChange(
                            localInputToUnixSeconds(event.target.value)
                          )
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Leave empty to allow the match to start as soon as the player requirement is met.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <div className='flex flex-col justify-end gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => startMatchMutation.mutate()}
                  disabled={startMatchMutation.isPending}
                >
                  {t('Start match now')}
                </Button>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Starts every waiting battle room immediately, even if the scheduled time or player count has not been reached.'
                  )}
                </p>
              </div>
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}

function quotaToUsd(quota: number, quotaPerUnit: number): number {
  return Number((quota / safeQuotaPerUnit(quotaPerUnit)).toFixed(6))
}

function usdToQuota(value: string, quotaPerUnit: number): number {
  if (value.trim() === '') return 0
  const amount = Number(value)
  if (!Number.isFinite(amount) || amount <= 0) return 0
  return Math.round(amount * safeQuotaPerUnit(quotaPerUnit))
}

function safeQuotaPerUnit(quotaPerUnit: number): number {
  return quotaPerUnit > 0 ? quotaPerUnit : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
}

function unixSecondsToLocalInput(value: number): string {
  if (!value) return ''
  const date = new Date(value * 1000)
  if (!Number.isFinite(date.getTime())) return ''
  const localDate = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return localDate.toISOString().slice(0, 16)
}

function localInputToUnixSeconds(value: string): number {
  if (!value) return 0
  const timestamp = new Date(value).getTime()
  if (!Number.isFinite(timestamp)) return 0
  return Math.floor(timestamp / 1000)
}
