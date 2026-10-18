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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ReferenceImagePicker } from '../components/reference-image-picker'

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
    ).toHaveClass('aspect-square', 'border-dashed')
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
})
