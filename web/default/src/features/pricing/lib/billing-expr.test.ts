/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import { parseTiersFromExpr } from './billing-expr'
import { formatDynamicRequestPrice } from './dynamic-price'

describe('parseTiersFromExpr', () => {
  it('exposes fixed request prices from parameterized tier expressions', () => {
    const expression =
      '(param("size") == "1K" || (img_o > 0 && img_o * 256 <= 2610000)) ? tier("standard", 300000.0 + (param("image.#") == nil || param("image.#") <= 1 ? 0.0 : (param("image.#") - 1) * 20000.0)) : tier("high_resolution", 600000.0 + (param("image.#") == nil || param("image.#") <= 1 ? 0.0 : (param("image.#") - 1) * 20000.0))'

    expect(parseTiersFromExpr(expression)).toMatchObject([
      { label: 'standard', requestPrice: 0.3 },
      { label: 'high_resolution', requestPrice: 0.6 },
    ])
  })

  it('keeps token price parsing unchanged', () => {
    expect(parseTiersFromExpr('tier("base", p * 2 + c * 8)')).toMatchObject([
      { label: 'base', inputPrice: 2, outputPrice: 8, requestPrice: 0 },
    ])
  })

  it('applies the selected group multiplier to request prices', () => {
    expect(
      formatDynamicRequestPrice(0.6, {
        tokenUnit: 'M',
        groupRatioMultiplier: 0.8,
      })
    ).toContain('0.48')
  })
})
