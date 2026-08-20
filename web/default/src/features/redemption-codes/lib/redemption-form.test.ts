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
import { afterEach, expect, test } from 'vitest'

import {
  getEditableQuotaStep,
  parseQuotaFromDollars,
  quotaUnitsToEditableAmount,
} from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { resolveRedemptionQuotaForUpdate } from './redemption-form'

afterEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

test('keeps the exact stored quota when another redemption field changes', () => {
  const storedQuota = 500001
  const displayed = quotaUnitsToEditableAmount(storedQuota)
  const converted = parseQuotaFromDollars(displayed)

  expect(displayed).toBe(1)
  expect(converted).toBe(500000)
  expect(resolveRedemptionQuotaForUpdate(storedQuota, converted, false)).toBe(
    storedQuota
  )
})

test('uses the converted quota after the operator edits the quota field', () => {
  expect(resolveRedemptionQuotaForUpdate(500001, 1000000, true)).toBe(1000000)
})

test('uses configured CNY precision for editable quota and input step', () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7.2,
    },
  })

  expect(quotaUnitsToEditableAmount(13888889)).toBe(200)
  expect(parseQuotaFromDollars(200)).toBe(13888889)
  expect(getEditableQuotaStep()).toBe(0.0001)
})
