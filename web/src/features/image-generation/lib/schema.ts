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

export const imageGenerationSchema = z.object({
  aspectRatio: z.string(),
  background: z.enum(['auto', 'transparent']),
  group: z.string().min(1, 'Select a group'),
  model: z.string().min(1, 'Select a model'),
  outputFormat: z.enum(['jpeg', 'png', 'webp']),
  prompt: z.string().trim().min(1, 'Enter an image prompt'),
  quality: z.string(),
  size: z.string(),
})
