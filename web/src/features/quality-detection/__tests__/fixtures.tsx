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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'

import type {
  QualityChannel,
  QualityPrompt,
  QualityRun,
  QualitySettings,
} from '../types'

export function renderQuality(children: ReactNode) {
  const i18n = createInstance()
  void i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    keySeparator: false,
    resources: { en: { translation: {} } },
    initAsync: false,
  })
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
  return {
    ...render(
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      </I18nextProvider>
    ),
    client,
    i18n,
  }
}
export const channel: QualityChannel = {
  id: 1,
  name: 'Channel A',
  groups: ['standard', 'premium'],
  models: ['model-a'],
  status: 1,
  supported: true,
  unsupported_reason: '',
  profile: {
    channel_id: 1,
    enabled: true,
    model: 'model-a',
    protocol: 'chat',
    prompt_version_id: 1,
    reasoning_effort: '',
    timeout_seconds: 0,
    public_alias: 'Public A',
    publish_sample: false,
    version: 1,
  },
}
export const settings: QualitySettings = {
  version: 2,
  enabled: false,
  interval_minutes: 60,
  concurrency: 3,
  timeout_seconds: 600,
  max_output_tokens: 8192,
  daily_call_limit: 500,
  daily_token_limit: 5000000,
  retention_days: 14,
  status_page_enabled: false,
  public_groups: [],
  renderer_available: false,
}
export const prompt: QualityPrompt = {
  id: 1,
  key: 'visual',
  version: 1,
  name: 'Visual task',
  content: 'Create a self-contained page.',
  rule: 'visual',
  archived: false,
  created_at: 100,
}

export const run: QualityRun = {
  id: 4,
  public_id: 'run-public',
  batch_id: 2,
  channel_id: 1,
  channel_name: 'Channel A',
  model: 'model-a',
  mapped_model: 'model-a',
  prompt_version_id: 1,
  prompt_name: 'Visual task',
  status: 'completed',
  quality_result: 'needs_review',
  created_at: 100,
  started_at: 101,
  completed_at: 110,
  first_text_ms: 12,
  generation_ms: 9000,
  input_tokens: null,
  output_tokens: null,
  error_code: '',
  has_output: true,
  has_html: true,
  has_thumbnail: false,
  render_status: 'unavailable',
  review_status: '',
  review_note: '',
  baseline_id: 0,
}
