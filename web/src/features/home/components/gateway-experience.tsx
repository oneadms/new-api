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
import { Copy01Icon, Tick02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { GatewayVisual } from '@/components/gateway-visual'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useSystemConfig } from '@/hooks/use-system-config'

const EXAMPLES = [
  { provider: 'OpenAI', model: 'gpt-4.1' },
  { provider: 'Claude', model: 'claude-sonnet-4' },
  { provider: 'Gemini', model: 'gemini-2.5-pro' },
] as const

export function GatewayExperience() {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()
  const [selected, setSelected] = useState(0)
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const example = EXAMPLES[selected]
  const request = [
    'const response = await client.chat.completions.create({',
    `  model: '${example.model}',`,
    "  messages: [{ role: 'user', content: 'Hello!' }],",
    '})',
  ].join('\n')

  return (
    <div className='gateway-experience'>
      <GatewayVisual name={systemName} />
      <div className='gateway-console'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div
            className='gateway-models'
            role='group'
            aria-label={t('Choose a model')}
          >
            {EXAMPLES.map((item, index) => (
              <button
                key={item.provider}
                type='button'
                aria-pressed={selected === index}
                onClick={() => setSelected(index)}
              >
                {item.provider}
              </button>
            ))}
          </div>
          <Button
            variant='ghost'
            size='icon'
            aria-label={t('Copy example')}
            onClick={() => void copyToClipboard(request)}
          >
            <HugeiconsIcon
              icon={copiedText === request ? Tick02Icon : Copy01Icon}
            />
          </Button>
        </div>
        <pre tabIndex={0} aria-label={t('Request example')}>
          <code>
            <span className='gateway-code-muted'>
              client.chat.completions.create({'{'}
            </span>
            {'\n'}
            {'  '}model:{' '}
            <span className='gateway-code-model'>'{example.model}'</span>,{'\n'}
            {'  '}messages: [{'{'} role:{' '}
            <span className='gateway-code-string'>'user'</span>, … {'}'}]{'\n'}
            <span className='gateway-code-muted'>{'}'})</span>
          </code>
        </pre>
        <p className='gateway-console-caption'>
          {t('Interactive example · no request is sent')}
        </p>
      </div>
    </div>
  )
}
