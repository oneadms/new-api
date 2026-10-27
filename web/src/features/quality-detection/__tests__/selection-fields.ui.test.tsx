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
import { useForm } from 'react-hook-form'
import { describe, expect, it, vi } from 'vitest'

import { QualityField } from '../components/QualityField'
import { renderQuality } from './fixtures'

function PromptPicker(props: {
  onSubmit: (values: { prompt: number }) => void
  disabled?: boolean
}) {
  const form = useForm({ defaultValues: { prompt: 0 } })
  return (
    <form onSubmit={form.handleSubmit(props.onSubmit)}>
      <QualityField
        form={form}
        name='prompt'
        type='number'
        label='Prompt version'
        disabled={props.disabled}
        options={[
          { value: 0, label: 'Select prompt version' },
          { value: 7, label: 'Visual task · v7' },
        ]}
      />
      <button type='submit'>Save</button>
    </form>
  )
}

describe('themed quality dropdowns', () => {
  it('selects by keyboard, restores focus on Escape and submits a numeric prompt ID', async () => {
    const save = vi.fn()
    const user = userEvent.setup()
    renderQuality(<PromptPicker onSubmit={save} />)
    const trigger = screen.getByRole('combobox', { name: 'Prompt version' })
    expect(trigger).toHaveTextContent('Select prompt version')
    await user.tab()
    await user.keyboard('{Enter}')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{ArrowDown}{Enter}')
    await waitFor(() => expect(trigger).toHaveTextContent('Visual task · v7'))
    await user.click(trigger)
    await waitFor(() =>
      expect(
        screen.getByRole('option', { name: 'Visual task · v7' })
      ).toHaveFocus()
    )
    await user.keyboard('{Escape}')
    await waitFor(() => expect(trigger).toHaveFocus())
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith({ prompt: 7 }, expect.anything())
    )
  })

  it('keeps a disabled dropdown closed and preserves the zero placeholder value', async () => {
    const save = vi.fn()
    const user = userEvent.setup()
    renderQuality(<PromptPicker onSubmit={save} disabled />)
    const trigger = screen.getByRole('combobox', { name: 'Prompt version' })
    expect(trigger).toBeDisabled()
    await user.click(trigger)
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(save).toHaveBeenCalledWith({ prompt: 0 }, expect.anything())
    )
  })
})
