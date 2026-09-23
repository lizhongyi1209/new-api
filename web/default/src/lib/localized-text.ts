export type LocalizedTextValue = string | Record<string, string> | null

export function resolveLocalizedText(
  value: LocalizedTextValue | undefined,
  language: string
): string {
  if (typeof value === 'string') return value.trim()
  if (!value) return ''

  const baseLanguage = language.split('-')[0]
  for (const key of [language, baseLanguage, 'en', 'zh']) {
    const text = value[key]?.trim()
    if (text) return text
  }
  return (
    Object.values(value)
      .find((text) => text.trim())
      ?.trim() ?? ''
  )
}
