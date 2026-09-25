import { expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

test('new channels default to OSS while explicit passthrough omits the strategy key', () => {
  expect(CHANNEL_FORM_DEFAULT_VALUES.image_output_strategy).toBe('oss')
  const form = {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Image channel',
    models: 'image-model',
  }
  const defaultPayload = transformFormDataToCreatePayload(form).channel
  expect(
    JSON.parse(defaultPayload.settings || '{}').image_output_strategy
  ).toBe('oss')

  const r2Payload = transformFormDataToCreatePayload({
    ...form,
    image_output_strategy: 'r2',
  }).channel
  expect(JSON.parse(r2Payload.settings || '{}').image_output_strategy).toBe(
    'r2'
  )

  const passthrough = transformFormDataToCreatePayload({
    ...form,
    image_output_strategy: 'passthrough',
  }).channel
  expect(JSON.parse(passthrough.settings || '{}')).not.toHaveProperty(
    'image_output_strategy'
  )
})

test('editing a channel without an output strategy keeps passthrough', () => {
  const channel = channelSchema.parse({
    id: 41,
    name: 'Existing image channel',
    type: 1,
    key: '',
    status: 1,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'image-model',
    group: 'default',
    settings: '{}',
  })
  const form = transformChannelToFormDefaults(channel)
  expect(form.image_output_strategy).toBe('passthrough')

  const payload = transformFormDataToUpdatePayload(form, channel.id)
  expect(JSON.parse(payload.settings || '{}')).not.toHaveProperty(
    'image_output_strategy'
  )
})

test('editing a channel keeps an existing local URL strategy until another option is chosen', () => {
  const channel = channelSchema.parse({
    id: 42,
    name: 'Legacy image channel',
    type: 1,
    key: '',
    status: 1,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'image-model',
    group: 'default',
    settings: JSON.stringify({ image_output_strategy: 'local_temp_cf' }),
  })
  const form = transformChannelToFormDefaults(channel)
  expect(form.image_output_strategy).toBe('local_temp_cf')
  const retained = transformFormDataToUpdatePayload(form, channel.id)
  expect(JSON.parse(retained.settings || '{}').image_output_strategy).toBe(
    'local_temp_cf'
  )

  const replaced = transformFormDataToUpdatePayload(
    { ...form, image_output_strategy: 'r2' },
    channel.id
  )
  expect(JSON.parse(replaced.settings || '{}').image_output_strategy).toBe('r2')
})

test('Gemini URL input capability persists for Gemini and Vertex channels only', () => {
  for (const type of [24, 41]) {
    const payload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'Gemini image channel',
      models: 'gemini-image',
      type,
      gemini_file_data_enabled: true,
    }).channel
    expect(JSON.parse(payload.settings || '{}').gemini_file_data_enabled).toBe(
      true
    )
  }

  const payload = transformFormDataToUpdatePayload(
    {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'OpenAI image channel',
      models: 'gpt-image-1',
      type: 1,
      settings: '{"gemini_file_data_enabled":true}',
      gemini_file_data_enabled: true,
    },
    42
  )
  expect(JSON.parse(payload.settings || '{}')).not.toHaveProperty(
    'gemini_file_data_enabled'
  )
})
