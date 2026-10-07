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
import type { MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { TopNavLink } from '../types'

interface PublicNavLinkProps {
  link: TopNavLink
  pathname: string
  onNavigate: (event: MouseEvent<HTMLAnchorElement>, link: TopNavLink) => void
}

export function PublicNavLink(props: PublicNavLinkProps) {
  const { t } = useTranslation()
  const className = cn(
    'public-nav-link',
    props.link.disabled && 'pointer-events-none opacity-50'
  )
  if (props.link.external) {
    return (
      <a
        href={props.link.href}
        target='_blank'
        rel='noopener noreferrer'
        aria-disabled={props.link.disabled}
        tabIndex={props.link.disabled ? -1 : undefined}
        onClick={(event) => props.onNavigate(event, props.link)}
        className={className}
      >
        {t(props.link.title)}
      </a>
    )
  }
  return (
    <Link
      to={props.link.href}
      disabled={props.link.disabled}
      aria-current={props.pathname === props.link.href ? 'page' : undefined}
      onClick={(event) => props.onNavigate(event, props.link)}
      className={className}
    >
      {t(props.link.title)}
    </Link>
  )
}
