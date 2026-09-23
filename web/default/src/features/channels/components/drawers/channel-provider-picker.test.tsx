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
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { CHANNEL_TYPE_OPTIONS } from '../../constants'
import { ChannelProviderPicker } from './channel-provider-picker'

test('all local providers remain available and gateway/custom filters use local channel numbers', () => {
  render(
    <ChannelProviderPicker
      currentType={1}
      canBindTaskPlugin
      disabled={false}
      onSelect={vi.fn()}
    />
  )
  expect(screen.getAllByRole('option')).toHaveLength(
    CHANNEL_TYPE_OPTIONS.length
  )
  fireEvent.click(screen.getByRole('tab', { name: 'Gateways' }))
  expect(
    screen
      .getAllByRole('option')
      .map((option) => option.getAttribute('data-value'))
  ).toEqual(['63', '64'])
  fireEvent.click(screen.getByRole('tab', { name: 'Custom' }))
  expect(
    screen
      .getAllByRole('option')
      .map((option) => option.getAttribute('data-value'))
  ).toEqual(['8', '59'])
})

test('searching a local video type supports keyboard selection without treating it as AdvancedCustom', async () => {
  const select = vi.fn()
  const user = userEvent.setup()
  render(
    <ChannelProviderPicker
      currentType={1}
      canBindTaskPlugin
      disabled={false}
      onSelect={select}
    />
  )
  await user.type(
    screen.getByRole('combobox', { name: 'Search providers or type numbers' }),
    '58'
  )
  expect(screen.getAllByRole('option')).toHaveLength(1)
  expect(screen.getByRole('option')).toHaveAttribute('data-value', '58')
  await user.keyboard('{ArrowDown}{Enter}')
  expect(select).toHaveBeenCalledWith(58)
})

test('unknown saved provider numbers remain selectable while numeric search rejects unsafe values', () => {
  const select = vi.fn()
  render(
    <ChannelProviderPicker
      currentType={999}
      canBindTaskPlugin
      disabled={false}
      onSelect={select}
    />
  )
  fireEvent.change(
    screen.getByRole('combobox', { name: 'Search providers or type numbers' }),
    { target: { value: '999' } }
  )
  fireEvent.click(screen.getByRole('option', { name: '#999 #999' }))
  expect(select).toHaveBeenCalledWith(999)
  fireEvent.change(
    screen.getByRole('combobox', { name: 'Search providers or type numbers' }),
    { target: { value: '9007199254740992' } }
  )
  expect(screen.queryByRole('option')).not.toBeInTheDocument()
})

test('disabled providers cannot be selected', () => {
  const select = vi.fn()
  render(
    <ChannelProviderPicker
      currentType={1}
      canBindTaskPlugin
      disabled
      onSelect={select}
    />
  )
  fireEvent.click(screen.getByRole('option', { name: 'OpenAI #1' }))
  expect(select).not.toHaveBeenCalled()
})
