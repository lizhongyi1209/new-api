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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import {
  getImageGenerationHistory,
  getImageGenerationModels,
  submitImageGeneration,
} from '../api'
import { ImageGeneration } from '../index'

vi.mock('../api', () => ({
  getImageGenerationHistory: vi
    .fn()
    .mockResolvedValue({ items: [], next_before_id: 0 }),
  getImageGenerationModels: vi.fn().mockResolvedValue([]),
  getImageGenerationTask: vi.fn(),
  submitImageGeneration: vi.fn(),
  uploadReferenceImage: vi.fn(),
  deleteImageGenerationImages: vi.fn(),
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
    expect(resultCard).toHaveClass('overflow-visible', 'xl:overflow-hidden')
    expect(
      screen.queryByRole('heading', { name: 'Image Generation' })
    ).toBeNull()
    expect(screen.queryByRole('combobox', { name: 'Group' })).toBeNull()
    expect(settingsCard?.parentElement).toHaveClass('gap-3')
    expect(settingsCard?.parentElement?.parentElement).toHaveClass(
      'p-2',
      'sm:p-3',
      'lg:p-4'
    )
    await waitFor(() =>
      expect(getImageGenerationModels).toHaveBeenCalledWith('auto')
    )
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

  it('keeps results below settings on small screens and preserves the draft', async () => {
    const user = userEvent.setup()
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <ImageGeneration />
      </QueryClientProvider>
    )

    const prompt = screen.getByRole('textbox', { name: 'Prompt' })
    await user.type(prompt, 'A quiet harbor at dusk')
    const settingsCard = screen
      .getByText('Generation settings')
      .closest('[data-slot="card"]')
    const resultCard = (
      await screen.findByText('Your generated images will appear here')
    ).closest('[data-slot="card"]')

    expect(settingsCard?.parentElement).toHaveClass(
      'auto-rows-max',
      'grid-cols-1',
      'xl:grid-cols-[minmax(0,3fr)_minmax(0,7fr)]'
    )
    expect(settingsCard?.nextElementSibling).toBe(resultCard)
    expect(settingsCard).not.toHaveClass('hidden')
    expect(resultCard).not.toHaveClass('hidden')
    expect(screen.queryByRole('tab')).toBeNull()
    expect(prompt).toHaveValue('A quiet harbor at dusk')
  })

  it('keeps the result panel below the settings after submitting', async () => {
    const user = userEvent.setup()
    vi.mocked(getImageGenerationModels).mockResolvedValueOnce([
      {
        label: 'gemini-nano-banana-2.1-sp',
        value: 'gemini-nano-banana-2.1-sp',
      },
    ])
    vi.mocked(submitImageGeneration).mockResolvedValueOnce({
      status: 'PENDING',
      task_id: 'mobile-task',
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <ImageGeneration />
      </QueryClientProvider>
    )

    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Model' })).toHaveTextContent(
        'Nano Banana 2.1'
      )
    )
    await user.type(screen.getByRole('textbox', { name: 'Prompt' }), 'Sunrise')
    await user.click(screen.getByRole('button', { name: 'Generate image' }))

    await waitFor(() =>
      expect(submitImageGeneration).toHaveBeenCalledWith(
        expect.objectContaining({ prompt: 'Sunrise' })
      )
    )
    const settingsCard = screen
      .getByText('Generation settings')
      .closest('[data-slot="card"]')
    const resultCard = settingsCard?.nextElementSibling
    expect(resultCard).toHaveAttribute('data-slot', 'card')
    expect(resultCard).not.toHaveClass('hidden')
  })

  it('loads a saved result into the editor and regenerates from its durable URL', async () => {
    const user = userEvent.setup()
    vi.mocked(getImageGenerationModels).mockResolvedValueOnce([
      {
        label: 'gemini-nano-banana-2.1-sp',
        value: 'gemini-nano-banana-2.1-sp',
      },
    ])
    vi.mocked(getImageGenerationHistory).mockResolvedValueOnce({
      next_before_id: 0,
      items: [
        {
          task_id: 'saved-task',
          model: 'gemini-nano-banana-2.1-sp',
          prompt: 'A blue coat',
          created: 100,
          options: { size: '1K', aspect_ratio: '1:1' },
          references: [
            'https://images.example.com/model.png',
            'https://images.example.com/coat-reference.png',
          ],
          can_regenerate: true,
          images: [{ url: 'https://images.example.com/coat.png' }],
        },
      ],
    })
    vi.mocked(submitImageGeneration).mockResolvedValueOnce({
      status: 'PENDING',
      task_id: 'retry-task',
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <ImageGeneration />
      </QueryClientProvider>
    )

    const resultTile = (
      await screen.findByRole('img', {
        name: 'Generated image 1',
      })
    ).closest('figure') as HTMLElement
    await user.click(
      within(resultTile).getByRole('button', { name: 'Image 1 actions' })
    )
    await user.click(screen.getByRole('menuitem', { name: 'Edit' }))
    expect(screen.getByRole('textbox', { name: 'Prompt' })).toHaveValue(
      'A blue coat'
    )
    expect(
      screen.getByRole('img', { name: /Reference image 1/ })
    ).toHaveAttribute('src', 'https://images.example.com/coat.png')

    await user.click(
      within(resultTile).getByRole('button', { name: 'Image 1 actions' })
    )
    await user.click(screen.getByRole('menuitem', { name: 'Regenerate' }))
    await waitFor(() =>
      expect(submitImageGeneration).toHaveBeenCalledWith(
        expect.objectContaining({
          model: 'gemini-nano-banana-2.1-sp',
          prompt: 'A blue coat',
          images: [
            'https://images.example.com/model.png',
            'https://images.example.com/coat-reference.png',
          ],
        })
      )
    )
  })
})
