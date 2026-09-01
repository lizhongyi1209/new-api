import { describe, expect, it } from 'vitest'

import {
  SEEDANCE_HC_MODELS,
  SEEDANCE_HC_TO_MAX_MODEL_MAPPING,
  resolveSeedanceMaxChannelModelDefaults,
} from '../channel-type-config'

describe('Seedance MAX channel defaults', () => {
  it('keeps HC models public and maps each one to its MAX upstream model', () => {
    const defaults = resolveSeedanceMaxChannelModelDefaults(60, '', '')

    expect(defaults.models.split(',')).toEqual([...SEEDANCE_HC_MODELS])
    expect(JSON.parse(defaults.modelMapping)).toEqual(
      SEEDANCE_HC_TO_MAX_MODEL_MAPPING
    )
    expect(defaults.models).not.toContain('-max')
  })

  it('preserves explicit channel models and mappings', () => {
    const defaults = resolveSeedanceMaxChannelModelDefaults(
      60,
      'custom-seedance',
      '{"custom-seedance":"custom-upstream"}'
    )

    expect(defaults).toEqual({
      models: 'custom-seedance',
      modelMapping: '{"custom-seedance":"custom-upstream"}',
    })
  })

  it('does not add Seedance defaults to other channel types', () => {
    expect(resolveSeedanceMaxChannelModelDefaults(1, '', '')).toEqual({
      models: '',
      modelMapping: '',
    })
  })
})
