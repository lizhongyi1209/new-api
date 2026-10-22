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

import { Zip, ZipPassThrough } from 'fflate'

export type ImageArchiveEntry = {
  taskId: string
  index: number
  outputFormat?: string
}

export async function createImageDownloadArchive(
  entries: ImageArchiveEntry[],
  download: (taskId: string, index: number) => Promise<Blob>
): Promise<Blob> {
  const chunks: Uint8Array<ArrayBuffer>[] = []
  let completed = false
  let archiveError: Error | null = null
  const zip = new Zip((error, chunk, final) => {
    if (error) {
      archiveError = error
      return
    }
    chunks.push(new Uint8Array(chunk))
    completed = final
  })

  try {
    for (const { taskId, index, outputFormat } of entries) {
      const blob = await download(taskId, index)
      const extension =
        (
          {
            'image/png': 'png',
            'image/jpeg': 'jpg',
            'image/webp': 'webp',
            'image/avif': 'avif',
            'image/gif': 'gif',
          } as Record<string, string>
        )[blob.type.toLowerCase()] ||
        (outputFormat && /^[a-z0-9]+$/i.test(outputFormat)
          ? outputFormat
          : 'img')
      const safeTaskId = taskId.replaceAll(/[^a-zA-Z0-9_-]/g, '_')
      const file = new ZipPassThrough(
        `generated-image-${safeTaskId}-${index + 1}.${extension}`
      )
      zip.add(file)
      file.push(new Uint8Array(await blob.arrayBuffer()), true)
      if (archiveError) throw archiveError
    }
    zip.end()
    if (archiveError) throw archiveError
    if (!completed) throw new Error('Image archive did not finish')
    return new Blob(chunks, { type: 'application/zip' })
  } catch (error) {
    zip.terminate()
    throw error
  }
}
