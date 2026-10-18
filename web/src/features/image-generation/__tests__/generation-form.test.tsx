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

import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { GenerationForm } from '../components/generation-form'

const models = [
  { label: 'gemini-nano-banana-2.1-sp', value: 'gemini-nano-banana-2.1-sp' },
  { label: 'gemini-nano-banana-2.1-sd', value: 'gemini-nano-banana-2.1-sd' },
  { label: 'gemini-nano-banana-2.1', value: 'gemini-nano-banana-2.1' },
]

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('image generation form', () => {
  it('opens batch groups when randomUUID is unavailable on a public HTTP origin', async () => {
    const user = userEvent.setup()
    const getRandomValues = globalThis.crypto.getRandomValues.bind(
      globalThis.crypto
    )
    vi.stubGlobal('crypto', { getRandomValues })
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={vi.fn()}
      />
    )

    await user.click(screen.getByRole('switch', { name: 'Batch mode' }))
    await user.click(screen.getByRole('button', { name: 'Add group' }))
    expect(
      screen.getAllByRole('textbox', { name: /Group .* name/ })
    ).toHaveLength(2)
  })

  it('keeps the phone prompt compact and stretches it in the desktop panel', () => {
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={vi.fn()}
      />
    )

    const prompt = screen.getByRole('textbox', { name: 'Prompt' })
    expect(prompt).toHaveClass(
      'field-sizing-fixed',
      'h-36',
      'xl:h-full',
      'max-h-none'
    )
    expect(prompt.closest('[data-slot="form-item"]')).toHaveClass('xl:flex-1')
  })

  it('places model and tier in separate rows above reference images', () => {
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={vi.fn()}
      />
    )

    const referencePicker = screen.getByText('Reference images')
    const modelLabel = screen.getByText('Model')
    const tierLabel = screen.getByText('Tier')
    expect(
      modelLabel.compareDocumentPosition(tierLabel) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    expect(
      tierLabel.compareDocumentPosition(referencePicker) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    expect(
      modelLabel.closest('[data-slot="form-item"]')?.parentElement
    ).toHaveClass('grid', 'gap-3')
    expect(
      modelLabel.closest('[data-slot="form-item"]')?.parentElement
    ).not.toHaveClass('@sm:grid-cols-2')
    expect(screen.queryByRole('combobox', { name: 'Group' })).toBeNull()
  })

  it('places image count below output options and above the generate button', () => {
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={vi.fn()}
      />
    )

    const outputSize = screen.getByRole('combobox', {
      name: 'Output image size',
    })
    const count = screen.getByRole('combobox', { name: 'Image count' })
    const generate = screen.getByRole('button', { name: 'Generate image' })
    expect(
      outputSize.compareDocumentPosition(count) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    expect(
      count.compareDocumentPosition(generate) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  it('opens the model menu repeatedly without locking the page or moving the popup', async () => {
    const user = userEvent.setup()
    const bodyOverflow = document.body.style.overflow
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={vi.fn()}
      />
    )

    const trigger = screen.getByRole('combobox', { name: 'Model' })
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(document.body.style.overflow).toBe(bodyOverflow)
    const popup = document.querySelector('[data-slot="select-content"]')
    expect(popup).toHaveClass('animate-none', 'transition-none')
    expect(popup).not.toHaveClass('data-open:animate-in')
    expect(trigger).toHaveClass('transition-none')
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })

  it('selects a model series and tier separately while submitting the exact variant ID', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={onSubmit}
      />
    )

    await user.click(screen.getByRole('combobox', { name: 'Model' }))
    expect(
      screen.getByRole('option', { name: 'Nano Banana 2.1' })
    ).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await user.click(screen.getByRole('combobox', { name: 'Tier' }))
    expect(
      screen.getByRole('option', { name: 'Special price' })
    ).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Premium' })).toBeInTheDocument()
    await user.click(screen.getByRole('option', { name: 'Enterprise' }))

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: 'Model' })).toHaveTextContent(
        'Nano Banana 2.1'
      )
      expect(screen.getByRole('combobox', { name: 'Tier' })).toHaveTextContent(
        'Enterprise'
      )
    })

    await user.type(screen.getByRole('textbox', { name: 'Prompt' }), 'A lake')
    await user.click(screen.getByRole('button', { name: 'Generate image' }))
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ model: 'gemini-nano-banana-2.1' }),
        [[]]
      )
    })
  })

  it('submits nine images when the output count is set to nine', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={onSubmit}
      />
    )

    const count = screen.getByRole('combobox', { name: 'Image count' })
    expect(count).toHaveTextContent('1')
    await user.click(count)
    expect(screen.getAllByRole('option')).toHaveLength(9)
    await user.click(screen.getByRole('option', { name: '9' }))
    await user.type(screen.getByRole('textbox', { name: 'Prompt' }), 'A lake')
    await user.click(screen.getByRole('button', { name: 'Generate 9 images' }))

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ count: 9 }),
        [[]]
      )
    })
  })

  it('offers only available tiers after changing to a different model series', async () => {
    const user = userEvent.setup()
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={[
          { label: 'gemini-nano-banana-2.1', value: 'gemini-nano-banana-2.1' },
          { label: 'gpt-image-2-c-sp', value: 'gpt-image-2-c-sp' },
        ]}
        onSubmit={vi.fn()}
      />
    )

    await user.click(screen.getByRole('combobox', { name: 'Model' }))
    await user.click(screen.getByRole('option', { name: 'GPT Image 2' }))

    expect(screen.getByRole('combobox', { name: 'Tier' })).toHaveTextContent(
      'Special price'
    )
    await user.click(screen.getByRole('combobox', { name: 'Tier' }))
    expect(screen.getByRole('option', { name: 'Premium' })).toHaveAttribute(
      'aria-disabled',
      'true'
    )
    expect(screen.getByRole('option', { name: 'Enterprise' })).toHaveAttribute(
      'aria-disabled',
      'true'
    )
  })

  it('combines fixed images with each batch group alternative', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    let nextPreview = 0
    vi.spyOn(URL, 'createObjectURL').mockImplementation(
      () => `blob:batch-${++nextPreview}`
    )
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={onSubmit}
      />
    )

    await user.click(screen.getByRole('switch', { name: 'Batch mode' }))
    expect(
      screen.getByRole('combobox', { name: 'Combination mode' })
    ).toHaveTextContent('All combinations')
    const inputs = document.querySelectorAll<HTMLInputElement>(
      'input[type="file"][multiple]'
    )
    const model = new File(['model'], 'model.png', { type: 'image/png' })
    const outfitA = new File(['outfit-a'], 'outfit-a.png', {
      type: 'image/png',
    })
    const outfitB = new File(['outfit-b'], 'outfit-b.png', {
      type: 'image/png',
    })
    await user.upload(inputs[0], model)
    await user.upload(inputs[1], [outfitA, outfitB])
    await user.type(
      screen.getByRole('textbox', { name: 'Prompt' }),
      'Dress model'
    )
    await user.click(screen.getByRole('button', { name: 'Generate 2 images' }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const referenceSets = onSubmit.mock.calls[0][1]
    expect(
      referenceSets.map((set: { file: File }[]) =>
        set.map((image) => image.file.name)
      )
    ).toEqual([
      ['model.png', 'outfit-a.png'],
      ['model.png', 'outfit-b.png'],
    ])
  })

  it('confirms the total task count when batch alternatives each request nine images', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:batch-count')
    render(
      <GenerationForm
        disabled={false}
        isLoadingModels={false}
        models={models}
        onSubmit={onSubmit}
      />
    )

    await user.click(screen.getByRole('combobox', { name: 'Image count' }))
    await user.click(screen.getByRole('option', { name: '9' }))
    await user.click(screen.getByRole('switch', { name: 'Batch mode' }))
    const batchInput = document.querySelectorAll<HTMLInputElement>(
      'input[type="file"][multiple]'
    )[1]
    await user.upload(batchInput, [
      new File(['a'], 'outfit-a.png', { type: 'image/png' }),
      new File(['b'], 'outfit-b.png', { type: 'image/png' }),
    ])
    await user.type(screen.getByRole('textbox', { name: 'Prompt' }), 'Outfits')
    await user.click(screen.getByRole('button', { name: 'Generate 18 images' }))

    const dialog = screen.getByRole('alertdialog')
    expect(dialog).toHaveTextContent('18 separately billed image tasks')
    await user.click(
      within(dialog).getByRole('button', { name: 'Generate 18 images' })
    )
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ count: 9 }),
      expect.arrayContaining([expect.any(Array), expect.any(Array)])
    )
  })
})
