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
import { act, renderHook } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { afterEach, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import * as channelApi from '../../api'
import { CHANNEL_FORM_DEFAULT_VALUES } from '../../lib'
import type { Channel } from '../../types'
import { useChannelMutateForm } from '../use-channel-mutate-form'

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  return function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
  }
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

it('updates the strategy of an existing multi-key channel without replacing blank keys', async () => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'root',
    role: ROLE.SUPER_ADMIN,
  })
  const update = vi
    .spyOn(channelApi, 'updateChannel')
    .mockResolvedValue({ success: true })
  const onSuccess = vi.fn()
  const { result } = renderHook(
    () =>
      useChannelMutateForm({
        currentRow: { id: 42 } as Channel,
        isEditing: true,
        isMultiKeyChannel: true,
        onSuccess,
      }),
    { wrapper: createWrapper() }
  )

  await act(() =>
    result.current.mutateAsync({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      key: '',
      multi_key_type: 'polling',
    })
  )

  expect(update).toHaveBeenCalledWith(
    42,
    expect.objectContaining({ id: 42, multi_key_mode: 'polling' })
  )
  expect(update.mock.calls[0]?.[1]).not.toHaveProperty('key')
  expect(onSuccess).toHaveBeenCalledOnce()
})

it('does not send multi-key strategy changes without sensitive-write permission', async () => {
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'limited-admin',
    role: ROLE.ADMIN,
  })
  const update = vi
    .spyOn(channelApi, 'updateChannel')
    .mockResolvedValue({ success: true })
  const { result } = renderHook(
    () =>
      useChannelMutateForm({
        currentRow: { id: 42 } as Channel,
        isEditing: true,
        isMultiKeyChannel: true,
        onSuccess: vi.fn(),
      }),
    { wrapper: createWrapper() }
  )

  await act(() =>
    result.current.mutateAsync({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      key: 'replacement-secret',
      multi_key_type: 'polling',
    })
  )

  const payload = update.mock.calls[0]?.[1]
  expect(payload).not.toHaveProperty('key')
  expect(payload).not.toHaveProperty('multi_key_mode')
})
