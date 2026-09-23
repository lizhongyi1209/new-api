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
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { DataTablePagination } from './pagination'

const data: { id: number }[] = []
const columns = [{ accessorKey: 'id' }]

function PaginationHarness(props: { total: number }) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    rowCount: props.total,
    initialState: { pagination: { pageIndex: 0, pageSize: 20 } },
  })
  return <DataTablePagination table={table} compact />
}

test('compact pagination advances and returns while respecting both boundaries', async () => {
  const user = userEvent.setup()
  render(<PaginationHarness total={75} />)
  const previous = screen.getByRole('button', { name: 'Go to previous page' })
  const next = screen.getByRole('button', { name: 'Go to next page' })
  expect(previous).toBeDisabled()
  expect(screen.getByText('1 / 4')).toBeVisible()
  await user.click(next)
  expect(screen.getByText('2 / 4')).toBeVisible()
  await user.click(previous)
  expect(screen.getByText('1 / 4')).toBeVisible()
  await user.click(next)
  await user.click(next)
  await user.click(next)
  expect(screen.getByText('4 / 4')).toBeVisible()
  expect(next).toBeDisabled()
  expect(previous).toBeEnabled()
})

test('empty compact lists display one page and disable navigation', () => {
  render(<PaginationHarness total={0} />)
  expect(screen.getByText('1 / 1')).toBeVisible()
  expect(
    screen.getByRole('button', { name: 'Go to previous page' })
  ).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Go to next page' })).toBeDisabled()
})
