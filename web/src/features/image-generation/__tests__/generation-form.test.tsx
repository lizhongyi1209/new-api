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

  it('shows readable tier labels while selecting the exact model ID', async () => {
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
      screen.getByRole('option', { name: 'Nano Banana 2.1 · Special price' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('option', { name: 'Nano Banana 2.1 · Premium' })
    ).toBeInTheDocument()
    await user.click(
      screen.getByRole('option', { name: 'Nano Banana 2.1 · Enterprise' })
    )

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: 'Model' })).toHaveTextContent(
        'Nano Banana 2.1 · Enterprise'
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
})
