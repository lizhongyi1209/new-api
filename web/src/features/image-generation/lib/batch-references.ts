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

import type {
  BatchCombinationMode,
  BatchReferenceGroup,
  ReferenceImage,
} from '../types'
import { MAX_REFERENCE_IMAGES } from './image-generation'

export const MAX_BATCH_GROUPS = 3
export const MAX_BATCH_JOBS = 100
export const MAX_BATCH_UPLOAD_IMAGES = 30

export type BatchReferencePlan =
  | { sets: ReferenceImage[][]; error?: never }
  | {
      sets?: never
      error:
        | 'empty_group'
        | 'group_count'
        | 'reference_count'
        | 'upload_count'
        | 'uneven_groups'
        | 'job_count'
    }

export function buildBatchReferencePlan(
  fixed: ReferenceImage[],
  groups: BatchReferenceGroup[],
  mode: BatchCombinationMode
): BatchReferencePlan {
  if (
    groups.length === 0 ||
    groups.some((group) => group.images.length === 0)
  ) {
    return { error: 'empty_group' }
  }
  if (groups.length > MAX_BATCH_GROUPS) return { error: 'group_count' }
  if (fixed.length + groups.length > MAX_REFERENCE_IMAGES) {
    return { error: 'reference_count' }
  }
  if (
    fixed.length +
      groups.reduce((count, group) => count + group.images.length, 0) >
    MAX_BATCH_UPLOAD_IMAGES
  ) {
    return { error: 'upload_count' }
  }

  if (mode === 'paired') {
    const jobCount = Math.max(...groups.map((group) => group.images.length))
    if (
      groups.some(
        (group) => group.images.length !== 1 && group.images.length !== jobCount
      )
    ) {
      return { error: 'uneven_groups' }
    }
    return {
      sets: Array.from({ length: jobCount }, (_, index) => [
        ...fixed,
        ...groups.map(
          (group) => group.images[group.images.length === 1 ? 0 : index]
        ),
      ]),
    }
  }

  let sets: ReferenceImage[][] = [[...fixed]]
  for (const group of groups) {
    if (sets.length * group.images.length > MAX_BATCH_JOBS) {
      return { error: 'job_count' }
    }
    sets = sets.flatMap((current) =>
      group.images.map((image) => [...current, image])
    )
  }
  return { sets }
}
