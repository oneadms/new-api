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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { useDashboardContentVisibility } from '@/features/dashboard/hooks/use-status-data'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useSidebarView } from '../use-sidebar-view'
import { useTopNavLinks } from '../use-top-nav-links'

function NavigationHarness() {
  const links = useTopNavLinks()
  const sidebar = useSidebarView()
  const panels = useDashboardContentVisibility()
  return (
    <>
      <nav aria-label='Top'>
        {links.map((link) => (
          <a key={link.href} href={link.href}>
            {link.title}
          </a>
        ))}
      </nav>
      <nav aria-label='Sidebar'>
        {sidebar.navGroups
          .flatMap((group) => group.items)
          .map((item) =>
            'url' in item ? (
              <a key={item.title} href={item.url}>
                {item.title}
              </a>
            ) : null
          )}
      </nav>
      {panels.statusPage && <section aria-label='Native status summary' />}
    </>
  )
}

async function renderNavigation(status: Record<string, unknown>) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  client.setQueryData(['status'], status)
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    keySeparator: false,
    resources: { en: { translation: {} } },
  })
  const root = createRootRoute({ component: NavigationHarness })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  const view = render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return { ...view, client }
}

beforeEach(() => useAuthStore.getState().auth.reset())
afterEach(() => useAuthStore.getState().auth.reset())

describe('native status integration', () => {
  it('uses the native flag for both navigation and dashboard visibility, ignoring the old flag', async () => {
    const view = await renderNavigation({ uptime_kuma_enabled: true })
    expect(
      within(screen.getByRole('navigation', { name: 'Top' })).queryByRole(
        'link',
        { name: 'Status' }
      )
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('region', { name: 'Native status summary' })
    ).not.toBeInTheDocument()
    act(() => {
      view.client.setQueryData(['status'], {
        status_page_enabled: true,
        uptime_kuma_enabled: false,
      })
    })
    expect(await screen.findByRole('link', { name: 'Status' })).toHaveAttribute(
      'href',
      '/status'
    )
    expect(
      screen.getByRole('region', { name: 'Native status summary' })
    ).toBeInTheDocument()
    act(() => {
      view.client.setQueryData(['status'], { status_page_enabled: false })
    })
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Status' })
      ).not.toBeInTheDocument()
    )
    view.unmount()
    view.client.clear()
  })

  it('shows quality detection only to administrators and honors the sidebar module switch', async () => {
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'user', role: ROLE.USER })
    const view = await renderNavigation({})
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Quality detection' })
      ).not.toBeInTheDocument()
    )
    act(() =>
      useAuthStore
        .getState()
        .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
    )
    expect(
      await screen.findByRole('link', { name: 'Quality detection' })
    ).toHaveAttribute('href', '/quality-detection')
    act(() => {
      view.client.setQueryData(['status'], {
        SidebarModulesAdmin: JSON.stringify({
          admin: { enabled: true, quality_detection: false },
        }),
      })
    })
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Quality detection' })
      ).not.toBeInTheDocument()
    )
    expect(screen.getByRole('link', { name: 'Channels' })).toBeInTheDocument()
    view.unmount()
    view.client.clear()
  })
})
