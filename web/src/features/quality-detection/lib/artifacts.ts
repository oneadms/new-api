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

import { QualityRequestError } from './request'

export function publicThumbnailPath(id: string): string {
  return `/api/status-page/artifacts/${encodeURIComponent(id)}/thumbnail`
}

export async function loadThumbnail(
  path: string,
  signal?: AbortSignal
): Promise<Blob> {
  if (
    !/^\/api\/(?:status-page\/artifacts\/[a-zA-Z0-9_-]+|quality-detection\/runs\/\d+)\/thumbnail$/.test(
      path
    )
  ) {
    throw new Error('Invalid artifact path')
  }
  try {
    const response = await api.get<Blob>(path, {
      responseType: 'blob',
      signal,
      // React Query owns sharing and cancellation. The generic GET cache can
      // otherwise reuse an already-aborted request when the image remounts.
      disableDuplicate: true,
      skipErrorHandler: true,
    })
    if (response.data.type.split(';')[0] !== 'image/png') {
      throw new Error('Invalid artifact type')
    }
    return response.data
  } catch (error) {
    if (isCancel(error)) throw error
    handleServerError(error)
    throw new QualityRequestError(
      isAxiosError(error) ? (error.response?.status ?? 0) : 0,
      'Thumbnail unavailable'
    )
  }
}
