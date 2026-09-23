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
import { expect, test } from 'vitest'

import { TimestampCell, ActivityTimeCell } from './activity-time-cell'

test('missing activity timestamps stay empty instead of showing an epoch date', () => {
  render(
    <ActivityTimeCell
      createdAt={0}
      lastAt={-1}
      lastLabel='Last Used'
      format='absolute'
    />
  )
  expect(screen.getAllByText('-')).toHaveLength(2)
  expect(screen.getByText('Created')).toBeVisible()
  expect(screen.getByText('Last Used')).toBeVisible()
})

test('recent timestamps show Just now while future timestamps remain a relative time', () => {
  const now = Date.now()
  const recent = Math.floor(now / 1000) - 30
  const future = Math.floor(now / 1000) + 60
  const view = render(
    <TimestampCell timestamp={recent} now={now} justNowLabel='Just now' />
  )
  expect(screen.getByText('Just now')).toBeVisible()
  view.rerender(
    <TimestampCell timestamp={future} now={now} justNowLabel='Just now' />
  )
  expect(screen.queryByText('Just now')).not.toBeInTheDocument()
})
