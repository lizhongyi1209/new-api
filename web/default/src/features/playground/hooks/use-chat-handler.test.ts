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
import { act, renderHook } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/http-client'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../constants'
import type { Message } from '../types'
import { useChatHandler } from './use-chat-handler'

const fixtures = vi.hoisted(() => {
  const sources: ControlledSource[] = []
  class ControlledSource {
    readyState = 0
    closed = false
    listeners = new Map<
      string,
      Array<(event: Event & { data?: string; readyState?: number }) => void>
    >()
    constructor() {
      sources.push(this)
    }
    addEventListener(
      type: string,
      listener: (event: Event & { data?: string; readyState?: number }) => void
    ) {
      const listeners = this.listeners.get(type) ?? []
      listeners.push(listener)
      this.listeners.set(type, listeners)
    }
    stream() {}
    close() {
      this.closed = true
    }
    emit(data: string) {
      for (const listener of this.listeners.get('message') ?? []) {
        listener(Object.assign(new Event('message'), { data }))
      }
    }
  }
  return { sources, ControlledSource }
})
vi.mock('sse.js', () => ({ SSE: fixtures.ControlledSource }))

const originalAdapter = api.defaults.adapter
const messages: Message[] = [
  {
    key: 'question',
    from: 'user',
    versions: [{ id: 'u1', content: 'Synthetic question' }],
  },
  {
    key: 'answer',
    from: 'assistant',
    versions: [{ id: 'a1', content: '' }],
    status: 'loading',
  },
]

beforeEach(() => {
  fixtures.sources.length = 0
  vi.spyOn(toast, 'error').mockReturnValue('fixture-error')
})
afterEach(() => {
  api.defaults.adapter = originalAdapter
  vi.useRealTimers()
})

function delta(content: string) {
  return JSON.stringify({ choices: [{ delta: { content } }] })
}

test('stopping retains buffered content even when message updaters run after cancellation', async () => {
  const updates: Array<(previous: Message[]) => Message[]> = []
  const hook = renderHook(() =>
    useChatHandler({
      config: DEFAULT_CONFIG,
      parameterEnabled: DEFAULT_PARAMETER_ENABLED,
      onMessageUpdate: (update) => updates.push(update),
    })
  )
  await act(async () => {
    hook.result.current.sendChat(messages)
  })
  const source = fixtures.sources[0]
  expect(source).toBeDefined()
  act(() => {
    source?.emit(delta('Buffered final words'))
    hook.result.current.stopGeneration()
  })
  source?.emit(delta('stale words'))
  source?.emit('[DONE]')
  const final = updates.reduce((previous, update) => update(previous), messages)
  expect(final[1]?.versions[0]?.content).toBe('Buffered final words')
  expect(final[1]?.status).toBe('complete')
  expect(source?.closed).toBe(true)
  expect(hook.result.current.isGenerating).toBe(false)
  expect(toast.error).not.toHaveBeenCalled()
})

test('batches deltas and flushes remaining content when the current stream completes', async () => {
  vi.useFakeTimers()
  let current = messages
  const update = vi.fn((updater: (previous: Message[]) => Message[]) => {
    current = updater(current)
  })
  const hook = renderHook(() =>
    useChatHandler({
      config: DEFAULT_CONFIG,
      parameterEnabled: DEFAULT_PARAMETER_ENABLED,
      onMessageUpdate: update,
    })
  )
  await act(async () => {
    hook.result.current.sendChat(messages)
  })
  const source = fixtures.sources[0]
  act(() => {
    source?.emit(delta('First '))
    source?.emit(delta('second '))
  })
  expect(update).not.toHaveBeenCalled()
  act(() => vi.advanceTimersByTime(50))
  expect(update).toHaveBeenCalledTimes(1)
  act(() => {
    source?.emit(delta('last'))
    source?.emit('[DONE]')
  })
  expect(current[1]?.versions[0]?.content).toBe('First second last')
  expect(current[1]?.status).toBe('complete')
})

test('a streaming replacement aborts an older ordinary request and ignores its late response', async () => {
  let finish: (() => void) | undefined
  api.defaults.adapter = async (config) => {
    await new Promise<void>((resolve) => {
      finish = resolve
    })
    return {
      data: {
        choices: [
          {
            message: { role: 'assistant', content: 'Outdated ordinary reply' },
          },
        ],
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  let current = messages
  const hook = renderHook(
    ({ stream }) =>
      useChatHandler({
        config: { ...DEFAULT_CONFIG, stream },
        parameterEnabled: DEFAULT_PARAMETER_ENABLED,
        onMessageUpdate: (updater) => {
          current = updater(current)
        },
      }),
    { initialProps: { stream: false } }
  )
  await act(async () => {
    hook.result.current.sendChat(messages)
  })
  expect(finish).toBeDefined()
  hook.rerender({ stream: true })
  await act(async () => {
    hook.result.current.sendChat(messages)
  })
  await act(async () => {
    finish?.()
  })
  expect(hook.result.current.isGenerating).toBe(true)
  act(() => {
    fixtures.sources[0]?.emit(delta('Current stream reply'))
    fixtures.sources[0]?.emit('[DONE]')
  })
  expect(current[1]?.versions[0]?.content).toBe('Current stream reply')
  expect(toast.error).not.toHaveBeenCalled()
})

test('unmount closes the active source and suppresses queued and late callbacks', async () => {
  const update = vi.fn()
  const hook = renderHook(() =>
    useChatHandler({
      config: DEFAULT_CONFIG,
      parameterEnabled: DEFAULT_PARAMETER_ENABLED,
      onMessageUpdate: update,
    })
  )
  await act(async () => {
    hook.result.current.sendChat(messages)
  })
  const source = fixtures.sources[0]
  act(() => source?.emit(delta('not yet committed')))
  hook.unmount()
  source?.emit(delta('late'))
  source?.emit('[DONE]')
  expect(source?.closed).toBe(true)
  expect(update).not.toHaveBeenCalled()
})
