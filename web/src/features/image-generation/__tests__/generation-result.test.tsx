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
import { createImageDownloadArchive } from '../lib/download-archive'

vi.mock('../api', () => ({ getImageGenerationDownload: vi.fn() }))
vi.mock('../lib/download-archive', () => ({
  createImageDownloadArchive: vi.fn(),
}))

afterEach(() => {
  vi.restoreAllMocks()
  vi.mocked(getImageGenerationDownload).mockReset()
  vi.mocked(createImageDownloadArchive).mockReset()
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
  it('uses the compact grid default after the viewport becomes narrow', () => {
    const history = [
      {
        task_id: 'responsive-gallery',
        model: 'gemini-nano-banana-2.1',
        created: 100,
        images: [{ url: 'https://images.example.com/responsive.png' }],
      },
    ]
    const { rerender } = render(
      <GenerationResult {...defaultProps} history={history} compact={false} />
    )
    const slider = screen.getByRole('group', { name: 'Image grid size' })
    expect(screen.queryByText('Image grid size')).not.toBeInTheDocument()
    expect(screen.getByText('Generated images')).toHaveClass('text-base')
    expect(screen.getByLabelText('1 saved images')).toHaveTextContent('1')
    const range = slider.querySelector('input[type="range"]')
    expect(range).toHaveAttribute('aria-valuenow', '220')

    rerender(<GenerationResult {...defaultProps} history={history} compact />)
    expect(range).toHaveAttribute('aria-valuenow', '130')

    fireEvent.change(range as HTMLInputElement, { target: { value: '120' } })
    expect(range).toHaveAttribute('aria-valuenow', '120')
    rerender(
      <GenerationResult {...defaultProps} history={history} compact={false} />
    )
    expect(range).toHaveAttribute('aria-valuenow', '120')
  })

  it('shows a flowing twelve-image page on small screens without clipping tiles', () => {
    render(
      <GenerationResult
        {...defaultProps}
        compact
        history={[
          {
            task_id: 'mobile-gallery',
            model: 'gemini-nano-banana-2.1',
            created: 100,
            images: Array.from({ length: 13 }, (_, index) => ({
              url: `https://images.example.com/${index + 1}.png`,
            })),
          },
        ]}
      />
    )

    expect(
      screen.getAllByRole('img', { name: /Generated image/ })
    ).toHaveLength(12)
    const grid = screen
      .getByRole('img', { name: 'Generated image 1' })
      .closest('.grid')
    expect(grid).toHaveClass('overflow-visible', 'xl:overflow-hidden')
    expect(screen.getAllByRole('button', { name: 'Download' })[0]).toHaveClass(
      'size-7'
    )
  })

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

  it('keeps selections across pages and downloads multiple images as a ZIP', async () => {
    const user = userEvent.setup()
    const archive = new Blob(['zip-bytes'], { type: 'application/zip' })
    vi.mocked(createImageDownloadArchive).mockResolvedValue(archive)
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:selected-images')
    const clickedDownloads: string[] = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
      function (this: HTMLAnchorElement) {
        clickedDownloads.push(this.download)
      }
    )
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

    await user.click(
      screen.getByRole('checkbox', { name: 'Select generated image 1' })
    )
    await user.click(screen.getByRole('button', { name: 'Next' }))
    await user.click(
      screen.getByRole('checkbox', { name: 'Select generated image 7' })
    )
    expect(
      screen.getByRole('button', { name: /Download selected/ })
    ).toHaveTextContent('2')
    await user.click(screen.getByRole('button', { name: /Download selected/ }))

    await waitFor(() => {
      expect(createImageDownloadArchive).toHaveBeenCalledWith(
        [
          { taskId: 'gallery-task', index: 0, outputFormat: undefined },
          { taskId: 'gallery-task', index: 6, outputFormat: undefined },
        ],
        getImageGenerationDownload
      )
      expect(clickedDownloads).toEqual(['generated-images.zip'])
    })
    await user.click(screen.getByRole('button', { name: 'Previous' }))
    expect(
      screen.getByRole('checkbox', { name: 'Select generated image 1' })
    ).toBeChecked()
    await user.click(screen.getByRole('button', { name: 'Clear selection' }))
    expect(
      screen.queryByRole('button', { name: /Download selected/ })
    ).not.toBeInTheDocument()
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
    expect(grid).toHaveClass('overflow-visible', 'xl:overflow-hidden')
    await user.click(screen.getByRole('button', { name: 'Next' }))
    expect(
      screen.getByRole('img', { name: 'Generated image 7' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('img', { name: 'Generated image 1' })
    ).not.toBeInTheDocument()
  })
})
