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

export type GroupOption = SelectOption & {
  desc?: string
  ratio: number
}

export type ReferenceImage = {
  file: File
  id: string
  previewUrl: string
}

export type TemporaryImageUpload = {
  content_type: string
  expires_at: number
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
  output_format?: 'jpeg' | 'png' | 'webp'
  prompt: string
  quality?: string
  response_modalities?: ['IMAGE']
  size?: string
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

export type ImageGenerationTask = {
  data?: ImageGenerationResult
  error?: string
  progress?: string
  status: string
  task_id: string
}

export type ImageGenerationFormValue = {
  aspectRatio: string
  background: 'auto' | 'transparent'
  group: string
  model: string
  outputFormat: 'jpeg' | 'png' | 'webp'
  prompt: string
  quality: string
  size: string
}
