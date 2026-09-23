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
import { afterEach, beforeEach, expect, test } from 'vitest'

import { UserQuotaCell } from '@/features/users/components/user-quota-cell'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { ApiKey } from '../../types'
import { ApiKeyQuotaCell } from '../api-key-quota-cell'

const apiKey: ApiKey = {
  id: 1,
  name: 'test',
  key: '',
  status: 1,
  remain_quota: 300000,
  used_quota: 200000,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 0,
  accessed_time: 0,
  group: '',
  auto_groups: null,
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}

beforeEach(() =>
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
)
afterEach(() =>
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
)

test('shows remaining, usage, current total and bounded percentage for a limited key', async () => {
  const user = userEvent.setup()
  render(<ApiKeyQuotaCell apiKey={apiKey} now={0} />)
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '60')
  await user.click(
    screen.getByRole('button', {
      name: 'Remaining 0.6; Remaining percentage 60%; Used amount 0.4',
    })
  )
  const popover = screen.getByRole('dialog')
  expect(within(popover).getByText('Current total quota')).toBeInTheDocument()
  expect(within(popover).getByText('1', { selector: 'dd' })).toBeInTheDocument()
  expect(
    within(popover).getByText('60%', { selector: 'dd' })
  ).toBeInTheDocument()
})

test('shows usage and the wallet dependency for an unlimited key without a percentage or artificial total', async () => {
  const user = userEvent.setup()
  render(
    <ApiKeyQuotaCell
      apiKey={{ ...apiKey, unlimited_quota: true }}
      now={0}
      variant='card'
    />
  )
  expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  await user.click(
    screen.getByRole('button', { name: /Unlimited; Used amount 0.4/ })
  )
  expect(
    screen.getByText(
      'This API key has no quota limit. Requests still require available wallet or subscription quota.'
    )
  ).toBeInTheDocument()
  expect(screen.queryByText('Current total quota')).not.toBeInTheDocument()
})

test.each([0, -100000])(
  'preserves a remaining quota of %s and keeps progress nonnegative',
  async (remaining) => {
    const user = userEvent.setup()
    render(
      <ApiKeyQuotaCell
        apiKey={{ ...apiKey, remain_quota: remaining }}
        now={0}
      />
    )
    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '0'
    )
    await user.click(screen.getByRole('button', { name: /^Remaining/ }))
    const popover = screen.getByRole('dialog')
    expect(
      within(popover).getByText(remaining === 0 ? '0' : '-0.2', {
        selector: 'dd',
      })
    ).toBeInTheDocument()
  }
)

test('makes zero user quota inspectable and preserves negative balances', async () => {
  const user = userEvent.setup()
  const view = render(<UserQuotaCell remaining={0} used={0} />)
  await user.click(screen.getByRole('button', { name: 'No Quota' }))
  expect(
    within(screen.getByRole('dialog')).getAllByText('0', { selector: 'dd' })
  ).toHaveLength(2)
  view.unmount()
  render(<UserQuotaCell remaining={-500000} used={1000000} />)
  await user.click(
    screen.getByRole('button', { name: 'Available Balance -1; Used amount 2' })
  )
  expect(
    within(screen.getByRole('dialog')).getByText('-1', { selector: 'dd' })
  ).toBeInTheDocument()
})
