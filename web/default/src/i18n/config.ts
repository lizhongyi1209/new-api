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
