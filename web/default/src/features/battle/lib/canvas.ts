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
import capImageUrl from '@/assets/battle/cap.png'
import playerThrow1ImageUrl from '@/assets/battle/people-throw-1.png'
import playerThrow2ImageUrl from '@/assets/battle/people-throw-2.png'
import playerThrow3ImageUrl from '@/assets/battle/people-throw-3.png'
import playerThrow4ImageUrl from '@/assets/battle/people-throw-4.png'
import playerImageUrl from '@/assets/battle/people.png'
import type {
  BattleBullet,
  BattleDrop,
  BattlePlayer,
  BattleSnapshot,
} from '../types'

export type CanvasMetrics = {
  scale: number
  offsetX: number
  offsetY: number
  width: number
  height: number
}

const PLAYER_RADIUS = 18
const DROP_RADIUS = 9
const MAX_DEVICE_PIXEL_RATIO = 2
const CAP_IMAGE_WIDTH = 93
const CAP_IMAGE_HEIGHT = 62
const CAP_IMAGE_RATIO = CAP_IMAGE_HEIGHT / CAP_IMAGE_WIDTH
const PLAYER_IMAGE_WIDTH = 107
const PLAYER_IMAGE_HEIGHT = 156
const PLAYER_IMAGE_RATIO = PLAYER_IMAGE_HEIGHT / PLAYER_IMAGE_WIDTH

let capImage: HTMLImageElement | null = null
let playerImage: HTMLImageElement | null = null
let playerThrowImages: HTMLImageElement[] | null = null

function getCapImage(): HTMLImageElement | null {
  if (typeof Image === 'undefined') return null
  if (!capImage) {
    capImage = new Image()
    capImage.src = capImageUrl
  }
  return capImage
}

function getPlayerImage(): HTMLImageElement | null {
  if (typeof Image === 'undefined') return null
  if (!playerImage) {
    playerImage = new Image()
    playerImage.src = playerImageUrl
  }
  return playerImage
}

function getPlayerThrowImages(): HTMLImageElement[] {
  if (typeof Image === 'undefined') return []
  if (!playerThrowImages) {
    playerThrowImages = [
      playerThrow1ImageUrl,
      playerThrow2ImageUrl,
      playerThrow3ImageUrl,
      playerThrow4ImageUrl,
    ].map((url) => {
      const image = new Image()
      image.src = url
      return image
    })
  }
  return playerThrowImages
}

export function resizeCanvas(canvas: HTMLCanvasElement): void {
  const rect = canvas.getBoundingClientRect()
  const dpr = Math.min(window.devicePixelRatio || 1, MAX_DEVICE_PIXEL_RATIO)
  const width = Math.max(320, Math.floor(rect.width * dpr))
  const height = Math.max(240, Math.floor(rect.height * dpr))
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width
    canvas.height = height
  }
}

export function getCanvasMetrics(
  canvas: HTMLCanvasElement,
  snapshot: BattleSnapshot | null
): CanvasMetrics {
  const mapWidth = snapshot?.map_width || 1600
  const mapHeight = snapshot?.map_height || 900
  const scale = Math.min(canvas.width / mapWidth, canvas.height / mapHeight)
  const width = mapWidth * scale
  const height = mapHeight * scale
  return {
    scale,
    offsetX: (canvas.width - width) / 2,
    offsetY: (canvas.height - height) / 2,
    width,
    height,
  }
}

export function screenToWorld(
  x: number,
  y: number,
  metrics: CanvasMetrics
): { x: number; y: number } {
  return {
    x: (x - metrics.offsetX) / metrics.scale,
    y: (y - metrics.offsetY) / metrics.scale,
  }
}

export function drawBattleCanvas(
  canvas: HTMLCanvasElement,
  snapshot: BattleSnapshot | null,
  emptyLabel: string
): void {
  resizeCanvas(canvas)
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const metrics = getCanvasMetrics(canvas, snapshot)
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  ctx.fillStyle = '#0b1020'
  ctx.fillRect(0, 0, canvas.width, canvas.height)
  drawArena(
    ctx,
    metrics,
    snapshot?.map_width || 1600,
    snapshot?.map_height || 900
  )

  if (!snapshot) {
    ctx.fillStyle = 'rgba(226, 232, 240, 0.82)'
    ctx.font = '600 18px system-ui, sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(emptyLabel, canvas.width / 2, canvas.height / 2)
    return
  }

  const drops = Array.isArray(snapshot.drops) ? snapshot.drops : []
  const bullets = Array.isArray(snapshot.bullets) ? snapshot.bullets : []
  const players = Array.isArray(snapshot.players) ? snapshot.players : []

  drops.forEach((drop) => drawQuotaCap(ctx, drop, metrics))

  bullets.forEach((bullet) => drawFlyingCap(ctx, bullet, metrics))

  const me = players.find((player) => player.user_id === snapshot.me)
  const throwingPlayerIds = new Set(bullets.map((bullet) => bullet.owner_id))
  players.forEach((player) => {
    drawPlayer(
      ctx,
      player,
      metrics,
      player.user_id === snapshot.me,
      throwingPlayerIds.has(player.user_id)
    )
  })

  if (me?.alive) {
    const point = toScreen(me.x, me.y, metrics)
    ctx.beginPath()
    ctx.strokeStyle = 'rgba(34, 211, 238, 0.22)'
    ctx.lineWidth = 2
    ctx.arc(point.x, point.y, 58 * metrics.scale, 0, Math.PI * 2)
    ctx.stroke()
  }
}

function drawArena(
  ctx: CanvasRenderingContext2D,
  metrics: CanvasMetrics,
  mapWidth: number,
  mapHeight: number
): void {
  ctx.save()
  ctx.translate(metrics.offsetX, metrics.offsetY)
  const arenaGradient = ctx.createLinearGradient(
    0,
    0,
    metrics.width,
    metrics.height
  )
  arenaGradient.addColorStop(0, '#0f172a')
  arenaGradient.addColorStop(0.48, '#172033')
  arenaGradient.addColorStop(1, '#0d1f1b')
  ctx.fillStyle = arenaGradient
  ctx.fillRect(0, 0, metrics.width, metrics.height)
  ctx.strokeStyle = 'rgba(148, 163, 184, 0.14)'
  ctx.lineWidth = 1

  const grid = 100 * metrics.scale
  for (let x = 0; x <= metrics.width; x += grid) {
    ctx.beginPath()
    ctx.moveTo(x, 0)
    ctx.lineTo(x, metrics.height)
    ctx.stroke()
  }
  for (let y = 0; y <= metrics.height; y += grid) {
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(metrics.width, y)
    ctx.stroke()
  }

  ctx.strokeStyle = 'rgba(226, 232, 240, 0.32)'
  ctx.lineWidth = 2
  ctx.strokeRect(0, 0, mapWidth * metrics.scale, mapHeight * metrics.scale)

  ctx.fillStyle = 'rgba(34, 197, 94, 0.12)'
  ctx.strokeStyle = 'rgba(34, 197, 94, 0.24)'
  ctx.lineWidth = Math.max(1, metrics.scale)
  const platformHeight = 18 * metrics.scale
  const platforms = [
    { x: 0.08, y: 0.22, w: 0.26 },
    { x: 0.46, y: 0.36, w: 0.32 },
    { x: 0.18, y: 0.58, w: 0.24 },
    { x: 0.62, y: 0.72, w: 0.28 },
  ]
  platforms.forEach((platform) => {
    const x = platform.x * metrics.width
    const y = platform.y * metrics.height
    const width = platform.w * metrics.width
    ctx.fillRect(x, y, width, platformHeight)
    ctx.strokeRect(x, y, width, platformHeight)
  })
  ctx.restore()
}

function drawFlyingCap(
  ctx: CanvasRenderingContext2D,
  bullet: BattleBullet,
  metrics: CanvasMetrics
): void {
  const point = toScreen(bullet.x, bullet.y, metrics)
  const spin =
    ((typeof performance !== 'undefined' ? performance.now() : Date.now()) /
      95 +
      hashString(bullet.id)) %
    (Math.PI * 2)

  ctx.save()
  ctx.shadowColor = 'rgba(34, 197, 94, 0.65)'
  ctx.shadowBlur = 16 * metrics.scale
  drawCapSprite(ctx, point.x, point.y, 54 * metrics.scale, spin, 1)
  ctx.restore()
}

function drawQuotaCap(
  ctx: CanvasRenderingContext2D,
  drop: BattleDrop,
  metrics: CanvasMetrics
): void {
  const point = toScreen(drop.x, drop.y, metrics)
  const bob =
    Math.sin(
      ((typeof performance !== 'undefined' ? performance.now() : Date.now()) +
        hashString(drop.id) * 120) /
        360
    ) *
    4 *
    metrics.scale

  ctx.save()
  ctx.shadowColor = 'rgba(132, 204, 22, 0.72)'
  ctx.shadowBlur = 18 * metrics.scale
  drawCapSprite(
    ctx,
    point.x,
    point.y - DROP_RADIUS * metrics.scale + bob,
    48 * metrics.scale,
    -0.12,
    1
  )
  ctx.shadowBlur = 0
  ctx.fillStyle = 'rgba(240, 253, 244, 0.95)'
  ctx.font = `${Math.max(10, 11 * metrics.scale)}px system-ui, sans-serif`
  ctx.textAlign = 'center'
  ctx.fillText(
    formatCompactQuota(drop.quota),
    point.x,
    point.y + 23 * metrics.scale
  )
  ctx.restore()
}

function drawPlayer(
  ctx: CanvasRenderingContext2D,
  player: BattlePlayer,
  metrics: CanvasMetrics,
  isMe: boolean,
  isThrowing: boolean
): void {
  const point = toScreen(player.x, player.y, metrics)
  const radius = PLAYER_RADIUS * metrics.scale
  const alpha = player.alive ? 1 : 0.58
  const spriteWidth = 58 * metrics.scale
  const spriteHeight = spriteWidth * PLAYER_IMAGE_RATIO
  const spriteBottom = point.y + radius * 1.15
  const spriteTop = spriteBottom - spriteHeight

  ctx.save()
  ctx.globalAlpha = alpha
  ctx.beginPath()
  ctx.fillStyle = 'rgba(15, 23, 42, 0.42)'
  ctx.ellipse(
    point.x,
    spriteBottom - radius * 0.16,
    radius * 1.25,
    radius * 0.42,
    0,
    0,
    Math.PI * 2
  )
  ctx.fill()

  ctx.shadowColor = isMe
    ? 'rgba(34, 211, 238, 0.45)'
    : 'rgba(34, 197, 94, 0.24)'
  ctx.shadowBlur = player.alive ? 16 : 0
  drawPlayerSprite(
    ctx,
    point.x,
    spriteTop,
    spriteWidth,
    spriteHeight,
    player.alive,
    isThrowing
  )
  ctx.shadowBlur = 0

  if (isMe && player.alive) {
    ctx.beginPath()
    ctx.strokeStyle = 'rgba(34, 211, 238, 0.58)'
    ctx.lineWidth = Math.max(2, 2.5 * metrics.scale)
    ctx.arc(point.x, point.y + radius * 0.18, radius * 1.35, 0, Math.PI * 2)
    ctx.stroke()
  }

  drawCapSprite(
    ctx,
    point.x + spriteWidth * 0.03,
    spriteTop + spriteHeight * 0.16,
    spriteWidth * 0.72,
    -0.1,
    alpha
  )

  const hpWidth = 54 * metrics.scale
  const hpHeight = Math.max(4, 6 * metrics.scale)
  const hpX = point.x - hpWidth / 2
  const hpY = spriteTop - 13 * metrics.scale
  ctx.fillStyle = 'rgba(15, 23, 42, 0.72)'
  ctx.fillRect(hpX, hpY, hpWidth, hpHeight)
  ctx.fillStyle = player.hp > 35 ? '#22c55e' : '#ef4444'
  ctx.fillRect(hpX, hpY, (hpWidth * Math.max(0, player.hp)) / 100, hpHeight)

  ctx.fillStyle = 'rgba(241, 245, 249, 0.92)'
  ctx.font = `${Math.max(11, 12 * metrics.scale)}px system-ui, sans-serif`
  ctx.textAlign = 'center'
  ctx.fillText(player.username, point.x, spriteBottom + 16 * metrics.scale)
  ctx.restore()
}

function drawPlayerSprite(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  alive: boolean,
  isThrowing: boolean
): void {
  const image = getPlayerSpriteImage(isThrowing)
  if (image?.complete && image.naturalWidth > 0) {
    ctx.drawImage(image, x - width / 2, y, width, height)
    return
  }

  ctx.fillStyle = alive ? '#f8fafc' : '#94a3b8'
  ctx.beginPath()
  ctx.ellipse(
    x,
    y + height * 0.48,
    width * 0.38,
    height * 0.44,
    -0.18,
    0,
    Math.PI * 2
  )
  ctx.fill()
  ctx.strokeStyle = 'rgba(15, 23, 42, 0.7)'
  ctx.lineWidth = Math.max(1, width * 0.04)
  ctx.stroke()
}

function getPlayerSpriteImage(isThrowing: boolean): HTMLImageElement | null {
  if (!isThrowing) return getPlayerImage()
  const throwImages = getPlayerThrowImages().filter(
    (image) => image.complete && image.naturalWidth > 0
  )
  if (throwImages.length === 0) return getPlayerImage()

  const frame =
    Math.floor(
      (typeof performance !== 'undefined' ? performance.now() : Date.now()) / 90
    ) % throwImages.length
  return throwImages[frame]
}

function drawCapSprite(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  rotation: number,
  alpha: number
): void {
  const image = getCapImage()
  const height = width * CAP_IMAGE_RATIO

  ctx.save()
  ctx.translate(x, y)
  ctx.rotate(rotation)
  ctx.globalAlpha *= alpha

  if (image?.complete && image.naturalWidth > 0) {
    ctx.drawImage(image, -width / 2, -height / 2, width, height)
  } else {
    drawFallbackCap(ctx, width, height)
  }

  ctx.restore()
}

function drawFallbackCap(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number
): void {
  ctx.fillStyle = '#22c55e'
  ctx.beginPath()
  ctx.ellipse(0, height * 0.1, width * 0.44, height * 0.28, 0, 0, Math.PI * 2)
  ctx.fill()
  ctx.fillStyle = '#16a34a'
  ctx.fillRect(-width * 0.28, -height * 0.22, width * 0.56, height * 0.36)
  ctx.fillStyle = 'rgba(240, 253, 244, 0.32)'
  ctx.fillRect(-width * 0.2, -height * 0.18, width * 0.4, height * 0.08)
}

function toScreen(
  x: number,
  y: number,
  metrics: CanvasMetrics
): { x: number; y: number } {
  return {
    x: metrics.offsetX + x * metrics.scale,
    y: metrics.offsetY + y * metrics.scale,
  }
}

function hashString(value: string): number {
  let hash = 0
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) % 997
  }
  return hash
}

function formatCompactQuota(value: number): string {
  const amount = Math.abs(value)
  if (amount >= 1000000) return `${Math.round(value / 100000) / 10}M`
  if (amount >= 1000) return `${Math.round(value / 100) / 10}K`
  return `${value}`
}
