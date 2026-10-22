/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { t } from 'i18next'

import { submitImageGeneration } from '../api'
import type { ImageGenerationRequest } from '../types'
import { MAX_BATCH_JOBS } from './batch-references'
import { MAX_OUTPUT_COUNT } from './image-generation'

export async function submitImageGenerationJobs(
  requests: ImageGenerationRequest[],
  onSubmitted: (taskId: string) => void
): Promise<string[]> {
  if (requests.length > MAX_BATCH_JOBS) {
    throw new RangeError(t('A batch can contain up to 100 generations.'))
  }

  const submittedIDs: string[] = []
  let nextIndex = 0
  let firstError: unknown
  const workerCount = Math.min(MAX_OUTPUT_COUNT, requests.length)

  await Promise.all(
    Array.from({ length: workerCount }, async () => {
      while (nextIndex < requests.length) {
        const request = requests[nextIndex++]
        try {
          const submission = await submitImageGeneration(request)
          submittedIDs.push(submission.task_id)
          onSubmitted(submission.task_id)
        } catch (error) {
          firstError ??= error
        }
      }
    })
  )

  if (firstError) throw firstError
  return submittedIDs
}
