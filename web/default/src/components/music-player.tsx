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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowDown01Icon,
  ArrowUp01Icon,
  MusicNote02Icon,
  NextIcon,
  PauseIcon,
  PlayIcon,
  PreviousIcon,
  TextAlignLeftIcon,
  VolumeHighIcon,
  VolumeMute02Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon, type IconSvgElement } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useSystemConfigStore } from '@/stores/system-config-store'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

type LyricLine = {
  time: number
  text: string
}

const LRC_TIME_PATTERN = /\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]/g

function parseLrc(value?: string): LyricLine[] {
  if (!value) return []

  const lines: LyricLine[] = []
  for (const rawLine of value.split(/\r?\n/)) {
    const matches = [...rawLine.matchAll(LRC_TIME_PATTERN)]
    if (matches.length === 0) continue

    const text = rawLine.replace(LRC_TIME_PATTERN, '').trim()
    if (!text) continue

    for (const match of matches) {
      const minutes = Number(match[1] ?? 0)
      const seconds = Number(match[2] ?? 0)
      const fraction = match[3] ?? ''
      const milliseconds = fraction
        ? Number(fraction.padEnd(3, '0').slice(0, 3))
        : 0

      lines.push({
        time: minutes * 60 + seconds + milliseconds / 1000,
        text,
      })
    }
  }

  return lines.sort((a, b) => a.time - b.time)
}

function getActiveLyricIndex(lines: LyricLine[], currentTime: number): number {
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    if (currentTime >= lines[index].time) return index
  }
  return -1
}

function formatTime(value: number): string {
  if (!Number.isFinite(value) || value < 0) return '0:00'
  const totalSeconds = Math.floor(value)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}

type PlayerIconButtonProps = {
  label: string
  icon: IconSvgElement
  onClick: () => void
  disabled?: boolean
  pressed?: boolean
}

function PlayerIconButton({
  label,
  icon,
  onClick,
  disabled,
  pressed,
}: PlayerIconButtonProps) {
  const button = (
    <Button
      type='button'
      variant='ghost'
      size='icon-sm'
      aria-label={label}
      aria-pressed={pressed}
      disabled={disabled}
      onClick={onClick}
    >
      <HugeiconsIcon data-icon='inline-start' icon={icon} strokeWidth={2} />
    </Button>
  )

  return (
    <Tooltip>
      <TooltipTrigger render={button}></TooltipTrigger>
      <TooltipContent side='top'>
        <p>{label}</p>
      </TooltipContent>
    </Tooltip>
  )
}

export function MusicPlayer() {
  const { t } = useTranslation()
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const activeLyricRef = useRef<HTMLParagraphElement | null>(null)
  const pendingPlayRef = useRef(false)
  const lastErrorUrlRef = useRef('')
  const musicPlayer = useSystemConfigStore((state) => state.config.musicPlayer)
  const tracks = musicPlayer?.playlist ?? []
  const enabled = Boolean(musicPlayer?.enabled && tracks.length > 0)
  const [trackIndex, setTrackIndex] = useState(0)
  const [isPlaying, setIsPlaying] = useState(false)
  const [duration, setDuration] = useState(0)
  const [currentTime, setCurrentTime] = useState(0)
  const [volume, setVolume] = useState(80)
  const [muted, setMuted] = useState(false)
  const [expanded, setExpanded] = useState(false)

  const currentTrack = enabled ? tracks[trackIndex] : undefined
  const lyrics = useMemo(
    () => parseLrc(currentTrack?.lyrics),
    [currentTrack?.lyrics]
  )
  const activeLyricIndex = getActiveLyricIndex(lyrics, currentTime)
  const activeLyric = activeLyricIndex >= 0 ? lyrics[activeLyricIndex] : null
  const progress =
    duration > 0 ? Math.min(100, (currentTime / duration) * 100) : 0
  const displayTitle = currentTrack?.title || t('Untitled track')
  const displayArtist = currentTrack?.artist || t('Unknown artist')
  const canSkip = tracks.length > 1

  const playAudio = useCallback(async () => {
    const audio = audioRef.current
    if (!audio) return

    try {
      await audio.play()
      setIsPlaying(true)
    } catch {
      setIsPlaying(false)
      toast.error(t('Audio playback failed'))
    }
  }, [t])

  const pauseAudio = useCallback(() => {
    const audio = audioRef.current
    if (!audio) return
    audio.pause()
    setIsPlaying(false)
  }, [])

  const changeTrack = useCallback(
    (direction: number) => {
      if (!canSkip) return
      pendingPlayRef.current = isPlaying
      setTrackIndex((current) => {
        const next = (current + direction + tracks.length) % tracks.length
        return next
      })
    },
    [canSkip, isPlaying, tracks.length]
  )

  useEffect(() => {
    if (trackIndex >= tracks.length) {
      setTrackIndex(0)
    }
  }, [trackIndex, tracks.length])

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) return

    audio.volume = volume / 100
    audio.muted = muted
  }, [muted, volume])

  useEffect(() => {
    if (!enabled) {
      pauseAudio()
    }
  }, [enabled, pauseAudio])

  useEffect(() => {
    setCurrentTime(0)
    setDuration(0)
    lastErrorUrlRef.current = ''

    if (!pendingPlayRef.current) return
    pendingPlayRef.current = false

    const timer = window.setTimeout(() => {
      void playAudio()
    }, 0)

    return () => window.clearTimeout(timer)
  }, [currentTrack?.url, playAudio])

  useEffect(() => {
    if (!enabled || !musicPlayer?.autoplay || !currentTrack?.url) return

    const start = () => {
      void playAudio()
    }

    window.addEventListener('pointerdown', start, { once: true })
    window.addEventListener('keydown', start, { once: true })

    return () => {
      window.removeEventListener('pointerdown', start)
      window.removeEventListener('keydown', start)
    }
  }, [currentTrack?.url, enabled, musicPlayer?.autoplay, playAudio])

  useEffect(() => {
    if (!expanded || activeLyricIndex < 0) return
    activeLyricRef.current?.scrollIntoView({ block: 'nearest' })
  }, [activeLyricIndex, expanded])

  if (!enabled || !currentTrack) return null

  const togglePlayback = () => {
    if (isPlaying) {
      pauseAudio()
    } else {
      void playAudio()
    }
  }

  const seekToPercent = (nextValue: readonly number[] | number) => {
    const next = Array.isArray(nextValue) ? nextValue[0] : nextValue
    if (!Number.isFinite(next) || duration <= 0) return

    const nextTime = (Math.min(100, Math.max(0, next)) / 100) * duration
    setCurrentTime(nextTime)
    if (audioRef.current) {
      audioRef.current.currentTime = nextTime
    }
  }

  const updateVolume = (nextValue: readonly number[] | number) => {
    const next = Array.isArray(nextValue) ? nextValue[0] : nextValue
    if (!Number.isFinite(next)) return
    setVolume(Math.min(100, Math.max(0, next)))
    if (next > 0 && muted) setMuted(false)
  }

  const handleEnded = () => {
    if (canSkip) {
      pendingPlayRef.current = true
      setTrackIndex((current) => (current + 1) % tracks.length)
      return
    }

    setIsPlaying(false)
    setCurrentTime(0)
  }

  const handleAudioError = () => {
    setIsPlaying(false)
    if (lastErrorUrlRef.current === currentTrack.url) return

    lastErrorUrlRef.current = currentTrack.url
    toast.error(t('Audio playback failed'))
  }

  return (
    <TooltipProvider delay={150}>
      <div className='fixed bottom-4 left-4 z-40 w-[min(calc(100vw-2rem),22rem)]'>
        <audio
          ref={audioRef}
          src={currentTrack.url}
          preload='metadata'
          onLoadedMetadata={(event) => {
            setDuration(event.currentTarget.duration || 0)
          }}
          onTimeUpdate={(event) => {
            setCurrentTime(event.currentTarget.currentTime)
          }}
          onPlay={() => setIsPlaying(true)}
          onPause={() => setIsPlaying(false)}
          onEnded={handleEnded}
          onError={handleAudioError}
        />

        <div className='border-border bg-popover/95 text-popover-foreground ring-foreground/10 supports-[backdrop-filter]:bg-popover/85 flex min-w-0 flex-col gap-3 rounded-lg border p-3 shadow-lg ring-1 backdrop-blur'>
          <div className='flex min-w-0 items-center gap-3'>
            <div className='bg-muted text-muted-foreground flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg'>
              {currentTrack.cover ? (
                <img
                  src={currentTrack.cover}
                  alt={displayTitle}
                  className='size-full object-cover'
                />
              ) : (
                <HugeiconsIcon icon={MusicNote02Icon} strokeWidth={2} />
              )}
            </div>

            <div className='min-w-0 flex-1'>
              <div className='flex min-w-0 items-center gap-2'>
                <p className='truncate text-sm font-medium'>{displayTitle}</p>
                {canSkip ? (
                  <span className='text-muted-foreground shrink-0 text-[0.7rem] tabular-nums'>
                    {trackIndex + 1}/{tracks.length}
                  </span>
                ) : null}
              </div>
              <p className='text-muted-foreground truncate text-xs'>
                {displayArtist}
              </p>
              {!expanded && musicPlayer.showLyrics && activeLyric ? (
                <p className='mt-1 truncate text-xs'>{activeLyric.text}</p>
              ) : null}
            </div>

            <PlayerIconButton
              label={expanded ? t('Collapse player') : t('Expand player')}
              icon={expanded ? ArrowDown01Icon : ArrowUp01Icon}
              onClick={() => setExpanded((value) => !value)}
              pressed={expanded}
            />
          </div>

          <div className='flex items-center gap-2'>
            <span className='text-muted-foreground w-9 shrink-0 text-xs tabular-nums'>
              {formatTime(currentTime)}
            </span>
            <Slider
              aria-label={t('Progress')}
              min={0}
              max={100}
              step={0.1}
              value={[progress]}
              disabled={duration <= 0}
              onValueChange={seekToPercent}
              className='min-w-0 flex-1'
            />
            <span className='text-muted-foreground w-9 shrink-0 text-right text-xs tabular-nums'>
              {formatTime(duration)}
            </span>
          </div>

          <div className='flex min-w-0 items-center gap-2'>
            <PlayerIconButton
              label={t('Previous track')}
              icon={PreviousIcon}
              onClick={() => changeTrack(-1)}
              disabled={!canSkip}
            />
            <Button
              type='button'
              size='icon'
              aria-label={isPlaying ? t('Pause') : t('Play')}
              aria-pressed={isPlaying}
              onClick={togglePlayback}
            >
              <HugeiconsIcon
                data-icon='inline-start'
                icon={isPlaying ? PauseIcon : PlayIcon}
                strokeWidth={2}
              />
            </Button>
            <PlayerIconButton
              label={t('Next track')}
              icon={NextIcon}
              onClick={() => changeTrack(1)}
              disabled={!canSkip}
            />
            <PlayerIconButton
              label={muted || volume === 0 ? t('Unmute') : t('Mute')}
              icon={muted || volume === 0 ? VolumeMute02Icon : VolumeHighIcon}
              onClick={() => setMuted((value) => !value)}
              pressed={muted}
            />
            <Slider
              aria-label={t('Volume')}
              min={0}
              max={100}
              step={1}
              value={[muted ? 0 : volume]}
              onValueChange={updateVolume}
              className='min-w-0 flex-1'
            />
          </div>

          {expanded && musicPlayer.showLyrics ? (
            <div className='bg-muted/30 flex max-h-40 min-h-20 flex-col gap-1 overflow-y-auto rounded-lg p-2'>
              <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
                <HugeiconsIcon icon={TextAlignLeftIcon} strokeWidth={2} />
                <span>{t('Lyrics')}</span>
              </div>
              {lyrics.length > 0 ? (
                <div className='flex flex-col gap-1'>
                  {lyrics.map((line, index) => (
                    <p
                      key={`${line.time}-${index}`}
                      ref={index === activeLyricIndex ? activeLyricRef : null}
                      aria-current={
                        index === activeLyricIndex ? 'true' : undefined
                      }
                      className={cn(
                        'rounded-md px-2 py-1 text-xs leading-5 transition-colors',
                        index === activeLyricIndex
                          ? 'bg-primary text-primary-foreground'
                          : 'text-muted-foreground'
                      )}
                    >
                      {line.text}
                    </p>
                  ))}
                </div>
              ) : (
                <p className='text-muted-foreground px-2 py-4 text-center text-xs'>
                  {t('No lyrics for this track')}
                </p>
              )}
            </div>
          ) : null}
        </div>
      </div>
    </TooltipProvider>
  )
}
