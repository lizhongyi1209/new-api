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
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getAllTaskLogs } from '../api'

afterEach(() => vi.restoreAllMocks())

test('admin task logs request the registered list route', async () => {
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 20 },
    },
  })

  await getAllTaskLogs({ p: 1, page_size: 20 })

  expect(get).toHaveBeenCalledOnce()
  const requestUrl = new URL(String(get.mock.calls[0]?.[0]), 'http://localhost')
  expect(requestUrl.pathname).toBe('/api/task/')
  expect(requestUrl.searchParams.get('p')).toBe('1')
  expect(requestUrl.searchParams.get('page_size')).toBe('20')
})
