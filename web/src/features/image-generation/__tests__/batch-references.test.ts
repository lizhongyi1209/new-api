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

import { buildBatchReferencePlan } from '../lib/batch-references'
import type { BatchReferenceGroup, ReferenceImage } from '../types'

const image = (id: string): ReferenceImage => ({
  id,
  file: new File([id], `${id}.png`, { type: 'image/png' }),
  previewUrl: `blob:${id}`,
})
const group = (id: string, count: number): BatchReferenceGroup => ({
  id,
  name: id,
  images: Array.from({ length: count }, (_, index) => image(`${id}-${index}`)),
})

describe('batch reference plan', () => {
  it('broadcasts one model across ten outfits while retaining fixed references', () => {
    const fixed = [image('fixed-model'), image('fixed-detail')]
    const plan = buildBatchReferencePlan(fixed, [group('outfit', 10)], 'paired')
    expect(plan.error).toBeUndefined()
    expect(plan.sets).toHaveLength(10)
    expect(plan.sets?.[0].map((item) => item.id)).toEqual([
      'fixed-model',
      'fixed-detail',
      'outfit-0',
    ])
    expect(plan.sets?.[9].map((item) => item.id)).toEqual([
      'fixed-model',
      'fixed-detail',
      'outfit-9',
    ])
  })

  it('pairs ten models with ten outfits by position', () => {
    const plan = buildBatchReferencePlan(
      [],
      [group('model', 10), group('outfit', 10)],
      'paired'
    )
    expect(plan.sets).toHaveLength(10)
    expect(plan.sets?.[9].map((item) => item.id)).toEqual([
      'model-9',
      'outfit-9',
    ])
  })

  it('can make every model and outfit combination when requested', () => {
    const plan = buildBatchReferencePlan(
      [],
      [group('model', 10), group('outfit', 10)],
      'all'
    )
    expect(plan.sets).toHaveLength(100)
    expect(plan.sets?.[0].map((item) => item.id)).toEqual([
      'model-0',
      'outfit-0',
    ])
    expect(plan.sets?.[99].map((item) => item.id)).toEqual([
      'model-9',
      'outfit-9',
    ])
  })

  it('rejects ambiguous paired lengths and reference overflow', () => {
    expect(
      buildBatchReferencePlan(
        [],
        [group('model', 5), group('outfit', 10)],
        'paired'
      ).error
    ).toBe('uneven_groups')
    expect(
      buildBatchReferencePlan(
        Array.from({ length: 9 }, (_, index) => image(`fixed-${index}`)),
        [group('model', 1), group('outfit', 1)],
        'paired'
      ).error
    ).toBe('reference_count')
    expect(
      buildBatchReferencePlan([], [group('empty', 0)], 'paired').error
    ).toBe('empty_group')
  })
})
