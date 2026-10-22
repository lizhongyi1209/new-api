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

export type ImageModelFamily = 'banana' | 'gpt-image'

export type SelectOption = {
  label: string
  value: string
}

export type ReferenceImage = {
  id: string
  previewUrl: string
} & (
  | { file: File; sourceUrl?: never; name?: never }
  | { file?: never; sourceUrl: string; name: string }
)

export type BatchReferenceGroup = {
  id: string
  name: string
  images: ReferenceImage[]
}

export type BatchCombinationMode = 'paired' | 'all'

export type TemporaryImageUpload = {
  content_type: string
  expires_at?: number
  filename: string
  size: number
  url: string
}

export type ImageGenerationRequest = {
  aspect_ratio?: string
  background?: 'auto' | 'transparent'
  group: string
  images?: string[]
  model: string
  moderation?: 'auto' | 'low'
  output_format?: 'jpeg' | 'png' | 'webp'
  prompt: string
  quality?: string
  response_modalities?: ['IMAGE']
  size?: string
  thinking_level?: 'high' | 'minimal'
}

export type ImageGenerationSubmission = {
  status: string
  task_id: string
}

export type GeneratedImage = {
  b64_json?: string
  mime_type?: string
  name?: string
  output_format?: string
  size?: string
  url?: string
}

export type ImageGenerationResult = {
  created: number
  images: GeneratedImage[]
  model: string
}

export type ImageGalleryOptions = {
  size?: string
  aspect_ratio?: string
  quality?: string
  background?: string
  output_format?: string
  thinking_level?: string
}

export type ImageGenerationHistoryItem = {
  task_id: string
  model: string
  prompt?: string
  options?: ImageGalleryOptions
  references?: string[]
  can_regenerate?: boolean
  created: number
  images: GeneratedImage[]
}

export type ImageGallerySelection = {
  task_id: string
  index: number
}

export type ImageResultAction = ImageGallerySelection & {
  model: string
  prompt: string
  url: string
  outputFormat?: string
  options?: ImageGalleryOptions
  references?: string[]
  canRegenerate?: boolean
}

export type ImageGenerationHistoryPage = {
  items: ImageGenerationHistoryItem[]
  next_before_id: number
}

export type ImageGenerationTask = {
  data?: ImageGenerationResult
  error?: string
  prompt?: string
  options?: ImageGalleryOptions
  references?: string[]
  can_regenerate?: boolean
  progress?: string
  status: string
  task_id: string
}

export type ImageGenerationFormValue = {
  aspectRatio: string
  background: 'auto' | 'transparent'
  count: number
  model: string
  outputFormat: 'jpeg' | 'png' | 'webp'
  prompt: string
  quality: string
  size: string
  thinkingLevel: 'default' | 'high' | 'minimal'
}
