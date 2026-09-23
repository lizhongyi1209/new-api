import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import * as keysApi from '@/features/keys/api'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'

import { fetchActiveChatKey, useActiveChatKey } from './use-active-chat-key'

const bundle: AuthBundle = {
  access_token: 'synthetic-access',
  token_type: 'Bearer',
  access_expires_at: 1_900_000_000,
  user: { id: 1, username: 'test-user', role: 1 },
  session: {
    sid: 'session-one',
    current: true,
    login_method: 'password',
    ip: '127.0.0.1',
    user_agent: 'test',
    created_at: 1,
    last_active_at: 1,
    expires_at: 1_900_000_000,
  },
}

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset('idle')
})

test('navigation without an explicit credential requirement does not fetch tokens or keys', async () => {
  useAuthStore.getState().auth.setBundle(bundle)
  const list = vi.spyOn(keysApi, 'getApiKeys')
  const key = vi.spyOn(keysApi, 'fetchTokenKey')
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = (props: PropsWithChildren) => (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
  const { unmount } = renderHook(() => useActiveChatKey(false), { wrapper })
  await act(async () => {})
  expect(list).not.toHaveBeenCalled()
  expect(key).not.toHaveBeenCalled()
  unmount()
  client.clear()
})

test('list failure never proceeds to credential retrieval', async () => {
  useAuthStore.getState().auth.setBundle(bundle)
  vi.spyOn(keysApi, 'getApiKeys').mockResolvedValue({
    success: false,
    message: 'Request failed',
  })
  const key = vi.spyOn(keysApi, 'fetchTokenKey')
  await expect(fetchActiveChatKey()).rejects.toThrow('Request failed')
  expect(key).not.toHaveBeenCalled()
})

test.each(['after list', 'after key'])(
  'rejects credentials when the authentication session changes %s',
  async (phase) => {
    useAuthStore.getState().auth.setBundle(bundle)
    const switchSession = () =>
      useAuthStore.getState().auth.setBundle({
        ...bundle,
        session: { ...bundle.session, sid: 'session-two' },
      })
    vi.spyOn(keysApi, 'getApiKeys').mockImplementation(async () => {
      if (phase === 'after list') switchSession()
      return {
        success: true,
        data: {
          items: [{ id: 10, status: 1 }],
          total: 1,
          page: 1,
          page_size: 50,
        },
      } as Awaited<ReturnType<typeof keysApi.getApiKeys>>
    })
    const key = vi
      .spyOn(keysApi, 'fetchTokenKey')
      .mockImplementation(async () => {
        if (phase === 'after key') switchSession()
        return { success: true, data: { key: 'synthetic' } }
      })
    await expect(fetchActiveChatKey()).rejects.toThrow('Session expired!')
    expect(key).toHaveBeenCalledTimes(phase === 'after list' ? 0 : 1)
  }
)

test('anonymous direct navigation fails before any token request', async () => {
  const list = vi.spyOn(keysApi, 'getApiKeys')
  await expect(fetchActiveChatKey()).rejects.toThrow('Session expired!')
  expect(list).not.toHaveBeenCalled()
})

test('explicit credential retrieval uses only the enabled token and rejects business failure', async () => {
  useAuthStore.getState().auth.setBundle(bundle)
  vi.spyOn(keysApi, 'getApiKeys').mockResolvedValue({
    success: true,
    data: {
      items: [
        { id: 9, status: 2 },
        { id: 10, status: 1 },
      ],
      total: 2,
    },
  } as Awaited<ReturnType<typeof keysApi.getApiKeys>>)
  const key = vi
    .spyOn(keysApi, 'fetchTokenKey')
    .mockResolvedValue({ success: false, message: 'Request failed' })
  await expect(fetchActiveChatKey()).rejects.toThrow('Request failed')
  expect(key).toHaveBeenCalledOnce()
  expect(key).toHaveBeenCalledWith(10)
})
