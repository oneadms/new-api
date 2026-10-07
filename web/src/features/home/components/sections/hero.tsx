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
  ArrowUpRight01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

import { GatewayExperience } from '../gateway-experience'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const canRegister =
    !status?.self_use_mode_enabled && status?.register_enabled !== false
  let actionUrl: '/dashboard' | '/sign-up' | '/sign-in' = '/sign-in'
  let actionLabel = t('Sign in')
  if (props.isAuthenticated) {
    actionUrl = '/dashboard'
    actionLabel = t('Go to Dashboard')
  } else if (canRegister) {
    actionUrl = '/sign-up'
    actionLabel = t('Get Started')
  }

  return (
    <section className={cn('home-hero', props.className)}>
      <div className='home-container home-hero-grid'>
        <div className='home-hero-copy'>
          <p className='home-eyebrow'>
            <span className='home-signal-dot' />
            {t('AI Application Infrastructure Foundation')}
          </p>
          <h1>
            {t('Every model.')}
            <br />
            <span>{t('One connection.')}</span>
          </h1>
          <p className='home-hero-description'>
            {t(
              'Bring your models, keys, and usage together. One gateway, from your first request to your next big idea.'
            )}
          </p>
          <div className='home-hero-actions'>
            <Button size='lg' render={<Link to={actionUrl} />}>
              {actionLabel}
              <HugeiconsIcon icon={ArrowRight02Icon} data-icon='inline-end' />
            </Button>
            <Button size='lg' variant='outline' render={<Link to='/pricing' />}>
              {t('Explore models')}
            </Button>
          </div>
          <a
            className='home-docs-link'
            href={docsUrl}
            target={docsUrl.startsWith('http') ? '_blank' : undefined}
            rel='noopener noreferrer'
          >
            {t('Read the documentation')}
            <HugeiconsIcon
              icon={ArrowUpRight01Icon}
              size={16}
              aria-hidden='true'
            />
          </a>
          <div className='home-compatible'>
            <p>{t('Supported Applications')}</p>
            <div className='flex flex-wrap items-center gap-x-5 gap-y-2'>
              <a
                href='https://cherry-ai.com'
                target='_blank'
                rel='noopener noreferrer'
              >
                Cherry Studio <span aria-hidden='true'>↗</span>
              </a>
              <a
                href='https://ccswitch.io'
                target='_blank'
                rel='noopener noreferrer'
              >
                CC Switch <span aria-hidden='true'>↗</span>
              </a>
              <span>{t('More Apps')}</span>
            </div>
            <p className='home-compatible-note'>
              {t(
                'Supports one-click configuration and perfectly adapts to NewAPI multi-protocol configuration.'
              )}
            </p>
          </div>
        </div>
        <GatewayExperience />
      </div>
    </section>
  )
}
