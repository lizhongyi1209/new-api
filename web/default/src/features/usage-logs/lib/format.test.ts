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
  it('uses the settled fixed zero instead of a time branch or guessed tier price', () => {
    const other: LogOtherData = {
      billing_mode: 'tiered_expr',
      billing_unit: 'request',
      fixed_price: 0,
      matched_tier: 'same',
      expr_b64: btoa(
        'hour("UTC") < 8 ? tier("same", fixed(0.025)) : tier("same", p * 2)'
      ),
    }
    expect(getTieredBillingSummary(other)?.priceEntries).toEqual([
      {
        field: 'fixedPrice',
        shortLabel: 'Per-call',
        price: 0,
        unit: 'request',
      },
    ])
    expect(
      getTieredBillingSummary({ ...other, image_count: 2 })?.priceEntries[0]
        .unit
    ).toBe('image')
  })

  it('shows explicit free image cache prices only with actual cache facts and preserves unknown matches', () => {
    const other: LogOtherData = {
      billing_mode: 'tiered_expr',
      matched_tier: 'base',
      image_cache_tokens: 100,
      billing_tokens: { img_cr: 100 },
      expr_b64: btoa('tier("base", p * 2 + img_cr * 0)'),
    }
    expect(getTieredBillingSummary(other)?.priceEntries).toMatchObject([
      { field: 'inputPrice', price: 2 },
      { field: 'imageCachePrice', price: 0 },
    ])
    expect(
      getTieredBillingSummary({ ...other, matched_tier: 'unknown' })
    ).toBeNull()
  })
})
