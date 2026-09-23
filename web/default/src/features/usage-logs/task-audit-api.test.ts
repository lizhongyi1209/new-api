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
import type { AxiosAdapter } from 'axios'
import { toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getTaskAuditDetails } from './api'

const originalAdapter = api.defaults.adapter
afterEach(() => {
  api.defaults.adapter = originalAdapter
  vi.restoreAllMocks()
})

test('reopening task details gets a fresh request after the previous dialog was canceled', async () => {
  const errorToast = vi.spyOn(toast, 'error')
  const details = {
    schema_version: 1,
    task_id: 'task_reopen',
    channel_id: 12,
    quota: 500000,
    request_id: 'gateway-request',
  }
  api.defaults.adapter = vi.fn<AxiosAdapter>(async (config) => ({
    data: { success: true, data: details },
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  }))
  const firstController = new AbortController()
  const first = getTaskAuditDetails(details.task_id, firstController.signal)
  firstController.abort()
  const reopened = getTaskAuditDetails(
    details.task_id,
    new AbortController().signal
  )
  await expect(first).rejects.toMatchObject({ name: 'CanceledError' })
  await expect(reopened).resolves.toEqual(details)
  expect(errorToast).not.toHaveBeenCalled()
})

test('rejects a business failure instead of presenting empty administrator diagnostics', async () => {
  api.defaults.adapter = vi.fn<AxiosAdapter>(async (config) => ({
    data: { success: false, message: 'task not found' },
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  }))
  await expect(getTaskAuditDetails('task_missing')).rejects.toThrow(
    'task not found'
  )
})
