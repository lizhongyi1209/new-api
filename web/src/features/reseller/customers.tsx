import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTableView,
  DataTablePagination,
  MobileCardList,
  useDataTable,
} from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect } from '@/components/ui/native-select'
import { toIntlLocale } from '@/i18n/languages'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { useSystemConfigStore } from '@/stores/system-config-store'

import type { Customer } from './api'
import { beijingStamp } from './period'

export function CustomerTable(props: {
  customers: Customer[]
  onLogs: (id: number) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const currency = useSystemConfigStore((state) => state.config.currency)
  const [search, setSearch] = useState('')
  const [sort, setSort] = useState('registered')
  const [low, setLow] = useState('')
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const data = useMemo(() => {
    const term = search.trim().toLocaleLowerCase()
    let rate = 1
    if (currency.quotaDisplayType === 'CNY') rate = currency.usdExchangeRate
    if (currency.quotaDisplayType === 'CUSTOM') {
      rate = currency.customCurrencyExchangeRate
    }
    if (currency.quotaDisplayType === 'TOKENS') rate = currency.quotaPerUnit
    const result = props.customers.filter(
      (row) =>
        (row.username.toLocaleLowerCase().includes(term) ||
          String(row.id).includes(term)) &&
        (!low || (row.quota / currency.quotaPerUnit) * rate < 1000)
    )
    result.sort((a, b) => {
      if (sort === 'name') return a.username.localeCompare(b.username, locale)
      if (sort === 'balance') return a.quota - b.quota
      if (sort === 'used') return b.used_quota - a.used_quota
      return b.id - a.id
    })
    return result
  }, [props.customers, search, sort, low, locale, currency])
  const columns: ColumnDef<Customer>[] = [
    {
      accessorKey: 'username',
      header: t('Customer name'),
      size: 220,
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <span className='ledger-customer-name'>{row.original.username}</span>
      ),
    },
    {
      id: 'balance',
      header: t('Balance'),
      size: 270,
      cell: ({ row }) => (
        <span className='ledger-balance'>
          {formatQuotaWithCurrency(row.original.quota, {
            showSymbol: false,
            abbreviate: false,
            digitsLarge: 2,
            digitsSmall: 2,
            locale,
          })}
          <span className='ledger-muted'>
            {' '}
            / {t('Used credits')}{' '}
            {formatQuotaWithCurrency(row.original.used_quota, {
              showSymbol: false,
              abbreviate: false,
              digitsLarge: 2,
              digitsSmall: 2,
              locale,
            })}
          </span>
        </span>
      ),
    },
    { accessorKey: 'id', header: 'ID', size: 90 },
    {
      accessorKey: 'last_login_at',
      header: t('Last online'),
      size: 185,
      cell: ({ row }) => (
        <time>{beijingStamp(row.original.last_login_at).slice(0, 16)}</time>
      ),
    },
    {
      accessorKey: 'created_at',
      header: t('Registration time'),
      size: 185,
      cell: ({ row }) => (
        <time>{beijingStamp(row.original.created_at).slice(0, 16)}</time>
      ),
    },
    {
      id: 'actions',
      header: '',
      size: 70,
      cell: ({ row }) => (
        <Button
          variant='link'
          onClick={() => props.onLogs(row.original.id)}
          aria-label={t('View logs for {{name}}', {
            name: row.original.username,
          })}
        >
          {t('Logs')}
        </Button>
      ),
    },
  ]
  const { table } = useDataTable({
    data,
    columns,
    pagination,
    onPaginationChange: setPagination,
    withPaginationRowModel: true,
    enableRowSelection: false,
    autoResetPageIndex: false,
  })
  const resetPage = () => setPagination({ pageIndex: 0, pageSize: 10 })
  return (
    <section aria-label={t('Customer balances')}>
      <div className='ledger-toolbar'>
        <Input
          type='search'
          aria-label={t('Customer name / ID')}
          placeholder={t('Customer name / ID')}
          value={search}
          onChange={(event) => {
            setSearch(event.target.value)
            resetPage()
          }}
        />
        <NativeSelect
          aria-label={t('Balance filter')}
          value={low}
          onChange={(event) => {
            setLow(event.target.value)
            resetPage()
          }}
        >
          <option value=''>{t('All balances')}</option>
          <option value='low'>{t('Below 1,000 credits')}</option>
        </NativeSelect>
        <NativeSelect
          aria-label={t('Customer order')}
          value={sort}
          onChange={(event) => {
            setSort(event.target.value)
            resetPage()
          }}
        >
          <option value='registered'>{t('Recently registered')}</option>
          <option value='name'>{t('Customer name')}</option>
          <option value='balance'>{t('Balance ascending')}</option>
          <option value='used'>{t('Usage descending')}</option>
        </NativeSelect>
      </div>
      <div className='ledger-desktop'>
        <DataTableView
          table={table}
          applyHeaderSize
          containerClassName='ledger-table'
          tableClassName='ledger-customer-table'
          emptyTitle={t('No customers')}
        />
      </div>
      <div className='ledger-mobile'>
        <MobileCardList
          table={table}
          enableRowSelection={false}
          emptyTitle={t('No customers')}
        />
      </div>
      <div className='ledger-pagination'>
        <DataTablePagination table={table} compact />
      </div>
    </section>
  )
}
