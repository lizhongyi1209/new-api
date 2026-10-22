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

import { afterEach, describe, expect, it, vi } from 'vitest'

import { submitImageGeneration } from '../api'
import { submitImageGenerationJobs } from '../lib/submit-image-jobs'
import type { ImageGenerationRequest } from '../types'

vi.mock('../api', () => ({ submitImageGeneration: vi.fn() }))

afterEach(() => vi.resetAllMocks())

describe('image generation submission', () => {
  it('runs nine independent requests concurrently and starts queued requests as slots free up', async () => {
    let releaseFirstNine: () => void = () => {}
    const gate = new Promise<void>((resolve) => {
      releaseFirstNine = resolve
    })
    let active = 0
    let peakActive = 0
    vi.mocked(submitImageGeneration).mockImplementation(async (request) => {
      active++
      peakActive = Math.max(peakActive, active)
      if (Number(request.prompt) < 9) await gate
      active--
      return { status: 'PENDING', task_id: `task-${request.prompt}` }
    })
    const requests: ImageGenerationRequest[] = Array.from(
      { length: 12 },
      (_, index) => ({
        group: 'auto',
        model: 'gemini-nano-banana-2.1',
        prompt: String(index),
      })
    )
    const onSubmitted = vi.fn()

    const result = submitImageGenerationJobs(requests, onSubmitted)
    expect(submitImageGeneration).toHaveBeenCalledTimes(9)
    expect(peakActive).toBe(9)
    releaseFirstNine()
    const ids = await result

    expect(submitImageGeneration).toHaveBeenCalledTimes(12)
    expect(peakActive).toBe(9)
    expect(ids).toHaveLength(12)
    expect(onSubmitted).toHaveBeenCalledTimes(12)
  })

  it('keeps submitted task IDs visible when one request fails', async () => {
    vi.mocked(submitImageGeneration)
      .mockRejectedValueOnce(new Error('Upstream unavailable'))
      .mockResolvedValueOnce({ status: 'PENDING', task_id: 'saved-task' })
    const onSubmitted = vi.fn()
    const requests: ImageGenerationRequest[] = [
      { group: 'auto', model: 'gpt-image-2', prompt: 'one' },
      { group: 'auto', model: 'gpt-image-2', prompt: 'two' },
    ]

    await expect(
      submitImageGenerationJobs(requests, onSubmitted)
    ).rejects.toThrow('Upstream unavailable')
    expect(onSubmitted).toHaveBeenCalledWith('saved-task')
    expect(submitImageGeneration).toHaveBeenCalledTimes(2)
  })

  it('rejects more than one hundred tasks before billing any of them', async () => {
    const requests: ImageGenerationRequest[] = Array.from(
      { length: 101 },
      () => ({ group: 'auto', model: 'gpt-image-2', prompt: 'A lake' })
    )

    await expect(submitImageGenerationJobs(requests, vi.fn())).rejects.toThrow(
      'A batch can contain up to 100 generations.'
    )
    expect(submitImageGeneration).not.toHaveBeenCalled()
  })
})
