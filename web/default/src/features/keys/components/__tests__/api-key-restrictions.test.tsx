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
import { expect, test } from 'vitest'

import type { ApiKey } from '../../types'
import { ModelLimitsCell, IpRestrictionsCell } from '../api-keys-cells'

const apiKey: ApiKey = {
  id: 1,
  name: 'UI test',
  key: '',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 0,
  accessed_time: 0,
  group: '',
  auto_groups: null,
  cross_group_retry: false,
  model_limits_enabled: true,
  model_limits: 'model-a,model-b',
  allow_ips: '127.0.0.1\n192.0.2.0/24',
}

test('mobile model and IP details open on click and return focus on Escape', async () => {
  const user = userEvent.setup()
  render(
    <>
      <ModelLimitsCell apiKey={apiKey} detailsTrigger='click' />
      <IpRestrictionsCell apiKey={apiKey} detailsTrigger='click' />
    </>
  )
  const models = screen.getByRole('button', { name: 'Models: 2 models' })
  await user.click(models)
  expect(screen.getByRole('dialog', { name: 'Models' })).toBeVisible()
  expect(screen.getByText('model-a')).toBeVisible()
  expect(screen.getByText('model-b')).toBeVisible()
  await user.keyboard('{Escape}')
  expect(models).toHaveFocus()
  await user.click(
    screen.getByRole('button', { name: 'IP Restriction: 2 IP(s)' })
  )
  expect(screen.getByRole('dialog', { name: 'IP Restriction' })).toBeVisible()
  expect(screen.getByText('192.0.2.0/24')).toBeVisible()
})

test('disabled model limits and empty IP restrictions describe their unrestricted state without detail buttons', () => {
  render(
    <>
      <ModelLimitsCell
        apiKey={{ ...apiKey, model_limits_enabled: false }}
        detailsTrigger='click'
      />
      <IpRestrictionsCell
        apiKey={{ ...apiKey, allow_ips: '  \n ' }}
        detailsTrigger='click'
      />
    </>
  )
  expect(screen.getByText('Unlimited')).toBeVisible()
  expect(screen.getByText('No restriction')).toBeVisible()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})
