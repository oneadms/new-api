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
  createRootRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { PublicHeader } from '../components/public-header'

async function renderHeader() {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(['status'], { docs_link: 'https://docs.newapi.pro' })
  queryClient.setQueryData(['notice'], { success: true, data: '' })
  const root = createRootRoute({
    component: () => (
      <PublicHeader
        showThemeSwitch={false}
        showLanguageSwitcher={false}
        showNotifications={false}
      />
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  const result = render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </I18nextProvider>
  )
  return { ...result, queryClient }
}

beforeEach(() => {
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.setState({
    ...useSystemConfigStore.getInitialState(),
    loading: false,
  })
})

describe('public mobile navigation', () => {
  it('keeps the closed menu out of the accessible navigation tree', async () => {
    const { queryClient } = await renderHeader()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Home' })).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: 'Toggle navigation menu' })
    ).toHaveAttribute('aria-expanded', 'false')
    queryClient.clear()
  })

  it('opens an accessible menu and returns focus to its trigger after Escape', async () => {
    const user = userEvent.setup()
    const { queryClient } = await renderHeader()
    const trigger = screen.getByRole('button', {
      name: 'Toggle navigation menu',
    })
    await user.click(trigger)
    const menu = await screen.findByRole('dialog')
    expect(
      within(menu).getByRole('navigation', { name: 'Main navigation' })
    ).toBeVisible()
    expect(within(menu).getByRole('link', { name: 'Home' })).toBeVisible()
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{Escape}')
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    await waitFor(() => expect(trigger).toHaveFocus())
    queryClient.clear()
  })

  it('selecting a mobile navigation link dismisses the menu', async () => {
    const user = userEvent.setup()
    const { queryClient } = await renderHeader()
    await user.click(
      screen.getByRole('button', { name: 'Toggle navigation menu' })
    )
    const menu = await screen.findByRole('dialog')
    await user.click(within(menu).getByRole('link', { name: 'Home' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    queryClient.clear()
  })
})
