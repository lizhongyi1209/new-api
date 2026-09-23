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
import { act, fireEvent, render, screen } from '@testing-library/react'
import { createRef } from 'react'
import { beforeEach, describe, expect, it } from 'vitest'

import {
  ModelPricingEditorPanel,
  type ModelPricingEditorPanelHandle,
} from '@/features/system-settings/models/model-pricing-sheet'
import { formatPricingNumber } from '@/features/system-settings/models/pricing-format'
import { usePricingPreferencesStore } from '@/stores/pricing-preferences-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { getSitePricingCurrency, isValidPricingCurrency } from '../currency'
import { pricingFromDraft } from '../pricing'

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7,
    },
  })
  usePricingPreferencesStore.getState().setCurrency('site')
})

async function commit(
  ref: React.RefObject<ModelPricingEditorPanelHandle | null>
) {
  let draft: Awaited<ReturnType<ModelPricingEditorPanelHandle['commitDraft']>> =
    null
  await act(async () => {
    if (!ref.current) throw new Error('Editor not mounted')
    draft = await ref.current.commitDraft()
  })
  return draft
}

async function savedPricing(
  ref: React.RefObject<ModelPricingEditorPanelHandle | null>
) {
  const draft = await commit(ref)
  if (!draft) throw new Error('Expected a valid pricing draft')
  return pricingFromDraft(draft)
}

describe('pricing currency save contract', () => {
  it('preserves a visual expression until a real monetary edit, then converts that edit to USD', async () => {
    const ref = createRef<ModelPricingEditorPanelHandle>()
    const expression =
      'v1:len <= 200000 ? tier("small", p * 2.25 + c * 3.5 + cr * 0.25) : tier("large", p * 4 + c * 5 + cr * 0.5)'
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{
          name: 'fixture-visual',
          billingMode: 'tiered_expr',
          billingExpr: expression,
        }}
      />
    )
    expect((await savedPricing(ref))['billing_setting.billing_expr']).toBe(
      expression
    )
    act(() => usePricingPreferencesStore.getState().setCurrency('USD'))
    expect((await savedPricing(ref))['billing_setting.billing_expr']).toBe(
      expression
    )
    act(() => usePricingPreferencesStore.getState().setCurrency('site'))
    const input = screen.getAllByRole('textbox', { name: 'Input price' })[0]
    if (!input) throw new Error('Input price missing')
    fireEvent.change(input, { target: { value: '28' } })
    const saved = (await savedPricing(ref))['billing_setting.billing_expr']
    expect(saved).toContain('tier("small", p * 4 + c * 3.5 + cr * 0.25)')
    expect(saved).toContain('tier("large", p * 4 + c * 5 + cr * 0.5)')
  })
  it('switching currency or exchange rate never changes an untouched USD fixed price', async () => {
    const ref = createRef<ModelPricingEditorPanelHandle>()
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{ name: 'fixture-fixed', price: '0.0123456789123456' }}
      />
    )
    const original = await savedPricing(ref)
    act(() => usePricingPreferencesStore.getState().setCurrency('USD'))
    expect(await savedPricing(ref)).toEqual(original)
    act(() => {
      useSystemConfigStore.getState().setConfig({
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CUSTOM',
          customCurrencyExchangeRate: 150,
          customCurrencySymbol: '円',
        },
      })
      usePricingPreferencesStore.getState().setCurrency('site')
    })
    expect(await savedPricing(ref)).toEqual(original)
    expect(original.ModelPrice).toBe(0.0123456789123456)
  })

  it('saves a site-currency fixed-price edit as USD, including explicit zero', async () => {
    const ref = createRef<ModelPricingEditorPanelHandle>()
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{ name: 'fixture-fixed', price: '1' }}
      />
    )
    const input = screen.getByRole('textbox', { name: 'Fixed price' })
    fireEvent.change(input, { target: { value: '0.28' } })
    expect((await savedPricing(ref)).ModelPrice).toBe(0.04)
    fireEvent.change(input, { target: { value: '0' } })
    expect((await savedPricing(ref)).ModelPrice).toBe(0)
  })

  it('converts token prices while preserving disabled lanes and the local video lane', async () => {
    const ref = createRef<ModelPricingEditorPanelHandle>()
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{
          name: 'fixture-token',
          ratio: '1',
          completionRatio: '2',
          videoCompletionRatio: '3',
        }}
      />
    )
    act(() => usePricingPreferencesStore.getState().setCurrency('USD'))
    const unchanged = await savedPricing(ref)
    expect(unchanged).toMatchObject({
      ModelRatio: 1,
      CompletionRatio: 2,
      VideoCompletionRatio: 3,
    })
    act(() => usePricingPreferencesStore.getState().setCurrency('site'))
    expect(await savedPricing(ref)).toEqual(unchanged)
    fireEvent.change(screen.getByRole('textbox', { name: 'Input price' }), {
      target: { value: '28' },
    })
    const changed = await savedPricing(ref)
    expect(changed).toMatchObject({
      ModelRatio: 2,
      CompletionRatio: 1,
      VideoCompletionRatio: 1.5,
    })
    expect(changed).not.toHaveProperty('CacheRatio')
  })

  it('keeps a supported time expression and request rules intact when switching currency', async () => {
    const ref = createRef<ModelPricingEditorPanelHandle>()
    const expression =
      'hour("UTC") < 8 ? tier("night", p * 1 + c * 2) : tier("day", p * 3 + c * 4)'
    const rules = '(header("X-Fixture") == "yes" ? 0.5 : 1)'
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{
          name: 'fixture-expr',
          billingMode: 'tiered_expr',
          billingExpr: expression,
          requestRuleExpr: rules,
        }}
      />
    )
    const original = await savedPricing(ref)
    act(() => usePricingPreferencesStore.getState().setCurrency('USD'))
    expect(await savedPricing(ref)).toEqual(original)
    expect(original['billing_setting.billing_expr']).toContain(expression)
    expect(original['billing_setting.billing_expr']).toContain(rules)
  })

  it('blocks invalid converted drafts instead of saving the previous valid price', async () => {
    const ref = createRef<ModelPricingEditorPanelHandle>()
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{ name: 'fixture-fixed', price: '1' }}
      />
    )
    const input = screen.getByRole('textbox', { name: 'Fixed price' })
    fireEvent.change(input, { target: { value: '9'.repeat(310) } })
    expect(await commit(ref)).toBeNull()
    fireEvent.change(input, { target: { value: '14' } })
    expect((await savedPricing(ref)).ModelPrice).toBe(2)
    act(() =>
      useSystemConfigStore.getState().setConfig({
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CUSTOM',
          customCurrencyExchangeRate: 1e300,
        },
      })
    )
    fireEvent.change(input, {
      target: { value: '0.000000000000000000000000000001' },
    })
    expect(await commit(ref)).toBeNull()
  })

  it('preserves very small non-zero prices and rejects invalid site rates', async () => {
    for (const rate of [0, -1, Number.NaN, Infinity]) {
      expect(
        isValidPricingCurrency(
          getSitePricingCurrency({
            ...DEFAULT_CURRENCY_CONFIG,
            quotaDisplayType: 'CNY',
            usdExchangeRate: rate,
          })
        )
      ).toBe(false)
    }
    expect(formatPricingNumber(0.1 + 0.2)).toBe('0.3')
    expect(formatPricingNumber(1.25e-13)).toBe('1.25e-13')
    const ref = createRef<ModelPricingEditorPanelHandle>()
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{ name: 'fixture-small', ratio: '1' }}
      />
    )
    fireEvent.change(screen.getByRole('textbox', { name: 'Input price' }), {
      target: { value: '0.000000000000875' },
    })
    expect((await savedPricing(ref)).ModelRatio).toBe(6.25e-14)
  })
})
