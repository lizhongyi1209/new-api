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
import { AxiosError, CanceledError } from 'axios'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { getAboutContent } from '@/features/about/api'
import { getHomePageContent } from '@/features/home/api'
import { getPrivacyPolicy, getUserAgreement } from '@/features/legal/api'
import { getNotice } from '@/lib/api'
import {
  handleServerError,
  markServerErrorHandled,
} from '@/lib/handle-server-error'
import { api } from '@/lib/http-client'
import { resolveLegacyRoute } from '@/lib/legacy-route'
import { createAppQueryClient } from '@/lib/query-client'
import {
  createServerError,
  getServerErrorMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('request failure contracts', () => {
  it('rejects business failures without turning the raw transport contract into a rejection', async () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    const payload = { success: false, message: 'Cannot save this change' }
    api.defaults.adapter = async (config) => ({
      data: payload,
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    const response = await api.post('/fixture', {})
    expect(response.data).toBe(payload)
    expect(() => requireServerSuccess(response.data)).toThrow(
      'Cannot save this change'
    )
    handleServerError(createServerError(response.data))
    expect(notify).toHaveBeenCalledExactlyOnceWith('Cannot save this change')
  })

  it('keeps failed public document responses out of the successful query state', async () => {
    vi.spyOn(toast, 'error').mockReturnValue('failure')
    api.defaults.adapter = async (config) => ({
      data: { success: false, message: 'Document unavailable' },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    const client = createAppQueryClient()
    for (const [name, queryFn] of [
      ['about', getAboutContent],
      ['privacy', getPrivacyPolicy],
    ] as const) {
      await expect(
        client.fetchQuery({ queryKey: [name], queryFn, retry: false })
      ).rejects.toThrow('Document unavailable')
      expect(client.getQueryState([name])?.status).toBe('error')
      expect(client.getQueryData([name])).toBeUndefined()
    }
    client.clear()
  })

  it('reports one transport failure once across caller wrappers', async () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    api.defaults.adapter = async (config) => {
      throw new AxiosError(
        'Request failed with status code 400',
        'ERR_BAD_REQUEST',
        config,
        undefined,
        {
          data: { success: false, message: 'Invalid value' },
          status: 400,
          statusText: 'Bad Request',
          headers: {},
          config,
        }
      )
    }
    await api.put('/fixture', {}).catch((error) => {
      handleServerError(error)
      handleServerError(createServerError(error))
    })
    expect(notify).toHaveBeenCalledExactlyOnceWith('Invalid value')
  })

  it.each([false, true])(
    'reports only the final query failure, recovery=%s',
    async (recovers) => {
      const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
      let attempts = 0
      api.defaults.adapter = async (config) => {
        attempts++
        if (recovers && attempts === 2) {
          return {
            data: { success: true, data: 'Recovered' },
            status: 200,
            statusText: 'OK',
            headers: {},
            config,
          }
        }
        throw new AxiosError(
          'Request failed with status code 503',
          'ERR_BAD_RESPONSE',
          config,
          undefined,
          {
            data: { message: 'Temporarily unavailable' },
            status: 503,
            statusText: 'Unavailable',
            headers: {},
            config,
          }
        )
      }
      const client = createAppQueryClient()
      const query = client.fetchQuery({
        queryKey: ['retry', recovers],
        retry: 1,
        retryDelay: 0,
        queryFn: async () =>
          requireServerSuccess((await api.get('/fixture/retry')).data),
      })
      if (recovers) {
        await expect(query).resolves.toEqual({
          success: true,
          data: 'Recovered',
        })
      } else await expect(query).rejects.toThrow('Temporarily unavailable')
      expect(attempts).toBe(2)
      if (recovers) expect(notify).not.toHaveBeenCalled()
      else {
        expect(notify).toHaveBeenCalledExactlyOnceWith(
          'Temporarily unavailable'
        )
      }
      client.clear()
    }
  )

  it('preserves the session-expiry message without a duplicate caller notification', async () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    api.defaults.adapter = async (config) => {
      throw new AxiosError(
        'Request failed with status code 401',
        'ERR_BAD_REQUEST',
        config,
        undefined,
        {
          data: { message: 'Internal fixture session failure' },
          status: 401,
          statusText: 'Unauthorized',
          headers: {},
          config,
        }
      )
    }
    await api
      .get('/fixture/session', { skipAuthRefresh: true })
      .catch((error) => handleServerError(createServerError(error)))
    expect(notify).toHaveBeenCalledExactlyOnceWith('Session expired!')
  })

  it('reports independent failures separately and respects an inline presentation', () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    handleServerError(new Error('Invalid value'))
    handleServerError(new Error('Invalid value'))
    const inline = new Error('Shown inline')
    markServerErrorHandled(inline)
    handleServerError(createServerError(inline))
    expect(notify).toHaveBeenCalledTimes(2)
  })

  it('keeps wrapped cancellations silent and never presents an HTML proxy error document', () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    for (const failure of [
      new CanceledError(),
      new DOMException('Aborted', 'AbortError'),
    ]) {
      handleServerError(createServerError(failure))
    }
    expect(notify).not.toHaveBeenCalled()
    expect(
      getServerErrorMessage(
        { message: '<html>proxy failure</html>' },
        'Request failed'
      )
    ).toBe('Request failed')
  })

  it('walks cyclic error origins without inspecting request configuration', () => {
    const failure: { message: string; cause?: unknown; config: unknown } = {
      message: 'Public failure',
      config: { message: 'Private credential details' },
    }
    failure.cause = failure
    expect(getServerErrorMessage(failure)).toBe('Public failure')
    expect(
      getServerErrorMessage(
        { config: { message: 'Private credential details' } },
        'Request failed'
      )
    ).toBe('Request failed')
  })

  it('lets a mutation-specific callback own its message and avoids a second cache toast', async () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    const client = createAppQueryClient()
    const mutation = client.getMutationCache().build(client, {
      mutationFn: async () => {
        throw new Error('Mutation rejected')
      },
      onError: (error) => handleServerError(error),
    })
    await mutation.execute(undefined).catch(handleServerError)
    expect(notify).toHaveBeenCalledExactlyOnceWith('Mutation rejected')
    client.clear()
  })

  it('respects silent query metadata and leaves successful business payloads unchanged', async () => {
    const notify = vi.spyOn(toast, 'error').mockReturnValue('failure')
    const client = createAppQueryClient()
    await expect(
      client.fetchQuery({
        queryKey: ['inline'],
        queryFn: async () => {
          throw new Error('Inline error')
        },
        retry: false,
        meta: { errorToast: false },
      })
    ).rejects.toThrow('Inline error')
    expect(notify).not.toHaveBeenCalled()
    const response = { success: true, data: 'Document' }
    expect(requireServerSuccess(response)).toBe(response)
    client.clear()
  })
})

it('relaxes browser storage only for public documents and preserves private-request no-store', async () => {
  const requests: Array<{ url?: string; cacheControl: unknown }> = []
  api.defaults.adapter = async (config) => {
    requests.push({
      url: config.url,
      cacheControl: config.headers.get('Cache-Control'),
    })
    return {
      data: { success: true, data: 'Public document' },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  for (const load of [
    getAboutContent,
    getPrivacyPolicy,
    getUserAgreement,
    getNotice,
    getHomePageContent,
  ]) {
    await load()
  }
  await api.get('/api/user/self')
  expect(requests.slice(0, 5).map((request) => request.cacheControl)).toEqual([
    null,
    null,
    null,
    null,
    null,
  ])
  expect(requests[5]).toEqual({
    url: '/api/user/self',
    cacheControl: 'no-cache, no-store',
  })
})

it('retains legacy query parameters and fragments while leaving login migration to batch six', () => {
  expect(resolveLegacyRoute('/console/token/?group=a&group=b#details')).toBe(
    '/keys?group=a&group=b#details'
  )
  expect(
    resolveLegacyRoute('/console/setting?tab=payment&receipt=example#pending')
  ).toBe('/system-settings/billing/payment?tab=payment&receipt=example#pending')
  expect(resolveLegacyRoute('/console/topup?receipt=example#pending')).toBe(
    '/wallet?receipt=example#pending'
  )
  expect(resolveLegacyRoute('/console/chat/chat-123?view=compact#last')).toBe(
    '/chat/chat-123?view=compact#last'
  )
  expect(resolveLegacyRoute('/login?redirect=%2Fkeys')).toBeNull()
  expect(resolveLegacyRoute('/keys')).toBeNull()
})

it('preserves an existing authentication error safe message through caller wrappers', () => {
  const failure = new Error('Please try again later.', {
    cause: {
      response: {
        status: 500,
        data: { message: 'Internal fixture diagnostic', code: 'UNKNOWN_CODE' },
      },
    },
  })
  failure.name = 'AuthOperationError'
  expect(getServerErrorMessage(failure)).toBe('Please try again later.')
  expect(getServerErrorMessage(createServerError(failure))).toBe(
    'Please try again later.'
  )
})
