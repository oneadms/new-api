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
import { z } from 'zod'

const invalid = 'Invalid configuration value'
const positive = z
  .number({ error: invalid })
  .int(invalid)
  .min(1, invalid)
  .max(Number.MAX_SAFE_INTEGER, invalid)
export const settingsSchema = z.object({
  version: z.number().int().nonnegative(),
  enabled: z.boolean(),
  interval_minutes: z
    .number({ error: invalid })
    .int(invalid)
    .min(5, invalid)
    .max(10080, invalid),
  concurrency: z
    .number({ error: invalid })
    .int(invalid)
    .min(1, invalid)
    .max(10, invalid),
  timeout_seconds: z
    .number({ error: invalid })
    .int(invalid)
    .min(60, invalid)
    .max(1800, invalid),
  max_output_tokens: positive,
  daily_call_limit: positive,
  daily_token_limit: positive,
  retention_days: positive,
  status_page_enabled: z.boolean(),
  public_groups: z
    .array(
      z.object({
        name: z.string().trim().min(1, invalid),
        alias: z.string().trim().min(1, invalid),
      })
    )
    .refine(
      (groups) =>
        new Set(groups.map((group) => group.name)).size === groups.length,
      { message: 'Each group can only be published once' }
    ),
})
export type SettingsValues = z.infer<typeof settingsSchema>

export const profileSchema = z
  .object({
    channel_id: z.number().int().positive(),
    version: z.number().int().nonnegative(),
    enabled: z.boolean(),
    model: z.string().trim(),
    protocol: z.enum(['chat', 'responses', 'claude']),
    prompt_version_id: z
      .number({ error: invalid })
      .int(invalid)
      .nonnegative(invalid),
    reasoning_effort: z.string().trim(),
    timeout_seconds: z
      .number({ error: invalid })
      .int(invalid)
      .refine(
        (value) => value === 0 || (value >= 60 && value <= 1800),
        invalid
      ),
    public_alias: z.string().trim(),
    publish_sample: z.boolean(),
  })
  .superRefine((value, context) => {
    if (value.enabled && !value.model) {
      context.addIssue({ code: 'custom', path: ['model'], message: invalid })
    }
    if (value.enabled && !value.prompt_version_id) {
      context.addIssue({
        code: 'custom',
        path: ['prompt_version_id'],
        message: invalid,
      })
    }
    if (value.publish_sample && !value.public_alias) {
      context.addIssue({
        code: 'custom',
        path: ['public_alias'],
        message: invalid,
      })
    }
  })

export const promptSchema = z.object({
  key: z.string().optional(),
  name: z.string().trim().min(1, invalid),
  content: z
    .string()
    .min(1, invalid)
    .refine(
      (value) => new TextEncoder().encode(value).length <= 16000,
      'Prompt must be at most 16000 UTF-8 bytes'
    ),
  rule: z.enum(['visual', 'interactive']),
})
export const reviewSchema = z.object({
  result: z.enum(['accepted', 'rejected']),
  note: z.string().trim().min(1, invalid).max(4000, invalid),
})
