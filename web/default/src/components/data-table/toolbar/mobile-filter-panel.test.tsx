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
import { expect, test, vi } from 'vitest'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { DataTableMobileFilterPanel } from './mobile-filter-panel'

test('keeps actions accessible while collapsed and preserves the selected filter when expanded', async () => {
  const refresh = vi.fn()
  function Filters() {
    const [value, setValue] = useState('')
    return (
      <DataTableMobileFilterPanel
        compact
        actions={<Button onClick={refresh}>Refresh</Button>}
      >
        <Input
          aria-label='Model filter'
          value={value}
          onChange={(event) => setValue(event.target.value)}
        />
      </DataTableMobileFilterPanel>
    )
  }
  const user = userEvent.setup()
  render(<Filters />)
  await user.type(
    screen.getByRole('textbox', { name: 'Model filter' }),
    'image-model'
  )
  await user.click(screen.getByRole('button', { name: 'Collapse' }))
  expect(
    screen.queryByRole('textbox', { name: 'Model filter' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Refresh' }))
  expect(refresh).toHaveBeenCalledTimes(1)
  await user.click(screen.getByRole('button', { name: 'Expand' }))
  expect(screen.getByRole('textbox', { name: 'Model filter' })).toHaveValue(
    'image-model'
  )
})
