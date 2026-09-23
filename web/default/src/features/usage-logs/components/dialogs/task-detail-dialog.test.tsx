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
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { getTaskAuditDetails } from '../../api'
import type { TaskLog } from '../../types'
import { TaskDetailDialog } from './task-detail-dialog'

vi.mock('../../api', () => ({ getTaskAuditDetails: vi.fn() }))
const log: TaskLog = {
  id: 1,
  task_id: 'task-local',
  user_id: 2,
  username: 'test-user',
  channel_id: 12,
  platform: 'generateImage',
  channel_type_name: 'Local custom image',
  action: 'GENERATE',
  status: 'SUCCESS',
  submit_time: 1700000000,
  finish_time: 1700000008,
  quota: 500000,
  group: 'default',
  progress: '100%',
  properties: {
    origin_model_name: 'original-image',
    upstream_model_name: 'actual-image',
  },
}
let client: QueryClient

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  vi.mocked(getTaskAuditDetails).mockReset()
  vi.mocked(getTaskAuditDetails).mockResolvedValue({
    schema_version: 1,
    task_id: log.task_id,
    request_id: 'gateway-request',
    upstream_task_id: 'upstream-task',
    channel_id: 12,
    quota: 500000,
  })
})
afterEach(() => client.clear())

test('shows self task fields without requesting or displaying administrator audit data', () => {
  client.setQueryData(['usage-logs', 'task-audit', log.task_id, true], {
    request_id: 'private-request',
  })
  render(
    <QueryClientProvider client={client}>
      <TaskDetailDialog
        log={log}
        open
        isAdmin={false}
        sensitiveVisible={false}
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
  expect(screen.getByText('original-image')).toBeInTheDocument()
  expect(screen.getByText('actual-image')).toBeInTheDocument()
  expect(screen.queryByText('Admin Only')).not.toBeInTheDocument()
  expect(screen.queryByText('private-request')).not.toBeInTheDocument()
  expect(getTaskAuditDetails).not.toHaveBeenCalled()
})

test('loads administrator diagnostics while respecting the identity visibility setting', async () => {
  render(
    <QueryClientProvider client={client}>
      <TaskDetailDialog
        log={log}
        open
        isAdmin
        sensitiveVisible={false}
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
  expect(await screen.findByText('gateway-request')).toBeInTheDocument()
  expect(screen.getByText('upstream-task')).toBeInTheDocument()
  expect(screen.queryByText('test-user')).not.toBeInTheDocument()
  expect(screen.queryByText('#12')).not.toBeInTheDocument()
  expect(getTaskAuditDetails).toHaveBeenCalledWith(
    log.task_id,
    expect.any(AbortSignal)
  )
})

test('keeps basic task information usable after an audit failure and allows retry', async () => {
  vi.mocked(getTaskAuditDetails).mockRejectedValueOnce(new Error('Not found'))
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={client}>
      <TaskDetailDialog
        log={log}
        open
        isAdmin
        sensitiveVisible
        onOpenChange={() => undefined}
      />
    </QueryClientProvider>
  )
  expect(
    await screen.findByText('Failed to load task details')
  ).toBeInTheDocument()
  expect(screen.getByText('task-local')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /Retry/ }))
  await waitFor(() =>
    expect(screen.getByText('gateway-request')).toBeInTheDocument()
  )
  expect(getTaskAuditDetails).toHaveBeenCalledTimes(2)
})
