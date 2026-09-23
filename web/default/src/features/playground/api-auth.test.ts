import { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { sendChatCompletion } from './api'

const mocks = vi.hoisted(() => ({ getHeaders: vi.fn() }))
vi.mock('@/lib/auth-session', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/auth-session')>()
  return { ...actual, getFreshAuthHeaders: mocks.getHeaders }
})

const originalAdapter = api.defaults.adapter
afterEach(() => {
  api.defaults.adapter = originalAdapter
  vi.resetAllMocks()
})

test('non-stream generation resolves authentication before sending the request', async () => {
  const requests: InternalAxiosRequestConfig[] = []
  mocks.getHeaders.mockImplementation(async () => {
    expect(requests).toHaveLength(0)
    return { Authorization: 'Bearer synthetic-fresh-access' }
  })
  api.defaults.adapter = async (config) => {
    requests.push(config)
    return {
      config,
      data: { choices: [] },
      status: 200,
      statusText: 'OK',
      headers: {},
    }
  }
  await sendChatCompletion({ model: 'test-model', messages: [], stream: false })
  expect(requests).toHaveLength(1)
  expect(requests[0].headers.get('Authorization')).toBe(
    'Bearer synthetic-fresh-access'
  )
  expect(requests[0].skipAuthRefresh).toBe(true)
})

test('a rejected generation request is never replayed by authentication refresh', async () => {
  mocks.getHeaders.mockResolvedValue({
    Authorization: 'Bearer synthetic-fresh-access',
  })
  const adapter = vi.fn(async (config: InternalAxiosRequestConfig) => {
    throw new AxiosError('Unauthorized', undefined, config, undefined, {
      config,
      data: {},
      status: 401,
      statusText: 'Unauthorized',
      headers: {},
    })
  })
  api.defaults.adapter = adapter
  await expect(
    sendChatCompletion({ model: 'test-model', messages: [], stream: false })
  ).rejects.toThrow('Unauthorized')
  expect(adapter).toHaveBeenCalledTimes(1)
  expect(mocks.getHeaders).toHaveBeenCalledTimes(1)
})

test('failed credential resolution does not send a non-stream generation request', async () => {
  mocks.getHeaders.mockRejectedValue(new Error('Session expired!'))
  const adapter = vi.fn()
  api.defaults.adapter = adapter
  await expect(
    sendChatCompletion({ model: 'test-model', messages: [], stream: false })
  ).rejects.toThrow('Session expired!')
  expect(adapter).not.toHaveBeenCalled()
})
