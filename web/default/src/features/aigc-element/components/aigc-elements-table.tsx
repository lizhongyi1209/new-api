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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useMediaQuery } from '@/hooks'
import { useTranslation } from 'react-i18next'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { getAigcElements } from '../api'
import { useAigcElementsColumns } from './aigc-elements-columns'
import { useAigcElements } from './aigc-elements-provider'

const route = getRouteApi('/_authenticated/aigc-element/')

export function AigcElementsTable() {
  const { t } = useTranslation()
  const columns = useAigcElementsColumns()
  const { refreshTrigger, showAll } = useAigcElements()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const { pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: route.useSearch(),
      navigate: route.useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'aigc-elements',
      pagination.pageIndex + 1,
      pagination.pageSize,
      showAll,
      refreshTrigger,
    ],
    queryFn: async () => {
      const result = await getAigcElements({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        all: showAll,
      })
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const { table } = useDataTable({
    data: data?.items || [],
    columns,
    pagination,
    onPaginationChange,
    manualPagination: true,
    totalCount: data?.total || 0,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Subjects Found')}
      emptyDescription={t(
        'No subjects yet. Create your first subject to get started.'
      )}
      skeletonKeyPrefix='aigc-elements-skeleton'
      applyHeaderSize
    />
  )
}
