import type { PricingGroup } from './api'

export interface DateRange {
  from: string
  to: string
  start: string
  end: string
  precise: boolean
}
export interface Markup {
  base: number
  overrides: Record<string, number>
}

const beijingClock = new Intl.DateTimeFormat('sv-SE', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hourCycle: 'h23',
})
export function beijingStamp(seconds: number): string {
  return seconds ? beijingClock.format(new Date(seconds * 1000)) : '—'
}
export function beijingToday(): string {
  return beijingStamp(Date.now() / 1000).slice(0, 10)
}
export function dayRange(from: string, to = from): DateRange {
  return { from, to, start: '00:00:00', end: '23:59:59', precise: false }
}
export function rangeBounds(range: DateRange): { from: number; to: number } {
  return {
    from:
      Date.parse(
        `${range.from}T${range.precise ? range.start : '00:00:00'}+08:00`
      ) / 1000,
    to:
      Date.parse(
        `${range.to}T${range.precise ? range.end : '23:59:59'}+08:00`
      ) / 1000,
  }
}
export function pricingGroupKey(group: PricingGroup): string {
  return JSON.stringify([group.user_group, group.billing_group])
}
export function estimatedProfit(
  revenue: number,
  groups: PricingGroup[],
  markup: Markup
): number | null {
  if (!Number.isFinite(markup.base) || markup.base <= 0) return null
  let supply = 0
  for (const group of groups) {
    const rate = markup.overrides[pricingGroupKey(group)] ?? markup.base
    if (!Number.isFinite(rate) || rate <= 0) return null
    supply += group.net_quota / rate
  }
  if (!groups.length) supply = revenue / markup.base
  const profit = revenue - supply
  return Number.isFinite(profit) ? profit : null
}
