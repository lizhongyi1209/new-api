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
import { describe, expect, test } from 'vitest'

import { resources } from '../config'
import {
  convertDetectedLanguage,
  INTERFACE_LANGUAGE_OPTIONS,
  normalizeInterfaceLanguage,
  toIntlLocale,
} from '../languages'

describe('supported interface languages', () => {
  test('loads and offers only Simplified Chinese and English', () => {
    expect(Object.keys(resources)).toEqual(['en', 'zhCN'])
    expect(INTERFACE_LANGUAGE_OPTIONS.map((language) => language.code)).toEqual(
      ['zhCN', 'en']
    )
  })

  test.each(['zh', 'zh-CN', 'zh_Hans', 'zh-TW', 'zh-Hant', 'zhTW'])(
    'normalizes Chinese locale %s to the Simplified Chinese pack',
    (language) => {
      expect(normalizeInterfaceLanguage(language)).toBe('zhCN')
      expect(convertDetectedLanguage(language)).toBe('zhCN')
    }
  )

  test('falls unsupported saved preferences back to English', () => {
    expect(normalizeInterfaceLanguage('fr')).toBe('en')
    expect(normalizeInterfaceLanguage('ja-JP')).toBe('en')
    expect(convertDetectedLanguage('fr-FR')).toBe('fr-FR')
  })

  test('converts the internal Chinese code for Intl formatting', () => {
    expect(toIntlLocale('zhCN')).toBe('zh-CN')
    expect(toIntlLocale('en')).toBe('en')
    expect(toIntlLocale('zhTW')).toBe('zh-TW')
  })
})
