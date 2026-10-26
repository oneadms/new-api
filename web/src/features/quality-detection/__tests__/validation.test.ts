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
import { describe, expect, it } from 'vitest'

import { profileSchema, promptSchema, settingsSchema } from '../lib/schemas'

const profile = {
  channel_id: 1,
  version: 2,
  enabled: true,
  model: 'model-a',
  protocol: 'chat',
  prompt_version_id: 1,
  reasoning_effort: '',
  timeout_seconds: 0,
  public_alias: 'Channel A',
  publish_sample: false,
}
const settings = {
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
}

describe('quality configuration validation', () => {
  it('accepts an inherited timeout and rejects overrides outside the supported interval', () => {
    expect(profileSchema.parse(profile).timeout_seconds).toBe(0)
    expect(
      profileSchema.safeParse({ ...profile, timeout_seconds: 59 }).success
    ).toBe(false)
    expect(
      profileSchema.safeParse({ ...profile, timeout_seconds: 1801 }).success
    ).toBe(false)
  })

  it('rejects duplicate publication of the same real group even with different aliases', () => {
    const result = settingsSchema.safeParse({
      ...settings,
      public_groups: [
        { name: 'standard', alias: 'Standard' },
        { name: 'standard', alias: 'Premium' },
      ],
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe(
        'Each group can only be published once'
      )
    }
  })

  it('measures the prompt limit in UTF-8 bytes instead of character count', () => {
    const prompt = {
      name: 'Clock',
      rule: 'visual',
      content: `${'钟'.repeat(5333)}a`,
    }
    expect(promptSchema.safeParse(prompt).success).toBe(true)
    const result = promptSchema.safeParse({
      ...prompt,
      content: `${prompt.content}a`,
    })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe(
        'Prompt must be at most 16000 UTF-8 bytes'
      )
    }
  })
})
