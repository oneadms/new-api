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
import type { QualityBatch, QualityChannel } from '../types'

export function eligibleChannels(
  channels: QualityChannel[],
  group = ''
): QualityChannel[] {
  const unique = new Map<number, QualityChannel>()
  for (const channel of channels) {
    if (
      channel.supported &&
      channel.status === 1 &&
      channel.profile.enabled &&
      channel.profile.model !== '' &&
      channel.models.includes(channel.profile.model) &&
      channel.profile.prompt_version_id > 0 &&
      (!group || channel.groups.includes(group))
    ) {
      unique.set(channel.id, channel)
    }
  }
  return [...unique.values()]
}

export function isBatchActive(batch: QualityBatch): boolean {
  return ['queued', 'pending', 'running'].includes(batch.status)
}
