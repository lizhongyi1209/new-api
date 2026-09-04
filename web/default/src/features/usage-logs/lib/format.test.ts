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

import type { LogOtherData } from '../types'
import { getTieredBillingSummary } from './format'

describe('getTieredBillingSummary', () => {
  it('exposes a fixed per-request price for usage-log summaries', () => {
    const expression =
      '(param("size") == "1K" || (img_o > 0 && img_o * 256 <= 2610000)) ? tier("standard", 300000 + (param("image.#") == nil || param("image.#") <= 1 ? 0 : (param("image.#") - 1) * 20000)) : tier("high_resolution", 600000 + (param("image.#") == nil || param("image.#") <= 1 ? 0 : (param("image.#") - 1) * 20000))'
    const other = {
      billing_mode: 'tiered_expr',
      expr_b64: btoa(expression),
      matched_tier: 'high_resolution',
    } as LogOtherData

    const summary = getTieredBillingSummary(other)

    expect(summary).not.toBeNull()
    expect(summary?.requestPrice).toBe(0.6)
    expect(summary?.priceEntries).toEqual([])
  })
})
