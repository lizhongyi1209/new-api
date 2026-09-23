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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import type { ComponentProps } from 'react'
import { toast } from 'sonner'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/http-client'

import type { RechargeFormCard } from './components/recharge-form-card'
import { Wallet } from './index'

const fixture = vi.hoisted(() => ({
  recharge: null as ComponentProps<typeof RechargeFormCard> | null,
  topupInfo: {
    pay_methods: [{ name: 'Fixture wallet method', type: 'wxpay' }],
    enable_online_topup: true,
    enable_waffo_topup: true,
    min_topup: 10,
    waffo_min_topup: 10,
    waffo_pay_methods: [
      { name: 'Fixture card zero' },
      { name: 'Fixture card one' },
    ],
  },
}))
vi.mock('@/hooks/use-status', () => ({ useStatus: () => ({ status: {} }) }))
vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({ currency: null }),
}))
vi.mock('@/components/layout', () => ({
  SectionPageLayout: Object.assign(
    (props: { children: React.ReactNode }) => props.children,
    {
      Title: (props: { children: React.ReactNode }) => props.children,
      Content: (props: { children: React.ReactNode }) => props.children,
    }
  ),
}))
vi.mock('./hooks', async () => ({
  ...(await import('./hooks/use-payment')),
  ...(await import('./hooks/use-waffo-payment')),
  ...(await import('./hooks/use-waffo-pancake-payment')),
  useTopupInfo: () => ({
    topupInfo: fixture.topupInfo,
    presetAmounts: [],
    loading: false,
  }),
  useAffiliate: () => ({
    affiliateLink: '',
    loading: false,
    transferQuota: vi.fn(),
    transferring: false,
  }),
  useRedemption: () => ({ redeeming: false, redeemCode: vi.fn() }),
  useCreemPayment: () => ({ processing: false, processCreemPayment: vi.fn() }),
}))
vi.mock('./components/recharge-form-card', () => ({
  RechargeFormCard: (props: ComponentProps<typeof RechargeFormCard>) => {
    fixture.recharge = props
    return null
  },
}))
vi.mock('./components/wallet-stats-card', () => ({
  WalletStatsCard: () => null,
}))
vi.mock('./components/subscription-plans-card', () => ({
  SubscriptionPlansCard: () => null,
}))
vi.mock('./components/affiliate-rewards-card', () => ({
  AffiliateRewardsCard: () => null,
}))
vi.mock('./components/dialogs/billing-history-dialog', () => ({
  BillingHistoryDialog: () => null,
}))
vi.mock('./components/dialogs/creem-confirm-dialog', () => ({
  CreemConfirmDialog: () => null,
}))
vi.mock('./components/dialogs/transfer-dialog', () => ({
  TransferDialog: () => null,
}))

interface PendingRequest {
  config: InternalAxiosRequestConfig
  resolve: (response: AxiosResponse) => void
}
const quotes: PendingRequest[] = []
const payments: PendingRequest[] = []
const originalAdapter = api.defaults.adapter

beforeEach(() => {
  fixture.recharge = null
  quotes.length = 0
  payments.length = 0
  vi.spyOn(toast, 'error').mockReturnValue('fixture-error')
  vi.spyOn(toast, 'success').mockReturnValue('fixture-success')
  let initialQuote = true
  api.defaults.adapter = async (config) => {
    if (config.url === '/api/user/self') {
      return {
        config,
        data: { success: true, data: { id: 1, quota: 100 } },
        status: 200,
        statusText: 'OK',
        headers: {},
      }
    }
    if (initialQuote && config.url === '/api/user/amount') {
      initialQuote = false
      return {
        config,
        data: { message: 'success', data: '10.00' },
        status: 200,
        statusText: 'OK',
        headers: {},
      }
    }
    return new Promise((resolve) => {
      const request = { config, resolve }
      if (config.url?.endsWith('/amount')) quotes.push(request)
      else payments.push(request)
    })
  }
})
afterEach(() => {
  api.defaults.adapter = originalAdapter
})

function recharge() {
  if (!fixture.recharge) throw new Error('Missing recharge fixture')
  return fixture.recharge
}
async function selectWaffo(index: number) {
  await act(async () => {
    const method = fixture.topupInfo.waffo_pay_methods[index]
    if (!method) throw new Error('Missing Waffo method fixture')
    recharge().onWaffoMethodSelect?.(method, index)
  })
}
async function respond(request: PendingRequest | undefined, data: unknown) {
  expect(request).toBeDefined()
  if (!request) throw new Error('Missing payment fixture request')
  await act(async () => {
    request.resolve({
      config: request.config,
      data,
      status: 200,
      statusText: 'OK',
      headers: {},
    })
  })
}
async function openWallet() {
  render(<Wallet />)
  await waitFor(() => expect(recharge().topupAmount).toBe(10))
  await waitFor(() => expect(recharge().calculating).toBe(false))
}

test('selecting Waffo only quotes; cancelling confirmation does not create an order', async () => {
  await openWallet()
  await selectWaffo(0)
  expect(quotes[0]?.config.url).toBe('/api/user/waffo/amount')
  expect(JSON.parse(quotes[0]?.config.data)).toEqual({ amount: 10 })
  expect(payments).toHaveLength(0)
  await respond(quotes[0], { message: 'success', data: '12.80' })
  expect(screen.getByRole('alertdialog')).toHaveTextContent('Fixture card zero')
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(payments).toHaveLength(0)
})

test.each([0, 1])(
  'confirmation sends method index %i exactly once despite repeated clicks',
  async (index) => {
    await openWallet()
    await selectWaffo(index)
    await respond(quotes[0], { message: 'success', data: '12.80' })
    const confirm = screen.getByRole('button', { name: 'Confirm Payment' })
    act(() => {
      fireEvent.click(confirm)
      fireEvent.click(confirm)
    })
    await waitFor(() => expect(payments).toHaveLength(1))
    expect(payments[0]?.config.url).toBe('/api/user/waffo/pay')
    expect(JSON.parse(payments[0]?.config.data)).toEqual({
      amount: 10,
      pay_method_index: index,
    })
    expect(confirm).toBeDisabled()
    await respond(payments[0], {
      message: 'error',
      data: 'synthetic checkout failure',
    })
    expect(confirm).not.toBeDisabled()
    expect(toast.error).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  }
)

test('an unsuccessful quote preserves the server reason and never opens confirmation', async () => {
  await openWallet()
  await selectWaffo(0)
  await respond(quotes[0], {
    message: 'error',
    data: 'top-up quota limit exceeded',
  })
  expect(recharge().calculationError).toBe('top-up quota limit exceeded')
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  expect(payments).toHaveLength(0)
})

test('changing the amount invalidates an in-flight selection and its old quote', async () => {
  await openWallet()
  await selectWaffo(0)
  await act(async () => recharge().onTopupAmountChange(20))
  await respond(quotes[0], { message: 'success', data: '12.80' })
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  await respond(quotes[1], { message: 'success', data: '25.60' })
  expect(recharge().paymentAmount).toBe(25.6)
  expect(recharge().topupAmount).toBe(20)
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  expect(payments).toHaveLength(0)
})

test('an older method quote cannot open confirmation or clear the current selection loading', async () => {
  await openWallet()
  await selectWaffo(0)
  await selectWaffo(1)
  await respond(quotes[0], { message: 'success', data: '12.80' })
  expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  expect(recharge().paymentLoading).toBe('waffo-1')
  await respond(quotes[1], { message: 'success', data: '13.00' })
  expect(screen.getByRole('alertdialog')).toHaveTextContent('Fixture card one')
  expect(recharge().paymentLoading).toBeNull()
})

test('a successful synthetic checkout closes confirmation and opens its payment URL', async () => {
  const open = vi.spyOn(window, 'open').mockReturnValue(null)
  await openWallet()
  await selectWaffo(0)
  await respond(quotes[0], { message: 'success', data: '12.80' })
  fireEvent.click(screen.getByRole('button', { name: 'Confirm Payment' }))
  await waitFor(() => expect(payments).toHaveLength(1))
  await respond(payments[0], {
    message: 'success',
    data: { payment_url: 'https://payments.example.test/checkout' },
  })
  expect(open).toHaveBeenCalledWith(
    'https://payments.example.test/checkout',
    '_blank'
  )
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(toast.success).toHaveBeenCalledTimes(1)
})

test.each([
  {
    type: 'wxpay',
    quote: '/api/user/amount',
    pay: '/api/user/pay',
    request: { amount: 10, payment_method: 'wxpay' },
  },
  {
    type: 'waffo_pancake',
    quote: '/api/user/waffo-pancake/amount',
    pay: '/api/user/waffo-pancake/pay',
    request: { amount: 10 },
  },
])('keeps the existing $type quote and payment dispatch', async (method) => {
  await openWallet()
  await act(async () => {
    recharge().onPaymentMethodSelect({
      type: method.type,
      name: 'Synthetic existing method',
    })
  })
  expect(quotes[0]?.config.url).toBe(method.quote)
  await respond(quotes[0], { message: 'success', data: '12.80' })
  fireEvent.click(screen.getByRole('button', { name: 'Confirm Payment' }))
  await waitFor(() => expect(payments).toHaveLength(1))
  expect(payments[0]?.config.url).toBe(method.pay)
  expect(JSON.parse(payments[0]?.config.data)).toEqual(method.request)
  await respond(payments[0], { message: 'error', data: 'synthetic failure' })
})
