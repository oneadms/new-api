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
import { isAxiosError, isCancel } from 'axios'

import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'

import { QualityRequestError, qualityRequest } from './lib/request'
import type {
  BatchDetail,
  BatchPage,
  BatchSelection,
  PromptInput,
  QualityBatch,
  QualityChannel,
  QualityProfile,
  QualityPrompt,
  QualityRun,
  QualitySettings,
} from './types'

const root = '/api/quality-detection'

export const getSettings = (signal?: AbortSignal): Promise<QualitySettings> =>
  qualityRequest({ url: `${root}/settings`, signal })
export const saveSettings = (
  settings: Omit<QualitySettings, 'renderer_available'>
): Promise<QualitySettings> =>
  qualityRequest({ method: 'PUT', url: `${root}/settings`, data: settings })
export const getChannels = (signal?: AbortSignal): Promise<QualityChannel[]> =>
  qualityRequest({ url: `${root}/channels`, signal })
export const saveProfile = (profile: QualityProfile): Promise<QualityProfile> =>
  qualityRequest({
    method: 'PUT',
    url: `${root}/channels/${profile.channel_id}`,
    data: profile,
  })
export const getPrompts = (signal?: AbortSignal): Promise<QualityPrompt[]> =>
  qualityRequest({ url: `${root}/prompts`, signal })
export const createPrompt = (prompt: PromptInput): Promise<QualityPrompt> =>
  qualityRequest({ method: 'POST', url: `${root}/prompts`, data: prompt })
export const archivePrompt = (id: number, archived: boolean): Promise<null> =>
  qualityRequest({
    method: 'POST',
    url: `${root}/prompts/${id}/archive`,
    data: { archived },
  })
export const createBatch = (
  selection: BatchSelection,
  key: string
): Promise<QualityBatch> =>
  qualityRequest({
    method: 'POST',
    url: `${root}/batches`,
    data: selection,
    headers: { 'Idempotency-Key': key },
  })
export const getBatches = (
  page: number,
  signal?: AbortSignal
): Promise<BatchPage> =>
  qualityRequest({
    url: `${root}/batches`,
    params: { page, page_size: 20 },
    signal,
  })
export const getBatch = (
  id: number,
  signal?: AbortSignal
): Promise<BatchDetail> =>
  qualityRequest({ url: `${root}/batches/${id}`, signal })
export const cancelBatch = (id: number): Promise<null> =>
  qualityRequest({ method: 'POST', url: `${root}/batches/${id}/cancel` })
export const getRun = (id: number, signal?: AbortSignal): Promise<QualityRun> =>
  qualityRequest({ url: `${root}/runs/${id}`, signal })
export const reviewRun = (
  id: number,
  review: { result: 'accepted' | 'rejected'; note: string }
): Promise<null> =>
  qualityRequest({
    method: 'POST',
    url: `${root}/runs/${id}/review`,
    data: review,
  })
export const setBaseline = (id: number): Promise<null> =>
  qualityRequest({ method: 'POST', url: `${root}/runs/${id}/baseline` })

export async function getOutput(
  id: number,
  kind: 'raw' | 'html',
  signal?: AbortSignal
): Promise<string> {
  try {
    const response = await api.get<string>(`${root}/runs/${id}/output`, {
      params: { kind },
      responseType: 'text',
      signal,
      skipErrorHandler: true,
    })
    return response.data
  } catch (error) {
    if (isCancel(error)) throw error
    handleServerError(error)
    throw new QualityRequestError(
      isAxiosError(error) ? (error.response?.status ?? 0) : 0,
      'Request failed'
    )
  }
}
