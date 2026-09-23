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
import { QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosAdapter } from 'axios'
import { createElement, type ReactNode } from 'react'
import { toast } from 'sonner'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { ChannelsProvider } from '@/features/channels/components/channels-provider'
import { ChannelMutateDrawer } from '@/features/channels/components/drawers/channel-mutate-drawer'
import { useChannelMutateForm } from '@/features/channels/hooks/use-channel-mutate-form'
import { handleDeleteChannel } from '@/features/channels/lib/channel-actions'
import { transformChannelToFormDefaults } from '@/features/channels/lib/channel-form'
import { channelSchema } from '@/features/channels/types'
import { vendorErrorMessage } from '@/features/models/vendor-api'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { UserInfoDialog } from '@/features/usage-logs/components/dialogs/user-info-dialog'
import { UsersMutateDrawer } from '@/features/users/components/users-mutate-drawer'
import { UsersProvider } from '@/features/users/components/users-provider'
import type { User } from '@/features/users/types'
import { api } from '@/lib/http-client'
import { createAppQueryClient } from '@/lib/query-client'
import {
  createServerError,
  requireServerSuccess,
} from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

const originalAdapter = api.defaults.adapter
const channel = channelSchema.parse({
  id: 42,
  type: 1,
  key: '',
  status: 1,
  name: 'Fixture channel',
  models: 'fixture-model',
  group: 'default',
  created_time: 1,
  test_time: 0,
  response_time: 0,
  balance_updated_time: 0,
})

beforeEach(() => {
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'root-fixture', role: 100 })
  vi.spyOn(toast, 'error').mockReturnValue('fixture-error')
  vi.spyOn(toast, 'success').mockReturnValue('fixture-success')
})

afterEach(() => {
  api.defaults.adapter = originalAdapter
  useAuthStore.getState().auth.reset()
})

it('rejects a failed setting save without invalidation or a success notification', async () => {
  const client = createAppQueryClient()
  const invalidate = vi.spyOn(client, 'invalidateQueries')
  api.defaults.adapter = async (config) => ({
    data: { success: false, message: 'Setting rejected' },
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  })
  const wrapper = (props: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, props.children)
  const hook = renderHook(() => useUpdateOption(), { wrapper })
  await act(async () => {
    await expect(
      hook.result.current.mutateAsync({ key: 'Notice', value: 'fixture' })
    ).rejects.toThrow('Setting rejected')
  })
  expect(invalidate).not.toHaveBeenCalled()
  expect(toast.success).not.toHaveBeenCalled()
  expect(toast.error).toHaveBeenCalledExactlyOnceWith('Setting rejected')
  hook.unmount()
  client.clear()
})

it('never invokes channel save success callbacks after a rejected update', async () => {
  const client = createAppQueryClient()
  const onSuccess = vi.fn()
  api.defaults.adapter = async (config) => ({
    data: { success: false, message: 'Channel rejected' },
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  })
  const wrapper = (props: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, props.children)
  const hook = renderHook(
    () =>
      useChannelMutateForm({
        currentRow: channel,
        isEditing: true,
        isMultiKeyChannel: false,
        onSuccess,
      }),
    { wrapper }
  )
  await act(async () => {
    await expect(
      hook.result.current.mutateAsync(transformChannelToFormDefaults(channel))
    ).rejects.toThrow('Channel rejected')
  })
  expect(onSuccess).not.toHaveBeenCalled()
  expect(toast.success).not.toHaveBeenCalled()
  expect(toast.error).toHaveBeenCalledExactlyOnceWith('Channel rejected')
  hook.unmount()
  client.clear()
})

it('retains successful query data when a later business response fails', async () => {
  const client = createAppQueryClient()
  const previous = { success: true, data: [{ id: 42 }] }
  client.setQueryData(['fixture-list'], previous)
  await expect(
    client.fetchQuery({
      queryKey: ['fixture-list'],
      staleTime: 0,
      retry: false,
      queryFn: async () =>
        requireServerSuccess({
          success: false,
          message: 'List unavailable',
          data: [],
        }),
    })
  ).rejects.toThrow('List unavailable')
  expect(client.getQueryData(['fixture-list'])).toBe(previous)
  expect(client.getQueryState(['fixture-list'])?.status).toBe('error')
  client.clear()
})

it('keeps optional diagnostics on their page while ordinary server failures still invoke the redirect', async () => {
  const redirect = vi.fn()
  const client = createAppQueryClient(redirect)
  const failure = createServerError(
    new AxiosError('Server error', 'ERR_BAD_RESPONSE', undefined, undefined, {
      data: { message: 'Diagnostic unavailable' },
      status: 500,
      statusText: 'Error',
      headers: {},
      config: {} as never,
    })
  )
  await client
    .fetchQuery({
      queryKey: ['optional'],
      queryFn: async () => {
        throw failure
      },
      retry: false,
      meta: { errorToast: false, errorRedirect: false },
    })
    .catch(() => undefined)
  expect(redirect).not.toHaveBeenCalled()
  expect(toast.error).not.toHaveBeenCalled()
  await client
    .fetchQuery({
      queryKey: ['ordinary'],
      queryFn: async () => {
        throw failure
      },
      retry: false,
    })
    .catch(() => undefined)
  expect(redirect).toHaveBeenCalledOnce()
  client.clear()
})

it('preserves vendor reference counts and conflict guidance through error wrappers', () => {
  expect(
    vendorErrorMessage(
      createServerError({
        success: false,
        code: 'VENDOR_REFERENCED',
        reference_counts: { first: 2, second: 3 },
        message: 'Vendor is referenced',
      })
    )
  ).toContain('5 linked model records')
  expect(
    vendorErrorMessage(
      createServerError({
        success: false,
        code: 'VENDOR_CONFLICT',
        message: 'Version conflict',
      })
    )
  ).toContain('Preview again')
})

it('blocks a channel edit when its full detail response fails', async () => {
  const client = createAppQueryClient()
  const writes: string[] = []
  api.defaults.adapter = async (config) => {
    if (config.method !== 'get') writes.push(config.url || '')
    let data: unknown = { success: true, data: [] }
    if (config.url === '/api/channel/42') {
      data = { success: false, message: 'Channel detail unavailable' }
    }
    if (config.url === '/api/group/') {
      data = { success: true, data: ['default'] }
    }
    if (config.url === '/api/channel/default_base_urls') {
      data = { success: true, data: {} }
    }
    return { data, status: 200, statusText: 'OK', headers: {}, config }
  }
  render(
    <QueryClientProvider client={client}>
      <ChannelsProvider>
        <ChannelMutateDrawer open currentRow={channel} onOpenChange={vi.fn()} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  expect(await screen.findByText('Failed to load channel')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Update Channel' })).toBeDisabled()
  const form = document.querySelector('#channel-form')
  if (!form) throw new Error('Channel form unavailable')
  fireEvent.submit(form)
  await waitFor(() => expect(writes).toEqual([]))
  client.clear()
})

it('blocks a user update after a failed fresh detail load even if fields are filled', async () => {
  const client = createAppQueryClient()
  const writes: string[] = []
  const user: User = {
    id: 7,
    username: 'fixture-user',
    display_name: 'Fixture',
    quota: 100,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
  }
  const adapter: AxiosAdapter = async (config) => {
    if (config.method !== 'get') writes.push(config.url || '')
    let data: unknown = { success: true, data: [] }
    if (config.url === '/api/user/7') {
      data = { success: false, message: 'User detail unavailable' }
    }
    if (config.url === '/api/group/') {
      data = { success: true, data: ['default'] }
    }
    if (config.url === '/api/authz/catalog') {
      data = { success: true, data: { resources: [], roles: [] } }
    }
    return { data, status: 200, statusText: 'OK', headers: {}, config }
  }
  api.defaults.adapter = adapter
  render(
    <QueryClientProvider client={client}>
      <UsersProvider>
        <UsersMutateDrawer open currentRow={user} onOpenChange={vi.fn()} />
      </UsersProvider>
    </QueryClientProvider>
  )
  await waitFor(() =>
    expect(toast.error).toHaveBeenCalledWith('User detail unavailable')
  )
  await userEvent.type(screen.getByLabelText('Username'), 'filled-fixture')
  expect(screen.getByRole('button', { name: 'Save changes' })).toBeDisabled()
  const form = document.querySelector('#user-form')
  if (!form) throw new Error('User form unavailable')
  await act(async () => {
    fireEvent.submit(form)
  })
  expect(writes).toEqual([])
  client.clear()
})

it.each([200, 400])(
  'keeps a failed direct channel deletion out of its success callback, status=%s',
  async (status) => {
    const client = createAppQueryClient()
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    const onSuccess = vi.fn()
    api.defaults.adapter = async (config) => {
      const response = {
        data: { success: false, message: 'Deletion rejected' },
        status,
        statusText: 'Rejected',
        headers: {},
        config,
      }
      if (status === 400) {
        throw new AxiosError(
          'Request failed with status code 400',
          'ERR_BAD_REQUEST',
          config,
          undefined,
          response
        )
      }
      return response
    }
    await handleDeleteChannel(42, client, onSuccess)
    expect(onSuccess).not.toHaveBeenCalled()
    expect(invalidate).not.toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledExactlyOnceWith('Deletion rejected')
    client.clear()
  }
)

it('does not display an outdated user detail response after switching targets', async () => {
  let completeFirst: (() => void) | undefined
  api.defaults.adapter = async (config) => {
    if (config.url === '/api/user/7') {
      await new Promise<void>((resolve) => {
        completeFirst = resolve
      })
    }
    const username =
      config.url === '/api/user/7' ? 'older-fixture' : 'current-fixture'
    return {
      data: {
        success: true,
        data: { username, quota: 0, used_quota: 0, request_count: 0 },
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  const view = render(<UserInfoDialog open userId={7} onOpenChange={vi.fn()} />)
  await waitFor(() => expect(completeFirst).toBeDefined())
  view.rerender(<UserInfoDialog open userId={8} onOpenChange={vi.fn()} />)
  expect(await screen.findByText('current-fixture')).toBeInTheDocument()
  await act(async () => {
    completeFirst?.()
  })
  expect(screen.queryByText('older-fixture')).not.toBeInTheDocument()
  expect(screen.getByText('current-fixture')).toBeInTheDocument()
})
