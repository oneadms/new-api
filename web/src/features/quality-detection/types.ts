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
export type QualitySettings = {
  version: number
  enabled: boolean
  interval_minutes: number
  concurrency: number
  timeout_seconds: number
  max_output_tokens: number
  daily_call_limit: number
  daily_token_limit: number
  retention_days: number
  status_page_enabled: boolean
  public_groups: { name: string; alias: string }[]
  renderer_available: boolean
}

export type QualityProfile = {
  channel_id: number
  enabled: boolean
  model: string
  protocol: 'chat' | 'responses' | 'claude'
  prompt_version_id: number
  reasoning_effort: string
  timeout_seconds: number
  public_alias: string
  publish_sample: boolean
  version: number
}

export type QualityChannel = {
  id: number
  name: string
  groups: string[]
  models: string[]
  status: number
  supported: boolean
  unsupported_reason: string
  profile: QualityProfile
}

export type QualityPrompt = {
  id: number
  key: string
  version: number
  name: string
  content: string
  rule: 'visual' | 'interactive'
  archived: boolean
  created_at: number
}

export type QualityBatch = {
  id: number
  task_id: string
  trigger: 'manual' | 'scheduled'
  status: string
  total: number
  completed: number
  failed: number
  cancel_requested: boolean
  created_at: number
  completed_at: number
  summary: string
}

export type QualityRun = {
  id: number
  public_id: string
  batch_id: number
  channel_id: number
  channel_name: string
  model: string
  mapped_model: string
  prompt_version_id: number
  prompt_name: string
  status:
    | 'queued'
    | 'running'
    | 'completed'
    | 'failed'
    | 'cancelled'
    | 'interrupted'
    | 'skipped'
  quality_result:
    | 'not_evaluated'
    | 'rules_passed'
    | 'needs_review'
    | 'suspected_degradation'
  created_at: number
  started_at: number
  completed_at: number
  first_text_ms: number | null
  generation_ms: number | null
  input_tokens: number | null
  output_tokens: number | null
  error_code: string
  has_output: boolean
  has_html: boolean
  has_thumbnail: boolean
  render_status: string
  review_status: string
  review_note: string
  baseline_id: number
}

export type BatchPage = {
  items: QualityBatch[]
  total: number
  page: number
  page_size: number
}
export type BatchDetail = { batch: QualityBatch; runs: QualityRun[] }
export type BatchSelection = { channel_ids?: number[]; group_names?: string[] }
export type PromptInput = Pick<QualityPrompt, 'name' | 'content' | 'rule'> & {
  key?: string
}
