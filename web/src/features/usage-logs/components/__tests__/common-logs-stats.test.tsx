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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { formatLogQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { CommonLogsStats } from '../common-logs-stats'
import { UsageLogsProvider } from '../usage-logs-provider'

function StatsFixture() {
  return (
    <UsageLogsProvider>
      <CommonLogsStats />
    </UsageLogsProvider>
  )
}

function renderStats(role: number) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'tester', role })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { quota: 500000, refund_quota: 250000, rpm: 2, tpm: 100 },
    },
  })

  const root = createRootRoute()
  const auth = createRoute({ getParentRoute: () => root, id: '_authenticated' })
  const logs = createRoute({
    getParentRoute: () => auth,
    path: '/usage-logs/$section',
    component: StatsFixture,
    validateSearch: (search: Record<string, unknown>) => search,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([logs])]),
    history: createMemoryHistory({ initialEntries: ['/usage-logs/common'] }),
  })
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.setUser(null)
})

it('shows the returned refund total to a root administrator', async () => {
  renderStats(ROLE.SUPER_ADMIN)
  expect(await screen.findByText('Refund quota')).toBeVisible()
  expect(screen.getByText(formatLogQuota(250000))).toBeVisible()
})

it('keeps the refund total hidden from an ordinary administrator', async () => {
  renderStats(ROLE.ADMIN)
  expect(await screen.findByText('Usage')).toBeVisible()
  expect(screen.queryByText('Refund quota')).toBeNull()
})
