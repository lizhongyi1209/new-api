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
import { act, renderHook } from '@testing-library/react'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { toast } from 'sonner'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/http-client'

import { useBillingHistory } from './use-billing-history'

const access = vi.hoisted(() => ({ isAdmin: false }))
vi.mock('@/hooks/use-admin', () => ({ useIsAdmin: () => access.isAdmin }))

interface PendingRequest {
  config: InternalAxiosRequestConfig
  resolve: (response: AxiosResponse) => void
}
const pending: PendingRequest[] = []
const originalAdapter = api.defaults.adapter

beforeEach(() => {
  vi.useFakeTimers()
  access.isAdmin = false
  pending.length = 0
  vi.spyOn(toast, 'error').mockReturnValue('fixture-error')
  vi.spyOn(toast, 'success').mockReturnValue('fixture-success')
  api.defaults.adapter = (config) =>
    new Promise((resolve) => {
      pending.push({ config, resolve })
    })
})
afterEach(() => {
  api.defaults.adapter = originalAdapter
  vi.useRealTimers()
})

async function respond(request: PendingRequest | undefined, data: unknown) {
  expect(request).toBeDefined()
  if (!request) throw new Error('Missing fixture request')
  await act(async () => {
    request.resolve({
      data,
      config: request.config,
      status: 200,
      statusText: 'OK',
      headers: {},
    })
  })
}
function history(tradeNo: string, total = 1) {
  return {
    success: true,
    data: { items: [{ id: 1, trade_no: tradeNo }], total },
  }
}

test('debounces typing and searches the final keyword on the first page', async () => {
  const hook = renderHook(() => useBillingHistory({ initialPage: 3 }))
  await act(async () => {})
  await respond(pending[0], history('initial'))
  act(() => hook.result.current.handleSearch('fixture'))
  act(() => vi.advanceTimersByTime(300))
  act(() => hook.result.current.handleSearch('fixture-order'))
  act(() => vi.advanceTimersByTime(499))
  expect(pending).toHaveLength(1)
  await act(async () => vi.advanceTimersByTime(1))
  expect(pending).toHaveLength(2)
  expect(pending[1]?.config.url).toBe(
    '/api/user/topup/self?p=1&page_size=10&keyword=fixture-order'
  )
  await respond(pending[1], history('matched'))
  expect(hook.result.current.records[0]?.trade_no).toBe('matched')
  expect(hook.result.current.page).toBe(1)
})

test('ignores old search data and does not end loading for a newer request', async () => {
  const hook = renderHook(() => useBillingHistory())
  await act(async () => {})
  act(() => hook.result.current.handleSearch('new-order'))
  await act(async () => vi.advanceTimersByTime(500))
  await respond(pending[0], history('obsolete', 99))
  expect(hook.result.current.records).toEqual([])
  expect(hook.result.current.loading).toBe(true)
  await respond(pending[1], history('new-order', 2))
  expect(hook.result.current.records[0]?.trade_no).toBe('new-order')
  expect(hook.result.current.total).toBe(2)
  expect(hook.result.current.loading).toBe(false)
})

test('ignores an old business failure after the latest page succeeds', async () => {
  const hook = renderHook(() => useBillingHistory())
  await act(async () => {})
  act(() => hook.result.current.handlePageSizeChange(20))
  await act(async () => {})
  expect(pending[1]?.config.url).toBe('/api/user/topup/self?p=1&page_size=20')
  await respond(pending[1], history('latest-page'))
  await respond(pending[0], { success: false, message: 'obsolete failure' })
  expect(hook.result.current.records[0]?.trade_no).toBe('latest-page')
  expect(toast.error).not.toHaveBeenCalled()
})

test('unmounting suppresses late request errors', async () => {
  const hook = renderHook(() => useBillingHistory())
  await act(async () => {})
  hook.unmount()
  await respond(pending[0], { success: false, message: 'late failure' })
  expect(toast.error).not.toHaveBeenCalled()
})

test('a completed admin order refreshes the current search rather than its old search', async () => {
  access.isAdmin = true
  const hook = renderHook(() => useBillingHistory())
  await act(async () => {})
  await respond(pending[0], history('initial'))
  let completion: Promise<boolean> | undefined
  act(() => {
    completion = hook.result.current.handleCompleteOrder('synthetic-order')
  })
  await act(async () => {})
  expect(pending[1]?.config.url).toBe('/api/user/topup/complete')
  act(() => hook.result.current.handleSearch('current'))
  await act(async () => vi.advanceTimersByTime(500))
  await respond(pending[2], history('current'))
  await respond(pending[1], { success: true })
  expect(pending[3]?.config.url).toBe(
    '/api/user/topup?p=1&page_size=10&keyword=current'
  )
  await respond(pending[3], history('refreshed'))
  await expect(completion).resolves.toBe(true)
  expect(hook.result.current.records[0]?.trade_no).toBe('refreshed')
})

test('a failed order completion does not refresh or report success', async () => {
  access.isAdmin = true
  const hook = renderHook(() => useBillingHistory())
  await act(async () => {})
  await respond(pending[0], history('pending-order'))
  let completion: Promise<boolean> | undefined
  act(() => {
    completion = hook.result.current.handleCompleteOrder('synthetic-order')
  })
  await act(async () => {})
  await respond(pending[1], { success: false, message: 'order unavailable' })
  await expect(completion).resolves.toBe(false)
  expect(pending).toHaveLength(2)
  expect(toast.success).not.toHaveBeenCalled()
  expect(toast.error).toHaveBeenCalledTimes(1)
  expect(hook.result.current.records[0]?.trade_no).toBe('pending-order')
})
