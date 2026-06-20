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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { formatTimestampToDate } from '@/lib/format'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { MaskedValueDisplay } from '@/components/masked-value-display'
import { ELEMENT_STATUSES } from '../constants'
import type { AigcElement } from '../types'
import { DataTableRowActions } from './data-table-row-actions'
import { ElementImagesCell } from './element-images-cell'

export function useAigcElementsColumns(): ColumnDef<AigcElement>[] {
  const { t } = useTranslation()
  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <TableId value={row.getValue('id') as number} className='w-[60px]' />
      ),
      size: 70,
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <span className='font-medium'>{row.getValue('name')}</span>
      ),
      size: 160,
    },
    {
      id: 'images',
      header: t('Images'),
      cell: ({ row }) => <ElementImagesCell element={row.original} />,
      enableSorting: false,
      size: 200,
    },
    {
      accessorKey: 'description',
      header: t('Description'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='text-muted-foreground line-clamp-2 max-w-[260px] text-sm'>
          {row.getValue('description')}
        </span>
      ),
      enableSorting: false,
      size: 280,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const status = String(row.getValue('status'))
        const config = ELEMENT_STATUSES[status]
        return (
          <StatusBadge
            label={config ? t(config.labelKey) : status}
            variant={config?.variant ?? 'neutral'}
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 110,
    },
    {
      accessorKey: 'platform',
      header: t('Platform'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const platform = String(row.getValue('platform') || 'kling')
        return (
          <StatusBadge
            label={platform}
            variant='info'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      enableSorting: false,
      size: 100,
    },
    {
      accessorKey: 'element_id',
      header: t('Element ID'),
      cell: function ElementIdCell({ row }) {
        const elementId = String(row.getValue('element_id') || '')
        if (!elementId) {
          return <span className='text-muted-foreground text-sm'>-</span>
        }
        const masked =
          elementId.length > 16
            ? `${elementId.slice(0, 10)}***${elementId.slice(-4)}`
            : elementId
        return (
          <MaskedValueDisplay
            label={t('Full Element ID')}
            fullValue={elementId}
            maskedValue={masked}
            copyTooltip={t('Copy Element ID')}
            copyAriaLabel={t('Copy Element ID')}
          />
        )
      },
      enableSorting: false,
      size: 240,
    },
    {
      accessorKey: 'reference_type',
      header: t('Reference Type'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='font-mono text-xs'>
          {row.getValue('reference_type')}
        </span>
      ),
      enableSorting: false,
      size: 120,
    },
    {
      accessorKey: 'username',
      header: t('Owner'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const username = String(row.original.username || '')
        const userId = row.original.user_id
        const tokenName = String(row.original.token_name || '')
        const tokenId = row.original.token_id || 0
        return (
          <div className='flex flex-col gap-0.5 text-sm'>
            <span className='font-medium'>
              {username || t('User {{id}}', { id: userId })}
            </span>
            {tokenId > 0 ? (
              <span className='text-muted-foreground text-xs'>
                {tokenName || t('Token')} (#{tokenId})
              </span>
            ) : (
              <span className='text-muted-foreground text-xs'>
                {t('Console')}
              </span>
            )}
          </div>
        )
      },
      enableSorting: false,
      size: 160,
    },
    {
      accessorKey: 'created_at',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[150px] font-mono text-sm'>
          {formatTimestampToDate(row.getValue('created_at'))}
        </div>
      ),
      size: 170,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      cell: ({ row }) => <DataTableRowActions row={row} />,
      meta: { pinned: 'right' as const },
      size: 88,
    },
  ]
}
