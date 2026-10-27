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
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosAdapter } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SidebarInset } from '@/components/ui/sidebar'
import { api } from '@/lib/api'

import { QualityDetection } from '../index'
import type { QualityBatch } from '../types'
import { channel, prompt, renderQuality, settings } from './fixtures'

const activeBatch: QualityBatch = {
  id: 7,
  task_id: 'batch-7',
  trigger: 'manual',
  status: 'running',
  total: 1,
  completed: 0,
  failed: 0,
  cancel_requested: false,
  created_at: 100,
  completed_at: 0,
  summary: '',
}
let previous: AxiosAdapter | AxiosAdapter[] | undefined
let batches: QualityBatch[]
let failedEndpoint: string

beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  batches = []
  failedEndpoint = ''
  api.defaults.adapter = async (config) => {
    if (config.url?.endsWith(failedEndpoint) && failedEndpoint) {
      throw new AxiosError('unavailable', undefined, config, undefined, {
        status: 503,
        statusText: 'Unavailable',
        config,
        headers: {},
        data: {},
      })
    }
    let data: unknown
    switch (config.url) {
      case '/api/quality-detection/channels':
        data = [channel]
        break
      case '/api/quality-detection/prompts':
        data = [prompt]
        break
      case '/api/quality-detection/settings':
        data = settings
        break
      case '/api/quality-detection/batches':
        data = { items: batches, total: batches.length, page: 1, page_size: 20 }
        break
      case '/api/quality-detection/batches/7':
        data = { batch: activeBatch, runs: [] }
        break
      default:
        throw new Error(`Unexpected request: ${config.url}`)
    }
    return {
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
      data: { success: true, data },
    }
  }
})
afterEach(() => {
  api.defaults.adapter = previous
})

async function renderQualityPage() {
  const root = createRootRoute({ component: Outlet })
  const quality = createRoute({
    getParentRoute: () => root,
    path: '/quality-detection',
    component: () => (
      <SidebarInset className='h-svh min-h-0 overflow-hidden'>
        <QualityDetection />
      </SidebarInset>
    ),
  })
  const status = createRoute({
    getParentRoute: () => root,
    path: '/status',
    component: () => <h1>Service Status</h1>,
  })
  const router = createRouter({
    routeTree: root.addChildren([quality, status]),
    history: createMemoryHistory({ initialEntries: ['/quality-detection'] }),
  })
  await router.load()
  return renderQuality(<RouterProvider router={router} />)
}

describe('quality workspace layout', () => {
  it('opens the channels tab with exactly one run selector inside its panel', async () => {
    await renderQualityPage()
    const runButton = await screen.findByRole('button', {
      name: 'Run detection',
    })
    expect(screen.getByRole('tab', { name: 'Channels' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(
      screen.getAllByRole('button', { name: 'Run detection' })
    ).toHaveLength(1)
    expect(screen.getByRole('tabpanel', { name: 'Channels' })).toContainElement(
      runButton
    )
  })

  it('keeps the final settings actions within the authenticated scroll region', async () => {
    const user = userEvent.setup()
    await renderQualityPage()
    await user.click(await screen.findByRole('tab', { name: 'Settings' }))
    const save = await screen.findByRole('button', { name: 'Save changes' })
    const scrollRegion = save.closest('.workspace-page-content')
    expect(scrollRegion).toHaveClass('min-h-0', 'flex-1', 'overflow-auto')
    expect(scrollRegion?.parentElement).toHaveClass('min-h-0', 'flex-1')
    expect(screen.getAllByRole('main')).toHaveLength(1)
    expect(screen.getByRole('tabpanel', { name: 'Settings' })).toContainElement(
      save
    )
    expect(
      screen.queryByRole('button', { name: 'Run detection' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Add public group' }))
    expect(scrollRegion).toContainElement(
      screen.getByRole('textbox', { name: 'Public alias' })
    )
    save.focus()
    expect(save).toHaveFocus()
  })

  it('lets keyboard users reach every tab while the run selector stays in channels', async () => {
    const user = userEvent.setup()
    await renderQualityPage()
    await screen.findByRole('button', { name: 'Run detection' })
    const channelsTab = screen.getByRole('tab', { name: 'Channels' })
    channelsTab.focus()
    for (const name of ['Run history', 'Prompt versions', 'Settings']) {
      await user.keyboard('{ArrowRight}{Enter}')
      const tab = screen.getByRole('tab', { name })
      expect(tab).toHaveFocus()
      expect(tab).toHaveAttribute('aria-selected', 'true')
      expect(screen.getByRole('tabpanel', { name })).toBeInTheDocument()
      expect(
        screen.queryByRole('button', { name: 'Run detection' })
      ).not.toBeInTheDocument()
    }
    await user.keyboard('{Home}{Enter}')
    expect(channelsTab).toHaveAttribute('aria-selected', 'true')
    expect(
      await screen.findByRole('button', { name: 'Run detection' })
    ).toBeInTheDocument()
    expect(screen.getByRole('tablist')).toHaveAttribute('data-variant', 'line')
    expect(screen.getByRole('tablist').parentElement).toHaveClass(
      'overflow-x-auto'
    )
  })

  it('opens a running batch in history and keeps new runs blocked', async () => {
    batches = [activeBatch]
    const user = userEvent.setup()
    await renderQualityPage()
    await user.click(
      await screen.findByRole('button', { name: 'Select filtered channels' })
    )
    expect(screen.getByRole('button', { name: 'Run detection' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'View batch #7' }))
    expect(screen.getByRole('tab', { name: 'Run history' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    const history = screen.getByRole('tabpanel', { name: 'Run history' })
    expect(
      await within(history).findByRole('heading', { name: 'Batch #7' })
    ).toBeInTheDocument()
    expect(
      within(history).getByRole('button', { name: 'Stop batch' })
    ).toBeEnabled()
    expect(
      screen.queryByRole('button', { name: 'Run detection' })
    ).not.toBeInTheDocument()
  })

  it('keeps new runs blocked when checking the latest batches fails and allows retry', async () => {
    failedEndpoint = '/batches'
    const user = userEvent.setup()
    await renderQualityPage()
    await screen.findByText('Unable to load saved data')
    await user.click(
      screen.getByRole('button', { name: 'Select filtered channels' })
    )
    expect(screen.getByRole('button', { name: 'Run detection' })).toBeDisabled()
    failedEndpoint = ''
    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Run detection' })
      ).toBeEnabled()
    )
  })

  it('retains navigation to the public status page', async () => {
    const user = userEvent.setup()
    await renderQualityPage()
    await user.click(
      await screen.findByRole('button', { name: 'Service Status' })
    )
    expect(
      await screen.findByRole('heading', { name: 'Service Status' })
    ).toBeInTheDocument()
  })
})
