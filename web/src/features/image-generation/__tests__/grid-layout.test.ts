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

import { describe, expect, it } from 'vitest'

import { calculateImageGridLayout } from '../lib/image-generation'

describe('image gallery capacity', () => {
  it('fits more square results when the tile slider is moved smaller', () => {
    const large = calculateImageGridLayout(800, 600, 360)
    const small = calculateImageGridLayout(800, 600, 120)

    expect(large).toEqual({ columns: 2, capacity: 2 })
    expect(small).toEqual({ columns: 6, capacity: 24 })
  })

  it('keeps square tiles within a short result panel', () => {
    expect(calculateImageGridLayout(800, 300, 360)).toEqual({
      columns: 3,
      capacity: 3,
    })
  })
})
