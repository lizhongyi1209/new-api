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
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ImageGeneration } from '../index'

vi.mock('../api', () => ({
  getImageGenerationGroups: vi.fn().mockResolvedValue([]),
  getImageGenerationHistory: vi
    .fn()
    .mockResolvedValue({ items: [], next_before_id: 0 }),
  getImageGenerationModels: vi.fn().mockResolvedValue([]),
  getImageGenerationTask: vi.fn(),
  submitImageGeneration: vi.fn(),
  uploadReferenceImage: vi.fn(),
}))

describe('image generation page', () => {
  it('keeps both panels stationary when the pointer hovers over them', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <ImageGeneration />
      </QueryClientProvider>
    )

    const settingsCard = screen
      .getByText('Generation settings')
      .closest('[data-slot="card"]')
    const resultCard = (
      await screen.findByText('Your generated images will appear here')
    ).closest('[data-slot="card"]')
    expect(settingsCard).toHaveAttribute('data-card-hover', 'false')
    expect(
      settingsCard?.querySelector('[data-slot="card-content"]')
    ).toHaveClass('xl:overflow-y-auto', 'xl:[scrollbar-width:none]')
    expect(resultCard).toHaveAttribute('data-card-hover', 'false')
    expect(resultCard).toHaveClass('overflow-hidden')
    expect(resultCard?.closest('.bg-muted\\/20')).toHaveClass(
      'xl:overflow-hidden'
    )
    const emptyStateContainer = screen
      .getByText('Your generated images will appear here')
      .closest('[data-slot="empty"]')?.parentElement?.parentElement
    expect(emptyStateContainer).toHaveClass(
      'flex-1',
      'items-center',
      'justify-center'
    )
  })
})
