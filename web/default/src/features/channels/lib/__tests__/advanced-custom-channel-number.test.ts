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
import { expect, test } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  buildSettingJSON,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

const config = {
  advanced_routes: [
    {
      incoming_path: '/v1/models',
      upstream_path: '/draft/models',
      converter: 'none',
    },
  ],
}
const fields = {
  ...CHANNEL_FORM_DEFAULT_VALUES,
  name: 'Synthetic channel',
  key: 'synthetic-key',
  models: 'gpt-test',
  group: ['default'],
  base_url: 'https://upstream.test',
}

test('local AdvancedCustom type 59 validates and retains route configuration for create and update', () => {
  const form = {
    ...fields,
    type: 59,
    advanced_custom: JSON.stringify(config),
    settings: JSON.stringify({ future_setting: 0 }),
  }
  expect(channelFormSchema.safeParse(form).success).toBe(true)
  for (const payload of [
    transformFormDataToCreatePayload(form).channel,
    transformFormDataToUpdatePayload(form, 42),
  ]) {
    expect(payload.type).toBe(59)
    expect(JSON.parse(payload.settings ?? '{}')).toMatchObject({
      advanced_custom: config,
      future_setting: 0,
    })
  }
  expect(
    channelFormSchema.safeParse({ ...form, advanced_custom: '' }).success
  ).toBe(false)
})

test('setting edits retain unknown zero/false fields while clearing old transport options when defaults are chosen', () => {
  const setting = JSON.parse(
    buildSettingJSON({
      ...fields,
      setting: JSON.stringify({
        future_zero: 0,
        future_flag: false,
        http_protocol: 'http1',
        http2_connection_shards: 4,
      }),
      http_protocol: 'auto',
      http2_connection_shards: 1,
    })
  )
  expect(setting).toMatchObject({ future_zero: 0, future_flag: false })
  expect(setting).not.toHaveProperty('http_protocol')
  expect(setting).not.toHaveProperty('http2_connection_shards')
})

test('local TencentVideo type 58 validates without AdvancedCustom configuration and keeps its provider type', () => {
  const form = { ...fields, type: 58, advanced_custom: '' }
  expect(channelFormSchema.safeParse(form).success).toBe(true)
  const payload = transformFormDataToCreatePayload(form).channel
  expect(payload.type).toBe(58)
  expect(JSON.parse(payload.settings ?? '{}')).not.toHaveProperty(
    'advanced_custom'
  )
})
