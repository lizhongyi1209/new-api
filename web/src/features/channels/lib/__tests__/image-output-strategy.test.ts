import { expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

test('new channels save OSS and R2 while passthrough omits the strategy key', () => {
  for (const strategy of ['oss', 'r2'] as const) {
    const form = {
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'Image channel',
      models: 'image-model',
      image_output_strategy: strategy,
    }
    const payload = transformFormDataToCreatePayload(form).channel
    expect(JSON.parse(payload.settings || '{}').image_output_strategy).toBe(
      strategy
    )
  }

  const passthrough = transformFormDataToCreatePayload({
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Image channel',
    models: 'image-model',
  }).channel
  expect(JSON.parse(passthrough.settings || '{}')).not.toHaveProperty(
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
