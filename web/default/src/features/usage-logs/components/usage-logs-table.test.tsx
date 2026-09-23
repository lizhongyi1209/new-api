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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import {
  UsageLogsProvider,
  resolveLogsViewAccess,
  useLogsViewScope,
} from './usage-logs-provider'
import { UsageLogsTable } from './usage-logs-table'

const mocks = vi.hoisted(() => ({ fetchLogs: vi.fn(), role: 100 }))
const columns = [
  { accessorKey: 'model_name', header: 'Model' },
  { accessorKey: 'token_name', header: 'Token' },
]
const search = {}
const filters: never[] = []
const pagination = { pageIndex: 0, pageSize: 20 }
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: <T,>(
    selector: (state: { auth: { user: { role: number } } }) => T
  ) => selector({ auth: { user: { role: mocks.role } } }),
}))
vi.mock('@tanstack/react-router', () => ({
  getRouteApi: () => ({ useSearch: () => search, useNavigate: () => vi.fn() }),
}))
vi.mock('@/hooks/use-table-url-state', () => ({
  useTableUrlState: () => ({
    columnFilters: filters,
    onColumnFiltersChange: vi.fn(),
    pagination,
    onPaginationChange: vi.fn(),
    ensurePageInRange: vi.fn(),
  }),
}))
vi.mock('../lib/utils', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/utils')>()),
  fetchLogsByCategory: mocks.fetchLogs,
}))
vi.mock('../lib/columns', () => ({ useColumnsByCategory: () => columns }))
vi.mock('./common-logs-filter-bar', () => ({ CommonLogsFilterBar: () => null }))
vi.mock('./task-logs-filter-bar', () => ({ TaskLogsFilterBar: () => null }))
vi.mock('./usage-logs-actions', () => ({ UsageLogsActions: () => null }))

function ScopeControls() {
  const scope = useLogsViewScope()
  return (
    <button
      type='button'
      onClick={() =>
        scope.setViewScope(scope.viewScope === 'all' ? 'self' : 'all')
      }
    >
      Switch scope
    </button>
  )
}

beforeEach(() => {
  mocks.fetchLogs.mockReset()
  mocks.role = 100
  localStorage.clear()
})

test('switching to self scope clears previous admin rows while the new request is pending and restores independent column preferences', async () => {
  let resolveSelf!: (value: unknown) => void
  const pendingSelf = new Promise((resolve) => {
    resolveSelf = resolve
  })
  mocks.fetchLogs.mockImplementation(({ isAdmin }) =>
    isAdmin
      ? Promise.resolve({
          success: true,
          data: {
            items: [{ model_name: 'admin-record', token_name: 'admin-token' }],
            total: 1,
          },
        })
      : pendingSelf
  )
  localStorage.setItem(
    'usage-logs:common:root:column-visibility',
    JSON.stringify({ token_name: false })
  )
  localStorage.setItem(
    'usage-logs:common:self:column-visibility',
    JSON.stringify({ token_name: true })
  )
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <ScopeControls />
        <UsageLogsTable logCategory='common' />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  expect(
    await screen.findByRole('cell', { name: 'admin-record' })
  ).toBeVisible()
  expect(
    screen.queryByRole('columnheader', { name: 'Token' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Switch scope' }))
  expect(
    screen.queryByRole('cell', { name: 'admin-record' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('columnheader', { name: 'Token' })).toBeVisible()
  resolveSelf({
    success: true,
    data: {
      items: [{ model_name: 'own-record', token_name: 'own-token' }],
      total: 1,
    },
  })
  expect(await screen.findByRole('cell', { name: 'own-token' })).toBeVisible()
  await waitFor(() =>
    expect(
      JSON.parse(
        localStorage.getItem('usage-logs:common:root:column-visibility') ?? '{}'
      )
    ).toEqual({ token_name: false })
  )
  client.clear()
})

test('root and admin views never share cached rows or column preferences despite using the same admin endpoint', async () => {
  let resolveAdmin!: (value: unknown) => void
  const pendingAdmin = new Promise((resolve) => {
    resolveAdmin = resolve
  })
  mocks.fetchLogs.mockImplementation(() =>
    mocks.role === 100
      ? Promise.resolve({
          success: true,
          data: {
            items: [{ model_name: 'root-record', token_name: 'root-token' }],
            total: 1,
          },
        })
      : pendingAdmin
  )
  localStorage.setItem(
    'usage-logs:common:root:column-visibility',
    JSON.stringify({ token_name: false })
  )
  localStorage.setItem(
    'usage-logs:common:admin:column-visibility',
    JSON.stringify({ token_name: true })
  )
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const element = (
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <UsageLogsTable logCategory='common' />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  const view = render(element)
  expect(await screen.findByRole('cell', { name: 'root-record' })).toBeVisible()
  mocks.role = 10
  view.rerender(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <UsageLogsTable logCategory='common' />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  expect(
    screen.queryByRole('cell', { name: 'root-record' })
  ).not.toBeInTheDocument()
  await waitFor(() => expect(mocks.fetchLogs).toHaveBeenCalledTimes(2))
  resolveAdmin({
    success: true,
    data: {
      items: [{ model_name: 'admin-record', token_name: 'admin-token' }],
      total: 1,
    },
  })
  expect(await screen.findByRole('cell', { name: 'admin-token' })).toBeVisible()
  expect(
    JSON.parse(
      localStorage.getItem('usage-logs:common:root:column-visibility') ?? '{}'
    )
  ).toEqual({ token_name: false })
  client.clear()
})

test.each([
  [1, 'all', 'self'],
  [10, 'all', 'admin'],
  [100, 'all', 'root'],
  [100, 'self', 'self'],
] as const)(
  'role %s with scope %s resolves independent log access %s',
  (role, scope, access) => {
    expect(resolveLogsViewAccess(role, scope)).toBe(access)
  }
)

test('a failed logs refresh preserves the last successful rows and records a query error', async () => {
  const previous = {
    items: [{ model_name: 'retained-record', token_name: 'retained-token' }],
    total: 1,
  }
  mocks.fetchLogs.mockResolvedValueOnce({ success: true, data: previous })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <UsageLogsTable logCategory='common' />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
  expect(
    await screen.findByRole('cell', { name: 'retained-record' })
  ).toBeVisible()
  mocks.fetchLogs.mockResolvedValue({
    success: false,
    message: 'Logs unavailable',
    data: { items: [], total: 0 },
  })
  await client.refetchQueries({ queryKey: ['logs'] })
  expect(screen.getByRole('cell', { name: 'retained-record' })).toBeVisible()
  const query = client
    .getQueryCache()
    .find({ queryKey: ['logs'], exact: false })
  expect(query?.state.status).toBe('error')
  expect(query?.state.data).toEqual(previous)
  client.clear()
})
