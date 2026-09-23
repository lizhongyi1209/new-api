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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useEffect } from 'react'
import { expect, test } from 'vitest'

import { LOG_TYPE_ENUM } from '../constants'
import { usageLogSchema, type UsageLog } from '../data/schema'
import { CommonLogMobileCard } from './common-log-mobile-card'
import { UsageLogsMobileList } from './usage-logs-mobile-card'
import { UsageLogsProvider, useUsageLogsContext } from './usage-logs-provider'

const columns = [
  { accessorKey: 'model_name' },
  { accessorKey: 'quota' },
  { accessorKey: 'created_at' },
  { id: 'user', accessorKey: 'username' },
  { accessorKey: 'channel' },
  { accessorKey: 'token_name' },
  { accessorKey: 'prompt_tokens' },
  { accessorKey: 'use_time' },
  { accessorKey: 'is_stream' },
  { accessorKey: 'content' },
]
const log = usageLogSchema.parse({
  id: 1,
  user_id: 42,
  created_at: 1789356000,
  type: LOG_TYPE_ENUM.CONSUME,
  content: 'Existing log detail',
  model_name: 'gpt-requested-model-with-a-long-name',
  username: 'private-user-name',
  channel: 1001,
  channel_name: 'private-channel-name',
  token_name: 'private-token-name',
  group: 'private-group-name',
  prompt_tokens: 10,
  completion_tokens: 20,
  other: JSON.stringify({
    is_model_mapped: true,
    upstream_model_name: 'actual-upstream-model',
    group_ratio: 2,
  }),
})

function Card(props: {
  log: UsageLog
  sensitiveVisible: boolean
  hiddenColumns?: string[]
}) {
  const context = useUsageLogsContext()
  const setSensitiveVisible = context.setSensitiveVisible
  useEffect(
    () => setSensitiveVisible(props.sensitiveVisible),
    [setSensitiveVisible, props.sensitiveVisible]
  )
  const table = useReactTable({
    data: [props.log],
    columns,
    state: {
      columnVisibility: Object.fromEntries(
        (props.hiddenColumns ?? []).map((id) => [id, false])
      ),
    },
    getCoreRowModel: getCoreRowModel(),
  })
  const cells = new Map(
    table
      .getRowModel()
      .rows[0].getVisibleCells()
      .map((cell) => [cell.column.id, cell])
  )
  return (
    <>
      <CommonLogMobileCard log={props.log} cells={cells} />
      <output>
        {context.userInfoDialogOpen
          ? `Selected user: ${context.selectedUserId}`
          : ''}
      </output>
    </>
  )
}

test('mobile model details reveal the full requested and mapped model and restore focus', async () => {
  const user = userEvent.setup()
  render(
    <UsageLogsProvider>
      <Card log={log} sensitiveVisible />
    </UsageLogsProvider>
  )
  const trigger = screen.getByRole('button', {
    name: `Model: ${log.model_name}`,
  })
  await user.click(trigger)
  const dialog = screen.getByRole('dialog', { name: 'Model' })
  expect(within(dialog).getByText(log.model_name)).toBeVisible()
  expect(within(dialog).getByText('actual-upstream-model')).toBeVisible()
  await user.keyboard('{Escape}')
  expect(trigger).toHaveFocus()
})

test('masking metadata closes its detail and removes user, channel, token, group and ratio text', async () => {
  const user = userEvent.setup()
  const view = render(
    <UsageLogsProvider>
      <Card log={log} sensitiveVisible />
    </UsageLogsProvider>
  )
  await user.click(
    screen.getByRole('button', { name: `Token: ${log.token_name}` })
  )
  expect(screen.getByRole('dialog', { name: 'Token' })).toBeVisible()
  view.rerender(
    <UsageLogsProvider>
      <Card log={log} sensitiveVisible={false} />
    </UsageLogsProvider>
  )
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  for (const value of [
    log.username,
    log.channel_name,
    log.token_name,
    log.group,
  ]) {
    expect(screen.queryByText(value ?? '')).not.toBeInTheDocument()
  }
  expect(screen.queryByText(/Group Ratio/)).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: /^Token:/ })
  ).not.toBeInTheDocument()
})

test('hidden columns stay absent from mobile summaries even when raw log data contains them', () => {
  render(
    <UsageLogsProvider>
      <Card
        log={log}
        sensitiveVisible
        hiddenColumns={[
          'model_name',
          'quota',
          'created_at',
          'user',
          'channel',
          'token_name',
          'prompt_tokens',
          'use_time',
          'is_stream',
        ]}
      />
    </UsageLogsProvider>
  )
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(screen.queryByText(log.model_name)).not.toBeInTheDocument()
  expect(screen.getByText(log.content)).toBeVisible()
})

test('cache-only logs retain input/output and split cache write totals', () => {
  render(
    <UsageLogsProvider>
      <Card
        log={{
          ...log,
          prompt_tokens: 0,
          completion_tokens: 0,
          other: JSON.stringify({
            cache_tokens: 7,
            cache_creation_tokens_5m: 2,
            cache_creation_tokens_1h: 3,
          }),
        }}
        sensitiveVisible
      />
    </UsageLogsProvider>
  )
  expect(screen.getByText(/Input/)).toBeVisible()
  expect(screen.getByText(/Output/)).toBeVisible()
  expect(screen.getByText('Cache ↓ 7')).toBeVisible()
  expect(screen.getByText('Cache ↑ 5')).toBeVisible()
})

test('user detail continues to open the existing user information workflow', async () => {
  const user = userEvent.setup()
  render(
    <UsageLogsProvider>
      <Card log={log} sensitiveVisible />
    </UsageLogsProvider>
  )
  await user.click(
    screen.getByRole('button', { name: `User: ${log.username}` })
  )
  await user.click(screen.getByRole('button', { name: 'User Information' }))
  expect(screen.getByText('Selected user: 42')).toBeVisible()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('asynchronous consume logs describe async mode without a non-stream TPS placeholder', () => {
  render(
    <UsageLogsProvider>
      <Card
        log={{ ...log, other: JSON.stringify({ is_task: true }) }}
        sensitiveVisible
      />
    </UsageLogsProvider>
  )
  expect(screen.getByText('Async')).toBeVisible()
  expect(screen.queryByText('Non-stream')).not.toBeInTheDocument()
  expect(screen.queryByText('—')).not.toBeInTheDocument()
})

function LogList(props: { logs: UsageLog[] }) {
  const table = useReactTable({
    data: props.logs,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  return <UsageLogsMobileList table={table} logCategory='common' />
}

test('refreshing a mobile log list cannot retarget an open detail to another record at the same row position', async () => {
  const user = userEvent.setup()
  const view = render(
    <UsageLogsProvider>
      <LogList logs={[log]} />
    </UsageLogsProvider>
  )
  await user.click(
    screen.getByRole('button', { name: `Model: ${log.model_name}` })
  )
  expect(screen.getByRole('dialog', { name: 'Model' })).toBeVisible()
  const replacement = { ...log, id: 2, model_name: 'replacement-log-model' }
  view.rerender(
    <UsageLogsProvider>
      <LogList logs={[replacement]} />
    </UsageLogsProvider>
  )
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Model: replacement-log-model' })
  ).toBeVisible()
})
