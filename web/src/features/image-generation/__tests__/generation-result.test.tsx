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

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { getImageGenerationDownload } from '../api'
import { GenerationResult } from '../components/generation-result'

vi.mock('../api', () => ({ getImageGenerationDownload: vi.fn() }))

afterEach(() => {
  vi.restoreAllMocks()
  vi.mocked(getImageGenerationDownload).mockReset()
})

const defaultProps = {
  hasMore: false,
  isError: false,
  isHistoryError: false,
  isHistoryLoading: false,
  isLoading: false,
  isLoadingMore: false,
  onLoadMore: vi.fn().mockResolvedValue(true),
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

  it('downloads through the authenticated API client without navigating away', async () => {
    const user = userEvent.setup()
    const blob = new Blob(['saved-image-bytes'], { type: 'image/png' })
    vi.mocked(getImageGenerationDownload).mockResolvedValue(blob)
    const createObjectURL = vi
      .spyOn(URL, 'createObjectURL')
      .mockReturnValue('blob:generated-download')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL')
    const clickedDownloads: Array<{ href: string; filename: string }> = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
      function (this: HTMLAnchorElement) {
        clickedDownloads.push({ href: this.href, filename: this.download })
      }
    )
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

    const download = screen.getByRole('button', { name: 'Download' })
    expect(download).not.toHaveAttribute('href')
    expect(download).toHaveClass('size-7')
    expect(download.textContent).toBe('')
    expect(download.querySelector('svg')).toBeInTheDocument()
    await user.click(download)

    await waitFor(() => {
      expect(getImageGenerationDownload).toHaveBeenCalledWith('saved-task', 0)
      expect(clickedDownloads).toEqual([
        {
          href: 'blob:generated-download',
          filename: 'generated-image-saved-task-1.png',
        },
      ])
    })
    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:generated-download')
  })

  it('keeps the gallery available when the authenticated download fails', async () => {
    const user = userEvent.setup()
    vi.mocked(getImageGenerationDownload).mockRejectedValue(
      new Error('download unavailable')
    )
    const createObjectURL = vi.spyOn(URL, 'createObjectURL')
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

    const download = screen.getByRole('button', { name: 'Download' })
    await user.click(download)

    await waitFor(() => {
      expect(getImageGenerationDownload).toHaveBeenCalledWith('saved-task', 0)
      expect(download).toBeEnabled()
    })
    expect(createObjectURL).not.toHaveBeenCalled()
    expect(screen.getByRole('img', { name: 'Generated image 1' })).toBeVisible()
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

  it('pages through results without making the panel scroll', async () => {
    const user = userEvent.setup()
    render(
      <GenerationResult
        {...defaultProps}
        history={[
          {
            task_id: 'gallery-task',
            model: 'gpt-image-2',
            created: 100,
            images: Array.from({ length: 7 }, (_, index) => ({
              url: `https://images.example.com/${index + 1}.png`,
            })),
          },
        ]}
      />
    )

    expect(
      screen.getAllByRole('img', { name: /Generated image/ })
    ).toHaveLength(6)
    const grid = screen
      .getByRole('img', { name: 'Generated image 1' })
      .closest('.grid')
    expect(grid).toHaveClass('overflow-hidden')
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(
      screen.getByRole('img', { name: 'Generated image 7' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('img', { name: 'Generated image 1' })
    ).not.toBeInTheDocument()
  })
})
