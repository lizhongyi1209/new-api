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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { fetchModels, fetchUpstreamModels } from '../api'
import type { FetchModelsResponse } from '../types'
import { ChannelModelDiscovery } from './channel-model-discovery'

vi.mock('../api', () => ({
  fetchModels: vi.fn(),
  fetchUpstreamModels: vi.fn(),
}))

const baseProps = {
  request: {
    type: 1,
    channel_id: 42,
    base_url: 'https://test.example',
    key: '',
  },
  enabled: true,
  selected: ['source-alias'],
  existingModels: ['source-alias'],
  redirectModels: ['mapped-upstream'],
  redirectSourceModels: ['source-alias'],
  onChange: vi.fn(),
}

test('operators without sensitive-write permission use the existing saved-channel discovery endpoint', async () => {
  const client = new QueryClient()
  vi.mocked(fetchUpstreamModels).mockResolvedValue({
    success: true,
    data: ['saved-model'],
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery {...baseProps} savedChannelId={42} />
    </QueryClientProvider>
  )
  fireEvent.click(screen.getByRole('button', { name: 'Fetch from Upstream' }))
  await screen.findByRole('checkbox', { name: 'saved-model' })
  expect(fetchUpstreamModels).toHaveBeenCalledWith(42, {
    signal: expect.any(AbortSignal),
  })
  expect(fetchModels).not.toHaveBeenCalled()
})

test('discovers from the unsaved configuration and only changes model selection through the form callback', async () => {
  const client = new QueryClient()
  vi.mocked(fetchModels).mockResolvedValue({
    success: true,
    data: ['gpt-test'],
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery {...baseProps} />
    </QueryClientProvider>
  )
  fireEvent.click(screen.getByRole('button', { name: 'Fetch from Upstream' }))
  const checkbox = await screen.findByRole('checkbox', { name: 'gpt-test' })
  expect(fetchModels).toHaveBeenCalledWith(baseProps.request, {
    signal: expect.any(AbortSignal),
  })
  expect(baseProps.onChange).not.toHaveBeenCalled()
  fireEvent.click(checkbox)
  expect(baseProps.onChange).toHaveBeenCalledWith(['source-alias', 'gpt-test'])
  expect(client.getMutationCache().getAll()[0].state.variables).toBeUndefined()
})

test('configuration changes abort discovery and discard an old response even when the transport ignores cancellation', async () => {
  let resolve!: (response: FetchModelsResponse) => void
  vi.mocked(fetchModels).mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done
      })
  )
  const client = new QueryClient()
  const view = render(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery {...baseProps} />
    </QueryClientProvider>
  )
  fireEvent.click(screen.getByRole('button', { name: 'Fetch from Upstream' }))
  await waitFor(() => expect(fetchModels).toHaveBeenCalledOnce())
  const signal = vi.mocked(fetchModels).mock.calls[0][1]?.signal
  view.rerender(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery
        {...baseProps}
        request={{ ...baseProps.request, base_url: 'https://changed.example' }}
      />
    </QueryClientProvider>
  )
  expect(signal?.aborted).toBe(true)
  await act(async () => resolve({ success: true, data: ['stale-model'] }))
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Fetch from Upstream' })
    ).toBeEnabled()
  )
  expect(screen.queryByText('stale-model')).not.toBeInTheDocument()
  expect(screen.queryByText('Failed to fetch models')).not.toBeInTheDocument()
  expect(baseProps.onChange).not.toHaveBeenCalled()
})

test('failure stays inline, preserves selected models, and can retry without a global mutation notification', async () => {
  const globalError = vi.fn()
  const client = new QueryClient({
    defaultOptions: { mutations: { onError: globalError } },
  })
  vi.mocked(fetchModels)
    .mockRejectedValueOnce(new Error('synthetic network failure'))
    .mockResolvedValueOnce({ success: true, data: ['retry-model'] })
  render(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery {...baseProps} />
    </QueryClientProvider>
  )
  fireEvent.click(screen.getByRole('button', { name: 'Fetch from Upstream' }))
  await screen.findByText('Failed to fetch models')
  expect(globalError).not.toHaveBeenCalled()
  expect(baseProps.onChange).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
  await screen.findByRole('checkbox', { name: 'retry-model' })
  expect(screen.queryByText('Failed to fetch models')).not.toBeInTheDocument()
})

test('closing or losing sensitive-write permission aborts the request and prevents further discovery', async () => {
  let reject!: (error: Error) => void
  vi.mocked(fetchModels).mockImplementation(
    () =>
      new Promise((_, fail) => {
        reject = fail
      })
  )
  const client = new QueryClient()
  const view = render(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery {...baseProps} />
    </QueryClientProvider>
  )
  fireEvent.click(screen.getByRole('button', { name: 'Fetch from Upstream' }))
  await waitFor(() => expect(fetchModels).toHaveBeenCalledOnce())
  const signal = vi.mocked(fetchModels).mock.calls[0][1]?.signal
  view.rerender(
    <QueryClientProvider client={client}>
      <ChannelModelDiscovery {...baseProps} enabled={false} />
    </QueryClientProvider>
  )
  expect(signal?.aborted).toBe(true)
  await act(async () => reject(new Error('old failure')))
  expect(
    screen.getByRole('button', { name: 'Fetch from Upstream' })
  ).toBeDisabled()
  expect(screen.queryByText('Failed to fetch models')).not.toBeInTheDocument()
})
