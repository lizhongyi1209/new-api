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
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { handleServerError } from '@/lib/handle-server-error'

import type { CustomOAuthProviderInfo } from '../types'
import { useOAuthLogin } from './use-oauth-login'

const mocks = vi.hoisted(() => ({
  logout: vi.fn(),
  clear: vi.fn(),
  flow: vi.fn(),
  authorization: vi.fn(),
  remember: vi.fn(),
  error: vi.fn(),
}))
vi.mock('../api', () => ({
  logout: mocks.logout,
  createOAuthFlow: mocks.flow,
  createOAuthAuthorization: mocks.authorization,
}))
vi.mock('@/lib/api', () => ({ clearAuthentication: mocks.clear }))
vi.mock('../lib/oauth-callback-mode', () => ({
  rememberOAuthLoginRedirect: mocks.remember,
}))
vi.mock('sonner', () => ({ toast: { error: mocks.error } }))

const provider: CustomOAuthProviderInfo = {
  id: 1,
  name: 'Test provider',
  slug: 'test-provider',
  icon: '',
  client_id: 'test-client',
  authorization_endpoint: 'https://provider.example/authorize',
  scopes: 'openid profile',
}

beforeEach(() => {
  mocks.logout.mockResolvedValue({ success: true })
  vi.spyOn(window, 'open').mockReturnValue(null)
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.resetAllMocks()
})

test('a failed logout keeps the current authentication and stops OAuth initialization', async () => {
  mocks.logout.mockResolvedValue({ success: false, message: 'Sign-out denied' })
  const { result } = renderHook(() =>
    useOAuthLogin({ github_client_id: 'client' })
  )
  await act(() => result.current.handleGitHubLogin())
  expect(mocks.clear).not.toHaveBeenCalled()
  expect(mocks.flow).not.toHaveBeenCalled()
  expect(window.open).not.toHaveBeenCalled()
  expect(mocks.error).toHaveBeenCalledExactlyOnceWith('Sign-out denied')
  expect(result.current.isLoading).toBe(false)
  expect(result.current.githubButtonDisabled).toBe(false)
})

test('a previously presented flow failure is not presented again or redirected', async () => {
  const error = new Error('Provider unavailable')
  handleServerError(error)
  mocks.flow.mockRejectedValue(error)
  const { result } = renderHook(() => useOAuthLogin(null))
  await act(() => result.current.handleCustomOAuthLogin(provider))
  expect(mocks.error).toHaveBeenCalledTimes(1)
  expect(mocks.remember).not.toHaveBeenCalled()
  expect(window.open).not.toHaveBeenCalled()
  expect(result.current.isLoading).toBe(false)
})

test('an unconfigured Telegram provider does not create a flow or end the current session', async () => {
  const { result } = renderHook(() =>
    useOAuthLogin({ telegram_oauth_configured: false })
  )
  await act(() => result.current.handleTelegramLogin())
  expect(mocks.authorization).not.toHaveBeenCalled()
  expect(mocks.logout).not.toHaveBeenCalled()
  expect(window.open).not.toHaveBeenCalled()
})

test('custom OAuth retains the issued state, provider parameters and return destination', async () => {
  mocks.flow.mockResolvedValue('synthetic-state')
  const { result } = renderHook(() => useOAuthLogin(null, '/pricing'))
  await act(() => result.current.handleCustomOAuthLogin(provider))
  expect(mocks.flow).toHaveBeenCalledExactlyOnceWith(provider.slug, 'login')
  expect(mocks.remember).toHaveBeenCalledExactlyOnceWith(
    'synthetic-state',
    '/pricing'
  )
  const openedUrl = new URL(vi.mocked(window.open).mock.calls[0][0] as string)
  expect(openedUrl.origin).toBe('https://provider.example')
  expect(Object.fromEntries(openedUrl.searchParams)).toEqual({
    client_id: 'test-client',
    redirect_uri: `${window.location.origin}/oauth/test-provider`,
    response_type: 'code',
    state: 'synthetic-state',
    scope: 'openid profile',
  })
})
