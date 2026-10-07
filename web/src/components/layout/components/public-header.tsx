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
import { Menu01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { LanguageSwitcher } from '@/components/language-switcher'
import { NotificationPopover } from '@/components/notification-popover'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { useNotifications } from '@/hooks/use-notifications'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { defaultTopNavLinks } from '../config/top-nav.config'
import type { TopNavLink } from '../types'
import { HeaderLogo } from './header-logo'
import { PublicNavLink } from './public-nav-link'

const AUTH_PROMPT_SECONDS = 5
type AuthPromptTarget = { title: string; href: string }

export interface PublicHeaderProps {
  navLinks?: TopNavLink[]
  mobileLinks?: TopNavLink[]
  navContent?: React.ReactNode
  showThemeSwitch?: boolean
  showLanguageSwitcher?: boolean
  logo?: React.ReactNode
  siteName?: string
  homeUrl?: string
  leftContent?: React.ReactNode
  rightContent?: React.ReactNode
  showNavigation?: boolean
  showAuthButtons?: boolean
  showNotifications?: boolean
  className?: string
}

export function PublicHeader(props: PublicHeaderProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })
  const [mobileOpen, setMobileOpen] = useState(false)
  const [authPromptTarget, setAuthPromptTarget] =
    useState<AuthPromptTarget | null>(null)
  const [authPromptSecondsLeft, setAuthPromptSecondsLeft] =
    useState(AUTH_PROMPT_SECONDS)
  const isAuthenticated = useAuthStore((state) => !!state.auth.user)
  const { systemName, logo, loading, logoLoaded } = useSystemConfig()
  const dynamicLinks = useTopNavLinks()
  const notifications = useNotifications()
  const links =
    dynamicLinks.length > 0
      ? dynamicLinks
      : (props.navLinks ?? defaultTopNavLinks)
  const mobileLinks = props.mobileLinks ?? links
  const displayName = props.siteName || systemName

  // 展开菜单交给 Sheet 管理焦点与滚动；切换到桌面布局时解除模态状态。
  useEffect(() => {
    const desktop = window.matchMedia('(min-width: 1024px)')
    const closeOnDesktop = () => {
      if (desktop.matches) setMobileOpen(false)
    }
    desktop.addEventListener('change', closeOnDesktop)
    return () => desktop.removeEventListener('change', closeOnDesktop)
  }, [])

  useEffect(() => {
    if (!authPromptTarget) return
    const intervalId = window.setInterval(() => {
      setAuthPromptSecondsLeft((seconds) => Math.max(seconds - 1, 0))
    }, 1000)
    const timeoutId = window.setTimeout(() => {
      setAuthPromptTarget(null)
      void navigate({
        to: '/sign-in',
        search: { redirect: authPromptTarget.href },
      })
    }, AUTH_PROMPT_SECONDS * 1000)
    return () => {
      window.clearInterval(intervalId)
      window.clearTimeout(timeoutId)
    }
  }, [authPromptTarget, navigate])

  const closeAuthPrompt = useCallback(() => {
    setAuthPromptTarget(null)
    setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
  }, [])

  const handleNavLinkClick = useCallback(
    (event: React.MouseEvent<HTMLAnchorElement>, link: TopNavLink) => {
      if (link.disabled) {
        event.preventDefault()
        return
      }
      setMobileOpen(false)
      if (link.requiresAuth) {
        event.preventDefault()
        setAuthPromptSecondsLeft(AUTH_PROMPT_SECONDS)
        setAuthPromptTarget({ title: t(link.title), href: link.href })
      }
    },
    [t]
  )

  let authControl = (
    <Button render={<Link to='/sign-in' />}>{t('Sign in')}</Button>
  )
  if (loading) {
    authControl = <Skeleton className='h-9 w-20 rounded-lg' />
  } else if (isAuthenticated) {
    authControl = <ProfileDropdown />
  }

  return (
    <>
      <a
        href='#public-content'
        className='bg-primary text-primary-foreground fixed top-2 left-2 z-50 -translate-y-20 rounded-lg px-4 py-3 text-sm focus:translate-y-0'
      >
        {t('Skip to Main')}
      </a>
      <header
        className={cn(
          'public-header fixed inset-x-0 top-0 z-40',
          props.className
        )}
      >
        <div className='public-header-inner'>
          <Link to={props.homeUrl || '/'} className='public-brand'>
            <div className='flex size-8 shrink-0 items-center justify-center'>
              {props.logo ?? (
                <HeaderLogo
                  src={logo}
                  loading={loading}
                  logoLoaded={logoLoaded}
                  className='size-full object-contain'
                />
              )}
            </div>
            {loading ? (
              <Skeleton className='h-5 w-20' />
            ) : (
              <span>{displayName}</span>
            )}
          </Link>
          {props.leftContent}
          <div className='hidden items-center gap-3 lg:flex'>
            {props.showNavigation !== false && (
              <nav
                aria-label={t('Main navigation')}
                className='flex items-center gap-0.5'
              >
                {props.navContent ??
                  links.map((link) => (
                    <PublicNavLink
                      key={link.href}
                      link={link}
                      pathname={pathname}
                      onNavigate={handleNavLinkClick}
                    />
                  ))}
              </nav>
            )}
            {props.rightContent ?? (
              <div className='flex items-center gap-1'>
                {props.showLanguageSwitcher !== false && <LanguageSwitcher />}
                {props.showThemeSwitch !== false && <ThemeSwitch />}
                {props.showNotifications !== false && (
                  <NotificationPopover
                    open={notifications.popoverOpen}
                    onOpenChange={notifications.setPopoverOpen}
                    unreadCount={notifications.unreadCount}
                    activeTab={notifications.activeTab}
                    onTabChange={notifications.setActiveTab}
                    notice={notifications.notice}
                    announcements={notifications.announcements}
                    loading={notifications.loading}
                  />
                )}
                {props.showAuthButtons !== false && (
                  <div className='ml-2'>{authControl}</div>
                )}
              </div>
            )}
          </div>
          <div className='flex items-center gap-1 lg:hidden'>
            {props.showThemeSwitch !== false && <ThemeSwitch />}
            {props.showAuthButtons !== false && !loading && isAuthenticated && (
              <ProfileDropdown />
            )}
            <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
              <SheetTrigger
                render={
                  <Button
                    variant='ghost'
                    size='icon'
                    aria-label={t('Toggle navigation menu')}
                  />
                }
              >
                <HugeiconsIcon icon={Menu01Icon} />
              </SheetTrigger>
              <SheetContent className='w-[min(90vw,380px)]' showCloseButton>
                <SheetHeader className='border-b px-6 py-6'>
                  <SheetTitle>{displayName}</SheetTitle>
                </SheetHeader>
                {props.showNavigation !== false && (
                  <nav
                    aria-label={t('Main navigation')}
                    className='public-mobile-nav flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-4'
                  >
                    {mobileLinks.map((link) => (
                      <PublicNavLink
                        key={link.href}
                        link={link}
                        pathname={pathname}
                        onNavigate={handleNavLinkClick}
                      />
                    ))}
                  </nav>
                )}
                <div className='mt-auto flex flex-wrap items-center justify-between gap-3 border-t p-6'>
                  {props.showLanguageSwitcher !== false && <LanguageSwitcher />}
                  {props.showAuthButtons !== false && (
                    <Button
                      render={
                        <Link
                          to={isAuthenticated ? '/dashboard' : '/sign-in'}
                          onClick={() => setMobileOpen(false)}
                        />
                      }
                    >
                      {isAuthenticated ? t('Go to Dashboard') : t('Sign in')}
                    </Button>
                  )}
                </div>
              </SheetContent>
            </Sheet>
          </div>
        </div>
      </header>
      <Dialog
        open={!!authPromptTarget}
        onOpenChange={(open) => {
          if (!open) closeAuthPrompt()
        }}
        title={t('Sign in required')}
        description={t('Please sign in to view {{module}}.', {
          module: authPromptTarget?.title || '',
        })}
        contentClassName='sm:max-w-md'
        contentHeight='auto'
        footer={
          <>
            <Button variant='outline' onClick={closeAuthPrompt}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={() => {
                const redirect = authPromptTarget?.href || '/'
                setAuthPromptTarget(null)
                void navigate({ to: '/sign-in', search: { redirect } })
              }}
            >
              {t('Sign in now')}
            </Button>
          </>
        }
      >
        <div className='bg-muted/40 text-muted-foreground rounded-lg px-3 py-2 text-sm'>
          {t('Redirecting to sign in in {{seconds}} seconds.', {
            seconds: authPromptSecondsLeft,
          })}
        </div>
      </Dialog>
    </>
  )
}
