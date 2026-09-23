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
import { beforeEach, expect, it } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { PricingModel } from '../types'
import { getDynamicPricingSummary } from './dynamic-price'

const base: PricingModel = {
  id: 1,
  model_name: 'model',
  quota_type: 0,
  model_ratio: 0.5,
  completion_ratio: 2,
  enable_groups: ['default'],
  group_ratio: { default: 1 },
  billing_mode: 'tiered_expr',
}
beforeEach(() =>
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'USD' },
  })
)

it('preserves explicit free request prices and image units across token display scales', () => {
  for (const tokenUnit of ['K', 'M'] as const) {
    const summary = getDynamicPricingSummary(
      { ...base, billing_expr: 'tier("image", fixed(0)) * image_count' },
      { tokenUnit }
    )
    expect(summary?.requestPriceEntry).toMatchObject({ value: 0 })
    expect(summary?.primaryEntries).toMatchObject([{ unit: 'image', value: 0 }])
    expect(summary?.isSpecialExpression).toBe(false)
  }
})

it('keeps zero image cache coefficients and rejects partial nonlinear prices', () => {
  const summary = getDynamicPricingSummary(
    { ...base, billing_expr: 'tier("base", p * 2 + img_cr * 0)' },
    { tokenUnit: 'M' }
  )
  expect(summary?.secondaryEntries).toMatchObject([
    { field: 'imageCachePrice', value: 0 },
  ])
  expect(
    getDynamicPricingSummary(
      { ...base, billing_expr: 'tier("base", p * 2 + c * c)' },
      { tokenUnit: 'M' }
    )?.isSpecialExpression
  ).toBe(true)
})

it('separates image quantities and quoted request rules without losing the fixed price', () => {
  const expression =
    'tier("image", fixed(0.04)) * image_count * ((param("quality") == "hd)") ? 2.0 : 1.0)'
  const summary = getDynamicPricingSummary(
    { ...base, billing_expr: expression },
    { tokenUnit: 'M' }
  )
  expect(summary?.requestPriceEntry).toMatchObject({ value: 0.04 })
  expect(summary?.primaryEntries).toMatchObject([
    { unit: 'image', value: 0.04 },
  ])
  expect(summary?.hasRequestRules).toBe(true)
  expect(summary?.isSpecialExpression).toBe(false)
})
