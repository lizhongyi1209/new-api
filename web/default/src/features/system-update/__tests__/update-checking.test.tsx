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
import {
  focusManager,
  onlineManager,
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query'
import {
  act,
  render,
  renderHook,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'

import { useSystemUpdatePreferencesStore, useSystemUpdateStore } from '../store'
import { SystemUpdateAction } from '../system-update-action'
import { SYSTEM_UPDATE_INTERVAL, useSystemUpdate } from '../use-system-update'

const release = {
  tag_name: 'v1.0.0-rc.37',
  draft: false,
  prerelease: true,
  body: 'Release notes.',
}
const fetchMock = vi.fn<typeof fetch>()
let client: QueryClient

function Wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

beforeEach(() => {
  localStorage.clear()
  useSystemUpdateStore.setState({ snapshot: null })
  useSystemUpdatePreferencesStore.setState({ ignoredVersionsByUserId: {} })
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
  focusManager.setFocused(true)
  onlineManager.setOnline(true)
  fetchMock.mockReset()
  fetchMock.mockImplementation(
    async () => new Response(JSON.stringify([release]))
  )
  vi.stubGlobal('fetch', fetchMock)
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(STATUS_QUERY_KEY, { version: 'v1.0.0-rc.16-custom' })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { version: 'v1.0.0-rc.16-custom' } },
  })
})

afterEach(() => {
  client.clear()
  useAuthStore.getState().auth.reset()
  useSystemUpdateStore.setState({ snapshot: null })
  useSystemUpdatePreferencesStore.setState({ ignoredVersionsByUserId: {} })
  localStorage.clear()
  focusManager.setFocused(undefined)
  onlineManager.setOnline(true)
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

test.each([null, ROLE.USER])(
  'does not request releases or show an entry for role %s',
  (role) => {
    useAuthStore
      .getState()
      .auth.setUser(role === null ? null : { id: 2, username: 'user', role })
    render(<SystemUpdateAction />, { wrapper: Wrapper })
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
  }
)

test('shows upstream availability without claiming that a custom version is older or current', async () => {
  const user = userEvent.setup()
  render(
    <>
      <SystemUpdateAction />
      <SystemUpdateAction compact={false} />
    </>,
    { wrapper: Wrapper }
  )
  const buttons = await screen.findAllByRole('button', {
    name: 'Upstream release: v1.0.0-rc.37',
  })
  expect(buttons).toHaveLength(2)
  expect(fetchMock).toHaveBeenCalledTimes(1)
  expect(fetchMock.mock.calls[0][1]?.credentials).toBe('omit')
  await user.click(buttons[0])
  expect(
    await screen.findByText(
      'This customized version cannot be compared automatically. Review the release notes for available updates.'
    )
  ).toBeInTheDocument()
  expect(
    screen.queryByText('No newer version available.')
  ).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: 'Go to GitHub' })).toHaveAttribute(
    'href',
    'https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.37'
  )
  await user.click(screen.getByRole('button', { name: 'Ignore this version' }))
  expect(screen.getByText('This version is ignored')).toBeInTheDocument()
  expect(buttons[0]).toHaveAccessibleName('Check for updates')
  expect(buttons[1]).toHaveAccessibleName('Check for updates')
  await user.click(
    screen.getByRole('button', { name: 'Restore notifications' })
  )
  expect(buttons[0]).toHaveAccessibleName('Upstream release: v1.0.0-rc.37')
})

test('isolates ignored releases by administrator and preserves them across rehydration', async () => {
  const hook = renderHook(useSystemUpdate, { wrapper: Wrapper })
  await waitFor(() => expect(hook.result.current.shouldNotify).toBe(true))
  act(() => hook.result.current.setIgnored(true))
  const saved = localStorage.getItem('system-update-preferences:v1')
  expect(saved).not.toBeNull()
  if (saved === null) throw new Error('Ignore preferences were not persisted')
  act(() =>
    useAuthStore
      .getState()
      .auth.setUser({ id: 2, username: 'other', role: ROLE.ADMIN })
  )
  expect(hook.result.current.isIgnored).toBe(false)
  act(() => {
    useSystemUpdatePreferencesStore.setState({ ignoredVersionsByUserId: {} })
    localStorage.setItem('system-update-preferences:v1', saved)
    void useSystemUpdatePreferencesStore.persist.rehydrate()
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
  })
  expect(hook.result.current.isIgnored).toBe(true)
  expect(hook.result.current.shouldNotify).toBe(false)
  expect(fetchMock).toHaveBeenCalledTimes(1)
})

test('retains the last successful release and throttles retries after rate limiting', async () => {
  const oldCheck = Date.now() - SYSTEM_UPDATE_INTERVAL - 1
  useSystemUpdateStore.getState().setSnapshot({
    release,
    lastCheckedAt: oldCheck,
    lastAttemptAt: oldCheck,
    error: null,
  })
  fetchMock.mockImplementation(async () => new Response('', { status: 429 }))
  const first = renderHook(useSystemUpdate, { wrapper: Wrapper })
  await waitFor(() =>
    expect(first.result.current.snapshot?.error).toBe('rate-limit')
  )
  expect(first.result.current.release?.tag_name).toBe(release.tag_name)
  expect(first.result.current.snapshot?.lastCheckedAt).toBe(oldCheck)
  first.unmount()
  client.clear()
  renderHook(useSystemUpdate, { wrapper: Wrapper })
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
})

test('defers checking while offline and checks on reconnect', async () => {
  onlineManager.setOnline(false)
  const hook = renderHook(useSystemUpdate, { wrapper: Wrapper })
  expect(fetchMock).not.toHaveBeenCalled()
  await act(async () => hook.result.current.checkNow())
  expect(fetchMock).not.toHaveBeenCalled()
  act(() => onlineManager.setOnline(true))
  await waitFor(() =>
    expect(hook.result.current.release?.tag_name).toBe(release.tag_name)
  )
  expect(fetchMock).toHaveBeenCalledTimes(1)
})
