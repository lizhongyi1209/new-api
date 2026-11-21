import { useQuery } from '@tanstack/react-query'
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTableView,
  DataTablePagination,
  MobileCardList,
  useDataTable,
} from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect } from '@/components/ui/native-select'
import { DetailsDialog } from '@/features/usage-logs/components/dialogs/details-dialog'
import type { UsageLog } from '@/features/usage-logs/data/schema'
import { formatLogQuota, formatTokens, formatUseTime } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  resellerGet,
  type Bootstrap,
  type LogFilters,
  type LogPage,
} from './api'
import { beijingStamp } from './period'

export function ResellerLogs(props: {
  bootstrap: Bootstrap
  bounds: { from: number; to: number }
  filters: LogFilters
  onFilters: (filters: LogFilters) => void
}) {
  const { t } = useTranslation()
  const owner = useAuthStore((state) => state.auth.user?.id)
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const [selected, setSelected] = useState<UsageLog | null>(null)
  const query = useQuery({
    queryKey: [
      'reseller',
      'logs',
      owner,
      props.bounds,
      props.filters,
      pagination.pageIndex,
    ],
    queryFn: ({ signal }) =>
      resellerGet<LogPage>(
        'logs',
        { ...props.bounds, ...props.filters, page: pagination.pageIndex + 1 },
        signal
      ),
    refetchInterval: 60000,
    meta: { errorToast: false },
  })
  const columns: ColumnDef<UsageLog>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      size: 175,
      cell: ({ row }) => <time>{beijingStamp(row.original.created_at)}</time>,
    },
    {
      accessorKey: 'username',
      header: t('Customer'),
      size: 135,
      meta: { mobileTitle: true },
    },
    {
      accessorKey: 'type',
      header: t('Type'),
      size: 70,
      cell: ({ row }) => (
        <span>
          {t(
            (
              { 2: 'Consume', 5: 'Error', 6: 'Refund' } as Record<
                number,
                string
              >
            )[row.original.type] || 'Unknown'
          )}
        </span>
      ),
    },
    { accessorKey: 'token_name', header: t('Token'), size: 130 },
    { accessorKey: 'model_name', header: t('Model'), size: 230 },
    { accessorKey: 'group', header: t('Group'), size: 120 },
    {
      accessorKey: 'use_time',
      header: t('Duration'),
      size: 90,
      cell: ({ row }) => formatUseTime(row.original.use_time),
    },
    {
      accessorKey: 'prompt_tokens',
      header: t('Input'),
      size: 90,
      cell: ({ row }) => formatTokens(row.original.prompt_tokens),
    },
    {
      accessorKey: 'completion_tokens',
      header: t('Output'),
      size: 90,
      cell: ({ row }) => formatTokens(row.original.completion_tokens),
    },
    {
      accessorKey: 'quota',
      header: t('Cost'),
      size: 100,
      cell: ({ row }) => formatLogQuota(row.original.quota),
    },
    {
      id: 'actions',
      header: '',
      size: 70,
      cell: ({ row }) => (
        <Button variant='link' onClick={() => setSelected(row.original)}>
          {t('Details')}
        </Button>
      ),
    },
  ]
  const { table } = useDataTable({
    data: query.isError ? [] : (query.data?.rows ?? []),
    columns,
    manualPagination: true,
    totalCount: query.data?.total ?? 0,
    pagination,
    onPaginationChange: setPagination,
    enableRowSelection: false,
  })
  function change(key: keyof LogFilters, value: string): void {
    setPagination({ pageIndex: 0, pageSize: 10 })
    props.onFilters({ ...props.filters, [key]: value })
  }
  if (query.isError) {
    return (
      <ErrorState
        title={t('Unable to load logs')}
        onRetry={() => void query.refetch()}
      />
    )
  }
  return (
    <section aria-label={t('Usage logs')}>
      <div className='ledger-toolbar'>
        <NativeSelect
          aria-label={t('Customer')}
          value={props.filters.user}
          onChange={(event) => change('user', event.target.value)}
        >
          <option value=''>{t('All customers')}</option>
          {props.bootstrap.customers.map((row) => (
            <option key={row.id} value={row.id}>
              {row.username}
            </option>
          ))}
        </NativeSelect>
        <NativeSelect
          aria-label={t('Model')}
          value={props.filters.model}
          onChange={(event) => change('model', event.target.value)}
        >
          <option value=''>{t('All models')}</option>
          {props.bootstrap.models.map((name) => (
            <option key={name}>{name}</option>
          ))}
        </NativeSelect>
        <NativeSelect
          aria-label={t('Group')}
          value={props.filters.group}
          onChange={(event) => change('group', event.target.value)}
        >
          <option value=''>{t('All groups')}</option>
          {props.bootstrap.groups.filter(Boolean).map((group) => (
            <option key={group}>{group}</option>
          ))}
        </NativeSelect>
        <NativeSelect
          aria-label={t('Type')}
          value={props.filters.type}
          onChange={(event) => change('type', event.target.value)}
        >
          <option value=''>{t('All types')}</option>
          <option value='2'>{t('Consume')}</option>
          <option value='5'>{t('Error')}</option>
          <option value='6'>{t('Refund')}</option>
        </NativeSelect>
        <details className='ledger-more'>
          <summary>{t('More filters')}</summary>
          <div>
            <Input
              aria-label={t('Token name')}
              placeholder={t('Token name')}
              value={props.filters.token}
              onChange={(event) => change('token', event.target.value)}
            />
            <Input
              aria-label={t('Request ID')}
              placeholder={t('Request ID')}
              value={props.filters.request}
              onChange={(event) => change('request', event.target.value)}
            />
          </div>
        </details>
      </div>
      <div className='ledger-desktop'>
        <DataTableView
          table={table}
          isLoading={query.isPending}
          containerClassName='ledger-table'
          tableClassName='ledger-log-table'
          emptyTitle={t('No logs in this period')}
        />
      </div>
      <div className='ledger-mobile'>
        <MobileCardList
          table={table}
          isLoading={query.isPending}
          enableRowSelection={false}
          emptyTitle={t('No logs in this period')}
        />
      </div>
      <div className='ledger-pagination'>
        <DataTablePagination table={table} compact />
      </div>
      {selected && (
        <DetailsDialog
          log={selected}
          isAdmin={false}
          isRoot={false}
          open
          onOpenChange={(open) => {
            if (!open) setSelected(null)
          }}
        />
      )}
    </section>
  )
}
