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

const BANANA_IMAGE_MODELS = [
  'gemini-nano-banana-2.1-sp',
  'gemini-nano-banana-2.1-sd',
  'gemini-nano-banana-2.1',
  'gemini-3-pro-image-c-sp',
  'gemini-3-pro-image-c-sd',
  'gemini-3-pro-image',
  'gemini-3.1-flash-image-c-sp',
  'gemini-3.1-flash-image-c-sd',
  'gemini-3.1-flash-image',
] as const

const GPT_IMAGE_MODELS = [
  'gpt-image-2.5-sunburst-sp',
  'gpt-image-2.5-sunburst-sd',
  'gpt-image-2.5-sunburst',
  'gpt-image-2.5-flare-sp',
  'gpt-image-2.5-flare-sd',
  'gpt-image-2.5-flare',
  'gpt-image-2-c-sp',
  'gpt-image-2-c-sd',
  'gpt-image-2',
] as const

export const DOCUMENTED_IMAGE_MODELS = [
  ...BANANA_IMAGE_MODELS,
  ...GPT_IMAGE_MODELS,
] as const

const bananaImageModelSet = new Set<string>(BANANA_IMAGE_MODELS)
const gptImageModelSet = new Set<string>(GPT_IMAGE_MODELS)

const imageModelSeries: Record<string, string> = {
  'gemini-nano-banana-2.1': 'Nano Banana 2.1',
  'gemini-3-pro-image': 'Gemini 3 Pro Image',
  'gemini-3.1-flash-image': 'Gemini 3.1 Flash Image',
  'gpt-image-2.5-sunburst': 'GPT Image 2.5 Sunburst',
  'gpt-image-2.5-flare': 'GPT Image 2.5 Flare',
  'gpt-image-2': 'GPT Image 2',
}

export function getImageModelFamily(model: string): ImageModelFamily | null {
  const normalized = model.trim().toLowerCase()
  if (gptImageModelSet.has(normalized)) return 'gpt-image'
  if (bananaImageModelSet.has(normalized)) return 'banana'
  return null
}

export function filterImageGenerationModels(models: string[]): string[] {
  const availableModels = new Set(models.map((model) => model.trim()))
  return DOCUMENTED_IMAGE_MODELS.filter((model) => availableModels.has(model))
}

export function getImageModelDisplayName(
  model: string,
  tiers: { specialPrice: string; premium: string; enterprise: string }
): string {
  const normalized = model.trim().toLowerCase()
  if (!getImageModelFamily(normalized)) return model

  const seriesId = normalized.replace(/(?:-c)?-(?:sp|sd)$/, '')
  const series = imageModelSeries[seriesId]
  if (!series) return model

  let tier = tiers.enterprise
  if (normalized.endsWith('-sp')) tier = tiers.specialPrice
  if (normalized.endsWith('-sd')) tier = tiers.premium
  return `${series} · ${tier}`
}

export function isGeminiFlashImageModel(model: string): boolean {
  return model.trim().toLowerCase().startsWith('gemini-3.1-flash-image')
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

  if (family === 'banana') {
    if (form.aspectRatio) request.aspect_ratio = form.aspectRatio
    if (
      isGeminiFlashImageModel(form.model) &&
      form.thinkingLevel !== 'default'
    ) {
      request.thinking_level = form.thinkingLevel
    }
    request.response_modalities = ['IMAGE']
  }

  if (family === 'gpt-image') {
    if (form.quality) request.quality = form.quality
    request.background = form.background
    request.moderation = form.moderation
    request.output_format = form.outputFormat
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
