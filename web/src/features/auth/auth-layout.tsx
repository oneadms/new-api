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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { GatewayVisual } from '@/components/gateway-visual'
import { LanguageSwitcher } from '@/components/language-switcher'
import { ThemeSwitch } from '@/components/theme-switch'
import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = { children: React.ReactNode }

export function AuthLayout(props: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()
  return (
    <div className='auth-shell'>
      <header className='auth-header'>
        <Link to='/' className='auth-brand'>
          {loading ? (
            <Skeleton className='size-8 rounded-lg' />
          ) : (
            <img src={logo} alt='' className='size-8 object-contain' />
          )}
          {loading ? (
            <Skeleton className='h-6 w-24' />
          ) : (
            <span>{systemName}</span>
          )}
        </Link>
        <div className='flex items-center gap-1'>
          <LanguageSwitcher />
          <ThemeSwitch />
        </div>
      </header>
      <aside className='auth-story'>
        <div className='auth-story-heading'>
          <p className='home-eyebrow'>
            {t('AI Application Infrastructure Foundation')}
          </p>
          <h2>
            {t('Every model.')}
            <br />
            <span>{t('One connection.')}</span>
          </h2>
          <p>
            {t(
              'The infrastructure stays in one place. Your ideas can go anywhere.'
            )}
          </p>
        </div>
        <GatewayVisual name={systemName} />
        <div className='auth-story-footer'>
          <span>OpenAI · Claude · Gemini · DeepSeek</span>
          <span>{t('Multi-protocol Compatible')}</span>
        </div>
      </aside>
      <main id='content' className='auth-form-panel'>
        <div className='auth-form-content'>{props.children}</div>
        <Link to='/' className='auth-back-link'>
          ← {t('Back to Home')}
        </Link>
      </main>
    </div>
  )
}
