import { render } from '@testing-library/react'
import { createElement, type ComponentType } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { Route } from '@/routes/_authenticated/chat2link'

const mocks = vi.hoisted(() => ({
  key: vi.fn(),
  presets: vi.fn(),
  resolve: vi.fn(),
  navigate: vi.fn(),
}))
vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return { ...actual, useNavigate: () => mocks.navigate }
})
vi.mock('@/features/chat/hooks/use-active-chat-key', () => ({
  useActiveChatKey: mocks.key,
}))
vi.mock('@/features/chat/hooks/use-chat-presets', () => ({
  useChatPresets: mocks.presets,
}))
vi.mock('@/features/chat/lib/chat-links', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/features/chat/lib/chat-links')>()
  return { ...actual, resolveChatUrl: mocks.resolve }
})

afterEach(() => vi.resetAllMocks())
const Page = Route.options.component as ComponentType

test('a keyless web chat resolves immediately without requesting credentials or reacting to cached key errors', () => {
  mocks.presets.mockReturnValue({
    chatPresets: [
      { id: '0', type: 'web', url: 'https://chat.example/{address}' },
    ],
    serverAddress: 'https://gateway.example',
  })
  mocks.key.mockReturnValue({
    data: undefined,
    error: new Error('stale key error'),
  })
  mocks.resolve.mockReturnValue('')
  render(createElement(Page))
  expect(mocks.key).toHaveBeenCalledWith(false)
  expect(mocks.resolve).toHaveBeenCalledWith({
    template: 'https://chat.example/{address}',
    apiKey: undefined,
    serverAddress: 'https://gateway.example',
  })
  expect(mocks.navigate).not.toHaveBeenCalled()
})

test('a credential-bearing web chat waits for the selected credential before resolving', () => {
  mocks.presets.mockReturnValue({
    chatPresets: [
      {
        id: '0',
        type: 'web',
        url: 'https://chat.example/?config={aqbotConfig}',
      },
    ],
    serverAddress: 'https://gateway.example',
  })
  mocks.key.mockReturnValue({ data: undefined, error: null })
  mocks.resolve.mockReturnValue('')
  const { rerender } = render(createElement(Page))
  expect(mocks.key).toHaveBeenCalledWith(true)
  expect(mocks.resolve).not.toHaveBeenCalled()
  mocks.key.mockReturnValue({ data: 'synthetic', error: null })
  rerender(createElement(Page))
  expect(mocks.resolve).toHaveBeenCalledWith({
    template: 'https://chat.example/?config={aqbotConfig}',
    apiKey: 'synthetic',
    serverAddress: 'https://gateway.example',
  })
})
