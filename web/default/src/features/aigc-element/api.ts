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
import type {
  AigcElement,
  ApiResponse,
  CreateAigcElementPayload,
  GetAigcElementsParams,
  GetAigcElementsResponse,
} from './types'

// ============================================================================
// Tencent VCLM 主体管理 (AIGC Element) API
// ============================================================================

// Get paginated element list. Pass all=true (admin only) to list every user's.
export async function getAigcElements(
  params: GetAigcElementsParams = {}
): Promise<GetAigcElementsResponse> {
  const { p = 1, page_size = 10, all = false } = params
  const res = await api.get(
    `/api/aigc_element/?p=${p}&page_size=${page_size}${all ? '&all=true' : ''}`
  )
  return res.data
}

// Create a subject against an enabled TencentVideo channel.
export async function createAigcElement(
  data: CreateAigcElementPayload
): Promise<ApiResponse<AigcElement>> {
  const res = await api.post('/api/aigc_element/', data)
  return res.data
}

// Re-query Tencent for the latest status/detail and update the local row.
export async function refreshAigcElement(
  id: number
): Promise<ApiResponse<{ element: AigcElement; detail: unknown }>> {
  const res = await api.post(`/api/aigc_element/${id}/refresh`)
  return res.data
}

// Delete the subject on Tencent's side and remove the local row.
// force=true removes the local row even when the remote delete fails.
export async function deleteAigcElement(
  id: number,
  force = false
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/aigc_element/${id}/${force ? '?force=true' : ''}`
  )
  return res.data
}

// Upload an image file; the backend auto-resizes to <=10MB and stores it,
// returning a public URL usable as a reference image.
export async function uploadAigcElementImage(
  file: File
): Promise<ApiResponse<{ url: string; resized: boolean; size_human: string }>> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post('/api/aigc_element/upload', formData)
  return res.data
}

