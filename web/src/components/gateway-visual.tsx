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

import { cn } from '@/lib/utils'

interface GatewayVisualProps {
  className?: string
  name?: string
}

/** 用汇流轨道表达统一网关；纯装饰层不进入键盘和读屏顺序。 */
export function GatewayVisual(props: GatewayVisualProps) {
  const { t } = useTranslation()

  return (
    <div className={cn('gateway-visual', props.className)} aria-hidden='true'>
      <div className='gateway-orbit gateway-orbit-outer' />
      <div className='gateway-orbit gateway-orbit-inner' />
      <div className='gateway-coordinate gateway-coordinate-top'>API / ∞</div>
      <div className='gateway-coordinate gateway-coordinate-bottom'>
        {t('One connection')}
      </div>
      <div className='gateway-stack'>
        <div className='gateway-plate gateway-plate-bottom' />
        <div className='gateway-plate gateway-plate-middle' />
        <div className='gateway-plate gateway-plate-top'>
          <svg viewBox='0 0 120 120' fill='none'>
            <path
              d='M30 84V36l60 48V36'
              stroke='currentColor'
              strokeWidth='12'
              strokeLinecap='round'
              strokeLinejoin='round'
            />
            <circle cx='30' cy='36' r='6' fill='currentColor' />
            <circle cx='90' cy='84' r='6' fill='currentColor' />
          </svg>
        </div>
      </div>
      <span className='gateway-node gateway-node-openai'>OpenAI</span>
      <span className='gateway-node gateway-node-claude'>Claude</span>
      <span className='gateway-node gateway-node-gemini'>Gemini</span>
      <span className='gateway-node gateway-node-deepseek'>DeepSeek</span>
      <div className='gateway-name'>{props.name || 'New API'}</div>
    </div>
  )
}
