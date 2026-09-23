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
import { fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { categorizeModels } from '../lib/model-categories'
import { UpstreamModelSelection } from './upstream-model-selection'

test('classifies provider-prefixed text and image models without treating every o1 substring as OpenAI', () => {
  expect(
    categorizeModels([
      'openai/gpt-4.1',
      'o3-mini',
      'custom-o1-identifier',
      'qwen/qwen3',
      'wan2.2',
      'flux.1',
      'claude-sonnet-4',
      'gemini-2.5-pro',
    ])
  ).toEqual({
    OpenAI: ['openai/gpt-4.1', 'o3-mini'],
    Other: ['custom-o1-identifier'],
    Qwen: ['qwen/qwen3'],
    Wan: ['wan2.2'],
    'Black Forest Labs': ['flux.1'],
    Anthropic: ['claude-sonnet-4'],
    Gemini: ['gemini-2.5-pro'],
  })
})

function ControlledSelection() {
  const [selected, setSelected] = useState(['source-alias', 'removed-model'])
  return (
    <UpstreamModelSelection
      models={['gpt-new', 'claude-new']}
      selected={selected}
      existingModels={['source-alias', 'removed-model']}
      redirectSourceModels={['source-alias']}
      onChange={setSelected}
    />
  )
}

test('excludes mapping aliases from removed models and keeps deselected candidates available to reselect', () => {
  render(<ControlledSelection />)
  fireEvent.click(screen.getByRole('tab', { name: 'Removed Models (1)' }))
  const removed = screen.getByRole('checkbox', { name: 'removed-model' })
  expect(removed).toBeChecked()
  expect(
    screen.queryByRole('checkbox', { name: 'source-alias' })
  ).not.toBeInTheDocument()
  fireEvent.click(removed)
  expect(
    screen.getByRole('checkbox', { name: 'removed-model' })
  ).not.toBeChecked()
  fireEvent.click(screen.getByRole('checkbox', { name: 'removed-model' }))
  expect(screen.getByRole('checkbox', { name: 'removed-model' })).toBeChecked()
})

test('category selection preserves unrelated selections and search limits the bulk operation', () => {
  const change = vi.fn()
  render(
    <UpstreamModelSelection
      models={['gpt-new', 'gpt-other', 'claude-new']}
      selected={['source-alias']}
      existingModels={[]}
      onChange={change}
      showChanges={false}
    />
  )
  fireEvent.change(screen.getByRole('textbox', { name: 'Search models...' }), {
    target: { value: 'gpt-new' },
  })
  fireEvent.click(
    screen.getByRole('button', { name: 'Select all matching models' })
  )
  expect(change).toHaveBeenCalledWith(['source-alias', 'gpt-new'])
})
