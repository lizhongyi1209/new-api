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
  GeneratedImage,
  ImageGenerationFormValue,
  ImageGenerationRequest,
  ImageModelFamily,
} from '../types'

export const MAX_REFERENCE_IMAGES = 10
export const MAX_REFERENCE_IMAGE_BYTES = 20 * 1024 * 1024
export const ACCEPTED_REFERENCE_IMAGE_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'image/webp',
])

export function getImageModelFamily(model: string): ImageModelFamily | null {
  const normalized = model.trim().toLowerCase()
  if (normalized.startsWith('gpt-image')) return 'gpt-image'
  if (normalized.startsWith('nano-banana')) return 'banana'
  if (normalized.startsWith('gemini') && normalized.includes('image')) {
    return 'banana'
  }
  return null
}

export function filterImageGenerationModels(models: string[]): string[] {
  return models.filter((model) => getImageModelFamily(model) !== null)
}

export function buildImageGenerationRequest(
  form: ImageGenerationFormValue,
  images: string[]
): ImageGenerationRequest {
  const family = getImageModelFamily(form.model)
  const request: ImageGenerationRequest = {
    group: form.group,
    model: form.model,
    prompt: form.prompt.trim(),
  }

  if (images.length > 0) request.images = images
  if (form.size) request.size = form.size
  if (form.outputFormat) request.output_format = form.outputFormat

  if (family === 'banana') {
    if (form.aspectRatio) request.aspect_ratio = form.aspectRatio
    request.response_modalities = ['IMAGE']
  }

  if (family === 'gpt-image') {
    if (form.quality) request.quality = form.quality
    request.background = form.background
  }

  return request
}

export function generatedImageSource(image: GeneratedImage): string {
  if (image.url) return image.url
  if (!image.b64_json) return ''
  const mimeType = image.mime_type || 'image/png'
  return `data:${mimeType};base64,${image.b64_json}`
}

export function isTerminalTaskStatus(status?: string): boolean {
  return status === 'SUCCESS' || status === 'FAILURE'
}
