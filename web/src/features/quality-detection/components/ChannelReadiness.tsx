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
import { useTranslation } from 'react-i18next'

import type { QualityChannel } from '../types'

export function ChannelReadiness(props: { channel: QualityChannel }) {
  const { t } = useTranslation()
  let message = t('Configuration required')
  switch (props.channel.unsupported_reason) {
    case 'executor_unavailable':
      message = t('The detection executor is unavailable.')
      break
    case 'unsupported_protocol':
      message = t(
        'This channel does not support the selected detection protocol.'
      )
      break
    case 'channel_disabled':
      message = t('Enable this channel in channel management first.')
      break
    case 'model_unavailable':
    case 'unsupported_model':
      message = t('Choose a model supported by this channel')
      break
    default:
      if (!props.channel.supported) message = t('Unsupported detection target')
      else if (props.channel.status !== 1) {
        message = t('Enable this channel in channel management first.')
      } else if (
        props.channel.profile.model &&
        props.channel.profile.prompt_version_id > 0 &&
        !props.channel.profile.enabled
      ) {
        message = t('Detection paused')
      } else if (
        props.channel.profile.enabled &&
        props.channel.profile.model &&
        props.channel.profile.prompt_version_id > 0
      ) {
        message = t('Choose an active prompt version before running detection.')
      }
  }
  return (
    <p className='text-muted-foreground text-xs leading-relaxed'>{message}</p>
  )
}
