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
import { TextAlignLeftIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import type { TFunction } from 'i18next'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const playlistExample = JSON.stringify(
  [
    {
      title: 'Song title',
      artist: 'Artist',
      url: 'https://example.com/song.mp3',
      cover: 'https://example.com/cover.jpg',
      lyrics: '[00:00.00] First lyric line\\n[00:12.50] Second lyric line',
    },
  ],
  null,
  2
)

const _musicPlayerSchema = z.object({
  MusicPlayerEnabled: z.boolean(),
  MusicPlayerAutoplay: z.boolean(),
  MusicPlayerShowLyrics: z.boolean(),
  MusicPlayerPlaylist: z.string(),
})

export type MusicPlayerFormValues = z.infer<typeof _musicPlayerSchema>

type MusicPlayerSectionProps = {
  defaultValues: MusicPlayerFormValues
}

function normalizeValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : String(value)
}

function normalizeBool(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') return value === 'true' || value === '1'
  return Boolean(value)
}

function isHttpUrl(value: string): boolean {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function getPlaylistValidationMessage(
  value: string,
  t: TFunction
): string | null {
  const trimmed = value.trim()
  if (!trimmed) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return t('Music player playlist must be a valid JSON array')
  }

  if (!Array.isArray(parsed)) {
    return t('Music player playlist must be a valid JSON array')
  }

  if (parsed.length > 50) {
    return t('Music player supports up to 50 tracks')
  }

  for (const item of parsed) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      return t('Each track needs an audio URL')
    }

    const track = item as Record<string, unknown>
    if (typeof track.url !== 'string' || !track.url.trim()) {
      return t('Each track needs an audio URL')
    }

    if (!isHttpUrl(track.url.trim())) {
      return t('Track URLs must start with http:// or https://')
    }

    if (
      track.cover !== undefined &&
      track.cover !== null &&
      (typeof track.cover !== 'string' ||
        (track.cover.trim() && !isHttpUrl(track.cover.trim())))
    ) {
      return t('Track URLs must start with http:// or https://')
    }

    for (const key of ['title', 'artist', 'lyrics']) {
      const field = track[key]
      if (field !== undefined && field !== null && typeof field !== 'string') {
        return t('Track titles, artists, covers, and lyrics must be strings')
      }
    }
  }

  return null
}

function normalizePlaylistText(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return '[]'

  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return value
  }
}

export function MusicPlayerSection({ defaultValues }: MusicPlayerSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const normalizedDefaults: MusicPlayerFormValues = {
    MusicPlayerEnabled: normalizeBool(defaultValues.MusicPlayerEnabled),
    MusicPlayerAutoplay: normalizeBool(defaultValues.MusicPlayerAutoplay),
    MusicPlayerShowLyrics: normalizeBool(defaultValues.MusicPlayerShowLyrics),
    MusicPlayerPlaylist: normalizePlaylistText(
      normalizeValue(defaultValues.MusicPlayerPlaylist)
    ),
  }

  const musicPlayerSchema = z.object({
    MusicPlayerEnabled: z.boolean(),
    MusicPlayerAutoplay: z.boolean(),
    MusicPlayerShowLyrics: z.boolean(),
    MusicPlayerPlaylist: z.string().superRefine((value, context) => {
      const message = getPlaylistValidationMessage(value, t)
      if (!message) return
      context.addIssue({
        code: 'custom',
        message,
      })
    }),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<MusicPlayerFormValues>({
      resolver: zodResolver(musicPlayerSchema) as Resolver<
        MusicPlayerFormValues,
        unknown,
        MusicPlayerFormValues
      >,
      defaultValues: normalizedDefaults,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          const nextValue =
            key === 'MusicPlayerPlaylist'
              ? normalizePlaylistText(normalizeValue(value))
              : value

          await updateOption.mutateAsync({
            key,
            value:
              typeof nextValue === 'boolean'
                ? nextValue
                : normalizeValue(nextValue),
          })
        }
      },
    })

  const formatPlaylist = () => {
    const currentValue = form.getValues('MusicPlayerPlaylist')
    const message = getPlaylistValidationMessage(currentValue, t)
    if (message) {
      toast.error(message)
      return
    }

    form.setValue('MusicPlayerPlaylist', normalizePlaylistText(currentValue), {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  return (
    <SettingsSection title={t('Music Player')}>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            onReset={handleReset}
            isSaving={isSubmitting || updateOption.isPending}
            isResetDisabled={!isDirty}
            saveLabel='Save music player'
          />

          <FormField
            control={form.control}
            name='MusicPlayerEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable music player')}</FormLabel>
                  <FormDescription>
                    {t('Show the floating player in the lower-left corner.')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='MusicPlayerAutoplay'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Autoplay after first interaction')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Browsers may block autoplay until the user interacts with the page.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='MusicPlayerShowLyrics'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Show lyrics')}</FormLabel>
                  <FormDescription>
                    {t('Display synced LRC lyrics when a track provides them.')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='MusicPlayerPlaylist'
            render={({ field }) => (
              <FormItem>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <div className='flex min-w-0 flex-col gap-1'>
                    <FormLabel>{t('Playlist JSON')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Paste a JSON array of tracks. Each track supports title, artist, url, cover, and lyrics.'
                      )}
                    </FormDescription>
                  </div>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={formatPlaylist}
                  >
                    <HugeiconsIcon
                      data-icon='inline-start'
                      icon={TextAlignLeftIcon}
                      strokeWidth={2}
                    />
                    <span>{t('Format JSON')}</span>
                  </Button>
                </div>
                <FormControl>
                  <Textarea
                    rows={12}
                    placeholder={playlistExample}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
