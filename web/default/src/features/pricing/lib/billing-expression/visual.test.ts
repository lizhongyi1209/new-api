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
import { describe, expect, it } from 'vitest'

import {
  buildEstimatorTokens,
  evalExprLocally,
  type ExtraTokenValues,
} from '../tier-expr'
import { compileBillingExpression } from './parser'
import { evaluateBillingExpression } from './runtime'
import {
  parseVisualBillingDocument,
  serializeVisualBillingDocument,
} from './visual'

const extra: ExtraTokenValues = {
  cacheReadTokens: 200,
  cacheCreateTokens: 100,
  cacheCreate1hTokens: 0,
  imageTokens: 300,
  imageCacheTokens: 0,
  imageOutputTokens: 0,
  audioInputTokens: 0,
  audioOutputTokens: 0,
}

describe('billing editor contracts', () => {
  it('preserves source, explicit zero and price order when editing a single price', () => {
    const source = 'v1:  tier("base", cr * 0.0 + c * 6 + p * 2.00)  '
    const document = parseVisualBillingDocument(source)
    expect(document).not.toBeNull()
    if (!document || document.root.kind !== 'tier') {
      throw new Error('Expected tier')
    }
    expect(serializeVisualBillingDocument(document)).toEqual({
      ok: true,
      source,
    })
    const inputPrice = document.root.prices.find(
      (price) => price.variable === 'p'
    )
    if (!inputPrice) throw new Error('Expected input price')
    inputPrice.value = '3'
    const result = serializeVisualBillingDocument(document)
    expect(result).toEqual({
      ok: true,
      source: source.replace('p * 2.00', 'p * 3'),
    })
    if (!result.ok) throw new Error('Expected valid edit')
    expect(
      evaluateBillingExpression(result.source, {
        tokens: { p: 100, c: 50, cr: 400 },
      })
    ).toMatchObject({ status: 'success', cost: 600 })
  })

  it('retains fixed zero as request pricing and rejects unfinished prices', () => {
    const document = parseVisualBillingDocument('tier("request", fixed(0.025))')
    if (!document || document.root.kind !== 'tier') {
      throw new Error('Expected tier')
    }
    document.root.fixedPrice = '0'
    const free = serializeVisualBillingDocument(document)
    if (!free.ok) throw new Error('Expected explicit free price')
    expect(evaluateBillingExpression(free.source)).toMatchObject({
      status: 'success',
      cost: 0,
      billingUnit: 'request',
      fixedPrice: 0,
    })
    document.root.fixedPrice = ''
    expect(serializeVisualBillingDocument(document)).toMatchObject({
      ok: false,
    })
    expect(document.source).toBe('tier("request", fixed(0.025))')
  })

  it('keeps crossing-midnight boolean conditions and rejects incomplete conditions', () => {
    const source =
      '(hour("UTC") >= 22 || hour("UTC") < 8) ? tier("night", p * 1) : tier("day", p * 2)'
    const document = parseVisualBillingDocument(source)
    if (
      !document ||
      document.root.kind !== 'branch' ||
      document.root.condition.kind !== 'any'
    ) {
      throw new Error('Expected night condition')
    }
    expect(serializeVisualBillingDocument(document)).toEqual({
      ok: true,
      source,
    })
    for (const [hour, cost] of [
      [23, 100],
      [7, 100],
      [8, 200],
    ] as const) {
      expect(
        evaluateBillingExpression(source, {
          tokens: { p: 100 },
          now: new Date(`2026-09-14T${String(hour).padStart(2, '0')}:00:00Z`),
        })
      ).toMatchObject({ status: 'success', cost })
    }
    const condition = document.root.condition.children[0]
    if (condition.kind !== 'comparison') throw new Error('Expected comparison')
    condition.value = ''
    expect(serializeVisualBillingDocument(document)).toMatchObject({
      ok: false,
    })
  })

  it('uses normalized counts without a second subtraction and honors explicit input length', () => {
    expect(buildEstimatorTokens(400, 50, extra).len).toBe(1000)
    expect(
      evalExprLocally(
        'tier("base", p * 2 + c * 6 + cr * 2 + cc * 2.5 + img * 2)',
        400,
        50,
        extra
      )
    ).toMatchObject({ cost: 2350, error: null })
    expect(
      evalExprLocally(
        'len >= 900 ? tier("large", p * 2) : tier("small", p * 1)',
        400,
        50,
        extra,
        { tokens: { len: 800 } }
      )
    ).toMatchObject({ cost: 400, matchedTier: 'small', error: null })
    expect(
      evalExprLocally('tier("base", img_cr * 2)', 0, 0, {
        ...extra,
        imageCacheTokens: 100,
      })
    ).toMatchObject({ cost: 200, error: null })
    expect(
      evalExprLocally('tier("base", p * 2)', Number.NaN, 0, extra).error
    ).not.toBeNull()
  })

  it('simulates request multipliers and image count without saving configuration', () => {
    const expression =
      'tier("image", fixed(0.04)) * image_count * ((param("quality") == "hd") ? 2.0 : 1.0) * ((header("X-Price") == "special") ? 1.5 : 1.0)'
    expect(
      evaluateBillingExpression(expression, {
        imageCount: 2,
        request: {
          body: { quality: 'hd' },
          headers: { 'x-price': ' special ' },
        },
      })
    ).toMatchObject({ status: 'success', cost: 240000, billingUnit: 'request' })
    expect(
      evaluateBillingExpression(expression, { imageCount: 2 })
    ).toMatchObject({ status: 'missing_context' })
    expect(
      evaluateBillingExpression(expression, { imageCount: 129, request: {} })
    ).toMatchObject({ status: 'invalid' })
  })

  it('retains unsupported expressions as source and keeps usage pricing gated', () => {
    expect(parseVisualBillingDocument('tier("nonlinear", p * p)')).toBeNull()
    expect(
      compileBillingExpression('tier("usage", u("seconds") * 0.4)').status
    ).toBe('unsupported')
    expect(
      compileBillingExpression('globalThis.process.exit()').status
    ).not.toBe('ready')
  })
})
