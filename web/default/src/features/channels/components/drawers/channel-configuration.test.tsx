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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { channelSchema, type Channel } from '../../types'
import { ChannelsProvider } from '../channels-provider'
import { ChannelMutateDrawer } from './channel-mutate-drawer'

const originalAuth = useAuthStore.getState().auth
let client: QueryClient
let channel: Channel

function EditingHarness() {
  const [open, setOpen] = useState(true)
  return (
    <QueryClientProvider client={client}>
      <ChannelsProvider>
        <ChannelMutateDrawer
          open={open}
          onOpenChange={setOpen}
          currentRow={channel}
        />
      </ChannelsProvider>
    </QueryClientProvider>
  )
}

test('opening the provider picker and returning preserves drafts on visited configuration pages', async () => {
  const user = userEvent.setup()
  render(<EditingHarness />)
  await screen.findByDisplayValue('Saved channel')
  fireEvent.change(screen.getByLabelText('Base URL'), {
    target: { value: 'https://draft.example' },
  })
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  fireEvent.change(screen.getByLabelText('Remark'), {
    target: { value: 'Provider draft note' },
  })
  await user.click(screen.getByRole('tab', { name: /Connection & Models/ }))
  await user.click(screen.getByRole('button', { name: /Change provider/ }))
  expect(
    screen.queryByRole('button', { name: 'Update Channel' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Back' }))
  expect(screen.getByLabelText('Base URL')).toHaveValue('https://draft.example')
  await user.click(screen.getByRole('button', { name: /Change provider/ }))
  await user.click(screen.getByRole('tab', { name: 'Gateways' }))
  await user.click(screen.getByRole('option', { name: /#64$/ }))
  expect(screen.getByLabelText('Base URL')).toHaveValue('https://draft.example')
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  expect(screen.getByLabelText('Remark')).toHaveValue('Provider draft note')
}, 15000)

beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  channel = channelSchema.parse({
    id: 42,
    name: 'Saved channel',
    type: 1,
    key: '',
    status: 2,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'source-alias',
    group: 'default',
    base_url: 'https://saved.example',
    model_mapping: '{"source-alias":"upstream-model"}',
    param_override: '{"temperature":0,"stream":false}',
    setting: '{"future_setting":false}',
    settings: '{"future_setting":0,"image_output_strategy":"r2"}',
    priority: 0,
    weight: 0,
    auto_ban: 0,
  })
  useAuthStore.setState({
    auth: {
      ...originalAuth,
      user: { id: 1, username: 'root', role: ROLE.SUPER_ADMIN },
    },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/channel/42') {
      return { data: { success: true, data: channel } }
    }
    if (url === '/api/channel/default_base_urls') {
      return {
        data: {
          success: true,
          data: {
            1: 'https://api.openai.com',
            22: 'https://fastgpt.run/api/openapi',
            43: 'https://api.deepseek.com',
            45: 'https://ark.cn-beijing.volces.com',
          },
        },
      }
    }
    if (url === '/api/channel/models') {
      return {
        data: {
          success: true,
          data: [{ id: 'source-alias' }, { id: 'upstream-model' }],
        },
      }
    }
    if (url === '/api/group/') {
      return { data: { success: true, data: ['default'] } }
    }
    if (url === '/api/prefill_group') {
      return { data: { success: true, data: [] } }
    }
    throw new Error(`Unexpected GET ${url}`)
  })
})

afterEach(() => {
  client.clear()
  useAuthStore.setState({ auth: originalAuth })
})

test.each([
  { type: 43, label: 'Base URL', placeholder: 'https://api.deepseek.com' },
  {
    type: 22,
    label: 'Private Deployment URL',
    placeholder: 'https://fastgpt.run/api/openapi',
  },
  { type: 64, label: 'Base URL', placeholder: 'Leave empty to use default' },
])(
  'type $type shows the backend hint without writing it into the saved address',
  async ({ type, label, placeholder }) => {
    channel.type = type
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const user = userEvent.setup()
    render(<EditingHarness />)
    await screen.findByDisplayValue('Saved channel')
    const address = screen.getByLabelText(label)
    await waitFor(() =>
      expect(address).toHaveAttribute('placeholder', placeholder)
    )
    expect(address).toHaveValue('https://saved.example')
    await user.clear(address)
    await user.click(screen.getByRole('button', { name: 'Update Channel' }))
    await waitFor(() => expect(put).toHaveBeenCalledOnce())
    expect(put.mock.calls[0]?.[1]).toMatchObject({ id: 42, type, base_url: '' })
  }
)

test('model mapping help opens on click and Escape returns focus without closing the channel', async () => {
  const user = userEvent.setup()
  render(<EditingHarness />)
  await screen.findByDisplayValue('Saved channel')
  await user.click(screen.getByRole('tab', { name: /Routing & Mapping/ }))
  const trigger = screen.getByRole('button', {
    name: 'How model mapping works',
  })
  await user.click(trigger)
  const help = await screen.findByRole('dialog', { name: 'Request flow' })
  expect(help).toBeVisible()
  expect(within(help).getByText('source-alias')).toBeVisible()
  await user.keyboard('{Escape}')
  await waitFor(() => expect(help).not.toBeInTheDocument())
  expect(trigger).toHaveFocus()
  expect(
    screen.getByRole('dialog', { name: 'Edit ChannelOpenAI' })
  ).toBeVisible()
})

test('switching configuration pages keeps unsaved fields and preserves stored credentials and extension settings on save', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const user = userEvent.setup()
  render(<EditingHarness />)
  await screen.findByDisplayValue('Saved channel')
  fireEvent.change(screen.getByLabelText('Base URL'), {
    target: { value: 'https://draft.example' },
  })
  await user.click(screen.getByRole('tab', { name: /Routing & Mapping/ }))
  fireEvent.change(screen.getByLabelText('Priority'), {
    target: { value: '7' },
  })
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  fireEvent.change(screen.getByLabelText('Remark'), {
    target: { value: 'Draft note' },
  })
  await user.click(screen.getByRole('tab', { name: /Request & Response/ }))
  expect(screen.getByLabelText('Priority')).not.toBeVisible()
  await user.click(screen.getByRole('tab', { name: /Connection & Models/ }))
  expect(screen.getByLabelText('Base URL')).toHaveValue('https://draft.example')
  await user.click(screen.getByRole('tab', { name: /Routing & Mapping/ }))
  expect(screen.getByLabelText('Priority')).toHaveValue(7)
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  expect(screen.getByLabelText('Remark')).toHaveValue('Draft note')
  await user.click(screen.getByRole('button', { name: 'Update Channel' }))
  await waitFor(() => expect(put).toHaveBeenCalledOnce())
  const payload = put.mock.calls[0]?.[1] as Record<string, unknown>
  expect(payload).toMatchObject({
    id: 42,
    type: 1,
    models: 'source-alias',
    base_url: 'https://draft.example',
    priority: 7,
    weight: 0,
    auto_ban: 0,
    remark: 'Draft note',
  })
  expect(payload).not.toHaveProperty('key')
  expect(JSON.parse(payload.setting as string).future_setting).toBe(false)
  expect(JSON.parse(payload.settings as string)).toMatchObject({
    future_setting: 0,
    image_output_strategy: 'r2',
  })
  expect(JSON.parse(payload.param_override as string)).toEqual({
    temperature: 0,
    stream: false,
  })
  expect(JSON.parse(payload.model_mapping as string)).toEqual({
    'source-alias': 'upstream-model',
  })
}, 15000)

test('saving from another page locates and focuses a required connection field without sending an update', async () => {
  const put = vi.spyOn(api, 'put')
  const user = userEvent.setup()
  render(<EditingHarness />)
  const name = await screen.findByDisplayValue('Saved channel')
  await user.clear(name)
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  await user.click(screen.getByRole('button', { name: 'Update Channel' }))
  await waitFor(() =>
    expect(
      screen.getByRole('tab', { name: /Connection & Models/ })
    ).toHaveAttribute('aria-selected', 'true')
  )
  await waitFor(() => expect(name).toHaveFocus())
  expect(name).toBeVisible()
  expect(put).not.toHaveBeenCalled()
})

test('an invalid proxy locates Other Settings after submission from Connection & Models', async () => {
  const put = vi.spyOn(api, 'put')
  const user = userEvent.setup()
  render(<EditingHarness />)
  await screen.findByDisplayValue('Saved channel')
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  const proxy = screen.getByLabelText('Proxy Address')
  fireEvent.change(proxy, { target: { value: 'invalid-proxy' } })
  await user.click(screen.getByRole('tab', { name: /Connection & Models/ }))
  await user.click(screen.getByRole('button', { name: 'Update Channel' }))
  await waitFor(() =>
    expect(screen.getByRole('tab', { name: /Other Settings/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  )
  await waitFor(() => expect(proxy).toHaveFocus())
  expect(proxy).toBeVisible()
  expect(put).not.toHaveBeenCalled()
})

test('an operator can edit routing while sensitive fields remain disabled across all pages', async () => {
  useAuthStore.setState({
    auth: {
      ...originalAuth,
      user: {
        id: 10,
        username: 'operator',
        role: ROLE.ADMIN,
        permissions: {
          admin_permissions: {
            channel: { read: true, write: true, operate: true },
          },
        },
      },
    },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const user = userEvent.setup()
  render(<EditingHarness />)
  await screen.findByDisplayValue('Saved channel')
  expect(screen.getByLabelText('Base URL')).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Change provider' })).toBeDisabled()
  await user.click(screen.getByRole('tab', { name: /Other Settings/ }))
  expect(screen.getByLabelText('Proxy Address')).toBeDisabled()
  await user.click(screen.getByRole('tab', { name: /Routing & Mapping/ }))
  fireEvent.change(screen.getByLabelText('Priority'), {
    target: { value: '8' },
  })
  await user.click(screen.getByRole('button', { name: 'Update Channel' }))
  await waitFor(() => expect(put).toHaveBeenCalledOnce())
  expect(put.mock.calls[0]?.[1]).toMatchObject({ id: 42, priority: 8 })
  for (const field of [
    'key',
    'setting',
    'settings',
    'base_url',
    'param_override',
    'header_override',
  ]) {
    expect(put.mock.calls[0]?.[1]).not.toHaveProperty(field)
  }
})
