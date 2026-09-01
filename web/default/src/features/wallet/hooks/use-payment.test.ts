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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { calculateAmount } from '../api'
import { usePayment } from './use-payment'

vi.mock('../api', () => ({
  calculateAmount: vi.fn(),
  calculateStripeAmount: vi.fn(),
  calculateWaffoPancakeAmount: vi.fn(),
  requestPayment: vi.fn(),
  requestStripePayment: vi.fn(),
  isApiSuccess: (response: { message?: string; success?: boolean }) =>
    response.success === true || response.message === 'success',
}))

describe('usePayment', () => {
  beforeEach(() => {
    vi.mocked(calculateAmount).mockReset()
  })

  it('exposes the server reason when an Epay amount cannot be quoted', async () => {
    vi.mocked(calculateAmount).mockResolvedValue({
      message: 'error',
      data: 'top-up quota limit exceeded',
    })
    const { result } = renderHook(() => usePayment())

    await act(async () => {
      await result.current.calculatePaymentAmount(50, 'wxpay')
    })

    expect(calculateAmount).toHaveBeenCalledWith({ amount: 50 })
    expect(result.current.calculationError).toBe('top-up quota limit exceeded')
    expect(result.current.calculating).toBe(false)
  })

  it('does not let an older failed quote replace a newer custom amount', async () => {
    let resolveFirst:
      | ((value: { message: string; data: string }) => void)
      | undefined
    vi.mocked(calculateAmount)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveFirst = resolve
          })
      )
      .mockResolvedValueOnce({ message: 'success', data: '101.60' })
    const { result } = renderHook(() => usePayment())

    let firstCalculation: Promise<number> | undefined
    act(() => {
      firstCalculation = result.current.calculatePaymentAmount(50, 'wxpay')
    })
    await act(async () => {
      await result.current.calculatePaymentAmount(100, 'wxpay')
    })
    await act(async () => {
      resolveFirst?.({ message: 'error', data: 'older request failed' })
      await firstCalculation
    })

    expect(result.current.amount).toBe(101.6)
    expect(result.current.calculationError).toBeNull()
  })
})
