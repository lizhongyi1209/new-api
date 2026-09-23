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
import { beforeEach, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { PricingModel } from '../types'
import { formatFixedPrice, formatPrice, formatRequestPrice } from './price'

const model: PricingModel = {
  id: 1,
  model_name: 'test',
  quota_type: 0,
  model_ratio: 0.5,
  completion_ratio: 2,
  enable_groups: ['default'],
  group_ratio: { default: 1 },
}

beforeEach(() =>
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'USD' },
  })
)

test('keeps explicit zero token and cache prices while missing optional ratios remain unset', () => {
  expect(formatPrice({ ...model, model_ratio: 0 }, 'input', 'M')).toBe('$0')
  expect(formatPrice({ ...model, completion_ratio: 0 }, 'output', 'M')).toBe(
    '$0'
  )
  expect(formatPrice({ ...model, cache_ratio: 0 }, 'cache', 'M')).toBe('$0')
  expect(formatPrice(model, 'cache', 'M')).toBe('-')
  expect(formatPrice({ ...model, cache_ratio: null }, 'cache', 'M')).toBe('-')
})

test('distinguishes missing request price from a configured free price in every group', () => {
  const request = { ...model, quota_type: 1 }
  expect(formatRequestPrice(request)).toBe('-')
  expect(formatRequestPrice({ ...request, model_price: 0 })).toBe('$0')
  expect(
    formatFixedPrice(request, 'default', false, 1, 1, { default: 1 })
  ).toBe('-')
  expect(
    formatFixedPrice({ ...request, model_price: 0 }, 'default', false, 1, 1, {
      default: 1,
    })
  ).toBe('$0')
})

test('keeps unit, recharge and selected group calculations equivalent across display switches', () => {
  expect(formatPrice(model, 'input', 'M')).toBe('$1')
  expect(formatPrice(model, 'input', 'K')).toBe('$0.001')
  expect(formatPrice(model, 'output', 'M', true, 4, 8)).toBe('$1')
  expect(
    formatPrice(
      { ...model, enable_groups: ['premium'], group_ratio: { premium: 2 } },
      'input',
      'M',
      false,
      1,
      1,
      'premium'
    )
  ).toBe('$2')
})
