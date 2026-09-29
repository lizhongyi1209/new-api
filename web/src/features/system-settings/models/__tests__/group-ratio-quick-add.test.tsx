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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test } from 'vitest'

import { GroupRatioVisualEditor } from '../group-ratio-visual-editor'

function QuickAddFixture(props: { selectableGroups?: string }) {
  const [groupGroupRatio, setGroupGroupRatio] = useState(
    '{"vip":{"default":0.7}}'
  )
  return (
    <>
      <GroupRatioVisualEditor
        section='overrides'
        onSectionChange={() => {}}
        defaultUseAutoGroupField={null}
        groupRatio='{"default":1,"vip":0.8,"economy":0.4}'
        topupGroupRatio='{}'
        userUsableGroups={
          props.selectableGroups ?? '{"default":"","vip":"","economy":""}'
        }
        groupGroupRatio={groupGroupRatio}
        autoGroups='[]'
        maxTokenAutoGroupsField={null}
        groupSpecialUsableGroup='{}'
        onChange={(field, value) => {
          if (field === 'GroupGroupRatio') setGroupGroupRatio(value)
        }}
      />
      <output aria-label='Special ratios'>{groupGroupRatio}</output>
    </>
  )
}

test('quick add starts with unconfigured selectable groups and saves previewed ratios', async () => {
  const user = userEvent.setup()
  render(<QuickAddFixture />)

  await user.click(
    screen.getByRole('button', { name: 'Quick add from selectable groups' })
  )
  const dialog = screen.getByRole('dialog', {
    name: 'Quick add from selectable groups',
  })
  expect(
    within(dialog).getByRole('spinbutton', { name: 'Distribution multiplier' })
  ).toHaveValue(0.8)
  expect(
    within(dialog).getByRole('checkbox', { name: 'default' })
  ).not.toBeChecked()
  expect(within(dialog).getByRole('checkbox', { name: 'vip' })).toBeChecked()
  expect(
    within(dialog).getByRole('checkbox', { name: 'economy' })
  ).toBeChecked()

  const multiplier = within(dialog).getByRole('spinbutton', {
    name: 'Distribution multiplier',
  })
  await user.clear(multiplier)
  await user.type(multiplier, '0.5')
  expect(
    within(dialog).getByRole('row', { name: /economy.*0\.4.*0\.2/ })
  ).toBeVisible()
  await user.click(
    within(dialog).getByRole('button', { name: 'Apply 2 ratios' })
  )

  expect(
    JSON.parse(
      screen.getByRole('status', { name: 'Special ratios' }).textContent ?? '{}'
    )
  ).toEqual({ vip: { default: 0.7, vip: 0.4, economy: 0.2 } })
})

test('quick add explains an empty selectable group list and cannot save', async () => {
  const user = userEvent.setup()
  render(<QuickAddFixture selectableGroups='{}' />)

  await user.click(
    screen.getByRole('button', { name: 'Quick add from selectable groups' })
  )
  const dialog = screen.getByRole('dialog', {
    name: 'Quick add from selectable groups',
  })
  expect(
    within(dialog).getByText(
      'No user-selectable groups configured. Add groups in the pricing table first.'
    )
  ).toBeVisible()
  expect(
    within(dialog).getByRole('button', { name: 'Apply 0 ratios' })
  ).toBeDisabled()
})

test('quick add rejects a negative multiplier before changing any ratios', async () => {
  const user = userEvent.setup()
  render(<QuickAddFixture />)

  await user.click(
    screen.getByRole('button', { name: 'Quick add from selectable groups' })
  )
  const dialog = screen.getByRole('dialog', {
    name: 'Quick add from selectable groups',
  })
  const multiplier = within(dialog).getByRole('spinbutton', {
    name: 'Distribution multiplier',
  })
  await user.clear(multiplier)
  await user.type(multiplier, '-1')

  expect(multiplier).toHaveAttribute('aria-invalid', 'true')
  expect(
    within(dialog).getByRole('button', { name: 'Apply 2 ratios' })
  ).toBeDisabled()
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  expect(
    JSON.parse(
      screen.getByRole('status', { name: 'Special ratios' }).textContent ?? '{}'
    )
  ).toEqual({ vip: { default: 0.7 } })
})
