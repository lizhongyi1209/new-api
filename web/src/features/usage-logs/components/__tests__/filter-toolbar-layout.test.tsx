/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { Button } from '@/components/ui/button'

import { LogsFilterToolbar } from '../logs-filter-toolbar'
import { UsageLogsActions } from '../usage-logs-actions'

function FilterFixture(props: { showExport: boolean }) {
  const table = useReactTable({
    data: [],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <LogsFilterToolbar
      table={table}
      primaryFilters={<span>Date Range</span>}
      mobilePinnedFilters={<span>Date Range</span>}
      actionStart={<Button>Hide</Button>}
      leadingAction={
        props.showExport ? (
          <UsageLogsActions isAdmin={false} searchParams={{}} />
        ) : null
      }
      hasActiveFilters={false}
      onReset={() => {}}
      onSearch={() => {}}
    />
  )
}

afterEach(() => vi.restoreAllMocks())

it.each([
  { mobile: false, showExport: true },
  { mobile: true, showExport: true },
  { mobile: false, showExport: false },
  { mobile: true, showExport: false },
])(
  'keeps log actions inside the full filter panel when mobile=$mobile and showExport=$showExport',
  ({ mobile, showExport }) => {
    const original = window.matchMedia
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      ...original(query),
      matches: mobile && query === '(max-width: 640px)',
    }))

    const { container } = render(
      <QueryClientProvider client={new QueryClient()}>
        <FilterFixture showExport={showExport} />
      </QueryClientProvider>
    )
    const panel = container.firstElementChild
    expect(panel).not.toBeNull()
    expect(panel).toHaveClass('w-full')
    expect(screen.queryByRole('combobox', { name: /Auto refresh/ })).toBeNull()
    const hide = screen.getByRole('button', { name: 'Hide' })
    if (showExport) {
      const exportButton = screen.getByRole('button', { name: 'Export logs' })
      expect(panel).toContainElement(exportButton)
      expect(
        exportButton.compareDocumentPosition(hide) &
          Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy()
    } else {
      expect(screen.queryByRole('button', { name: 'Export logs' })).toBeNull()
    }
    expect(panel).toContainElement(screen.getByText('Date Range'))
  }
)
