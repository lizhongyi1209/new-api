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
import { act, fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { useState } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { ModelMappingEditor } from './model-mapping-editor'

function MappingHarness() {
  const [value, setValue] = useState('{"customer-alias":"provider-model"}')
  return <ModelMappingEditor value={value} onChange={setValue} />
}

afterEach(async () => {
  await i18next.changeLanguage('en')
})

test('request and upstream names remain distinct through visual and JSON editing', () => {
  render(<MappingHarness />)
  expect(
    screen.getByRole('combobox', { name: 'Request Model Name' })
  ).toHaveValue('customer-alias')
  fireEvent.change(
    screen.getByRole('combobox', { name: 'Upstream Model Name' }),
    { target: { value: 'new-provider-model' } }
  )
  fireEvent.click(screen.getByRole('tab', { name: 'JSON' }))
  expect(
    screen.getByText(
      'JSON keys are request model names; values are upstream model names.'
    )
  ).toBeVisible()
  expect(screen.getByRole('textbox', { name: 'Model Mapping' })).toHaveValue(
    JSON.stringify({ 'customer-alias': 'new-provider-model' }, null, 2)
  )
  fireEvent.click(screen.getByRole('tab', { name: 'Visual' }))
  expect(
    screen.getByRole('combobox', { name: 'Request Model Name' })
  ).toHaveValue('customer-alias')
  expect(
    screen.getByRole('combobox', { name: 'Upstream Model Name' })
  ).toHaveValue('new-provider-model')
})

test('language changes preserve an unfinished visual mapping row', async () => {
  render(<MappingHarness />)
  fireEvent.click(screen.getByRole('button', { name: 'Add Mapping' }))
  expect(
    screen.getAllByRole('combobox', { name: 'Request Model Name' })
  ).toHaveLength(2)
  await act(async () => {
    await i18next.changeLanguage('fr')
  })
  expect(
    screen.getAllByRole('combobox', { name: 'Request Model Name' })
  ).toHaveLength(2)
  expect(
    screen.getAllByRole('combobox', { name: 'Request Model Name' })[1]
  ).toHaveValue('')
})

test('duplicate request aliases are rejected without overwriting one upstream target', () => {
  render(<MappingHarness />)
  fireEvent.click(screen.getByRole('button', { name: 'Add Mapping' }))
  fireEvent.change(
    screen.getAllByRole('combobox', { name: 'Upstream Model Name' })[1],
    { target: { value: 'second-provider-model' } }
  )
  fireEvent.change(
    screen.getAllByRole('combobox', { name: 'Request Model Name' })[1],
    { target: { value: 'customer-alias' } }
  )
  expect(
    screen.getByText('Duplicate source model mappings are not allowed')
  ).toBeVisible()
  expect(
    screen
      .getAllByRole('combobox', { name: 'Upstream Model Name' })
      .map((input) => (input as HTMLInputElement).value)
  ).toEqual(['provider-model', 'second-provider-model'])
})

test('an external reset replaces unfinished rows with the newly loaded mapping', () => {
  const onChange = vi.fn()
  const editor = render(
    <ModelMappingEditor value='{"first":"provider-one"}' onChange={onChange} />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Add Mapping' }))
  editor.rerender(
    <ModelMappingEditor value='{"second":"provider-two"}' onChange={onChange} />
  )
  expect(
    screen.getAllByRole('combobox', { name: 'Request Model Name' })
  ).toHaveLength(1)
  expect(
    screen.getByRole('combobox', { name: 'Request Model Name' })
  ).toHaveValue('second')
  expect(
    screen.getByRole('combobox', { name: 'Upstream Model Name' })
  ).toHaveValue('provider-two')
})
