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
import { t } from 'i18next'

import { api, type ApiRequestConfig } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'

export class QualityRequestError extends Error {
  constructor(
    public readonly status: number,
    message: string
  ) {
    super(message)
    this.name = 'QualityRequestError'
  }
}

// Keep transient errors local to this read-only view rather than redirecting
// away from an open sample when the global query cache sees an Axios 500.
export async function qualityRequest<T>(config: ApiRequestConfig): Promise<T> {
  try {
    const response = await api.request<{
      success: boolean
      message?: string
      data: T
    }>({
      ...config,
      skipErrorHandler: true,
      skipBusinessError: true,
    })
    if (!response.data.success) {
      throw new QualityRequestError(
        400,
        response.data.message || t('Request failed')
      )
    }
    return response.data.data
  } catch (error) {
    if (isCancel(error)) throw error
    handleServerError(error)
    if (error instanceof QualityRequestError) throw error
    const status = isAxiosError(error) ? (error.response?.status ?? 0) : 0
    const payload: unknown = isAxiosError(error)
      ? error.response?.data
      : undefined
    let message = t('Request failed')
    if (
      payload &&
      typeof payload === 'object' &&
      'message' in payload &&
      typeof payload.message === 'string'
    ) {
      message = payload.message
    }
    throw new QualityRequestError(status, message)
  }
}

export function isAccessError(error: unknown): boolean {
  return (
    error instanceof QualityRequestError &&
    [401, 403, 404, 410].includes(error.status)
  )
}
