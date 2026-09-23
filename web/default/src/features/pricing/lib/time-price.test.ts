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

import type { PricingModel } from '../types'
import { getDynamicPricingSummary } from './dynamic-price'
import { readTimePricingTiers } from './time-price'

const expression =
  '(hour("Asia/Shanghai") >= 22 || hour("Asia/Shanghai") < 8) ? tier("night", p * 1 + c * 0) : (len < 1000 ? tier("short", p * 3 + c * 4) : tier("long", p * 5 + c * 6))'

describe('readonly billing time preview', () => {
  it('selects the complete time branch at both overnight boundaries without selecting a usage tier', () => {
    for (const [time, labels] of [
      ['2026-09-14T13:59:00Z', ['short', 'long']],
      ['2026-09-14T14:00:00Z', ['night']],
      ['2026-09-14T23:59:00Z', ['night']],
      ['2026-09-15T00:00:00Z', ['short', 'long']],
    ] as const) {
      expect(
        readTimePricingTiers(expression, new Date(time))?.currentTiers.map(
          (tier) => tier.label
        )
      ).toEqual(labels)
    }
  })

  it('keeps all historical pricing branches independent of the preview clock', () => {
    const withoutClock = readTimePricingTiers(expression)
    if (!withoutClock) throw new Error('Time pricing missing')
    expect(withoutClock.currentTiers).toEqual([])
    expect(withoutClock.tiers.map((tier) => tier.label)).toEqual([
      'night',
      'short',
      'long',
    ])
    expect(
      readTimePricingTiers(expression, new Date('2026-09-14T14:00:00Z'))?.tiers
    ).toEqual(withoutClock.tiers)
    expect(
      readTimePricingTiers(expression, new Date('2026-09-15T00:00:00Z'))?.tiers
    ).toEqual(withoutClock.tiers)
  })

  it('handles DST, Sunday numbering and invalid-zone UTC fallback; server Local stays unknown', () => {
    const dst =
      'hour("America/New_York") == 1 ? tier("one", p * 1) : tier("other", p * 2)'
    for (const time of ['2026-11-01T05:30:00Z', '2026-11-01T06:30:00Z']) {
      expect(
        readTimePricingTiers(dst, new Date(time))?.currentTiers[0].label
      ).toBe('one')
    }
    const sunday =
      'weekday("UTC") == 0 ? tier("Sunday", p * 1) : tier("other", p * 2)'
    expect(
      readTimePricingTiers(sunday, new Date('2026-09-13T12:00:00Z'))
        ?.currentTiers[0].label
    ).toBe('Sunday')
    const invalid =
      'hour("invalid/zone") == 14 ? tier("UTC", p * 1) : tier("other", p * 2)'
    expect(
      readTimePricingTiers(invalid, new Date('2026-09-14T14:00:00Z'))
        ?.currentTiers[0].label
    ).toBe('UTC')
    expect(
      readTimePricingTiers(
        dst.replaceAll('America/New_York', 'Local'),
        new Date()
      )?.currentTiers
    ).toEqual([])
  })

  it('uses the current branch for monetary summaries and preserves explicit zero; unsupported billing stays raw', () => {
    const model = {
      billing_mode: 'tiered_expr',
      billing_expr: expression,
    } as PricingModel
    const summary = getDynamicPricingSummary(model, {
      tokenUnit: 'M',
      now: new Date('2026-09-14T14:00:00Z'),
    })
    expect(summary?.tier?.label).toBe('night')
    expect(summary?.primaryEntries.map((entry) => entry.value)).toEqual([1, 0])
    expect(
      readTimePricingTiers(
        'hour("UTC") < 8 ? tier("a", img_cr * 2) : tier("b", p * 2)',
        new Date('2026-09-14T07:00:00Z')
      )?.currentTiers
    ).toMatchObject([{ label: 'a', imageCachePrice: 2 }])
    expect(
      readTimePricingTiers(
        'hour("UTC") < 8 ? tier("a", fixed(1)) : tier("b", p * 2)',
        new Date('2026-09-14T07:00:00Z')
      )?.currentTiers
    ).toMatchObject([{ label: 'a', billingUnit: 'request', fixedPrice: 1 }])
    for (const body of ['u("seconds") * 0.4', 'p * 2 + unexpected(3)']) {
      expect(
        readTimePricingTiers(
          `hour("UTC") < 8 ? tier("a", ${body}) : tier("b", p * 2)`
        )
      ).toBeNull()
    }
  })
})
