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
import { useQuery } from '@tanstack/react-query'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { getChannels } from '../api'
import { RunBatch } from '../components/RunBatch'
import type { QualityChannel } from '../types'
import { channel, prompt, renderQuality } from './fixtures'

const unconfigured: QualityChannel = {
  ...channel,
  profile: {
    ...channel.profile,
    enabled: false,
    model: '',
    prompt_version_id: 0,
    version: 0,
  },
}

function Workspace() {
  const query = useQuery({
    queryKey: ['quality-detection', 'channels'],
    queryFn: () => getChannels(),
  })
  if (!query.data) return null
  return (
    <RunBatch
      channels={query.data}
      prompts={[prompt]}
      blocked={false}
      onCreated={vi.fn()}
    />
  )
}

let previous: AxiosAdapter | AxiosAdapter[] | undefined
beforeEach(() => {
  previous = api.defaults.adapter as typeof previous
})
afterEach(() => {
  api.defaults.adapter = previous
})

describe('channel selection and first-time configuration', () => {
  it('shows unconfigured channels and lets an administrator configure and select one without starting a model request', async () => {
    let current = unconfigured
    const requests: InternalAxiosRequestConfig[] = []
    api.defaults.adapter = async (config) => {
      requests.push(config)
      if (config.method === 'put') {
        current = {
          ...current,
          profile: { ...JSON.parse(config.data), version: 1 },
        }
      }
      return {
        status: 200,
        statusText: 'OK',
        config,
        headers: {},
        data: {
          success: true,
          data: config.method === 'put' ? current.profile : [current],
        },
      }
    }
    const user = userEvent.setup()
    renderQuality(<Workspace />)
    const checkbox = await screen.findByRole('checkbox', { name: 'Channel A' })
    expect(checkbox).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByText('Configuration required')).toBeVisible()
    expect(
      screen.queryByText('No saved data for this selection')
    ).not.toBeInTheDocument()
    await user.click(
      screen.getByRole('button', { name: 'Configure: Channel A' })
    )
    const dialog = await screen.findByRole('dialog')
    await user.click(
      within(dialog).getByRole('combobox', { name: 'Detection model' })
    )
    await user.click(await screen.findByRole('option', { name: 'model-a' }))
    await user.click(
      within(dialog).getByRole('combobox', { name: 'Prompt version' })
    )
    await user.click(
      await screen.findByRole('option', { name: 'Visual task · v1' })
    )
    await user.click(
      within(dialog).getByRole('switch', {
        name: 'Enable detection for this channel',
      })
    )
    await user.click(
      within(dialog).getByRole('button', { name: 'Save changes' })
    )
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    const ready = screen.getByRole('checkbox', { name: 'Channel A' })
    expect(ready).not.toHaveAttribute('aria-disabled', 'true')
    await user.click(ready)
    expect(ready).toBeChecked()
    expect(screen.getByText('1 unique channels selected')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Run detection' })).toBeEnabled()
    expect(requests.filter((request) => request.method !== 'get')).toHaveLength(
      1
    )
    expect(
      JSON.parse(
        requests.find((request) => request.method === 'put')?.data ?? '{}'
      )
    ).toMatchObject({ model: 'model-a', prompt_version_id: 1, enabled: true })
  })

  it('opens a searchable group menu and filters channels without losing prior selections', async () => {
    const second = {
      ...channel,
      id: 2,
      name: 'Channel B',
      groups: ['other'],
      profile: { ...channel.profile, channel_id: 2 },
    }
    const user = userEvent.setup()
    renderQuality(
      <RunBatch
        channels={[channel, second]}
        prompts={[prompt]}
        blocked={false}
        onCreated={vi.fn()}
      />
    )
    await user.click(screen.getByRole('checkbox', { name: 'Channel A' }))
    const filter = screen.getByRole('combobox', {
      name: 'Filter channels by group',
    })
    await user.click(filter)
    await waitFor(() => expect(filter).toHaveAttribute('aria-expanded', 'true'))
    const search = screen.getByRole('combobox', { name: 'Search...' })
    await user.type(search, 'oth')
    await user.click(screen.getByRole('option', { name: 'other' }))
    expect(
      screen.queryByRole('checkbox', { name: 'Channel A' })
    ).not.toBeInTheDocument()
    await user.click(
      screen.getByRole('button', { name: 'Select filtered channels' })
    )
    expect(screen.getByText('2 unique channels selected')).toBeVisible()
    await user.click(filter)
    await user.click(screen.getByRole('option', { name: 'All groups' }))
    expect(screen.getByRole('checkbox', { name: 'Channel A' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Channel B' })).toBeChecked()
  })

  it('keeps unsupported channels visible with a reason and excludes them from select-all', async () => {
    const unsupported = {
      ...channel,
      id: 2,
      name: 'Unsupported channel',
      supported: false,
      unsupported_reason: 'unsupported_protocol',
    }
    const user = userEvent.setup()
    renderQuality(
      <RunBatch
        channels={[channel, unsupported]}
        prompts={[prompt]}
        blocked={false}
        onCreated={vi.fn()}
      />
    )
    expect(
      screen.getByRole('checkbox', { name: 'Unsupported channel' })
    ).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByText(
        'This channel does not support the selected detection protocol.'
      )
    ).toBeVisible()
    await user.click(
      screen.getByRole('button', { name: 'Select filtered channels' })
    )
    expect(screen.getByText('1 unique channels selected')).toBeVisible()
  })
})
