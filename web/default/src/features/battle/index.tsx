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
import {
  CircleHelp,
  Gamepad2,
  LogOut,
  RefreshCw,
  Wifi,
  WifiOff,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Kbd, KbdGroup } from '@/components/ui/kbd'
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
  BattlePowerup,
  BattleServerMessage,
  BattleSnapshot,
  BattleStatus,
} from './types'

type ConnectionState = 'idle' | 'connecting' | 'connected' | 'closed'
type RuleControl = {
  keys: string[]
  label: string
}

const defaultRoomId = 'lobby'
const battleJoinBalanceMessage = 'Balance must be positive to join'
const battleJoinDepositMessage =
  'Balance must cover the match deposit to join. Recharge to enter this match.'
const hudUpdateIntervalMs = 150
const inputSendIntervalMs = 50
const inputHeartbeatIntervalMs = 250
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

function battleInputEquals(a: BattleInput, b: BattleInput): boolean {
  return (
    a.up === b.up &&
    a.down === b.down &&
    a.left === b.left &&
    a.right === b.right &&
    a.shoot === b.shoot &&
    a.jump === b.jump &&
    a.aim_x === b.aim_x &&
    a.aim_y === b.aim_y
  )
}

const battleEventTypes = new Set<BattleEvent['type']>([
  'hit',
  'cap_settlement',
  'settlement_failed',
  'powerup_pickup',
  'cap_storm_hit',
  'cap_invalid_insufficient_quota',
  'match_started',
  'match_ended',
  'player_forfeit',
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
    cap_storm_until: optionalPositiveNumber(value.cap_storm_until),
  }
}

function normalizeBullet(value: unknown): BattleBullet | null {
  if (!isRecord(value)) return null
  return {
    id: stringValue(value.id, ''),
    kind: stringValue(value.kind) || undefined,
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

function normalizePowerup(value: unknown): BattlePowerup | null {
  if (!isRecord(value)) return null
  const id = stringValue(value.id)
  if (!id) return null
  return {
    id,
    type: stringValue(value.type, 'cap_storm'),
    x: finiteNumber(value.x, 0),
    y: finiteNumber(value.y, 0),
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
    cap_count: optionalPositiveNumber(value.cap_count),
    reason: stringValue(value.reason) || undefined,
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
    match_phase: stringValue(value.match_phase) || undefined,
    match_starts_at: optionalPositiveNumber(value.match_starts_at),
    match_ends_at: optionalPositiveNumber(value.match_ends_at),
    match_min_players: optionalPositiveNumber(value.match_min_players),
    players: normalizeArray(value.players, normalizePlayer),
    bullets: normalizeArray(value.bullets, normalizeBullet),
    platforms: normalizeArray(value.platforms, normalizePlatform),
    powerups: normalizeArray(value.powerups, normalizePowerup),
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
  const lastInputSentAtRef = useRef(0)
  const lastInputSentRef = useRef<BattleInput | null>(null)
  const inputHistoryRef = useRef<BattleInputFrame[]>([])
  const snapshotRef = useRef<BattleSnapshot | null>(null)
  const snapshotFrameRef = useRef<BattleSnapshotFrame | null>(null)
  const snapshotBufferRef = useRef<BattleSnapshotFrame[]>([])
  const predictedPlayerRef = useRef<BattlePlayer | null>(null)
  const renderAtRef = useRef<number | null>(null)
  const hudUpdateAtRef = useRef(0)
  const hudUpdateTimerRef = useRef<number | null>(null)
  const eventToastSeenRef = useRef<Set<string>>(new Set())
  const [roomId, setRoomId] = useState('')
  const [connectionState, setConnectionState] =
    useState<ConnectionState>('idle')
  const [snapshot, setSnapshot] = useState<BattleSnapshot | null>(null)
  const [lastError, setLastError] = useState<string | null>(null)
  const [rulesOpen, setRulesOpen] = useState(false)
  const [leaveConfirmOpen, setLeaveConfirmOpen] = useState(false)

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
    const input = cloneBattleInput(inputRef.current)
    const inputChanged =
      !lastInputSentRef.current ||
      !battleInputEquals(input, lastInputSentRef.current)
    if (
      !inputChanged &&
      now - lastInputSentAtRef.current < inputHeartbeatIntervalMs
    ) {
      return
    }

    const seq = inputSeqRef.current + 1
    inputSeqRef.current = seq
    lastInputSentAtRef.current = now
    lastInputSentRef.current = input
    inputHistoryRef.current = trimBattleInputHistory(
      [...inputHistoryRef.current, { seq, input, sentAt: now }],
      now,
      snapshotRef.current?.ack_seq ?? 0
    )
    ws.send(JSON.stringify({ type: 'input', seq, input }))
  }, [])

  const disconnect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'leave', force_quit: true }))
    }
    wsRef.current?.close()
    wsRef.current = null
    inputRef.current = createEmptyInput()
    inputSeqRef.current = 0
    lastInputSentAtRef.current = 0
    lastInputSentRef.current = null
    inputHistoryRef.current = []
    snapshotFrameRef.current = null
    snapshotBufferRef.current = []
    predictedPlayerRef.current = null
    renderAtRef.current = null
    eventToastSeenRef.current.clear()
    clearHudUpdateTimer()
    setConnectionState('closed')
  }, [clearHudUpdateTimer])

  const requestDisconnect = useCallback(() => {
    setLeaveConfirmOpen(true)
  }, [])

  const connect = useCallback(() => {
    const statusData = battleStatus.data
    if (!statusData?.enabled) return
    const joinMessage = battleJoinBlockMessage(statusData)
    if (joinMessage) {
      setLastError(joinMessage)
      toast.error(t(joinMessage))
      return
    }
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
    lastInputSentAtRef.current = 0
    lastInputSentRef.current = null
    inputHistoryRef.current = []
    snapshotRef.current = null
    snapshotFrameRef.current = null
    snapshotBufferRef.current = []
    predictedPlayerRef.current = null
    renderAtRef.current = null
    eventToastSeenRef.current.clear()
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
      lastInputSentAtRef.current = 0
      lastInputSentRef.current = null
      inputHistoryRef.current = []
      snapshotFrameRef.current = null
      snapshotBufferRef.current = []
      predictedPlayerRef.current = null
      renderAtRef.current = null
      eventToastSeenRef.current.clear()
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
          t('Forgive Cap Battle'),
          t('Invalid')
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

  useEffect(() => {
    if (!snapshot) return
    for (const event of snapshot.events) {
      if (event.type !== 'cap_invalid_insufficient_quota') continue
      if (eventToastSeenRef.current.has(event.id)) continue
      eventToastSeenRef.current.add(event.id)
      if (
        event.user_id === snapshot.me ||
        event.target_user_id === snapshot.me
      ) {
        toast.info(battleInvalidCapToast(event, snapshot.me, t))
      }
    }
    if (eventToastSeenRef.current.size > 80) {
      eventToastSeenRef.current = new Set(
        [...eventToastSeenRef.current].slice(-40)
      )
    }
  }, [snapshot, t])

  const status = battleStatus.data
  const events = snapshot?.events ?? []
  const connected = connectionState === 'connected'
  const showPreJoinRules = !connected
  const hideRoomInput = Boolean(status?.hide_room_input)
  const joinBlockMessage = status ? battleJoinBlockMessage(status) : undefined
  const canAffordJoin = !joinBlockMessage
  const joinBlockReason = joinBlockMessage ? t(joinBlockMessage) : undefined
  const joinDisabled =
    connectionState === 'connecting' ||
    (!connected &&
      (battleStatus.isLoading || !status?.enabled || !canAffordJoin))
  const capQuotaText = formatQuota(status?.cap_quota ?? 0)
  const matchDepositText = formatQuota(status?.match_entry_quota ?? 0)
  const capStormSeconds = Math.max(
    0,
    Math.ceil(
      ((me?.cap_storm_until ?? 0) - (snapshot?.server_time ?? 0)) / 1000
    )
  )
  const capStormLabel =
    capStormSeconds > 0
      ? t('Cap Storm {{seconds}}s', { seconds: capStormSeconds })
      : t('No power-up')
  const matchPhase =
    snapshot?.match_phase ?? (status?.match_mode_enabled ? 'waiting' : 'free')
  const matchLabel = matchStatusText(
    matchPhase,
    snapshot,
    status?.match_min_players,
    t
  )
  const ruleControls = useMemo<RuleControl[]>(
    () => [
      { keys: ['A'], label: t('Move left') },
      { keys: ['D'], label: t('Move right') },
      { keys: ['S'], label: t('Drop down or fast fall') },
      { keys: ['J'], label: t('Throw a green cap') },
      { keys: ['K'], label: t('Jump') },
    ],
    [t]
  )
  const ruleNotes = useMemo(
    () => [
      t(
        'Jump across the brick platforms and throw green caps onto other players.'
      ),
      t('Every cap that lands on a player stacks visibly on their head.'),
      t('Random Cap Storm power-ups appear on the map.'),
      t(
        'Pick one up to fling every cap on your own head with J for a limited time.'
      ),
      t(
        'A Cap Storm hit moves as many caps as balance coverage allows; missed throws can be retried until the timer ends.'
      ),
      status?.allow_negative_balance
        ? t('This room allows battle settlement to take balances negative.')
        : t(
            'Hits only count when both players have enough balance coverage; otherwise the hit is marked Invalid and the cap does not stack.'
          ),
      ...(status?.match_mode_enabled && (status.match_entry_quota ?? 0) > 0
        ? [
            t(
              'In match mode, every player uses the same {{quota}} match deposit as the in-match loss cap.',
              { quota: matchDepositText }
            ),
          ]
        : []),
      t('Each cap is currently worth {{quota}}.', { quota: capQuotaText }),
      t(
        'When a match ends normally, players lose quota for caps on their own head and the throwers receive that quota.'
      ),
      t(
        'If you force quit, caps on your own head still settle as losses, but your pending rewards from caps on other players are cleared.'
      ),
    ],
    [
      capQuotaText,
      matchDepositText,
      status?.allow_negative_balance,
      status?.match_entry_quota,
      status?.match_mode_enabled,
      t,
    ]
  )
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
            type='button'
            variant='outline'
            onClick={() => setRulesOpen(true)}
            className='h-9'
          >
            <CircleHelp data-icon='inline-start' />
            {t('Game rules')}
          </Button>
          <Button
            onClick={connected ? requestDisconnect : connect}
            disabled={joinDisabled}
            className='h-9'
          >
            {connected ? (
              <LogOut data-icon='inline-start' />
            ) : (
              <Gamepad2 data-icon='inline-start' />
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
            <RefreshCw />
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
          {showPreJoinRules && (
            <div className='bg-background/88 absolute inset-0 flex items-center justify-center p-4 backdrop-blur-sm'>
              <BattleRulesEmptyState
                controls={ruleControls}
                notes={ruleNotes}
                disabled={joinDisabled}
                blockReason={joinBlockReason}
                onJoin={connect}
                onOpenRules={() => setRulesOpen(true)}
              />
            </div>
          )}
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
              <Stat label={t('Power-up')} value={capStormLabel} />
              {status?.match_mode_enabled && (
                <Stat label={t('Match')} value={matchLabel} />
              )}
              {status?.match_mode_enabled &&
                (status.match_entry_quota ?? 0) > 0 && (
                  <Stat
                    label={t('Match deposit')}
                    value={formatQuota(status.match_entry_quota)}
                  />
                )}
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
              {status?.match_mode_enabled &&
                (status.match_entry_quota ?? 0) > 0 && (
                  <LimitRow
                    label={t('Match deposit')}
                    value={formatQuota(status.match_entry_quota)}
                  />
                )}
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
      <BattleRulesDialog
        open={rulesOpen}
        onOpenChange={setRulesOpen}
        controls={ruleControls}
        notes={ruleNotes}
      />
      <Dialog open={leaveConfirmOpen} onOpenChange={setLeaveConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Force quit battle?')}</DialogTitle>
            <DialogDescription>
              {t(
                'Leaving now clears your pending rewards from caps you placed on other players. Caps on your own head will still settle as losses.'
              )}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setLeaveConfirmOpen(false)}
            >
              {t('Stay in battle')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              onClick={() => {
                setLeaveConfirmOpen(false)
                disconnect()
              }}
            >
              {t('Force quit')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function BattleRulesEmptyState(props: {
  controls: RuleControl[]
  notes: string[]
  disabled: boolean
  blockReason?: string
  onJoin: () => void
  onOpenRules: () => void
}) {
  const { t } = useTranslation()

  return (
    <Empty className='bg-background/95 max-w-3xl border shadow-sm'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>
          <Gamepad2 />
        </EmptyMedia>
        <EmptyTitle>{t('Ready to stack green caps?')}</EmptyTitle>
        <EmptyDescription>
          {t('Read the rules, then join the room when you are ready.')}
        </EmptyDescription>
      </EmptyHeader>
      <EmptyContent className='max-w-2xl'>
        <BattleControlList controls={props.controls} compact />
        <RuleNotes notes={props.notes.slice(0, 3)} />
        {props.blockReason && (
          <div className='text-destructive text-center text-sm font-medium'>
            {props.blockReason}
          </div>
        )}
        <div className='flex w-full flex-col gap-2 sm:flex-row sm:justify-center'>
          <Button
            type='button'
            onClick={props.onJoin}
            disabled={props.disabled}
          >
            <Gamepad2 data-icon='inline-start' />
            {t('Join')}
          </Button>
          <Button type='button' variant='outline' onClick={props.onOpenRules}>
            <CircleHelp data-icon='inline-start' />
            {t('Game rules')}
          </Button>
        </div>
      </EmptyContent>
    </Empty>
  )
}

function BattleRulesDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  controls: RuleControl[]
  notes: string[]
}) {
  const { t } = useTranslation()

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-hidden sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Game rules')}</DialogTitle>
          <DialogDescription>
            {t('Controls, cap stacking, and quota settlement.')}
          </DialogDescription>
        </DialogHeader>
        <div className='flex max-h-[60vh] flex-col gap-5 overflow-y-auto pr-1'>
          <section className='flex flex-col gap-3'>
            <h3 className='text-sm font-medium'>{t('Controls')}</h3>
            <BattleControlList controls={props.controls} />
          </section>
          <section className='flex flex-col gap-3'>
            <h3 className='text-sm font-medium'>{t('Settlement rules')}</h3>
            <RuleNotes notes={props.notes} />
          </section>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Got it')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function BattleControlList(props: {
  controls: RuleControl[]
  compact?: boolean
}) {
  return (
    <dl className='grid w-full gap-2 text-sm sm:grid-cols-2'>
      {props.controls.map((control) => (
        <div
          key={control.label}
          className='bg-background/70 flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-left'
        >
          <dt>
            <KbdGroup>
              {control.keys.map((key) => (
                <Kbd key={key}>{key}</Kbd>
              ))}
            </KbdGroup>
          </dt>
          <dd className='text-muted-foreground'>{control.label}</dd>
        </div>
      ))}
    </dl>
  )
}

function RuleNotes(props: { notes: string[] }) {
  return (
    <ul className='text-muted-foreground flex w-full flex-col gap-2 text-left text-sm leading-relaxed'>
      {props.notes.map((note) => (
        <li key={note}>{note}</li>
      ))}
    </ul>
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

function matchStatusText(
  phase: string,
  snapshot: BattleSnapshot | null,
  fallbackMinPlayers: number | undefined,
  t: TFunction
): string {
  const serverTime = snapshot?.server_time ?? Date.now()
  if (phase === 'running') {
    const seconds = Math.max(
      0,
      Math.ceil(((snapshot?.match_ends_at ?? serverTime) - serverTime) / 1000)
    )
    return t('Running, {{seconds}}s left', { seconds })
  }
  if (phase === 'waiting') {
    const startsAt = snapshot?.match_starts_at
    if (startsAt && startsAt > serverTime) {
      const seconds = Math.ceil((startsAt - serverTime) / 1000)
      return t('Waiting, starts in {{seconds}}s', { seconds })
    }
    return t('Waiting for players {{count}}/{{min}}', {
      count: snapshot?.players.length ?? 0,
      min: snapshot?.match_min_players ?? fallbackMinPlayers ?? 0,
    })
  }
  if (phase === 'ended') {
    return t('Match ended')
  }
  return t('Free play')
}

function battleJoinBlockMessage(status: BattleStatus): string | undefined {
  if (
    status.match_mode_enabled &&
    (status.join_required_quota ?? 0) > 1 &&
    status.quota < status.join_required_quota
  ) {
    return battleJoinDepositMessage
  }
  if (status.quota <= 0) {
    return battleJoinBalanceMessage
  }
  return undefined
}

function battleInvalidCapToast(
  event: BattleEvent,
  currentUserId: number,
  t: TFunction
): string {
  if (
    event.reason === 'thrower_insufficient_quota' &&
    event.user_id === currentUserId
  ) {
    return t(
      'This hit did not stack: your balance coverage is insufficient. Recharge to continue the match.'
    )
  }
  if (
    event.reason === 'target_insufficient_quota' &&
    event.target_user_id === currentUserId
  ) {
    return t(
      'This hit did not stack: your payable balance is insufficient. Recharge to continue the match.'
    )
  }
  if (event.reason === 'both_insufficient_quota') {
    if (
      event.user_id === currentUserId ||
      event.target_user_id === currentUserId
    ) {
      return t(
        'This hit did not stack: both players have insufficient balance coverage. Recharge to continue the match.'
      )
    }
  }
  if (event.reason === 'target_insufficient_quota') {
    return t(
      "This hit did not stack: opponent's payable balance is insufficient."
    )
  }
  if (event.reason === 'thrower_insufficient_quota') {
    return t(
      "This hit did not stack: thrower's balance coverage is insufficient."
    )
  }
  return t('This hit did not stack: balance coverage is insufficient.')
}

function invalidCapReasonText(
  event: BattleEvent,
  currentUserId: number,
  t: TFunction
): string {
  if (event.reason === 'target_insufficient_quota') {
    return event.target_user_id === currentUserId
      ? t('your payable balance is insufficient')
      : t("opponent's payable balance is insufficient")
  }
  if (event.reason === 'thrower_insufficient_quota') {
    return event.user_id === currentUserId
      ? t('your balance coverage is insufficient')
      : t("thrower's balance coverage is insufficient")
  }
  if (event.reason === 'both_insufficient_quota') {
    return t('both players have insufficient balance coverage')
  }
  return t('balance coverage is insufficient')
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
  if (event.type === 'powerup_pickup') {
    return t('{{user}} picked up Cap Storm', { user })
  }
  if (event.type === 'cap_storm_hit') {
    return t('{{user}} stormed {{count}} caps onto {{target}}', {
      user,
      target,
      count: event.cap_count ?? 0,
    })
  }
  if (event.type === 'cap_invalid_insufficient_quota') {
    const reason = invalidCapReasonText(event, snapshot.me, t)
    return t(
      '{{count}} cap(s) from {{user}} did not stack on {{target}}: {{reason}}',
      {
        user,
        target,
        count: event.cap_count ?? 1,
        reason,
      }
    )
  }
  if (event.type === 'cap_settlement') {
    return t('{{user}} settled {{quota}} from {{target}}', {
      user,
      target,
      quota: formatQuota(event.quota ?? 0),
    })
  }
  if (event.type === 'match_started') {
    return t('Match started')
  }
  if (event.type === 'match_ended') {
    return t('Match ended')
  }
  if (event.type === 'player_forfeit') {
    return t('{{user}} force quit and cleared pending rewards', { user })
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
