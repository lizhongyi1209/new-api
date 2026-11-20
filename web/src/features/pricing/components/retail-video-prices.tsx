import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { formatTaskUsageUnitPrice } from '../lib/dynamic-price'
import type { RetailVideoPrices } from '../types'

type RetailVideoProps = {
  prices: RetailVideoPrices
  priceRate?: number
  usdExchangeRate?: number
  showRechargePrice?: boolean
}

export function RetailVideoPriceRange(
  props: RetailVideoProps & { selectedGroup?: string }
) {
  const { t } = useTranslation()
  useSystemConfigStore((state) => state.config.currency)
  const selected = props.prices.groups.find(
    (entry) => entry.group === props.selectedGroup
  )
  const groups = selected ? [selected] : props.prices.groups
  const prices = groups.flatMap((entry) =>
    entry.rows.flatMap((row) => Object.values(row.prices))
  )
  if (!prices.length) return <span>{t('Price unavailable')}</span>
  const options = { ...props, tokenUnit: 'M' as const, groupRatioMultiplier: 1 }
  const min = formatTaskUsageUnitPrice(Math.min(...prices), options)
  const max = formatTaskUsageUnitPrice(Math.max(...prices), options)
  return (
    <span className='font-mono text-sm tabular-nums'>
      {min === max ? min : `${min} – ${max}`} / {t('output second')}
    </span>
  )
}

export function RetailVideoPriceTable(props: RetailVideoProps) {
  const { t } = useTranslation()
  useSystemConfigStore((state) => state.config.currency)
  if (!props.prices.groups.length) return <p>{t('Price unavailable')}</p>
  const options = { ...props, tokenUnit: 'M' as const, groupRatioMultiplier: 1 }
  return (
    <div className='space-y-4'>
      {props.prices.groups.map((group) => {
        const resolutions = ['720P', '1080P', '4K'].filter((key) =>
          group.rows.some((row) => key in row.prices)
        )
        return (
          <section key={group.group} className='space-y-2'>
            <div className='flex flex-wrap items-center gap-2'>
              <GroupBadge group={group.group} size='sm' />
              <span className='text-sm'>{t('Price per output second')}</span>
            </div>
            <StaticDataTable
              data={group.rows}
              getRowKey={(row) => row.feature}
              columns={[
                {
                  id: 'feature',
                  header: t('Applicable conditions'),
                  cell: (row) => t(row.feature),
                  cellClassName: 'whitespace-normal',
                },
                ...resolutions.map((resolution) => ({
                  id: resolution,
                  header: resolution,
                  className: 'text-right',
                  cellClassName: 'text-right font-mono tabular-nums',
                  cell: (row: (typeof group.rows)[number]) =>
                    row.prices[resolution] === undefined
                      ? '—'
                      : formatTaskUsageUnitPrice(
                          row.prices[resolution],
                          options
                        ),
                })),
              ]}
            />
            {Boolean(group.per_request) && (
              <p className='text-sm'>
                {t('Additional fee per request')} ·{' '}
                {formatTaskUsageUnitPrice(group.per_request!, options)}
              </p>
            )}
          </section>
        )
      })}
    </div>
  )
}
