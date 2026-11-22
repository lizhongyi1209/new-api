import type { ColumnDef } from '@tanstack/react-table'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePagination,
  DataTableView,
  MobileCardList,
  useDataTable,
} from '@/components/data-table'
import { Input } from '@/components/ui/input'
import { NativeSelect } from '@/components/ui/native-select'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import type { ResellerPricingCatalog, ResellerPricingModel } from './api'

function amount(value: number): string {
  return formatBillingCurrencyFromUSD(value, {
    showSymbol: false,
    abbreviate: false,
    digitsLarge: 4,
    digitsSmall: 6,
  })
}

function isDynamic(model: ResellerPricingModel): boolean {
  return Boolean(model.billing_mode && model.billing_mode !== 'ratio')
}

function available(model: ResellerPricingModel, group: string): boolean {
  const groups = model.enable_groups ?? []
  return groups.includes('all') || groups.includes(group)
}

function quote(model: ResellerPricingModel, ratio: number): string {
  if (isDynamic(model)) return 'dynamic'
  if (model.quota_type === 1) return amount(model.model_price * ratio)
  const input = model.model_ratio * 2 * ratio
  const output = input * model.completion_ratio
  return `${amount(input)} / ${amount(output)}`
}

function discountLabel(value: number | undefined): string {
  if (value == null || !Number.isFinite(value) || Math.abs(value) < 0.005) {
    return '—'
  }
  const formatted = Math.abs(value).toFixed(1).replace(/\.0$/, '')
  return value > 0 ? `${formatted}%` : `+${formatted}%`
}

export function ResellerPricing(props: { catalog: ResellerPricingCatalog }) {
  const { t } = useTranslation()
  const [group, setGroup] = useState(props.catalog.groups[0]?.name ?? '')
  const [search, setSearch] = useState('')
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 20 })
  const selected =
    props.catalog.groups.find((item) => item.name === group) ??
    props.catalog.groups[0]
  useEffect(() => {
    setPagination((current) =>
      current.pageIndex === 0 ? current : { ...current, pageIndex: 0 }
    )
  }, [group, search])
  const rows = useMemo(() => {
    const term = search.trim().toLocaleLowerCase()
    return props.catalog.models.filter((model) => {
      if (selected && !available(model, selected.name)) return false
      return model.model_name.toLocaleLowerCase().includes(term)
    })
  }, [props.catalog.models, search, selected])
  const columns: ColumnDef<ResellerPricingModel>[] = [
    {
      accessorKey: 'model_name',
      header: t('Model'),
      size: 280,
      cell: ({ row }) => (
        <span className='ledger-customer-name'>{row.original.model_name}</span>
      ),
    },
    {
      id: 'groups',
      header: t('Groups'),
      size: 190,
      cell: ({ row }) => (
        <span className='ledger-price-groups'>
          {(row.original.enable_groups ?? []).includes('all')
            ? t('All groups')
            : (row.original.enable_groups ?? []).join(' · ')}
        </span>
      ),
    },
    {
      id: 'official',
      header: t('Official price'),
      size: 180,
      cell: ({ row }) => {
        if (!selected || !available(row.original, selected.name)) {
          return <span className='ledger-muted'>—</span>
        }
        const value = quote(row.original, selected.official_ratio)
        return (
          <span className='ledger-price-stack'>
            <span>{value === 'dynamic' ? t('Usage based') : value}</span>
            {value !== 'dynamic' && row.original.quota_type === 0 && (
              <small>{t('Input / output · 1M tokens')}</small>
            )}
            {value !== 'dynamic' && row.original.quota_type === 1 && (
              <small>{t('Per request')}</small>
            )}
          </span>
        )
      },
    },
    {
      id: 'cost',
      header: t('Your cost'),
      size: 180,
      cell: ({ row }) => {
        if (!selected || !available(row.original, selected.name)) {
          return <span className='ledger-muted'>—</span>
        }
        const value = quote(row.original, selected.cost_ratio)
        return (
          <span className='ledger-price-stack ledger-price-cost'>
            <span>{value === 'dynamic' ? t('Usage based') : value}</span>
            {value !== 'dynamic' && row.original.quota_type === 0 && (
              <small>{t('Input / output · 1M tokens')}</small>
            )}
            {value !== 'dynamic' && row.original.quota_type === 1 && (
              <small>{t('Per request')}</small>
            )}
          </span>
        )
      },
    },
    {
      id: 'discount',
      header: t('Official discount'),
      size: 130,
      cell: ({ row }) => {
        if (!selected || !available(row.original, selected.name)) {
          return <span className='ledger-muted'>—</span>
        }
        return (
          <span
            className={
              selected.discount_percent && selected.discount_percent > 0
                ? 'ledger-discount'
                : 'ledger-muted'
            }
          >
            {discountLabel(selected.discount_percent)}
          </span>
        )
      },
    },
  ]
  const { table } = useDataTable({
    data: rows,
    columns,
    pagination,
    onPaginationChange: setPagination,
    withPaginationRowModel: true,
    enableRowSelection: false,
  })
  return (
    <section aria-label={t('Pricing table')}>
      <div className='ledger-price-summary'>
        <div>
          <span>{t('Your user group')}</span>
          <strong>{props.catalog.user_group}</strong>
          <small>{props.catalog.user_group_description}</small>
        </div>
        <div>
          <span>{t('Selected group')}</span>
          <strong>{selected?.name || '—'}</strong>
          <small>
            {selected
              ? t('Official {{official}}× · Cost {{cost}}×', {
                  official: selected.official_ratio,
                  cost: selected.cost_ratio,
                })
              : ''}
          </small>
        </div>
        <div>
          <span>{t('Official discount')}</span>
          <strong>{discountLabel(selected?.discount_percent)}</strong>
          <small>{t('Estimated from current group multipliers')}</small>
        </div>
      </div>
      <div className='ledger-toolbar ledger-pricing-toolbar'>
        <Input
          type='search'
          aria-label={t('Search models')}
          placeholder={t('Search models')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <NativeSelect
          aria-label={t('Pricing group')}
          value={group}
          onChange={(event) => setGroup(event.target.value)}
        >
          {props.catalog.groups.map((item) => (
            <option key={item.name} value={item.name}>
              {item.name} · {item.description}
            </option>
          ))}
        </NativeSelect>
      </div>
      <p className='ledger-muted ledger-pricing-note'>
        {t(
          'Official price uses the selected group standard multiplier. Your cost uses the reseller user-group multiplier.'
        )}
      </p>
      <div className='ledger-desktop'>
        <DataTableView
          table={table}
          applyHeaderSize
          containerClassName='ledger-table'
          tableClassName='ledger-pricing-table'
          emptyTitle={t('No models')}
        />
      </div>
      <div className='ledger-mobile'>
        <MobileCardList
          table={table}
          enableRowSelection={false}
          emptyTitle={t('No models')}
        />
      </div>
      <div className='ledger-pagination'>
        <DataTablePagination table={table} compact />
      </div>
    </section>
  )
}
