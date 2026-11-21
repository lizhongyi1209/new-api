import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  beijingToday,
  dayRange,
  estimatedProfit,
  pricingGroupKey,
  rangeBounds,
} from '../period'

afterEach(() => vi.useRealTimers())
describe('reseller reporting periods and profit estimates', () => {
  it('uses Beijing dates across midnight even when the browser clock is UTC', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-21T15:59:59Z'))
    expect(beijingToday()).toBe('2026-11-21')
    vi.setSystemTime(new Date('2026-11-21T16:00:00Z'))
    expect(beijingToday()).toBe('2026-11-22')
    const bounds = rangeBounds(dayRange(beijingToday()))
    expect(bounds.to - bounds.from).toBe(86399)
    expect(new Date(bounds.from * 1000).toISOString()).toBe(
      '2026-11-21T16:00:00.000Z'
    )
  })
  it('preserves the selected custom start and end times', () => {
    const bounds = rangeBounds({
      from: '2026-11-20',
      to: '2026-11-21',
      start: '08:30:00',
      end: '09:45:00',
      precise: true,
    })
    expect(new Date(bounds.from * 1000).toISOString()).toBe(
      '2026-11-20T00:30:00.000Z'
    )
    expect(new Date(bounds.to * 1000).toISOString()).toBe(
      '2026-11-21T01:45:00.000Z'
    )
  })
  it('estimates margin from net consumption and reverses negative refund-only periods', () => {
    expect(estimatedProfit(120, [], { base: 1.2, overrides: {} })).toBeCloseTo(
      20
    )
    expect(estimatedProfit(-120, [], { base: 1.2, overrides: {} })).toBeCloseTo(
      -20
    )
    expect(estimatedProfit(120, [], { base: 0, overrides: {} })).toBeNull()
  })
  it('retains overrides by customer and billing group instead of row order', () => {
    const first = {
      user_group: 'retail',
      billing_group: 'premium',
      net_quota: 120,
    }
    const second = {
      user_group: 'retail',
      billing_group: 'standard',
      net_quota: 200,
    }
    const rates = { base: 1.2, overrides: { [pricingGroupKey(second)]: 2 } }
    expect(estimatedProfit(320, [second, first], rates)).toBeCloseTo(120)
    expect(estimatedProfit(120, [first], rates)).toBeCloseTo(20)
  })
})
