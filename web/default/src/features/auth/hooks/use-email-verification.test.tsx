import { act, renderHook } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { useEmailVerification } from './use-email-verification'

afterEach(() => vi.restoreAllMocks())

test.each([true, false])(
  'consumes verification before a real request, including failures: %s',
  async (success) => {
    const attempted = vi.fn()
    const get = vi.spyOn(api, 'get').mockImplementation(async () => {
      expect(attempted).toHaveBeenCalledTimes(1)
      return {
        data: { success, message: success ? '' : 'Verification failed' },
      }
    })
    const { result } = renderHook(() =>
      useEmailVerification({
        turnstileToken: 'single-use-token',
        validateTurnstile: () => true,
        onVerificationAttempt: attempted,
      })
    )
    await act(async () => {
      expect(await result.current.sendCode('synthetic@example.com')).toBe(
        success
      )
    })
    expect(get).toHaveBeenCalledWith('/api/verification', {
      params: {
        email: 'synthetic@example.com',
        turnstile: 'single-use-token',
      },
    })
    expect(result.current.isActive).toBe(success)
  }
)

test('invalid email or missing human verification does not consume a challenge or send a request', async () => {
  const get = vi.spyOn(api, 'get')
  const attempted = vi.fn()
  const { result } = renderHook(() =>
    useEmailVerification({
      validateTurnstile: () => false,
      onVerificationAttempt: attempted,
    })
  )
  await act(async () => {
    expect(await result.current.sendCode('')).toBe(false)
    expect(await result.current.sendCode('synthetic@example.com')).toBe(false)
  })
  expect(get).not.toHaveBeenCalled()
  expect(attempted).not.toHaveBeenCalled()
})

test('a transport failure is presented and does not start the resend countdown', async () => {
  vi.spyOn(api, 'get').mockRejectedValue(new Error('Request failed'))
  const error = vi.spyOn(toast, 'error')
  const { result } = renderHook(() => useEmailVerification())
  await act(async () => {
    expect(await result.current.sendCode('synthetic@example.com')).toBe(false)
  })
  expect(error).toHaveBeenCalledTimes(1)
  expect(result.current.isActive).toBe(false)
  expect(result.current.isSending).toBe(false)
})
