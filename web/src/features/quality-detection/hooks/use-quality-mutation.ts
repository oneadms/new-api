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
import { useMutation, useQueryClient } from '@tanstack/react-query'

export function useQualityMutation<T = void>(
  mutationFn: (variables: T) => Promise<unknown>,
  onSuccess?: () => void
) {
  const client = useQueryClient()
  return useMutation({
    mutationFn,
    retry: false,
    onError: () => {
      /* Requests report errors through handleServerError. */
    },
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: ['quality-detection'] }),
        client.invalidateQueries({ queryKey: ['status-page'] }),
        client.invalidateQueries({ queryKey: ['status'] }),
      ])
      onSuccess?.()
    },
  })
}
