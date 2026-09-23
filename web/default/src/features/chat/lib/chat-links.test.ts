import { expect, test } from 'vitest'

import {
  chatLinkRequiresApiKey,
  parseChatConfig,
  resolveChatUrl,
} from './chat-links'

test('AQBot configuration encodes each value and preserves the New API identity', () => {
  const url = resolveChatUrl({
    template: 'aqbot://import?{aqbotConfig}',
    apiKey: 'synthetic&override=true',
    serverAddress: 'https://gateway.example/prefix?region=亚洲&version=2',
  })
  const parsed = new URL(url)
  expect(parsed.searchParams.get('name')).toBe('New API')
  expect(parsed.searchParams.get('baseurl')).toBe(
    'https://gateway.example/prefix?region=亚洲&version=2'
  )
  expect(parsed.searchParams.get('apikey')).toBe('sk-synthetic&override=true')
  expect(parsed.searchParams.get('type')).toBe('openai')
  expect(parsed.searchParams.has('override')).toBe(false)
  expect(chatLinkRequiresApiKey('aqbot://import?{aqbotConfig}')).toBe(true)
})

test('a server-only web preset does not require a key', () => {
  const [preset] = parseChatConfig([
    { 'existing web chat': 'https://chat.example/?server={address}' },
  ])
  expect(chatLinkRequiresApiKey(preset.url)).toBe(false)
  expect(
    resolveChatUrl({
      template: preset.url,
      serverAddress: 'https://gateway.example/v1',
    })
  ).toBe('https://chat.example/?server=https%3A%2F%2Fgateway.example%2Fv1')
})

test.each(['cherryConfig', 'aionuiConfig', 'deepchatConfig'])(
  'preserves the existing %s import protocol',
  (placeholder) => {
    const resolved = resolveChatUrl({
      template: `client://import?config={${placeholder}}`,
      apiKey: 'synthetic',
      serverAddress: 'https://gateway.example',
    })
    const payload = JSON.parse(
      atob(new URL(resolved).searchParams.get('config') ?? '')
    )
    expect(payload).toMatchObject({
      baseUrl: 'https://gateway.example',
      apiKey: 'sk-synthetic',
    })
    expect(chatLinkRequiresApiKey(`{${placeholder}}`)).toBe(true)
  }
)
