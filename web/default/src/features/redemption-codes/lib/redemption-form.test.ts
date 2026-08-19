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
import assert from 'node:assert/strict'
import { afterEach, test } from 'node:test'

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

  assert.equal(displayed, 1)
  assert.equal(converted, 500000)
  assert.equal(
    resolveRedemptionQuotaForUpdate(storedQuota, converted, false),
    storedQuota
  )
})

test('uses the converted quota after the operator edits the quota field', () => {
  assert.equal(resolveRedemptionQuotaForUpdate(500001, 1000000, true), 1000000)
})

test('uses configured CNY precision for editable quota and input step', () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7.2,
    },
  })

  assert.equal(quotaUnitsToEditableAmount(13888889), 200)
  assert.equal(parseQuotaFromDollars(200), 13888889)
  assert.equal(getEditableQuotaStep(), 0.0001)
})
