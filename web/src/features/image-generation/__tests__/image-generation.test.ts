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

import { describe, expect, it } from 'vitest'

import {
  buildImageGenerationRequest,
  filterImageGenerationModels,
  generatedImageSource,
  isTerminalTaskStatus,
} from '../lib/image-generation'
import type { ImageGenerationFormValue } from '../types'

const baseForm: ImageGenerationFormValue = {
  aspectRatio: '16:9',
  background: 'transparent',
  group: 'default',
  model: 'gemini-3.1-flash-image-c-sp',
  outputFormat: 'png',
  prompt: '  A quiet lake at sunrise  ',
  quality: 'max',
  size: '2K',
}

describe('image generation request contract', () => {
  it('keeps only Nano Banana and GPT Image models from the user catalog', () => {
    expect(
      filterImageGenerationModels([
        'gpt-4o',
        'gpt-image-2.5-flare-sd',
        'gemini-3.1-flash-image-c-sp',
        'nano-banana-pro',
        'dall-e-3',
      ])
    ).toEqual([
      'gpt-image-2.5-flare-sd',
      'gemini-3.1-flash-image-c-sp',
      'nano-banana-pro',
    ])
  })

  it('builds the native Banana payload without GPT-only options', () => {
    expect(
      buildImageGenerationRequest(baseForm, ['https://r2.test/input.png'])
    ).toEqual({
      aspect_ratio: '16:9',
      group: 'default',
      images: ['https://r2.test/input.png'],
      model: 'gemini-3.1-flash-image-c-sp',
      output_format: 'png',
      prompt: 'A quiet lake at sunrise',
      response_modalities: ['IMAGE'],
      size: '2K',
    })
  })

  it('builds the GPT Image payload without Banana-only options', () => {
    expect(
      buildImageGenerationRequest(
        {
          ...baseForm,
          model: 'gpt-image-2.5-sunburst-sp',
          size: '1536x1024',
        },
        []
      )
    ).toEqual({
      background: 'transparent',
      group: 'default',
      model: 'gpt-image-2.5-sunburst-sp',
      output_format: 'png',
      prompt: 'A quiet lake at sunrise',
      quality: 'max',
      size: '1536x1024',
    })
  })

  it('normalizes inline image output and terminal task states', () => {
    expect(
      generatedImageSource({ b64_json: 'aGVsbG8=', mime_type: 'image/webp' })
    ).toBe('data:image/webp;base64,aGVsbG8=')
    expect(isTerminalTaskStatus('SUCCESS')).toBe(true)
    expect(isTerminalTaskStatus('FAILURE')).toBe(true)
    expect(isTerminalTaskStatus('IN_PROGRESS')).toBe(false)
  })
})
