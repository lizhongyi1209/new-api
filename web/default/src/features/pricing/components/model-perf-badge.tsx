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
import { memo, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  formatLatency,
  formatThroughput,
  getSuccessRateDotClass,
} from '@/features/performance-metrics/lib/format'
import type { SuccessRatePoint } from '@/features/performance-metrics/types'
import { toIntlLocale } from '@/i18n/languages'
import { cn } from '@/lib/utils'

export type ModelPerfBadgeData = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
  recent_success_series?: SuccessRatePoint[]
  hourly_window_end_ts?: number
}

export interface ModelPerfBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  perf: ModelPerfBadgeData | undefined
}

const STATUS_SLOTS = Array.from({ length: 24 }, (_, slot) => slot)

export const ModelPerfBadge = memo(function ModelPerfBadge(
  props: ModelPerfBadgeProps
) {
  const { t, i18n } = useTranslation()
  const latencyText = formatLatency(props.perf?.avg_latency_ms ?? 0)
  const throughputText = formatThroughput(props.perf?.avg_tps ?? 0).replace(
    ' t/s',
    't/s'
  )
  const successRate = props.perf?.success_rate
  const hasSuccessRate =
    successRate != null &&
    Number.isFinite(successRate) &&
    successRate >= 0 &&
    successRate <= 100
  // Hourly timestamps are anchored to the server snapshot, so clock skew does not shift bars.
  // Hours without traffic stay gray. Slot 23 is the current, partial hour.
  const snapshotTs = props.perf?.hourly_window_end_ts
  const currentHourStart =
    Math.floor(
      (snapshotTs != null && Number.isFinite(snapshotTs) && snapshotTs > 0
        ? snapshotTs
        : Date.now() / 1000) / 3600
    ) * 3600
  const statusRates = useMemo(() => {
    const ratesByHour = new Map<number, number | null>()
    for (const point of props.perf?.recent_success_series ?? []) {
      if (Number.isInteger(point.ts) && point.ts % 3600 === 0) {
        ratesByHour.set(point.ts, point.success_rate)
      }
    }
    return STATUS_SLOTS.map((slot) => {
      const hourStart = currentHourStart - (23 - slot) * 3600
      return ratesByHour.get(hourStart)
    })
  }, [props.perf?.recent_success_series, currentHourStart])

  return (
    <div
      aria-label={t('Performance metrics for the last 24 hours')}
      className={cn(
        'flex w-full min-w-0 flex-wrap items-center justify-between gap-2',
        props.className
      )}
    >
      <dl className='flex min-w-0 items-start gap-2 text-xs tabular-nums sm:gap-4'>
        <div className='w-24 shrink-0'>
          <dt
            title={t('Request success rate sampled over the last 24 hours')}
            className='text-muted-foreground flex items-center justify-between gap-1 text-[11px] leading-4'
          >
            <span>{t('Status')}</span>
            <span className='font-mono'>
              {hasSuccessRate ? `${successRate.toFixed(1)}%` : '—%'}
            </span>
          </dt>
          <dd
            role='img'
            aria-label={t(
              'Hourly success rates for the last 24 hours; gray means no data. The final bar is the current partial hour.'
            )}
            title={t(
              'Hourly success rates for the last 24 hours; gray means no data. The final bar is the current partial hour.'
            )}
            className='mt-1 flex h-3 w-24 items-center justify-between'
          >
            {STATUS_SLOTS.map((slot) => {
              const rate = statusRates[slot]
              return (
                <span
                  key={slot}
                  title={`${new Date((currentHourStart - (23 - slot) * 3600) * 1000).toLocaleString(toIntlLocale(i18n.language), { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}: ${rate != null && Number.isFinite(rate) && rate >= 0 && rate <= 100 ? `${rate.toFixed(1)}%` : t('No data')}`}
                  aria-hidden
                  className={cn(
                    'h-full w-[3px] shrink-0 rounded-xs',
                    rate != null &&
                      Number.isFinite(rate) &&
                      rate >= 0 &&
                      rate <= 100
                      ? getSuccessRateDotClass(rate)
                      : 'bg-muted-foreground/15'
                  )}
                />
              )
            })}
          </dd>
        </div>
        <div title={t('Average latency')} className='min-w-0'>
          <dt className='text-muted-foreground text-[11px] leading-4'>
            {t('Latency short')}
          </dt>
          <dd className='mt-1 font-mono text-[10px] whitespace-nowrap sm:text-xs'>
            {latencyText === '—' ? '—s' : latencyText}
          </dd>
        </div>
        <div title={t('Throughput')} className='min-w-0'>
          <dt className='text-muted-foreground text-[11px] leading-4'>
            {t('Throughput short')}
          </dt>
          <dd className='mt-1 font-mono text-[10px] whitespace-nowrap sm:text-xs'>
            {throughputText === '—' ? '—t/s' : throughputText}
          </dd>
        </div>
      </dl>
      {props.children}
    </div>
  )
})
