import { describe, expect, it } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformFormDataToCreatePayload,
} from '../channel-form'

describe('channel output strategy', () => {
  it('defaults every new channel to upstream passthrough', () => {
    const payload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'test-channel',
      key: 'test-key',
    })

    expect(CHANNEL_FORM_DEFAULT_VALUES.image_output_strategy).toBe(
      'passthrough'
    )
    expect(JSON.parse(payload.channel.settings || '{}')).toMatchObject({
      image_output_strategy: 'passthrough',
    })
  })
})
