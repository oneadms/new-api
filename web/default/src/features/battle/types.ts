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
export type BattleStatus = {
  enabled: boolean
  hide_room_input: boolean
  quota: number
  daily_lost: number
  daily_won: number
  cap_quota: number
  max_round_loss: number
  max_round_gain: number
  max_daily_loss: number
  max_daily_gain: number
  max_players: number
  map_width: number
  map_height: number
  player_speed: number
}

export type BattleInput = {
  up: boolean
  down: boolean
  left: boolean
  right: boolean
  shoot: boolean
  jump: boolean
  aim_x: number
  aim_y: number
}

export type BattlePlayer = {
  user_id: number
  username: string
  x: number
  y: number
  vx: number
  vy: number
  alive: boolean
  direction: number
  on_ground: boolean
  round_loss: number
  round_gain: number
  cap_stack: number
  cap_storm_until?: number
}

export type BattleBullet = {
  id: string
  kind?: 'cap' | 'cap_storm' | string
  owner_id: number
  x: number
  y: number
  vx: number
  vy: number
}

export type BattleEvent = {
  id: string
  type:
    | 'hit'
    | 'cap_settlement'
    | 'settlement_failed'
    | 'powerup_pickup'
    | 'cap_storm_hit'
  user_id?: number
  target_user_id?: number
  quota?: number
  cap_count?: number
  created_at: number
}

export type BattlePlatform = {
  id: string
  x: number
  y: number
  w: number
  h: number
  one_way: boolean
}

export type BattlePowerup = {
  id: string
  type: 'cap_storm' | string
  x: number
  y: number
}

export type BattleSnapshot = {
  type: 'snapshot'
  room_id: string
  me: number
  ack_seq: number
  server_time: number
  map_width: number
  map_height: number
  player_speed: number
  players: BattlePlayer[]
  bullets: BattleBullet[]
  platforms: BattlePlatform[]
  powerups: BattlePowerup[]
  events: BattleEvent[]
}

export type BattleServerMessage =
  | BattleSnapshot
  | {
      type: 'joined' | 'error'
      room_id?: string
      message?: string
    }
