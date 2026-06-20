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
import type { StatusBadgeProps } from '@/components/status-badge'

// ============================================================================
// AIGC Element status / reference-type / tag configuration
// ============================================================================

// Tencent reports element creation status as pending / succeed / failed.
export const ELEMENT_STATUSES: Record<
  string,
  Pick<StatusBadgeProps, 'variant'> & { labelKey: string }
> = {
  pending: { labelKey: 'Processing', variant: 'warning' },
  succeed: { labelKey: 'Succeeded', variant: 'success' },
  failed: { labelKey: 'Failed', variant: 'danger' },
}

export const REFERENCE_TYPE = {
  IMAGE: 'image_refer',
  VIDEO: 'video_refer',
} as const

// Tencent's subject tag ids (o_101 ~ o_108) with their display labels.
export const ELEMENT_TAGS: { id: string; labelKey: string }[] = [
  { id: 'o_101', labelKey: 'Meme' },
  { id: 'o_102', labelKey: 'Person' },
  { id: 'o_103', labelKey: 'Animal' },
  { id: 'o_104', labelKey: 'Prop' },
  { id: 'o_105', labelKey: 'Apparel' },
  { id: 'o_106', labelKey: 'Scene' },
  { id: 'o_107', labelKey: 'Effect' },
  { id: 'o_108', labelKey: 'Other' },
]

export const SUCCESS_MESSAGES = {
  ELEMENT_CREATED: 'Subject created successfully',
  ELEMENT_REFRESHED: 'Subject status refreshed',
  ELEMENT_DELETED: 'Subject deleted successfully',
} as const
