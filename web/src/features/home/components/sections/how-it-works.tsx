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

export function HowItWorks() {
  const { t } = useTranslation()
  const steps = [
    {
      title: t('Configure'),
      description: t(
        'Add your API keys, set up channels and configure access permissions'
      ),
      code: 'API_KEY=sk-••••••••',
    },
    {
      title: t('Connect'),
      description: t(
        'Connect through OpenAI, Claude, Gemini, and other compatible API routes'
      ),
      code: 'POST /v1/chat/completions',
    },
    {
      title: t('Monitor'),
      description: t(
        'Track usage, costs and performance with real-time analytics'
      ),
      code: 'requests → tokens → usage',
    },
  ]
  return (
    <section className='home-workflow home-section'>
      <div className='home-container'>
        <div className='home-section-heading'>
          <div>
            <p className='home-eyebrow'>{t('How It Works')}</p>
            <h2>{t('Three steps to get started')}</h2>
          </div>
        </div>
        <ol className='home-steps'>
          {steps.map((step, index) => (
            <li key={step.title}>
              <span className='home-step-number' aria-hidden='true'>
                0{index + 1}
              </span>
              <h3>{step.title}</h3>
              <p>{step.description}</p>
              <code>{step.code}</code>
            </li>
          ))}
        </ol>
      </div>
    </section>
  )
}
