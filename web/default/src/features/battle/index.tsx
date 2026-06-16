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
import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { Gamepad2, LogOut, RefreshCw, Wifi, WifiOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getBattleStatus } from './api'
import { drawBattleCanvas } from './lib/canvas'
import {
  cloneBattleInput,
  interpolateBattleSnapshot,
  mergeLocalPlayer,
  predictLocalPlayer,
  smoothLocalPlayer,
  trimBattleInputHistory,
  type BattleInputFrame,
  type BattleSnapshotFrame,
} from './lib/prediction'
import type {
  BattleBullet,
  BattleEvent,
  BattleInput,
  BattlePlatform,
  BattlePlayer,
  BattleServerMessage,
  BattleSnapshot,
} from './types'

type ConnectionState = 'idle' | 'connecting' | 'connected' | 'closed'
const defaultRoomId = 'lobby'
const hudUpdateIntervalMs = 150
const inputSendIntervalMs = 50
const snapshotInterpolationDelayMs = 120
const snapshotBufferLimit = 16
const defaultPlayerSpeed = 260

function createEmptyInput(): BattleInput {
  return {
    up: false,
    down: false,
    left: false,
    right: false,
    shoot: false,
    jump: false,
    aim_x: 1,
    aim_y: 0,
  }
}

const battleEventTypes = new Set<BattleEvent['type']>([
  'hit',
  'cap_settlement',
  'settlement_failed',
])

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function stringValue(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback
}

function finiteNumber(value: unknown, fallback = 0): number {
  const next = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(next) ? next : fallback
}

function positiveNumber(value: unknown, fallback: number): number {
  const next = finiteNumber(value, fallback)
  return next > 0 ? next : fallback
}

function optionalPositiveNumber(value: unknown): number | undefined {
  const next = finiteNumber(value, 0)
  return next > 0 ? next : undefined
}

function normalizeArray<T>(
  value: unknown,
  normalize: (item: unknown) => T | null
): T[] {
  if (!Array.isArray(value)) return []
  return value.reduce<T[]>((items, item) => {
    const next = normalize(item)
    if (next) items.push(next)
    return items
  }, [])
}

function normalizePlayer(value: unknown): BattlePlayer | null {
  if (!isRecord(value)) return null
  const userId = finiteNumber(value.user_id, 0)
  if (userId <= 0) return null
  return {
    user_id: userId,
    username: stringValue(value.username, `#${userId}`),
    x: finiteNumber(value.x, 0),
    y: finiteNumber(value.y, 0),
    vx: finiteNumber(value.vx, 0),
    vy: finiteNumber(value.vy, 0),
    alive: Boolean(value.alive),
    direction: finiteNumber(value.direction, 1) >= 0 ? 1 : -1,
    on_ground: Boolean(value.on_ground),
    round_loss: finiteNumber(value.round_loss, 0),
    round_gain: finiteNumber(value.round_gain, 0),
    cap_stack: finiteNumber(value.cap_stack, 0),
  }
}

function normalizeBullet(value: unknown): BattleBullet | null {
  if (!isRecord(value)) return null
  return {
    id: stringValue(value.id, ''),
    owner_id: finiteNumber(value.owner_id, 0),
    x: finiteNumber(value.x, 0),
    y: finiteNumber(value.y, 0),
    vx: finiteNumber(value.vx, 0),
    vy: finiteNumber(value.vy, 0),
  }
}

function normalizePlatform(value: unknown): BattlePlatform | null {
  if (!isRecord(value)) return null
  const id = stringValue(value.id)
  if (!id) return null
  return {
    id,
    x: finiteNumber(value.x, 0),
    y: finiteNumber(value.y, 0),
    w: finiteNumber(value.w, 0),
    h: finiteNumber(value.h, 0),
    one_way: Boolean(value.one_way),
  }
}

function isBattleEventType(value: string): value is BattleEvent['type'] {
  return battleEventTypes.has(value as BattleEvent['type'])
}

function normalizeEvent(value: unknown): BattleEvent | null {
  if (!isRecord(value)) return null
  const type = stringValue(value.type)
  if (!isBattleEventType(type)) return null
  const createdAt = finiteNumber(value.created_at, Date.now())
  return {
    id: stringValue(value.id, `${type}-${createdAt}`),
    type,
    user_id: optionalPositiveNumber(value.user_id),
    target_user_id: optionalPositiveNumber(value.target_user_id),
    quota: optionalPositiveNumber(value.quota),
    created_at: createdAt,
  }
}

function normalizeSnapshot(value: Record<string, unknown>): BattleSnapshot {
  return {
    type: 'snapshot',
    room_id: stringValue(value.room_id, 'lobby') || 'lobby',
    me: finiteNumber(value.me, 0),
    ack_seq: finiteNumber(value.ack_seq, 0),
    server_time: finiteNumber(value.server_time, Date.now()),
    map_width: positiveNumber(value.map_width, 1600),
    map_height: positiveNumber(value.map_height, 900),
    player_speed: positiveNumber(value.player_speed, defaultPlayerSpeed),
    players: normalizeArray(value.players, normalizePlayer),
    bullets: normalizeArray(value.bullets, normalizeBullet),
    platforms: normalizeArray(value.platforms, normalizePlatform),
    events: normalizeArray(value.events, normalizeEvent),
  }
}

function parseBattleServerMessage(data: string): BattleServerMessage | null {
  let payload: unknown
  try {
    payload = JSON.parse(data)
  } catch {
    return null
  }
  if (!isRecord(payload)) return null

  const type = stringValue(payload.type)
  if (type === 'snapshot') {
    return normalizeSnapshot(payload)
  }
  if (type === 'joined' || type === 'error') {
    return {
      type,
      room_id: stringValue(payload.room_id) || undefined,
      message: stringValue(payload.message) || undefined,
    }
  }
  return null
}

export function Battle() {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const inputRef = useRef<BattleInput>(createEmptyInput())
  const inputSeqRef = useRef(0)
  const inputHistoryRef = useRef<BattleInputFrame[]>([])
  const snapshotRef = useRef<BattleSnapshot | null>(null)
  const snapshotFrameRef = useRef<BattleSnapshotFrame | null>(null)
  const snapshotBufferRef = useRef<BattleSnapshotFrame[]>([])
  const predictedPlayerRef = useRef<BattlePlayer | null>(null)
  const renderAtRef = useRef<number | null>(null)
  const hudUpdateAtRef = useRef(0)
  const hudUpdateTimerRef = useRef<number | null>(null)
  const [roomId, setRoomId] = useState('')
  const [connectionState, setConnectionState] =
    useState<ConnectionState>('idle')
  const [snapshot, setSnapshot] = useState<BattleSnapshot | null>(null)
  const [lastError, setLastError] = useState<string | null>(null)

  const battleStatus = useQuery({
    queryKey: ['battle-status'],
    queryFn: getBattleStatus,
    refetchInterval: 15000,
  })
  const playerSpeed = battleStatus.data?.player_speed ?? defaultPlayerSpeed

  const me = useMemo(() => {
    if (!snapshot) return null
    return (
      snapshot.players.find((player) => player.user_id === snapshot.me) ?? null
    )
  }, [snapshot])

  const leaderboard = useMemo(() => {
    return [...(snapshot?.players ?? [])].sort((a, b) => {
      if (a.cap_stack === b.cap_stack) return b.round_gain - a.round_gain
      return a.cap_stack - b.cap_stack
    })
  }, [snapshot])

  const clearHudUpdateTimer = useCallback(() => {
    if (hudUpdateTimerRef.current !== null) {
      window.clearTimeout(hudUpdateTimerRef.current)
      hudUpdateTimerRef.current = null
    }
  }, [])

  const publishSnapshot = useCallback(
    (nextSnapshot: BattleSnapshot) => {
      const now = window.performance?.now() ?? Date.now()
      const frame = { at: now, snapshot: nextSnapshot }
      snapshotRef.current = nextSnapshot
      snapshotFrameRef.current = frame
      snapshotBufferRef.current = [...snapshotBufferRef.current, frame].slice(
        -snapshotBufferLimit
      )
      inputHistoryRef.current = trimBattleInputHistory(
        inputHistoryRef.current,
        now,
        nextSnapshot.ack_seq
      )

      const elapsed = now - hudUpdateAtRef.current
      if (elapsed >= hudUpdateIntervalMs) {
        clearHudUpdateTimer()
        hudUpdateAtRef.current = now
        setSnapshot(nextSnapshot)
        return
      }

      if (hudUpdateTimerRef.current !== null) return

      hudUpdateTimerRef.current = window.setTimeout(() => {
        hudUpdateTimerRef.current = null
        hudUpdateAtRef.current = window.performance?.now() ?? Date.now()
        setSnapshot(snapshotRef.current)
      }, hudUpdateIntervalMs - elapsed)
    },
    [clearHudUpdateTimer]
  )

  const sendInput = useCallback(() => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const now = window.performance?.now() ?? Date.now()
    const seq = inputSeqRef.current + 1
    const input = cloneBattleInput(inputRef.current)
    inputSeqRef.current = seq
    inputHistoryRef.current = trimBattleInputHistory(
      [...inputHistoryRef.current, { seq, input, sentAt: now }],
      now,
      snapshotRef.current?.ack_seq ?? 0
    )
    ws.send(JSON.stringify({ type: 'input', seq, input }))
  }, [])

  const disconnect = useCallback(() => {
    wsRef.current?.close()
    wsRef.current = null
    inputRef.current = createEmptyInput()
    inputSeqRef.current = 0
    inputHistoryRef.current = []
    snapshotFrameRef.current = null
    snapshotBufferRef.current = []
    predictedPlayerRef.current = null
    renderAtRef.current = null
    clearHudUpdateTimer()
    setConnectionState('closed')
  }, [clearHudUpdateTimer])

  const connect = useCallback(() => {
    const statusData = battleStatus.data
    if (!statusData?.enabled) return
    wsRef.current?.close()

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const normalizedRoom = statusData.hide_room_input
      ? defaultRoomId
      : roomId.trim() || defaultRoomId
    const ws = new WebSocket(
      `${protocol}//${window.location.host}/api/battle/ws?room=${encodeURIComponent(
        normalizedRoom
      )}`
    )

    wsRef.current = ws
    inputRef.current = createEmptyInput()
    inputSeqRef.current = 0
    inputHistoryRef.current = []
    snapshotRef.current = null
    snapshotFrameRef.current = null
    snapshotBufferRef.current = []
    predictedPlayerRef.current = null
    renderAtRef.current = null
    setSnapshot(null)
    clearHudUpdateTimer()
    hudUpdateAtRef.current = 0
    setLastError(null)
    setConnectionState('connecting')

    ws.onopen = () => {
      setConnectionState('connected')
    }

    ws.onmessage = (event: MessageEvent<string>) => {
      const message = parseBattleServerMessage(event.data)
      if (!message) {
        setLastError('Connection failed')
        return
      }
      if (message.type === 'snapshot') {
        publishSnapshot(message)
        return
      }
      if (message.type === 'joined') {
        setConnectionState('connected')
        return
      }
      if (message.type === 'error') {
        const nextError = message.message || 'Connection failed'
        setLastError(nextError)
        toast.error(t(nextError))
      }
    }

    ws.onerror = () => {
      setLastError('Connection failed')
      toast.error(t('Connection failed'))
    }

    ws.onclose = () => {
      setConnectionState((current) =>
        current === 'connected' || current === 'connecting' ? 'closed' : current
      )
    }
  }, [battleStatus.data, clearHudUpdateTimer, publishSnapshot, roomId, t])

  useEffect(() => {
    return () => {
      wsRef.current?.close()
      inputRef.current = createEmptyInput()
      inputHistoryRef.current = []
      snapshotFrameRef.current = null
      snapshotBufferRef.current = []
      predictedPlayerRef.current = null
      renderAtRef.current = null
      clearHudUpdateTimer()
    }
  }, [clearHudUpdateTimer])

  useEffect(() => {
    const interval = window.setInterval(() => {
      sendInput()
    }, inputSendIntervalMs)
    return () => window.clearInterval(interval)
  }, [sendInput])

  useEffect(() => {
    let frame = 0
    const draw = () => {
      const canvas = canvasRef.current
      if (canvas) {
        const now = window.performance?.now() ?? Date.now()
        const interpolatedSnapshot = interpolateBattleSnapshot(
          snapshotBufferRef.current,
          now - snapshotInterpolationDelayMs
        )
        const targetPlayer = predictLocalPlayer(
          snapshotFrameRef.current,
          inputHistoryRef.current,
          inputRef.current,
          now,
          snapshotFrameRef.current?.snapshot.player_speed ?? playerSpeed
        )
        const dtMs =
          renderAtRef.current === null ? 16 : now - renderAtRef.current
        renderAtRef.current = now
        const predictedPlayer = smoothLocalPlayer(
          predictedPlayerRef.current,
          targetPlayer,
          dtMs
        )
        predictedPlayerRef.current = predictedPlayer
        drawBattleCanvas(
          canvas,
          mergeLocalPlayer(
            interpolatedSnapshot ?? snapshotRef.current,
            predictedPlayer
          ),
          t('Forgive Cap Battle')
        )
      }
      frame = window.requestAnimationFrame(draw)
    }
    draw()
    return () => window.cancelAnimationFrame(frame)
  }, [playerSpeed, t])

  useEffect(() => {
    const handleKey = (event: KeyboardEvent, pressed: boolean) => {
      if (
        event.target instanceof HTMLInputElement ||
        event.target instanceof HTMLTextAreaElement
      ) {
        return
      }
      const input = inputRef.current
      let changed = false
      const setFlag = (
        key: 'up' | 'down' | 'left' | 'right' | 'shoot' | 'jump'
      ) => {
        if (input[key] === pressed) return
        input[key] = pressed
        changed = true
      }
      switch (event.code) {
        case 'KeyA':
          setFlag('left')
          event.preventDefault()
          break
        case 'KeyS':
          setFlag('down')
          event.preventDefault()
          break
        case 'KeyD':
          setFlag('right')
          event.preventDefault()
          break
        case 'KeyJ':
          setFlag('shoot')
          event.preventDefault()
          break
        case 'KeyK':
          setFlag('jump')
          event.preventDefault()
          break
        default:
          break
      }
      if (changed) sendInput()
    }
    const keydown = (event: KeyboardEvent) => handleKey(event, true)
    const keyup = (event: KeyboardEvent) => handleKey(event, false)
    window.addEventListener('keydown', keydown)
    window.addEventListener('keyup', keyup)
    return () => {
      window.removeEventListener('keydown', keydown)
      window.removeEventListener('keyup', keyup)
    }
  }, [sendInput])

  const status = battleStatus.data
  const events = snapshot?.events ?? []
  const connected = connectionState === 'connected'
  const hideRoomInput = Boolean(status?.hide_room_input)
  const connectionLabel = {
    idle: t('Idle'),
    connecting: t('Connecting'),
    connected: t('Connected'),
    closed: t('Closed'),
  }[connectionState]

  return (
    <div className='flex h-full min-h-[calc(100vh-4rem)] flex-col gap-4 overflow-y-auto p-4 md:p-6 lg:overflow-hidden'>
      <div className='flex flex-col gap-3 border-b pb-4 lg:flex-row lg:items-center lg:justify-between'>
        <div>
          <h1 className='text-2xl font-semibold tracking-normal'>
            {t('Forgive Cap Battle')}
          </h1>
          <div className='text-muted-foreground mt-2 flex flex-wrap items-center gap-2 text-sm'>
            <Badge variant={status?.enabled ? 'default' : 'secondary'}>
              {status?.enabled ? t('Enabled') : t('Disabled')}
            </Badge>
            <span>
              {t('Balance')}: {formatQuota(status?.quota ?? 0)}
            </span>
            <span>
              {t('Daily loss')}: {formatQuota(status?.daily_lost ?? 0)}
            </span>
            <span>
              {t('Daily win')}: {formatQuota(status?.daily_won ?? 0)}
            </span>
          </div>
        </div>

        <div className='flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:items-center'>
          {hideRoomInput ? (
            <div className='border-input bg-muted/40 flex h-9 items-center rounded-md border px-3 text-sm font-medium whitespace-nowrap'>
              {t('Lobby')}
            </div>
          ) : (
            <Input
              value={roomId}
              onChange={(event) => setRoomId(event.target.value)}
              disabled={connected || connectionState === 'connecting'}
              placeholder={t('Lobby')}
              className='h-9 w-full sm:w-44'
              aria-label={t('Room')}
            />
          )}
          <Button
            onClick={connected ? disconnect : connect}
            disabled={
              battleStatus.isLoading ||
              !status?.enabled ||
              connectionState === 'connecting'
            }
            className='h-9'
          >
            {connected ? (
              <LogOut className='size-4' />
            ) : (
              <Gamepad2 className='size-4' />
            )}
            {connected ? t('Leave') : t('Join')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='icon'
            onClick={() => battleStatus.refetch()}
            disabled={battleStatus.isFetching}
            aria-label={t('Refresh')}
          >
            <RefreshCw className='size-4' />
          </Button>
        </div>
      </div>

      <div className='grid flex-none gap-4 lg:min-h-0 lg:flex-1 lg:grid-cols-[minmax(0,1fr)_320px]'>
        <div className='relative min-h-[420px] overflow-hidden rounded-md border bg-slate-950'>
          <canvas
            ref={canvasRef}
            className='h-full min-h-[420px] w-full touch-none'
          />
          <div className='absolute top-3 left-3 flex items-center gap-2 rounded-md border border-white/10 bg-slate-950/80 px-3 py-2 text-xs text-slate-100 backdrop-blur'>
            {connected ? (
              <Wifi className='size-4 text-emerald-300' />
            ) : (
              <WifiOff className='size-4 text-slate-400' />
            )}
            <span>{connectionLabel}</span>
          </div>
        </div>

        <aside className='flex min-h-0 flex-col gap-4 lg:overflow-y-auto lg:pr-1'>
          <section className='shrink-0 rounded-md border p-4'>
            <h2 className='text-base font-medium'>{t('Round')}</h2>
            <div className='mt-3 grid grid-cols-2 gap-3 text-sm'>
              <Stat
                label={t('Gain')}
                value={formatQuota(me?.round_gain ?? 0)}
              />
              <Stat
                label={t('Loss')}
                value={formatQuota(me?.round_loss ?? 0)}
              />
              <Stat
                label={t('Caps on head')}
                value={String(me?.cap_stack ?? 0)}
              />
              <Stat
                label={t('Movement')}
                value={
                  me?.on_ground
                    ? t('Grounded')
                    : me && me.vy < 0
                      ? t('Jumping')
                      : t('Falling')
                }
              />
            </div>
          </section>

          <section className='shrink-0 rounded-md border p-4'>
            <h2 className='text-base font-medium'>{t('Limits')}</h2>
            <div className='text-muted-foreground mt-3 space-y-2 text-sm'>
              <LimitRow
                label={t('Quota per cap')}
                value={formatQuota(status?.cap_quota ?? 0)}
              />
              <LimitRow
                label={t('Round loss')}
                value={formatQuota(status?.max_round_loss ?? 0)}
              />
              <LimitRow
                label={t('Round win')}
                value={formatQuota(status?.max_round_gain ?? 0)}
              />
              <LimitRow
                label={t('Daily loss')}
                value={formatQuota(status?.max_daily_loss ?? 0)}
              />
              <LimitRow
                label={t('Daily win')}
                value={formatQuota(status?.max_daily_gain ?? 0)}
              />
            </div>
          </section>

          <section className='shrink-0 rounded-md border p-4'>
            <h2 className='text-base font-medium'>{t('Leaderboard')}</h2>
            <div className='mt-3 space-y-2'>
              {leaderboard.length > 0 ? (
                leaderboard.map((player) => (
                  <PlayerRow
                    key={player.user_id}
                    player={player}
                    active={player.user_id === snapshot?.me}
                  />
                ))
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {t('No players yet')}
                </p>
              )}
            </div>
          </section>

          <section className='shrink-0 rounded-md border p-4'>
            <h2 className='text-base font-medium'>{t('Events')}</h2>
            <div className='text-muted-foreground mt-3 max-h-44 space-y-2 overflow-auto text-sm'>
              {events.length > 0 ? (
                events
                  .slice(-8)
                  .reverse()
                  .map((event) => (
                    <div key={event.id}>
                      {snapshot ? battleEventText(event, snapshot, t) : null}
                    </div>
                  ))
              ) : (
                <div>{t('No events yet')}</div>
              )}
              {lastError && (
                <div className='text-destructive'>{t(lastError)}</div>
              )}
            </div>
          </section>
        </aside>
      </div>
    </div>
  )
}

function Stat(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/50 rounded-md px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 truncate font-medium'>{props.value}</div>
    </div>
  )
}

function LimitRow(props: { label: string; value: string }) {
  return (
    <div className='flex items-center justify-between gap-3'>
      <span>{props.label}</span>
      <span className='text-foreground font-medium'>{props.value}</span>
    </div>
  )
}

function PlayerRow(props: { player: BattlePlayer; active: boolean }) {
  return (
    <div className='bg-muted/40 flex items-center justify-between gap-3 rounded-md px-3 py-2 text-sm'>
      <div className='min-w-0'>
        <div className='truncate font-medium'>
          {props.active ? '* ' : ''}
          {props.player.username}
        </div>
        <div className='text-muted-foreground text-xs'>
          x{props.player.cap_stack}
        </div>
      </div>
      <div className='text-right font-medium'>
        {formatQuota(props.player.round_gain)}
      </div>
    </div>
  )
}

function battleEventText(
  event: BattleEvent,
  snapshot: BattleSnapshot,
  t: TFunction
): string {
  const user = playerName(snapshot, event.user_id)
  const target = playerName(snapshot, event.target_user_id)
  if (event.type === 'hit') {
    return t('{{user}} put a green cap on {{target}}', { user, target })
  }
  if (event.type === 'cap_settlement') {
    return t('{{user}} settled {{quota}} from {{target}}', {
      user,
      target,
      quota: formatQuota(event.quota ?? 0),
    })
  }
  return t('Cap settlement failed')
}

function playerName(
  snapshot: BattleSnapshot,
  userId: number | undefined
): string {
  if (!userId) return '-'
  return (
    snapshot.players.find((player) => player.user_id === userId)?.username ||
    `#${userId}`
  )
}
