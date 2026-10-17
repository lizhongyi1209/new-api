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

import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import { filterImageGenerationModels } from './lib/image-generation'
import type {
  GroupOption,
  ImageGenerationRequest,
  ImageGenerationSubmission,
  ImageGenerationTask,
  SelectOption,
  TemporaryImageUpload,
} from './types'

export async function getImageGenerationModels(
  group: string
): Promise<SelectOption[]> {
  const response = await api.get('/api/user/models', { params: { group } })
  requireServerSuccess(response.data)
  const models = Array.isArray(response.data.data)
    ? (response.data.data as string[])
    : []
  return filterImageGenerationModels(models).map((model) => ({
    label: model,
    value: model,
  }))
}

export async function getImageGenerationGroups(): Promise<GroupOption[]> {
  const response = await api.get('/api/user/self/groups')
  requireServerSuccess(response.data)
  const groups = response.data.data as
    | Record<string, { desc?: string; ratio: number }>
    | undefined
  if (!groups) return []

  return Object.entries(groups).map(([group, info]) => ({
    desc: info.desc,
    label: group,
    ratio: info.ratio,
    value: group,
  }))
}

export async function uploadReferenceImage(
  file: File
): Promise<TemporaryImageUpload> {
  const body = new FormData()
  body.append('file', file)
  const response = await api.post<TemporaryImageUpload>(
    '/pg/images/uploads',
    body
  )
  return response.data
}

export async function submitImageGeneration(
  request: ImageGenerationRequest
): Promise<ImageGenerationSubmission> {
  const response = await api.post<ImageGenerationSubmission>(
    '/pg/images/generations',
    request
  )
  return response.data
}

export async function getImageGenerationTask(
  taskId: string
): Promise<ImageGenerationTask> {
  const response = await api.get<ImageGenerationTask>(
    `/pg/images/tasks/${encodeURIComponent(taskId)}`,
    { disableDuplicate: true }
  )
  return response.data
}
