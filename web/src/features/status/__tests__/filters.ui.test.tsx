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
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { renderQuality } from '@/features/quality-detection/__tests__/fixtures'

import { defaultStatusFilters } from '../api'
import { StatusFilterBar } from '../components/StatusFilterBar'

describe('status filters', () => {
  it('keeps the selected model visible while refreshed options are unavailable', () => {
    renderQuality(
      <StatusFilterBar
        filters={{ ...defaultStatusFilters, model: 'model-a' }}
        models={[]}
        onChange={vi.fn()}
      />
    )
    const model = screen.getByRole('combobox', { name: 'Model' })
    expect(model).toHaveTextContent('model-a')
    expect(model).toHaveAttribute('aria-expanded', 'false')
  })

  it('finds a model by typing in the themed dropdown and resets pagination', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    renderQuality(
      <StatusFilterBar
        filters={{ ...defaultStatusFilters, page: 3 }}
        models={[
          'model-a',
          'model-b',
          'very-long-model-name-for-a-specialized-provider',
        ]}
        onChange={onChange}
      />
    )
    await user.click(screen.getByRole('combobox', { name: 'Model' }))
    await user.keyboard('model-b')
    expect(
      screen.queryByRole('option', { name: 'model-a' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('option', { name: 'model-b' }))
    expect(onChange).toHaveBeenCalledWith({
      ...defaultStatusFilters,
      model: 'model-b',
      page: 1,
    })
  })

  it('allows keyboard selection and escape returns focus without changing the time window', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    renderQuality(
      <StatusFilterBar
        filters={defaultStatusFilters}
        models={[]}
        onChange={onChange}
      />
    )
    const window = screen.getByRole('combobox', { name: 'Time window' })
    window.focus()
    await user.keyboard('{Enter}')
    expect(window).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{Escape}')
    await waitFor(() =>
      expect(window).toHaveAttribute('aria-expanded', 'false')
    )
    expect(window).toHaveFocus()
    expect(onChange).not.toHaveBeenCalled()
  })

  it('keeps filter columns shrinkable so long model labels cannot widen a narrow screen', () => {
    renderQuality(
      <StatusFilterBar
        filters={{
          ...defaultStatusFilters,
          model: 'very-long-model-name-for-a-specialized-provider',
        }}
        models={[]}
        onChange={vi.fn()}
      />
    )
    const field = screen.getByLabelText('Search groups').parentElement
    expect(field).toHaveClass('min-w-0')
    expect(field?.parentElement).toHaveClass(
      'min-w-0',
      'sm:grid-cols-2',
      'xl:grid-cols-5'
    )
    expect(screen.getByRole('combobox', { name: 'Model' })).toHaveClass(
      'w-full',
      'min-w-0'
    )
  })

  it('resets the page without changing the model when the sort order changes', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    renderQuality(
      <StatusFilterBar
        filters={{ ...defaultStatusFilters, page: 4, model: 'model-a' }}
        models={['model-a']}
        onChange={onChange}
      />
    )
    await user.click(screen.getByRole('combobox', { name: 'Sort groups' }))
    await user.click(screen.getByRole('option', { name: 'Detection status' }))
    expect(onChange).toHaveBeenCalledWith({
      ...defaultStatusFilters,
      model: 'model-a',
      sort: 'state',
      page: 1,
    })
  })
})
