import { act, render } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { Turnstile } from './turnstile'

afterEach(() => {
  delete window.turnstile
  document.querySelector('#cf-turnstile')?.remove()
})

test('expiration clears verification and callbacks from replaced widgets are ignored', () => {
  const callbacks: Array<Record<string, unknown>> = []
  window.turnstile = {
    render: (_element, options) => {
      callbacks.push(options)
    },
  }
  const verify = vi.fn()
  const { rerender } = render(
    <Turnstile key='first' siteKey='test-site' onVerify={verify} />
  )
  act(() => (callbacks[0].callback as (value: string) => void)('valid-token'))
  act(() => (callbacks[0]['expired-callback'] as () => void)())
  expect(verify.mock.calls).toEqual([['valid-token'], ['']])
  rerender(<Turnstile key='second' siteKey='test-site' onVerify={verify} />)
  act(() => {
    ;(callbacks[0].callback as (value: string) => void)('stale-token')
    ;(callbacks[0]['error-callback'] as () => void)()
  })
  expect(verify.mock.calls).toEqual([['valid-token'], ['']])
  act(() => (callbacks[1]['error-callback'] as () => void)())
  expect(verify).toHaveBeenLastCalledWith('')
})

test('widgets mounted while the shared script loads each receive verification callbacks', () => {
  const first = vi.fn()
  const second = vi.fn()
  const renderWidget = vi.fn()
  render(<Turnstile siteKey='test-site' onVerify={first} />)
  render(<Turnstile siteKey='test-site' onVerify={second} />)
  window.turnstile = { render: renderWidget }
  act(() =>
    document.querySelector('#cf-turnstile')?.dispatchEvent(new Event('load'))
  )
  expect(renderWidget).toHaveBeenCalledTimes(2)
})
