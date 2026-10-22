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

import { Blob as NodeBlob } from 'node:buffer'

import { unzipSync } from 'fflate'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createImageDownloadArchive } from '../lib/download-archive'

beforeEach(() => vi.stubGlobal('Blob', NodeBlob))
afterEach(() => vi.unstubAllGlobals())

describe('image download archive', () => {
  it('packs the requested authenticated downloads under safe distinct names', async () => {
    const download = vi.fn(
      async (_taskId: string, index: number) =>
        new Blob([`image-${index}`], {
          type: index === 0 ? 'image/png' : 'image/jpeg',
        })
    )

    const archive = await createImageDownloadArchive(
      [
        { taskId: '../first', index: 0 },
        { taskId: 'second', index: 1 },
      ],
      download
    )
    const files = unzipSync(new Uint8Array(await archive.arrayBuffer()))
    const decoder = new TextDecoder()

    expect(archive.type).toBe('application/zip')
    expect(download.mock.calls).toEqual([
      ['../first', 0],
      ['second', 1],
    ])
    expect(Object.keys(files).sort()).toEqual([
      'generated-image-___first-1.png',
      'generated-image-second-2.jpg',
    ])
    expect(decoder.decode(files['generated-image-___first-1.png'])).toBe(
      'image-0'
    )
    expect(decoder.decode(files['generated-image-second-2.jpg'])).toBe(
      'image-1'
    )
  })

  it('fails the whole archive when one image cannot be downloaded', async () => {
    const download = vi
      .fn()
      .mockResolvedValueOnce(new Blob(['first'], { type: 'image/png' }))
      .mockRejectedValueOnce(new Error('unauthorized'))

    await expect(
      createImageDownloadArchive(
        [
          { taskId: 'first', index: 0 },
          { taskId: 'second', index: 0 },
        ],
        download
      )
    ).rejects.toThrow('unauthorized')
  })
})
