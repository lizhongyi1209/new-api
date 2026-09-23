import { act, renderHook } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { useStreamRequest } from './use-stream-request'

const mocks = vi.hoisted(() => ({
  getHeaders: vi.fn(),
  create: vi.fn(),
  stream: vi.fn(),
}))
vi.mock('@/lib/api', () => ({ getFreshAuthHeaders: mocks.getHeaders }))
vi.mock('sse.js', () => ({
  SSE: class {
    readyState = 0
    constructor(url: string, options: unknown) {
      mocks.create(url, options)
    }
    addEventListener() {}
    close() {}
    stream() {
      mocks.stream()
    }
  },
}))

afterEach(() => vi.resetAllMocks())

test('stream generation uses freshly resolved authentication headers', async () => {
  mocks.getHeaders.mockResolvedValue({
    Authorization: 'Bearer synthetic-fresh-access',
  })
  const { result } = renderHook(() => useStreamRequest())
  await act(async () => {
    await result.current.sendStreamRequest(
      { model: 'test-model', messages: [], stream: true },
      vi.fn(),
      vi.fn(),
      vi.fn()
    )
  })
  expect(mocks.create).toHaveBeenCalledWith(
    '/pg/chat/completions',
    expect.objectContaining({
      headers: { Authorization: 'Bearer synthetic-fresh-access' },
    })
  )
  expect(mocks.stream).toHaveBeenCalledTimes(1)
})

test('failed credential resolution never starts a generation request', async () => {
  mocks.getHeaders.mockRejectedValue(new Error('Session expired!'))
  const error = vi.fn()
  const { result } = renderHook(() => useStreamRequest())
  await act(async () => {
    await result.current.sendStreamRequest(
      { model: 'test-model', messages: [], stream: true },
      vi.fn(),
      vi.fn(),
      error
    )
  })
  expect(mocks.create).not.toHaveBeenCalled()
  expect(mocks.stream).not.toHaveBeenCalled()
  expect(error).toHaveBeenCalledWith('Session expired!')
})
