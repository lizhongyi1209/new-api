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

export type UploadCategory = '' | 'uploads' | 'elements' | 'temp'

export type FileInfo = {
  path: string
  name: string
  size: number
  mod_time: number
  category: Exclude<UploadCategory, ''>
  url: string
  thumbnail_url: string
  is_image: boolean
}

export type StorageStats = {
  uploads: { count: number; size: number }
  elements: { count: number; size: number }
  temp: { count: number; size: number }
  total: { count: number; size: number }
}

export type FileListResponse = {
  success: boolean
  data: FileInfo[]
  total: number
  page: number
}

export async function getUploadedFiles(params: {
  category?: string
  page: number
}): Promise<FileListResponse> {
  const response = await api.get<FileListResponse>(
    '/api/upload-management/files',
    { params: { category: params.category, p: params.page } }
  )
  return response.data
}

export async function deleteFile(path: string) {
  const response = await api.post('/api/upload-management/delete', { path })
  return response.data
}

export async function batchDeleteFiles(paths: string[]) {
  const response = await api.post<{
    success: boolean
    deleted: number
    failed: number
  }>('/api/upload-management/batch-delete', { paths })
  return response.data
}

export async function getUploadStats(): Promise<StorageStats> {
  const response = await api.get<{ success: boolean; data: StorageStats }>(
    '/api/upload-management/stats'
  )
  return response.data.data
}

export async function cleanOldFiles(params: {
  category: string
  days: number
}) {
  const response = await api.post<{
    success: boolean
    deleted: number
    size: number
    message: string
  }>('/api/upload-management/clean', params)
  return response.data
}

export async function clearUploadedFiles() {
  const response = await api.post<{
    success: boolean
    deleted: number
    failed: number
    size: number
  }>('/api/upload-management/clear')
  return response.data
}
