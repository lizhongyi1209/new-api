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
  it('opens the native picker once when the drop zone is clicked', () => {
    const inputClick = vi.spyOn(HTMLInputElement.prototype, 'click')
    render(
      <ReferenceImagePicker disabled={false} images={[]} onChange={vi.fn()} />
    )

    fireEvent.click(screen.getAllByLabelText('Add reference images')[0])

    expect(inputClick).toHaveBeenCalledTimes(1)
  })

  it('accepts a dropped image without secure-context crypto APIs', () => {
    const onChange = vi.fn()
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:reference-preview')
    const file = new File(['png'], 'reference.png', { type: 'image/png' })
    render(
      <ReferenceImagePicker disabled={false} images={[]} onChange={onChange} />
    )

    fireEvent.drop(screen.getAllByLabelText('Add reference images')[0], {
      dataTransfer: { files: [file] },
    })

    expect(onChange).toHaveBeenCalledWith([
      {
        file,
        id: 'blob:reference-preview',
        previewUrl: 'blob:reference-preview',
      },
    ])
  })
})
