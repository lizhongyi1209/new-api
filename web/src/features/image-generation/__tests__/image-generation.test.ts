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
  getImageModelOption,
  isTerminalTaskStatus,
} from '../lib/image-generation'
import type { ImageGenerationFormValue } from '../types'

const baseForm: ImageGenerationFormValue = {
  aspectRatio: '16:9',
  background: 'transparent',
  model: 'gemini-3.1-flash-image-c-sp',
  outputFormat: 'png',
  prompt: '  A quiet lake at sunrise  ',
  quality: 'max',
  size: '2K',
  thinkingLevel: 'high',
}

describe('image generation request contract', () => {
  it('keeps only Nano Banana and GPT Image models from the user catalog', () => {
    expect(
      filterImageGenerationModels([
        'gpt-4o',
        'gpt-image-2.5-flare-sd',
        'gemini-nano-banana-2.1-sp',
        'gemini-nano-banana-2.1-sd',
        'gemini-nano-banana-2.1',
        'gemini-3.1-flash-image-c-sp',
        'nano-banana-pro',
        'gpt-image-1',
        'gemini-2.5-flash-image-preview',
        'dall-e-3',
      ])
    ).toEqual([
      'gemini-nano-banana-2.1-sp',
      'gemini-nano-banana-2.1-sd',
      'gemini-nano-banana-2.1',
      'gemini-3.1-flash-image-c-sp',
      'gpt-image-2.5-flare-sd',
    ])
  })

  it('builds the native Banana payload without GPT-only options', () => {
    expect(
      buildImageGenerationRequest(baseForm, ['https://r2.test/input.png'])
    ).toEqual({
      aspect_ratio: '16:9',
      group: 'auto',
      images: ['https://r2.test/input.png'],
      model: 'gemini-3.1-flash-image-c-sp',
      prompt: 'A quiet lake at sunrise',
      response_modalities: ['IMAGE'],
      size: '2K',
      thinking_level: 'high',
    })
  })

  it('builds Nano Banana 2.1 without Flash-only thinking options', () => {
    expect(
      buildImageGenerationRequest(
        {
          ...baseForm,
          model: 'gemini-nano-banana-2.1-sp',
        },
        []
      )
    ).toEqual({
      aspect_ratio: '16:9',
      group: 'auto',
      model: 'gemini-nano-banana-2.1-sp',
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
      group: 'auto',
      model: 'gpt-image-2.5-sunburst-sp',
      moderation: 'auto',
      output_format: 'png',
      prompt: 'A quiet lake at sunrise',
      quality: 'max',
      size: '1536x1024',
    })
  })

  it('maps each variant ID to its model series and tier', () => {
    expect(getImageModelOption('gemini-nano-banana-2.1-sp')).toEqual({
      family: 'banana',
      seriesId: 'gemini-nano-banana-2.1',
      seriesLabel: 'Nano Banana 2.1',
      tier: 'special',
    })
    expect(getImageModelOption('gemini-3-pro-image-c-sd')).toEqual({
      family: 'banana',
      seriesId: 'gemini-3-pro-image',
      seriesLabel: 'Gemini 3 Pro Image',
      tier: 'premium',
    })
    expect(getImageModelOption('gpt-image-2')).toEqual({
      family: 'gpt-image',
      seriesId: 'gpt-image-2',
      seriesLabel: 'GPT Image 2',
      tier: 'enterprise',
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
