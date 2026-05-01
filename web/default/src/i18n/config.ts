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
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import en from './locales/en.json'
import fr from './locales/fr.json'
import ja from './locales/ja.json'
import ru from './locales/ru.json'
import vi from './locales/vi.json'
import zh from './locales/zh.json'

export const resources = {
  en,
  zh,
  fr,
  ru,
  ja,
  vi,
} as const

// Simple language detector without Chrome Built-in AI dependency
const storageKey = 'i18nextLng'
const customLanguageDetector = {
  name: 'customDetector',
  lookup() {
    try {
      const stored = localStorage.getItem(storageKey)
      if (stored) return stored
    } catch { /* empty */ }
    if (typeof navigator !== 'undefined' && navigator.language) {
      const lang = navigator.language.split('-')[0]
      if (lang && ['en', 'zh', 'fr', 'ru', 'ja', 'vi'].includes(lang)) return lang
    }
    return undefined
  },
  cacheUserLanguage(lng: string) {
    try { localStorage.setItem(storageKey, lng) } catch { /* empty */ }
  },
}

i18n
  .use({
    type: 'languageDetector' as const,
    init: () => {},
    detect: () => customLanguageDetector.lookup(),
    cacheUserLanguage: (lng: string) => customLanguageDetector.cacheUserLanguage(lng),
  })
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    supportedLngs: ['en', 'zh', 'fr', 'ru', 'ja', 'vi'],
    load: 'languageOnly',
    nsSeparator: false,
    showSupportNotice: false,
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false,
    },
  })

export default i18n
