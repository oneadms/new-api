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
import { ArrowRight02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

interface CTAProps {
  isAuthenticated?: boolean
}

export function CTA(props: CTAProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const canRegister =
    !status?.self_use_mode_enabled && status?.register_enabled !== false
  if (props.isAuthenticated) return null
  return (
    <section className='home-cta home-container'>
      <div className='home-cta-orbits' aria-hidden='true'>
        <i />
        <i />
        <i />
      </div>
      <div className='relative'>
        <p className='home-eyebrow'>{t('Your next idea starts here.')}</p>
        <h2>
          {t('Ready to simplify')}
          <br />
          {t('your AI integration?')}
        </h2>
        <p>
          {t(
            'Deploy your own gateway and start routing requests through your configured upstream services.'
          )}
        </p>
        <Button
          size='lg'
          render={<Link to={canRegister ? '/sign-up' : '/sign-in'} />}
        >
          {canRegister ? t('Get Started') : t('Sign in')}
          <HugeiconsIcon icon={ArrowRight02Icon} data-icon='inline-end' />
        </Button>
      </div>
    </section>
  )
}
