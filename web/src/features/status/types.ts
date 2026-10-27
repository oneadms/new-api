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
export type StatusState =
  | 'passed'
  | 'review'
  | 'partial'
  | 'failed'
  | 'unknown'
  | 'stale'
  | 'paused'
export type StatusSample = {
  public_id: string
  channel_name: string
  model: string
  status: string
  quality_result: string
  completed_at: number
  stale_after: number
  first_text_ms: number | null
  generation_ms: number | null
  thumbnail_url: string | null
  render_status: string
}
export type StatusGroup = {
  public_id: string
  name: string
  channel_count: number
  configured_count: number
  fresh_count: number
  state: StatusState
  success_rate: number | null
  avg_latency_ms: number | null
  tokens_per_second: number | null
  series: { ts: number; value: number | null }[]
  latest_sample: StatusSample | null
}
export type StatusPageData = {
  enabled: boolean
  paused: boolean
  generated_at: number
  metrics_data_until: number
  last_probe_completed_at: number
  window_start: number
  window_end: number
  bucket_seconds: number
  total: number
  page: number
  page_size: number
  models: string[]
  summary: { groups: number; attention: number; channels: number }
  groups: StatusGroup[]
}
export type StatusFilters = {
  hours: number
  model: string
  group: string
  state: string
  page: number
  page_size: number
  sort: string
}
export type StatusGroupDetail = {
  group: StatusGroup
  samples: StatusSample[]
  total: number
  page: number
  page_size: number
}
