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
import type {
  BattleBullet,
  BattleInput,
  BattlePlatform,
  BattlePlayer,
  BattleSnapshot,
} from '../types'

export type BattleSnapshotFrame = {
  at: number
  snapshot: BattleSnapshot
}

export type BattleInputFrame = {
  seq: number
  input: BattleInput
  sentAt: number
}

const maxPredictionMs = 110
const maxInputHistoryMs = 2000
const maxInputHistoryItems = 120
const correctionSnapDistance = 360
const correctionSmoothingMs = 55
const playerWidth = 46
const playerHeight = 70
const gravity = 1800
const jumpVelocity = -900
const maxFallSpeed = 1080
const fastFallAcceleration = 900

export function cloneBattleInput(input: BattleInput): BattleInput {
  return {
    up: input.up,
    down: input.down,
    left: input.left,
    right: input.right,
    shoot: input.shoot,
    jump: input.jump,
    aim_x: input.aim_x,
    aim_y: input.aim_y,
  }
}

export function trimBattleInputHistory(
  history: BattleInputFrame[],
  now: number,
  ackSeq: number
): BattleInputFrame[] {
  const recent = history.filter(
    (entry) => now - entry.sentAt <= maxInputHistoryMs || entry.seq >= ackSeq
  )
  if (recent.length <= maxInputHistoryItems) return recent
  return recent.slice(-maxInputHistoryItems)
}

export function interpolateBattleSnapshot(
  frames: BattleSnapshotFrame[],
  renderAt: number
): BattleSnapshot | null {
  if (frames.length === 0) return null
  if (frames.length === 1 || renderAt <= frames[0].at) {
    return frames[0].snapshot
  }

  let previous = frames[0]
  for (let index = 1; index < frames.length; index += 1) {
    const next = frames[index]
    if (next.at < renderAt) {
      previous = next
      continue
    }

    const span = next.at - previous.at
    const progress = span > 0 ? clamp((renderAt - previous.at) / span, 0, 1) : 1
    return interpolateSnapshot(previous.snapshot, next.snapshot, progress)
  }

  return frames[frames.length - 1].snapshot
}

export function predictLocalPlayer(
  frame: BattleSnapshotFrame | null,
  inputHistory: BattleInputFrame[],
  currentInput: BattleInput,
  now: number,
  playerSpeed: number
): BattlePlayer | null {
  const snapshot = frame?.snapshot
  if (!snapshot || playerSpeed <= 0) return null

  const authoritative = snapshot.players.find(
    (player) => player.user_id === snapshot.me
  )
  if (!authoritative || !authoritative.alive) return authoritative ?? null

  const predicted = { ...authoritative }
  const cappedNow = Math.min(now, frame.at + maxPredictionMs)
  let cursor = frame.at
  let activeInput =
    findInputAtOrBeforeSeq(inputHistory, snapshot.ack_seq) ??
    cloneBattleInput(currentInput)
  const entries = inputHistory
    .filter(
      (entry) => entry.seq > snapshot.ack_seq && entry.sentAt <= cappedNow
    )
    .sort((a, b) => a.seq - b.seq)

  for (const entry of entries) {
    if (entry.sentAt <= cursor) {
      activeInput = entry.input
      continue
    }
    movePlayer(
      predicted,
      activeInput,
      (entry.sentAt - cursor) / 1000,
      playerSpeed,
      snapshot.map_width,
      snapshot.map_height,
      snapshot.platforms
    )
    activeInput = entry.input
    cursor = entry.sentAt
  }

  if (cappedNow > cursor) {
    movePlayer(
      predicted,
      activeInput,
      (cappedNow - cursor) / 1000,
      playerSpeed,
      snapshot.map_width,
      snapshot.map_height,
      snapshot.platforms
    )
  }

  return predicted
}

export function mergeLocalPlayer(
  snapshot: BattleSnapshot | null,
  player: BattlePlayer | null
): BattleSnapshot | null {
  if (!snapshot || !player) return snapshot
  return {
    ...snapshot,
    players: snapshot.players.map((item) =>
      item.user_id === player.user_id ? player : item
    ),
  }
}

export function smoothLocalPlayer(
  previous: BattlePlayer | null,
  target: BattlePlayer | null,
  dtMs: number
): BattlePlayer | null {
  if (!target) return null
  if (
    !previous ||
    previous.user_id !== target.user_id ||
    previous.alive !== target.alive
  ) {
    return target
  }

  const distance = Math.hypot(target.x - previous.x, target.y - previous.y)
  if (distance > correctionSnapDistance) return target

  const progress = clamp(1 - Math.exp(-dtMs / correctionSmoothingMs), 0.12, 0.5)
  return {
    ...target,
    x: lerp(previous.x, target.x, progress),
    y: lerp(previous.y, target.y, progress),
  }
}

function interpolateSnapshot(
  previous: BattleSnapshot,
  next: BattleSnapshot,
  progress: number
): BattleSnapshot {
  return {
    ...next,
    server_time: Math.round(
      lerp(previous.server_time, next.server_time, progress)
    ),
    players: interpolatePlayers(previous.players, next.players, progress),
    bullets: interpolateBullets(previous.bullets, next.bullets, progress),
  }
}

function interpolatePlayers(
  previous: BattlePlayer[],
  next: BattlePlayer[],
  progress: number
): BattlePlayer[] {
  const previousById = new Map(
    previous.map((player) => [player.user_id, player])
  )
  return next.map((player) => {
    const before = previousById.get(player.user_id)
    if (!before || before.alive !== player.alive) return player
    return {
      ...player,
      x: lerp(before.x, player.x, progress),
      y: lerp(before.y, player.y, progress),
    }
  })
}

function interpolateBullets(
  previous: BattleBullet[],
  next: BattleBullet[],
  progress: number
): BattleBullet[] {
  const previousById = new Map(previous.map((bullet) => [bullet.id, bullet]))
  return next.map((bullet) => {
    const before = previousById.get(bullet.id)
    if (!before) return bullet
    return {
      ...bullet,
      x: lerp(before.x, bullet.x, progress),
      y: lerp(before.y, bullet.y, progress),
    }
  })
}

function findInputAtOrBeforeSeq(
  history: BattleInputFrame[],
  seq: number
): BattleInput | null {
  let found: BattleInputFrame | null = null
  for (const entry of history) {
    if (entry.seq > seq) continue
    if (!found || entry.seq > found.seq) found = entry
  }
  return found ? cloneBattleInput(found.input) : null
}

function movePlayer(
  player: BattlePlayer,
  input: BattleInput,
  dt: number,
  playerSpeed: number,
  mapWidth: number,
  mapHeight: number,
  platforms: BattlePlatform[]
): void {
  if (dt <= 0) return

  const moveX = numberFromBoolean(input.right) - numberFromBoolean(input.left)
  if (moveX < 0) player.direction = -1
  if (moveX > 0) player.direction = 1
  player.vx = moveX * playerSpeed
  player.x = clamp(
    player.x + player.vx * dt,
    playerWidth / 2,
    mapWidth - playerWidth / 2
  )
  resolveHorizontal(player, platforms, mapWidth)

  if (input.jump && player.on_ground) {
    player.vy = jumpVelocity
    player.on_ground = false
  }
  if (input.down && player.on_ground) {
    player.y += 2
    player.vy = Math.max(player.vy, playerSpeed)
    player.on_ground = false
  }

  player.vy = Math.min(
    player.vy + gravity * dt + (input.down ? fastFallAcceleration * dt : 0),
    maxFallSpeed
  )
  const oldY = player.y
  player.y += player.vy * dt
  resolveVertical(player, oldY, platforms, mapHeight)
}

function resolveHorizontal(
  player: BattlePlayer,
  platforms: BattlePlatform[],
  mapWidth: number
): void {
  if (player.vx === 0) return
  for (const platform of platforms) {
    if (platform.one_way) continue
    if (
      !rectsOverlap(
        playerLeft(player),
        playerTop(player),
        playerWidth,
        playerHeight,
        platform.x,
        platform.y,
        platform.w,
        platform.h
      )
    ) {
      continue
    }
    player.x =
      player.vx > 0
        ? platform.x - playerWidth / 2
        : platform.x + platform.w + playerWidth / 2
    player.vx = 0
  }
  player.x = clamp(player.x, playerWidth / 2, mapWidth - playerWidth / 2)
}

function resolveVertical(
  player: BattlePlayer,
  oldY: number,
  platforms: BattlePlatform[],
  mapHeight: number
): void {
  const oldTop = oldY - playerHeight / 2
  const oldBottom = oldY + playerHeight / 2
  const newTop = playerTop(player)
  const newBottom = playerBottom(player)
  player.on_ground = false

  for (const platform of platforms) {
    if (
      playerRight(player) <= platform.x ||
      playerLeft(player) >= platform.x + platform.w
    ) {
      continue
    }
    if (player.vy >= 0) {
      if (oldBottom <= platform.y && newBottom >= platform.y) {
        player.y = platform.y - playerHeight / 2
        player.vy = 0
        player.on_ground = true
        break
      }
      continue
    }
    if (platform.one_way) continue
    if (
      oldTop >= platform.y + platform.h &&
      newTop <= platform.y + platform.h
    ) {
      player.y = platform.y + platform.h + playerHeight / 2
      player.vy = 0
      break
    }
  }

  if (playerTop(player) < 0) {
    player.y = playerHeight / 2
    player.vy = 0
  }
  if (playerBottom(player) > mapHeight) {
    player.y = mapHeight - playerHeight / 2
    player.vy = 0
    player.on_ground = true
  }
}

function playerLeft(player: BattlePlayer): number {
  return player.x - playerWidth / 2
}

function playerRight(player: BattlePlayer): number {
  return player.x + playerWidth / 2
}

function playerTop(player: BattlePlayer): number {
  return player.y - playerHeight / 2
}

function playerBottom(player: BattlePlayer): number {
  return player.y + playerHeight / 2
}

function rectsOverlap(
  ax: number,
  ay: number,
  aw: number,
  ah: number,
  bx: number,
  by: number,
  bw: number,
  bh: number
): boolean {
  return ax < bx + bw && ax + aw > bx && ay < by + bh && ay + ah > by
}

function numberFromBoolean(value: boolean): number {
  return value ? 1 : 0
}

function lerp(from: number, to: number, progress: number): number {
  return from + (to - from) * progress
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}
