import { describe, expect, it } from 'vitest'

import {
  DOUBAO_SEEDANCE_MAX_MODELS,
  SEEDANCE_HC_MODELS,
  SEEDANCE_HC_TO_MAX_MODEL_MAPPING,
  SERVICE_INFERENCE_SEEDANCE_MODELS,
  resolveSeedanceMaxChannelModelDefaults,
} from '../channel-type-config'

describe('Seedance MAX channel defaults', () => {
  it('keeps HC mappings and exposes Doubao MAX models directly', () => {
    const defaults = resolveSeedanceMaxChannelModelDefaults(60, '', '')

    expect(defaults.models.split(',')).toEqual([
      ...SERVICE_INFERENCE_SEEDANCE_MODELS,
    ])
    expect(JSON.parse(defaults.modelMapping)).toEqual(
      SEEDANCE_HC_TO_MAX_MODEL_MAPPING
    )
    expect(defaults.models.split(',')).toEqual(
      expect.arrayContaining([...SEEDANCE_HC_MODELS])
    )
    expect(defaults.models.split(',')).toEqual(
      expect.arrayContaining([...DOUBAO_SEEDANCE_MAX_MODELS])
    )
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
