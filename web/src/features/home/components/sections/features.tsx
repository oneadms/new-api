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
import {
  ArrowRight02Icon,
  CodeIcon,
  Shield01Icon,
  ChartLineData01Icon,
  Route01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

export function Features() {
  const { t } = useTranslation()
  return (
    <section className='home-features home-section' id='capabilities'>
      <div className='home-container'>
        <div className='home-section-heading'>
          <div>
            <p className='home-eyebrow'>{t('Core Features')}</p>
            <h2>
              {t('Less complexity.')}
              <br />
              {t('More possibility.')}
            </h2>
          </div>
          <p>
            {t(
              'The infrastructure stays in one place. Your ideas can go anywhere.'
            )}
          </p>
        </div>
        <div className='home-feature-grid'>
          <article className='home-feature home-feature-routing'>
            <div className='home-feature-icon'>
              <HugeiconsIcon icon={Route01Icon} size={24} aria-hidden='true' />
            </div>
            <h3>{t('One gateway. Your choice of models.')}</h3>
            <p>
              {t(
                'Connect through OpenAI, Claude, Gemini, and other compatible API routes'
              )}
            </p>
            <div className='home-routing-map' aria-hidden='true'>
              <div>
                <span>OpenAI</span>
                <span>Claude</span>
                <span>Gemini</span>
              </div>
              <svg viewBox='0 0 260 80' fill='none'>
                <path
                  d='M36 0v20q0 20 20 20h54q20 0 20 20v20M130 0v80M224 0v20q0 20-20 20h-54q-20 0-20 20v20'
                  stroke='currentColor'
                  strokeWidth='1.5'
                />
              </svg>
              <strong>/v1</strong>
            </div>
            <Link to='/pricing' className='home-feature-link'>
              {t('Explore models')}
              <HugeiconsIcon
                icon={ArrowRight02Icon}
                size={18}
                aria-hidden='true'
              />
            </Link>
          </article>
          <article className='home-feature'>
            <div className='home-feature-icon'>
              <HugeiconsIcon icon={Shield01Icon} size={24} aria-hidden='true' />
            </div>
            <h3>{t('Access on your terms.')}</h3>
            <p>
              {t('Multi-user management with flexible permission allocation')}
            </p>
            <div className='home-access-demo' aria-hidden='true'>
              <span>
                {t('API Key')}
                <code>sk-••••••••</code>
              </span>
              <span>
                {t('Rate Limiting')}
                <i />
              </span>
              <span>
                {t('Access Control')}
                <i />
              </span>
            </div>
          </article>
          <article className='home-feature'>
            <div className='home-feature-icon'>
              <HugeiconsIcon
                icon={ChartLineData01Icon}
                size={24}
                aria-hidden='true'
              />
            </div>
            <h3>{t('See where every token goes.')}</h3>
            <p>
              {t('Track usage, costs and performance with real-time analytics')}
            </p>
            <div className='home-usage-demo' aria-hidden='true'>
              <span>{t('Cost Tracking')}</span>
              <svg viewBox='0 0 260 80' fill='none'>
                <path d='M0 66H260M0 36H260M0 6H260' stroke='var(--border)' />
                <path
                  d='M0 65L22 61 43 66 65 42 86 48 108 29 130 35 152 14 173 30 195 18 216 24 238 6 260 12'
                  stroke='currentColor'
                  strokeWidth='2.5'
                  strokeLinejoin='round'
                />
              </svg>
            </div>
          </article>
        </div>
        <div className='home-open-source'>
          <HugeiconsIcon icon={CodeIcon} size={24} aria-hidden='true' />
          <div>
            <h3>{t('Open Source')}</h3>
            <p>{t('Community driven, self-hosted, and extensible')}</p>
          </div>
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noopener noreferrer'
          >
            GitHub <span aria-hidden='true'>↗</span>
          </a>
        </div>
      </div>
    </section>
  )
}
