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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { SidebarInset } from '@/components/ui/sidebar'

import { SectionPageLayout } from '../components/section-page-layout'

describe('workspace page hierarchy', () => {
  it('provides a main landmark and a single page heading alongside its description and actions', () => {
    render(
      <SidebarInset id='content' tabIndex={-1}>
        <SectionPageLayout fixedContent>
          <SectionPageLayout.Title>API Keys</SectionPageLayout.Title>
          <SectionPageLayout.Description>
            Control access to your models.
          </SectionPageLayout.Description>
          <SectionPageLayout.Actions>
            <button type='button'>Create key</button>
          </SectionPageLayout.Actions>
          <SectionPageLayout.Content>
            <p>Key list</p>
          </SectionPageLayout.Content>
        </SectionPageLayout>
      </SidebarInset>
    )
    expect(screen.getAllByRole('main')).toHaveLength(1)
    expect(screen.getByRole('main')).toHaveAttribute('id', 'content')
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
    expect(screen.getByRole('heading', { name: 'API Keys' })).toBeVisible()
    expect(screen.getByText('Control access to your models.')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Create key' })).toBeVisible()
    expect(screen.getByText('Key list')).toBeVisible()
  })
})
