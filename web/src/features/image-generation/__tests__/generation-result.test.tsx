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

import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { GenerationResult } from '../components/generation-result'

const defaultProps = {
  hasMore: false,
  isError: false,
  isHistoryError: false,
  isHistoryLoading: false,
  isLoading: false,
  isLoadingMore: false,
  onLoadMore: vi.fn(),
  onRetry: vi.fn(),
}

describe('generation result gallery', () => {
  it('shows saved URL images in square tiles and lets the user resize the grid', () => {
    render(
      <GenerationResult
        {...defaultProps}
        history={[
          {
            task_id: 'saved-task',
            model: 'gemini-nano-banana-2.1',
            created: 100,
            images: [{ url: 'https://images.example.com/images/saved.png' }],
          },
        ]}
      />
    )

    const image = screen.getByRole('img', { name: 'Generated image 1' })
    expect(image).toHaveAttribute(
      'src',
      'https://images.example.com/images/saved.png'
    )
    expect(image.closest('figure')).toHaveClass('aspect-square')

    const slider = screen.getByRole('group', { name: 'Image grid size' })
    const range = slider.querySelector('input[type="range"]')
    expect(range).toHaveAttribute('aria-valuenow', '220')
    fireEvent.change(range as HTMLInputElement, { target: { value: '120' } })
    expect(range).toHaveAttribute('aria-valuenow', '120')
  })

  it('shows a square preview while a new task runs alongside saved results', () => {
    render(
      <GenerationResult
        {...defaultProps}
        history={[
          {
            task_id: 'saved-task',
            model: 'gpt-image-1',
            created: 100,
            images: [{ url: 'https://images.example.com/images/saved.png' }],
          },
        ]}
        task={{ task_id: 'running-task', status: 'IN_PROGRESS' }}
      />
    )

    expect(screen.getByLabelText('Generating your image')).toHaveClass(
      'aspect-square'
    )
    expect(
      screen.getByRole('img', { name: 'Generated image 1' })
    ).toBeInTheDocument()
  })
})
