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
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test } from 'vitest'

import { MultiSelect } from './multi-select'

function Harness(props: { allowCreate: boolean }) {
  const [selected, setSelected] = useState(['existing'])
  return (
    <>
      <MultiSelect
        options={[{ value: 'existing', label: 'existing' }]}
        selected={selected}
        onChange={setSelected}
        allowCreate={props.allowCreate}
        placeholder='Select models'
      />
      <output aria-label='Selected models'>{selected.join('|')}</output>
    </>
  )
}

test('pasting a separated list commits every value including the final draft and deduplicates existing selections', async () => {
  const user = userEvent.setup()
  render(<Harness allowCreate />)
  const input = screen.getByRole('combobox', { name: 'Select models' })
  await user.click(input)
  await user.paste(' existing , first，second\n third ')
  expect(screen.getByLabelText('Selected models')).toHaveTextContent(
    'existing|first|second|third'
  )
  expect(input).toHaveValue('')
})

test('pasting a separated search into a fixed option selector cannot create unknown selections', async () => {
  const user = userEvent.setup()
  render(<Harness allowCreate={false} />)
  await user.click(screen.getByRole('combobox', { name: 'Select models' }))
  await user.paste('unknown-one,unknown-two')
  expect(screen.getByLabelText('Selected models')).toHaveTextContent('existing')
})
