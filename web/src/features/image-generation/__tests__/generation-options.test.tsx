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

import { render, screen } from '@testing-library/react'
import { FormProvider, useForm } from 'react-hook-form'
import { describe, expect, it } from 'vitest'

import { GenerationOptions } from '../components/generation-options'
import type { ImageGenerationFormValue, ImageModelFamily } from '../types'

function OptionsFixture(props: { family: ImageModelFamily; model: string }) {
  const form = useForm<ImageGenerationFormValue>({
    defaultValues: {
      aspectRatio: '1:1',
      background: 'auto',
      googleSearch: false,
      group: 'default',
      model: props.model,
      moderation: 'auto',
      outputFormat: 'png',
      prompt: 'test',
      quality: 'auto',
      size: props.family === 'banana' ? '1K' : 'auto',
      thinkingLevel: 'default',
    },
  })

  return (
    <FormProvider {...form}>
      <GenerationOptions
        control={form.control}
        disabled={false}
        family={props.family}
        model={props.model}
      />
    </FormProvider>
  )
}

describe('generation model options', () => {
  it('hides output format for Banana and Gemini image models', () => {
    render(
      <OptionsFixture family='banana' model='gemini-3.1-flash-image-c-sp' />
    )

    expect(screen.queryByText('Output format')).not.toBeInTheDocument()
    expect(screen.getByText('Output aspect ratio')).toBeInTheDocument()
    expect(screen.getByText('Thinking level')).toBeInTheDocument()
    expect(screen.getByText('Google Search')).toBeInTheDocument()
  })

  it('does not offer Flash-only thinking controls to Nano Banana 2.1', () => {
    render(<OptionsFixture family='banana' model='gemini-nano-banana-2.1-sp' />)

    expect(screen.queryByText('Thinking level')).not.toBeInTheDocument()
    expect(screen.getByText('Google Search')).toBeInTheDocument()
  })

  it('shows output format for GPT Image models', () => {
    render(
      <OptionsFixture family='gpt-image' model='gpt-image-2.5-sunburst-sp' />
    )

    expect(screen.getByText('Output format')).toBeInTheDocument()
    expect(screen.getByText('Moderation')).toBeInTheDocument()
    expect(screen.queryByText('Google Search')).not.toBeInTheDocument()
  })
})
