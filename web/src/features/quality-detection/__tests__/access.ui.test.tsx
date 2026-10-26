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
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { screen } from '@testing-library/react'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { Route } from '@/routes/_authenticated/quality-detection'
import { useAuthStore } from '@/stores/auth-store'

import { renderQuality } from './fixtures'

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  previous = api.defaults.adapter as typeof previous
  useAuthStore.getState().auth.reset()
})
afterEach(() => {
  api.defaults.adapter = previous
  useAuthStore.getState().auth.reset()
})

describe('quality detection route access', () => {
  it('redirects a regular user before any quality detection data is requested', async () => {
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: { success: true, data: [] },
      }
    }
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'user', role: ROLE.USER })
    const root = createRootRoute({ component: Outlet })
    const quality = createRoute({
      getParentRoute: () => root,
      path: '/quality-detection',
      beforeLoad: () => {
        ;(Route.options.beforeLoad as () => void)()
      },
      component: Route.options.component,
    })
    const denied = createRoute({
      getParentRoute: () => root,
      path: '/403',
      component: () => <h1>Access denied</h1>,
    })
    const router = createRouter({
      routeTree: root.addChildren([quality, denied]),
      history: createMemoryHistory({ initialEntries: ['/quality-detection'] }),
    })
    await router.load()
    renderQuality(<RouterProvider router={router} />)
    expect(
      await screen.findByRole('heading', { name: 'Access denied' })
    ).toBeInTheDocument()
    expect(requests).toHaveLength(0)
  })
})
