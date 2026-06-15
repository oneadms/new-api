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
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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

const createSchema = (t: TFunction) =>
  z
    .object({
      enabled: z.boolean(),
      hideRoomInput: z.boolean(),
      minDropQuota: z.coerce.number().int().min(0),
      maxDropQuota: z.coerce.number().int().min(0),
      maxRoundLossQuota: z.coerce.number().int().min(0),
      maxRoundGainQuota: z.coerce.number().int().min(0),
      maxDailyLossQuota: z.coerce.number().int().min(0),
      maxDailyGainQuota: z.coerce.number().int().min(0),
      maxPlayersPerRoom: z.coerce.number().int().min(2).max(32),
      tickRate: z.coerce.number().int().min(10).max(60),
      playerSpeed: z.coerce.number().int().min(80).max(900),
      bulletSpeed: z.coerce.number().int().min(100).max(1800),
      bulletDamage: z.coerce.number().int().min(1).max(100),
      fireCooldownMs: z.coerce.number().int().min(80).max(2000),
      respawnSeconds: z.coerce.number().int().min(1).max(30),
      dropExpireSeconds: z.coerce.number().int().min(3).max(120),
    })
    .refine((values) => values.maxDropQuota >= values.minDropQuota, {
      path: ['maxDropQuota'],
      message: t('Maximum drop quota must be greater than minimum drop quota'),
    })

type Values = z.infer<ReturnType<typeof createSchema>>
type NumericFieldName = Exclude<keyof Values, 'enabled' | 'hideRoomInput'>

type BattleSettingsSectionProps = {
  defaultValues: Values
}

const optionKeys: Record<keyof Values, string> = {
  enabled: 'battle_setting.enabled',
  hideRoomInput: 'battle_setting.hide_room_input',
  minDropQuota: 'battle_setting.min_drop_quota',
  maxDropQuota: 'battle_setting.max_drop_quota',
  maxRoundLossQuota: 'battle_setting.max_round_loss_quota',
  maxRoundGainQuota: 'battle_setting.max_round_gain_quota',
  maxDailyLossQuota: 'battle_setting.max_daily_loss_quota',
  maxDailyGainQuota: 'battle_setting.max_daily_gain_quota',
  maxPlayersPerRoom: 'battle_setting.max_players_per_room',
  tickRate: 'battle_setting.tick_rate',
  playerSpeed: 'battle_setting.player_speed',
  bulletSpeed: 'battle_setting.bullet_speed',
  bulletDamage: 'battle_setting.bullet_damage',
  fireCooldownMs: 'battle_setting.fire_cooldown_ms',
  respawnSeconds: 'battle_setting.respawn_seconds',
  dropExpireSeconds: 'battle_setting.drop_expire_seconds',
}

export function BattleSettingsSection(props: BattleSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const savedValuesRef = useRef<Values>(props.defaultValues)
  const schema = useMemo(() => createSchema(t), [t])

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: props.defaultValues,
  })

  const enabled = form.watch('enabled')
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
    toast.success(t('Battle settings saved'))
  }

  const numericFields: Array<{
    name: NumericFieldName
    label: string
    description: string
    min: number
    max?: number
  }> = [
    {
      name: 'minDropQuota',
      label: t('Minimum drop quota'),
      description: t('Smallest quota amount created when a player is hit.'),
      min: 0,
    },
    {
      name: 'maxDropQuota',
      label: t('Maximum drop quota'),
      description: t('Largest quota amount created when a player is hit.'),
      min: 0,
    },
    {
      name: 'maxRoundLossQuota',
      label: t('Round loss cap'),
      description: t('Maximum quota one player can lose in one room session.'),
      min: 0,
    },
    {
      name: 'maxRoundGainQuota',
      label: t('Round win cap'),
      description: t('Maximum quota one player can gain in one room session.'),
      min: 0,
    },
    {
      name: 'maxDailyLossQuota',
      label: t('Daily loss cap'),
      description: t('Maximum quota one player can lose per day.'),
      min: 0,
    },
    {
      name: 'maxDailyGainQuota',
      label: t('Daily win cap'),
      description: t('Maximum quota one player can gain per day.'),
      min: 0,
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
      label: t('Bullet speed'),
      description: t('Bullet movement speed in map units per second.'),
      min: 100,
      max: 1800,
    },
    {
      name: 'bulletDamage',
      label: t('Bullet damage'),
      description: t('Health removed by each server-confirmed hit.'),
      min: 1,
      max: 100,
    },
    {
      name: 'fireCooldownMs',
      label: t('Fire cooldown'),
      description: t('Minimum milliseconds between two shots.'),
      min: 80,
      max: 2000,
    },
    {
      name: 'respawnSeconds',
      label: t('Respawn seconds'),
      description: t('Delay before a knocked-out player returns.'),
      min: 1,
      max: 30,
    },
    {
      name: 'dropExpireSeconds',
      label: t('Drop lifetime'),
      description: t('Seconds before an unclaimed quota drop disappears.'),
      min: 3,
      max: 120,
    },
  ]

  return (
    <SettingsSection title={t('Battle Arena')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending || isSubmitting}
            isSaveDisabled={!isDirty}
            saveLabel='Save battle settings'
          />

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable battle arena')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow authenticated users to join WebSocket rooms and settle real quota pickups.'
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
          )}

          {enabled && (
            <div className='grid gap-6 sm:grid-cols-2 xl:grid-cols-3'>
              {numericFields.map((item) => (
                <FormField
                  key={item.name}
                  control={form.control}
                  name={item.name}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{item.label}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          min={item.min}
                          max={item.max}
                          {...field}
                        />
                      </FormControl>
                      <FormDescription>{item.description}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ))}
            </div>
          )}
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
