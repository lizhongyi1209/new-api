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
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ReferenceImagePicker } from '../components/reference-image-picker'
import type { ReferenceImage } from '../types'

function referenceImages(): ReferenceImage[] {
  return ['first', 'second', 'third'].map((name) => ({
    file: new File(['png'], `${name}.png`, { type: 'image/png' }),
    id: name,
    previewUrl: `blob:${name}`,
  }))
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('reference image picker', () => {
  it('shows a square dashed upload button and the per-image size limit', () => {
    render(
      <ReferenceImagePicker disabled={false} images={[]} onChange={vi.fn()} />
    )

    expect(
      screen.getByRole('button', { name: 'Add reference images' })
    ).toHaveClass('size-20', 'shrink-0', 'border-dashed')
    expect(
      screen.getByText('PNG, JPEG or WebP, up to 10 MiB each')
    ).toBeInTheDocument()
  })

  it('opens the native picker once when the drop zone is clicked', () => {
    const inputClick = vi.spyOn(HTMLInputElement.prototype, 'click')
    render(
      <ReferenceImagePicker disabled={false} images={[]} onChange={vi.fn()} />
    )

    fireEvent.click(
      screen.getByRole('button', { name: 'Add reference images' })
    )

    expect(inputClick).toHaveBeenCalledTimes(1)
  })

  it('accepts a dropped image without secure-context crypto APIs', () => {
    const onChange = vi.fn()
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:reference-preview')
    const file = new File(['png'], 'reference.png', { type: 'image/png' })
    render(
      <ReferenceImagePicker disabled={false} images={[]} onChange={onChange} />
    )

    fireEvent.drop(
      screen.getByRole('button', { name: 'Add reference images' }),
      {
        dataTransfer: { files: [file] },
      }
    )

    expect(onChange).toHaveBeenCalledWith([
      {
        file,
        id: 'blob:reference-preview',
        previewUrl: 'blob:reference-preview',
      },
    ])
  })

  it('rejects a reference image over 10 MiB before upload', () => {
    const onChange = vi.fn()
    const file = new File(['png'], 'large.png', { type: 'image/png' })
    Object.defineProperty(file, 'size', { value: 10 * 1024 * 1024 + 1 })
    render(
      <ReferenceImagePicker disabled={false} images={[]} onChange={onChange} />
    )

    fireEvent.drop(
      screen.getByRole('button', { name: 'Add reference images' }),
      {
        dataTransfer: { files: [file] },
      }
    )

    expect(onChange).not.toHaveBeenCalled()
  })

  it('keeps multiple references in one horizontally scrollable row with numbered tiles', () => {
    render(
      <ReferenceImagePicker
        disabled={false}
        images={referenceImages()}
        onChange={vi.fn()}
      />
    )

    const strip = screen.getByRole('region', { name: 'Reference images' })
    expect(strip).toHaveClass('flex-nowrap', 'overflow-x-auto')
    expect(screen.getAllByRole('figure')).toHaveLength(3)
    expect(
      screen.getByRole('img', { name: 'Reference image 2: second.png' })
    ).toBeInTheDocument()
    expect(screen.getByText('2')).toHaveClass('absolute', 'top-1', 'left-1')
    expect(screen.getAllByRole('figure')[1]).toHaveClass('shrink-0')
  })

  it('moves a reference left from its actions menu', async () => {
    const user = userEvent.setup()
    const images = referenceImages()
    const onChange = vi.fn()
    render(
      <ReferenceImagePicker
        disabled={false}
        images={images}
        onChange={onChange}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Image 2 actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Move left' }))

    expect(onChange).toHaveBeenCalledWith([images[1], images[0], images[2]])
  })

  it('reorders images when one thumbnail is dropped on another', () => {
    const images = referenceImages()
    const onChange = vi.fn()
    render(
      <ReferenceImagePicker
        disabled={false}
        images={images}
        onChange={onChange}
      />
    )
    const dataTransfer = {
      effectAllowed: 'none',
      files: [],
      setData: vi.fn(),
    }
    const tiles = screen.getAllByRole('figure')

    fireEvent.dragStart(tiles[0], { dataTransfer })
    fireEvent.dragOver(tiles[2], { dataTransfer })
    fireEvent.drop(tiles[2], { dataTransfer })

    expect(onChange).toHaveBeenCalledWith([images[1], images[2], images[0]])
  })

  it('replaces only the chosen reference through its actions menu', async () => {
    const user = userEvent.setup()
    const images = referenceImages()
    const onChange = vi.fn()
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:replacement')
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    render(
      <ReferenceImagePicker
        disabled={false}
        images={images}
        onChange={onChange}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Image 2 actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Replace' }))
    const file = new File(['png'], 'replacement.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText('Replace reference image'), {
      target: { files: [file] },
    })

    expect(onChange).toHaveBeenCalledWith([
      images[0],
      { file, id: 'blob:replacement', previewUrl: 'blob:replacement' },
      images[2],
    ])
    expect(revoke).toHaveBeenCalledWith('blob:second')
  })

  it('keeps the old reference when its replacement exceeds 10 MiB', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <ReferenceImagePicker
        disabled={false}
        images={referenceImages()}
        onChange={onChange}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Image 1 actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Replace' }))
    const file = new File(['png'], 'large.png', { type: 'image/png' })
    Object.defineProperty(file, 'size', { value: 10 * 1024 * 1024 + 1 })
    fireEvent.change(screen.getByLabelText('Replace reference image'), {
      target: { files: [file] },
    })

    expect(onChange).not.toHaveBeenCalled()
  })
})
