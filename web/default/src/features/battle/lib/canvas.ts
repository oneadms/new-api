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
import type { BattlePlayer, BattleSnapshot } from '../types'

export type CanvasMetrics = {
  scale: number
  offsetX: number
  offsetY: number
  width: number
  height: number
}

const PLAYER_RADIUS = 18
const DROP_RADIUS = 9

export function resizeCanvas(canvas: HTMLCanvasElement): void {
  const rect = canvas.getBoundingClientRect()
  const dpr = window.devicePixelRatio || 1
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

  snapshot.drops.forEach((drop) => {
    const point = toScreen(drop.x, drop.y, metrics)
    ctx.save()
    ctx.translate(point.x, point.y)
    ctx.rotate(Math.PI / 4)
    ctx.fillStyle = '#f8c537'
    ctx.shadowColor = 'rgba(248, 197, 55, 0.7)'
    ctx.shadowBlur = 14
    ctx.fillRect(-DROP_RADIUS, -DROP_RADIUS, DROP_RADIUS * 2, DROP_RADIUS * 2)
    ctx.restore()
  })

  snapshot.bullets.forEach((bullet) => {
    const point = toScreen(bullet.x, bullet.y, metrics)
    ctx.beginPath()
    ctx.fillStyle = '#f97316'
    ctx.shadowColor = 'rgba(249, 115, 22, 0.8)'
    ctx.shadowBlur = 10
    ctx.arc(point.x, point.y, 5 * metrics.scale, 0, Math.PI * 2)
    ctx.fill()
    ctx.shadowBlur = 0
  })

  const me = snapshot.players.find((player) => player.user_id === snapshot.me)
  snapshot.players.forEach((player) => {
    drawPlayer(ctx, player, metrics, player.user_id === snapshot.me)
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
  ctx.fillStyle = '#111827'
  ctx.fillRect(0, 0, metrics.width, metrics.height)
  ctx.strokeStyle = 'rgba(148, 163, 184, 0.16)'
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
  ctx.restore()
}

function drawPlayer(
  ctx: CanvasRenderingContext2D,
  player: BattlePlayer,
  metrics: CanvasMetrics,
  isMe: boolean
): void {
  const point = toScreen(player.x, player.y, metrics)
  const radius = PLAYER_RADIUS * metrics.scale
  const color = player.alive ? (isMe ? '#22d3ee' : '#fb7185') : '#64748b'

  ctx.save()
  ctx.beginPath()
  ctx.fillStyle = color
  ctx.shadowColor = isMe
    ? 'rgba(34, 211, 238, 0.45)'
    : 'rgba(251, 113, 133, 0.3)'
  ctx.shadowBlur = player.alive ? 16 : 0
  ctx.arc(point.x, point.y, radius, 0, Math.PI * 2)
  ctx.fill()
  ctx.shadowBlur = 0

  ctx.lineWidth = Math.max(2, 3 * metrics.scale)
  ctx.strokeStyle = 'rgba(15, 23, 42, 0.8)'
  ctx.stroke()

  const hpWidth = 54 * metrics.scale
  const hpHeight = Math.max(4, 6 * metrics.scale)
  const hpX = point.x - hpWidth / 2
  const hpY = point.y - radius - 18 * metrics.scale
  ctx.fillStyle = 'rgba(15, 23, 42, 0.72)'
  ctx.fillRect(hpX, hpY, hpWidth, hpHeight)
  ctx.fillStyle = player.hp > 35 ? '#22c55e' : '#ef4444'
  ctx.fillRect(hpX, hpY, (hpWidth * Math.max(0, player.hp)) / 100, hpHeight)

  ctx.fillStyle = 'rgba(241, 245, 249, 0.92)'
  ctx.font = `${Math.max(11, 12 * metrics.scale)}px system-ui, sans-serif`
  ctx.textAlign = 'center'
  ctx.fillText(player.username, point.x, point.y + radius + 17 * metrics.scale)
  ctx.restore()
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
