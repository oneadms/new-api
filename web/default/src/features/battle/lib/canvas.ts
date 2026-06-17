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
import backgroundImageUrl from '@/assets/battle/background.jpg'
import brickImageUrl from '@/assets/battle/brick.png'
import capImageUrl from '@/assets/battle/cap.png'
import playerThrow1ImageUrl from '@/assets/battle/people-throw-1.png'
import playerThrow2ImageUrl from '@/assets/battle/people-throw-2.png'
import playerThrow3ImageUrl from '@/assets/battle/people-throw-3.png'
import playerThrow4ImageUrl from '@/assets/battle/people-throw-4.png'
import playerImageUrl from '@/assets/battle/people.png'
import type {
  BattleBullet,
  BattlePlayer,
  BattlePlatform,
  BattlePowerup,
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
const MAX_DEVICE_PIXEL_RATIO = 2
const CAP_IMAGE_WIDTH = 93
const CAP_IMAGE_HEIGHT = 62
const CAP_IMAGE_RATIO = CAP_IMAGE_HEIGHT / CAP_IMAGE_WIDTH
const PLAYER_IMAGE_WIDTH = 107
const PLAYER_IMAGE_HEIGHT = 156
const PLAYER_IMAGE_RATIO = PLAYER_IMAGE_HEIGHT / PLAYER_IMAGE_WIDTH
const PLAYER_SPRITE_WIDTH = 58
const CAP_STACK_GAP = 11
const BRICK_IMAGE_WIDTH = 60
const BRICK_IMAGE_HEIGHT = 20
const MAX_RENDERED_STACK_CAPS = 36
const HEAVY_BULLET_COUNT = 40

let backgroundImage: HTMLImageElement | null = null
let brickImage: HTMLImageElement | null = null
let capImage: HTMLImageElement | null = null
let playerImage: HTMLImageElement | null = null
let playerThrowImages: HTMLImageElement[] | null = null
let arenaCache: {
  key: string
  canvas: HTMLCanvasElement
} | null = null

function getBackgroundImage(): HTMLImageElement | null {
  if (typeof Image === 'undefined') return null
  if (!backgroundImage) {
    backgroundImage = new Image()
    backgroundImage.src = backgroundImageUrl
  }
  return backgroundImage
}

function getBrickImage(): HTMLImageElement | null {
  if (typeof Image === 'undefined') return null
  if (!brickImage) {
    brickImage = new Image()
    brickImage.src = brickImageUrl
  }
  return brickImage
}

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
  emptyLabel: string,
  invalidLabel: string
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
    snapshot?.map_height || 900,
    snapshot?.platforms
  )

  if (!snapshot) {
    ctx.fillStyle = 'rgba(226, 232, 240, 0.82)'
    ctx.font = '600 18px system-ui, sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(emptyLabel, canvas.width / 2, canvas.height / 2)
    return
  }

  const bullets = Array.isArray(snapshot.bullets) ? snapshot.bullets : []
  const players = Array.isArray(snapshot.players) ? snapshot.players : []
  const powerups = Array.isArray(snapshot.powerups) ? snapshot.powerups : []
  const heavyScene = bullets.length > HEAVY_BULLET_COUNT
  const invalidTargetIds = new Set(
    snapshot.events
      .filter(
        (event) =>
          event.type === 'cap_invalid_insufficient_quota' &&
          snapshot.server_time - event.created_at < 1200 &&
          event.target_user_id
      )
      .map((event) => event.target_user_id)
  )

  powerups.forEach((powerup) => drawPowerup(ctx, powerup, metrics))
  bullets.forEach((bullet) => drawFlyingCap(ctx, bullet, metrics, heavyScene))

  const throwingPlayerIds = new Set(bullets.map((bullet) => bullet.owner_id))
  players.forEach((player) => {
    drawPlayer(
      ctx,
      player,
      metrics,
      throwingPlayerIds.has(player.user_id),
      (player.cap_storm_until ?? 0) > snapshot.server_time,
      invalidTargetIds.has(player.user_id) ? invalidLabel : ''
    )
  })
}

function drawArena(
  ctx: CanvasRenderingContext2D,
  metrics: CanvasMetrics,
  mapWidth: number,
  mapHeight: number,
  platforms?: BattlePlatform[]
): void {
  const platformList =
    platforms && platforms.length > 0
      ? platforms
      : createFallbackPlatforms(mapWidth, mapHeight)
  const cacheWidth = Math.max(1, Math.ceil(metrics.width))
  const cacheHeight = Math.max(1, Math.ceil(metrics.height))
  const cacheKey = arenaCacheKey(
    cacheWidth,
    cacheHeight,
    mapWidth,
    mapHeight,
    platformList
  )
  if (
    !arenaCache ||
    arenaCache.key !== cacheKey ||
    arenaCache.canvas.width !== cacheWidth ||
    arenaCache.canvas.height !== cacheHeight
  ) {
    const cacheCanvas = document.createElement('canvas')
    cacheCanvas.width = cacheWidth
    cacheCanvas.height = cacheHeight
    const cacheCtx = cacheCanvas.getContext('2d')
    if (cacheCtx) {
      const cacheMetrics = {
        ...metrics,
        offsetX: 0,
        offsetY: 0,
        width: cacheWidth,
        height: cacheHeight,
      }
      drawBackground(cacheCtx, cacheMetrics)
      platformList.forEach((platform) =>
        drawBrickPlatform(cacheCtx, platform, cacheMetrics)
      )
    }
    arenaCache = { key: cacheKey, canvas: cacheCanvas }
  }
  ctx.save()
  ctx.translate(metrics.offsetX, metrics.offsetY)
  ctx.drawImage(arenaCache.canvas, 0, 0, metrics.width, metrics.height)
  ctx.restore()
}

function arenaCacheKey(
  width: number,
  height: number,
  mapWidth: number,
  mapHeight: number,
  platforms: BattlePlatform[]
): string {
  const backgroundReady = Boolean(
    backgroundImage?.complete && backgroundImage.naturalWidth > 0
  )
  const brickReady = Boolean(
    brickImage?.complete && brickImage.naturalWidth > 0
  )
  const platformKey = platforms
    .map(
      (platform) =>
        `${platform.id}:${platform.x}:${platform.y}:${platform.w}:${platform.h}:${platform.one_way ? 1 : 0}`
    )
    .join('|')
  return `${width}x${height}:${mapWidth}x${mapHeight}:${backgroundReady ? 1 : 0}:${brickReady ? 1 : 0}:${platformKey}`
}

function drawBackground(
  ctx: CanvasRenderingContext2D,
  metrics: CanvasMetrics
): void {
  const image = getBackgroundImage()
  if (image?.complete && image.naturalWidth > 0) {
    const imageRatio = image.naturalWidth / image.naturalHeight
    const arenaRatio = metrics.width / metrics.height
    let sx = 0
    let sy = 0
    let sw = image.naturalWidth
    let sh = image.naturalHeight
    if (imageRatio > arenaRatio) {
      sw = image.naturalHeight * arenaRatio
      sx = (image.naturalWidth - sw) / 2
    } else {
      sh = image.naturalWidth / arenaRatio
      sy = (image.naturalHeight - sh) / 2
    }
    ctx.drawImage(image, sx, sy, sw, sh, 0, 0, metrics.width, metrics.height)
    ctx.fillStyle = 'rgba(2, 6, 23, 0.38)'
    ctx.fillRect(0, 0, metrics.width, metrics.height)
    return
  }

  const arenaGradient = ctx.createLinearGradient(
    0,
    0,
    metrics.width,
    metrics.height
  )
  arenaGradient.addColorStop(0, '#111827')
  arenaGradient.addColorStop(0.52, '#1f2937')
  arenaGradient.addColorStop(1, '#052e16')
  ctx.fillStyle = arenaGradient
  ctx.fillRect(0, 0, metrics.width, metrics.height)
}

function drawBrickPlatform(
  ctx: CanvasRenderingContext2D,
  platform: BattlePlatform,
  metrics: CanvasMetrics
): void {
  const x = platform.x * metrics.scale
  const y = platform.y * metrics.scale
  const width = platform.w * metrics.scale
  const height = platform.h * metrics.scale
  const image = getBrickImage()

  ctx.save()
  if (image?.complete && image.naturalWidth > 0) {
    const tileWidth = BRICK_IMAGE_WIDTH * metrics.scale
    const tileHeight = BRICK_IMAGE_HEIGHT * metrics.scale
    for (let ty = y; ty < y + height; ty += tileHeight) {
      for (let tx = x; tx < x + width; tx += tileWidth) {
        const nextWidth = Math.min(tileWidth, x + width - tx)
        const nextHeight = Math.min(tileHeight, y + height - ty)
        ctx.drawImage(image, tx, ty, nextWidth, nextHeight)
      }
    }
  } else {
    ctx.fillStyle = '#8b1e27'
    ctx.fillRect(x, y, width, height)
  }
  ctx.restore()
}

function createFallbackPlatforms(
  mapWidth: number,
  mapHeight: number
): BattlePlatform[] {
  return [
    {
      id: 'floor',
      x: 0,
      y: mapHeight - 40,
      w: mapWidth,
      h: 40,
      one_way: false,
    },
    {
      id: 'middle',
      x: mapWidth * 0.25,
      y: mapHeight * 0.58,
      w: mapWidth * 0.42,
      h: 26,
      one_way: true,
    },
    {
      id: 'top',
      x: mapWidth * 0.48,
      y: mapHeight * 0.34,
      w: mapWidth * 0.34,
      h: 26,
      one_way: true,
    },
  ]
}

function drawFlyingCap(
  ctx: CanvasRenderingContext2D,
  bullet: BattleBullet,
  metrics: CanvasMetrics,
  heavyScene: boolean
): void {
  const point = toScreen(bullet.x, bullet.y, metrics)
  const isStormCap = bullet.kind === 'cap_storm'
  const spin =
    ((typeof performance !== 'undefined' ? performance.now() : Date.now()) /
      (isStormCap ? 58 : 95) +
      hashString(bullet.id)) %
    (Math.PI * 2)
  const arcTilt = Math.atan2(bullet.vy || 0, bullet.vx || 1) * 0.2

  ctx.save()
  ctx.shadowColor = heavyScene
    ? 'transparent'
    : isStormCap
      ? 'rgba(250, 204, 21, 0.82)'
      : 'rgba(34, 197, 94, 0.65)'
  ctx.shadowBlur = heavyScene ? 0 : (isStormCap ? 24 : 16) * metrics.scale
  drawCapSprite(
    ctx,
    point.x,
    point.y,
    (isStormCap ? 64 : 54) * metrics.scale,
    spin + arcTilt,
    1
  )
  if (isStormCap) {
    ctx.strokeStyle = 'rgba(187, 247, 208, 0.5)'
    ctx.lineWidth = Math.max(1, 2 * metrics.scale)
    ctx.beginPath()
    ctx.arc(point.x, point.y, 24 * metrics.scale, 0, Math.PI * 2)
    ctx.stroke()
  }
  ctx.restore()
}

function drawPowerup(
  ctx: CanvasRenderingContext2D,
  powerup: BattlePowerup,
  metrics: CanvasMetrics
): void {
  const point = toScreen(powerup.x, powerup.y, metrics)
  const now =
    typeof performance !== 'undefined' ? performance.now() : Date.now()
  const pulse = 0.5 + Math.sin(now / 220 + hashString(powerup.id)) * 0.5
  const radius = (28 + pulse * 5) * metrics.scale

  ctx.save()
  ctx.shadowColor = 'rgba(250, 204, 21, 0.72)'
  ctx.shadowBlur = 24 * metrics.scale
  ctx.fillStyle = 'rgba(20, 83, 45, 0.72)'
  ctx.beginPath()
  ctx.arc(point.x, point.y, radius, 0, Math.PI * 2)
  ctx.fill()
  ctx.strokeStyle = 'rgba(250, 204, 21, 0.86)'
  ctx.lineWidth = Math.max(2, 3 * metrics.scale)
  ctx.stroke()
  drawCapSprite(
    ctx,
    point.x,
    point.y - 2 * metrics.scale,
    56 * metrics.scale,
    Math.sin(now / 260) * 0.18,
    1
  )
  ctx.strokeStyle = 'rgba(240, 253, 244, 0.88)'
  ctx.lineWidth = Math.max(1, 2 * metrics.scale)
  ctx.beginPath()
  ctx.moveTo(point.x - 8 * metrics.scale, point.y + 17 * metrics.scale)
  ctx.lineTo(point.x + 2 * metrics.scale, point.y + 2 * metrics.scale)
  ctx.lineTo(point.x - 1 * metrics.scale, point.y + 2 * metrics.scale)
  ctx.lineTo(point.x + 9 * metrics.scale, point.y - 15 * metrics.scale)
  ctx.stroke()
  ctx.restore()
}

function drawPlayer(
  ctx: CanvasRenderingContext2D,
  player: BattlePlayer,
  metrics: CanvasMetrics,
  isThrowing: boolean,
  hasCapStorm: boolean,
  invalidLabel: string
): void {
  const point = toScreen(player.x, player.y, metrics)
  const radius = PLAYER_RADIUS * metrics.scale
  const alpha = player.alive ? 1 : 0.58
  const spriteWidth = PLAYER_SPRITE_WIDTH * metrics.scale
  const spriteHeight = spriteWidth * PLAYER_IMAGE_RATIO
  const spriteBottom = point.y + 35 * metrics.scale
  const spriteTop = spriteBottom - spriteHeight
  const capCount = Math.max(0, Math.floor(player.cap_stack || 0))

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

  ctx.shadowColor = hasCapStorm
    ? 'rgba(250, 204, 21, 0.42)'
    : 'rgba(34, 197, 94, 0.24)'
  ctx.shadowBlur = player.alive ? (hasCapStorm ? 24 : 16) : 0
  drawPlayerSprite(
    ctx,
    point.x,
    spriteTop,
    spriteWidth,
    spriteHeight,
    player.alive,
    isThrowing,
    player.direction
  )
  ctx.shadowBlur = 0

  drawCapStack(ctx, point.x, spriteTop + spriteHeight * 0.16, capCount, metrics)

  ctx.fillStyle = 'rgba(241, 245, 249, 0.92)'
  ctx.font = `${Math.max(11, 12 * metrics.scale)}px system-ui, sans-serif`
  ctx.textAlign = 'center'
  ctx.fillText(player.username, point.x, spriteBottom + 16 * metrics.scale)
  if (capCount > 0) {
    ctx.fillStyle = 'rgba(187, 247, 208, 0.95)'
    ctx.font = `700 ${Math.max(11, 13 * metrics.scale)}px system-ui, sans-serif`
    ctx.fillText(
      `x${capCount}`,
      point.x + spriteWidth * 0.58,
      spriteTop - Math.min(capCount, 10) * CAP_STACK_GAP * metrics.scale
    )
  }
  if (invalidLabel) {
    drawInvalidLabel(
      ctx,
      point.x,
      spriteTop - 18 * metrics.scale,
      invalidLabel,
      metrics
    )
  }
  ctx.restore()
}

function drawInvalidLabel(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  label: string,
  metrics: CanvasMetrics
): void {
  ctx.save()
  ctx.font = `800 ${Math.max(13, 16 * metrics.scale)}px system-ui, sans-serif`
  ctx.textAlign = 'center'
  ctx.lineWidth = Math.max(3, 4 * metrics.scale)
  ctx.strokeStyle = 'rgba(15, 23, 42, 0.88)'
  ctx.fillStyle = 'rgba(248, 113, 113, 0.98)'
  ctx.strokeText(label, x, y)
  ctx.fillText(label, x, y)
  ctx.restore()
}

function drawPlayerSprite(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  alive: boolean,
  isThrowing: boolean,
  direction: number
): void {
  const image = getPlayerSpriteImage(isThrowing)
  if (image?.complete && image.naturalWidth > 0) {
    ctx.save()
    ctx.translate(x, y)
    if (direction < 0) {
      ctx.scale(-1, 1)
    }
    ctx.drawImage(image, -width / 2, 0, width, height)
    ctx.restore()
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

function drawCapStack(
  ctx: CanvasRenderingContext2D,
  x: number,
  headY: number,
  capCount: number,
  metrics: CanvasMetrics
): void {
  if (capCount <= 0) return
  const capWidth = PLAYER_SPRITE_WIDTH * 0.72 * metrics.scale
  const gap = CAP_STACK_GAP * metrics.scale
  const now =
    typeof performance !== 'undefined' ? performance.now() : Date.now()
  const renderedCount = Math.min(capCount, MAX_RENDERED_STACK_CAPS)
  for (let drawIndex = 0; drawIndex < renderedCount; drawIndex += 1) {
    const stackIndex =
      capCount <= MAX_RENDERED_STACK_CAPS
        ? drawIndex
        : Math.round((drawIndex * (capCount - 1)) / (renderedCount - 1))
    const y = headY - stackIndex * gap
    const sway = Math.sin((now + stackIndex * 137) / 280) * 1.5 * metrics.scale
    const rotation = -0.1 + Math.sin((now + stackIndex * 83) / 420) * 0.08
    drawCapSprite(ctx, x + sway, y, capWidth, rotation, 1)
  }
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
