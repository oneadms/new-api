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
import { queryOptions } from '@tanstack/react-query'

import { qualityRequest } from '@/features/quality-detection/lib/request'

import type { StatusFilters, StatusGroupDetail, StatusPageData } from './types'

export const defaultStatusFilters: StatusFilters = {
  hours: 24,
  model: '',
  group: '',
  state: '',
  page: 1,
  page_size: 20,
  sort: 'name',
}

export function statusPageOptions(
  filters: StatusFilters = defaultStatusFilters
) {
  return queryOptions({
    queryKey: ['status-page', filters],
    queryFn: ({ signal }) =>
      qualityRequest<StatusPageData>({
        url: '/api/status-page',
        params: filters,
        signal,
      }),
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    retry: false,
    gcTime: 0,
  })
}

export function statusGroupOptions(id: string, hours: number, model: string) {
  return queryOptions({
    queryKey: ['status-page', 'group', id, hours, model],
    queryFn: ({ signal }) =>
      qualityRequest<StatusGroupDetail>({
        url: `/api/status-page/groups/${encodeURIComponent(id)}`,
        params: { hours, model },
        signal,
      }),
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    retry: false,
    gcTime: 0,
  })
}
