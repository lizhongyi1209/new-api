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
import { z } from 'zod'

// ============================================================================
// AIGC Element (Tencent VCLM 主体) Schema & Types
// ============================================================================

export const aigcElementSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  token_id: z.number().optional().default(0),
  token_name: z.string().optional().default(''),
  channel_id: z.number(),
  platform: z.string().optional().default('kling'),
  job_id: z.string(),
  element_id: z.string(),
  name: z.string(),
  description: z.string(),
  reference_type: z.string(),
  provider: z.string(),
  status: z.string(), // pending / succeed / failed
  fail_reason: z.string().optional().default(''),
  frontal_image: z.string().optional().default(''),
  refer_images: z.string().optional().default(''),
  video_list: z.string().optional().default(''),
  created_at: z.number(),
  updated_at: z.number(),
  username: z.string().optional().default(''),
})

export type AigcElement = z.infer<typeof aigcElementSchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetAigcElementsParams {
  p?: number
  page_size?: number
  all?: boolean
}

export interface GetAigcElementsResponse {
  success: boolean
  message?: string
  data?: {
    items: AigcElement[]
    total: number
    page: number
    page_size: number
  }
}

export interface CreateAigcElementPayload {
  channel_id?: number
  name: string
  description: string
  reference_type: string
  frontal_image?: string
  refer_images?: string[]
  video_list?: string[]
  provider?: string[]
  tag_ids?: string[]
  element_voice_id?: string
}

// ============================================================================
// Dialog Types
// ============================================================================

export type AigcElementsDialogType = 'create' | 'delete'
