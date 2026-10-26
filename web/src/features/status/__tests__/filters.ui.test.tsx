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
import { screen } from '@testing-library/react'
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
    expect(screen.getByRole('combobox', { name: 'Model' })).toHaveValue(
      'model-a'
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
    await user.selectOptions(
      screen.getByRole('combobox', { name: 'Sort groups' }),
      'state'
    )
    expect(onChange).toHaveBeenCalledWith({
      ...defaultStatusFilters,
      model: 'model-a',
      sort: 'state',
      page: 1,
    })
  })
})
