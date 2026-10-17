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

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { GenerationForm } from '../components/generation-form'

const models = [
  { label: 'gemini-nano-banana-2.1-sp', value: 'gemini-nano-banana-2.1-sp' },
  { label: 'gemini-nano-banana-2.1-sd', value: 'gemini-nano-banana-2.1-sd' },
  { label: 'gemini-nano-banana-2.1', value: 'gemini-nano-banana-2.1' },
]

describe('image generation form', () => {
  it('stretches the prompt by default to fill the settings panel', () => {
    render(
      <GenerationForm
        disabled={false}
        groups={[]}
        isLoadingModels={false}
        models={models}
        onGroupChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    )

    const prompt = screen.getByRole('textbox', { name: 'Prompt' })
    expect(prompt).toHaveClass('field-sizing-fixed', 'h-full', 'max-h-none')
    expect(prompt.closest('[data-slot="form-item"]')).toHaveClass('flex-1')
  })

  it('places reference images before the group and limits a long group menu to a scrollable height', async () => {
    const groups = Array.from({ length: 20 }, (_, index) => ({
      label: `group-${index}`,
      ratio: 1,
      value: `group-${index}`,
    }))
    render(
      <GenerationForm
        disabled={false}
        groups={groups}
        isLoadingModels={false}
        models={models}
        onGroupChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    )

    const referencePicker = screen.getByText('Reference images')
    const groupLabel = screen.getByText('Group')
    expect(groupLabel.closest('[data-slot="form-item"]')).toHaveClass(
      'sm:col-span-2'
    )
    expect(
      referencePicker.compareDocumentPosition(groupLabel) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()

    fireEvent.click(screen.getByRole('combobox', { name: 'Group' }))
    await waitFor(() => {
      const menu = document.querySelector('[data-slot="select-content"]')
      expect(menu).toHaveClass(
        'max-h-[min(15rem,var(--available-height))]',
        'overflow-y-auto'
      )
    })
    expect(screen.getByRole('option', { name: 'group-19' })).toBeInTheDocument()
  })

  it('opens the group menu repeatedly without locking the page or animating the popup', async () => {
    const user = userEvent.setup()
    const bodyOverflow = document.body.style.overflow
    render(
      <GenerationForm
        disabled={false}
        groups={[{ label: 'default', ratio: 1, value: 'default' }]}
        isLoadingModels={false}
        models={models}
        onGroupChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    )

    const trigger = screen.getByRole('combobox', { name: 'Group' })
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(document.body.style.overflow).toBe(bodyOverflow)
    expect(document.querySelector('[data-slot="select-content"]')).toHaveClass(
      'data-open:animate-none',
      'data-closed:animate-none'
    )
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
        groups={[{ label: 'default', ratio: 1, value: 'default' }]}
        isLoadingModels={false}
        models={models}
        onGroupChange={vi.fn()}
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
        []
      )
    })
  })

  it('offers only available tiers after changing to a different model series', async () => {
    const user = userEvent.setup()
    render(
      <GenerationForm
        disabled={false}
        groups={[{ label: 'default', ratio: 1, value: 'default' }]}
        isLoadingModels={false}
        models={[
          { label: 'gemini-nano-banana-2.1', value: 'gemini-nano-banana-2.1' },
          { label: 'gpt-image-2-c-sp', value: 'gpt-image-2-c-sp' },
        ]}
        onGroupChange={vi.fn()}
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
})
