// Interface language. Messages are keyed by their Simplified Chinese text (gettext
// style): zh-CN needs no table, and a key missing from another table falls back to it.
import de from '../locales/de'
import en from '../locales/en'
import fr from '../locales/fr'
import ja from '../locales/ja'
import zhTW from '../locales/zh-TW'

export type Locale = 'zh-CN' | 'zh-TW' | 'en' | 'ja' | 'fr' | 'de'

// Names are shown in their own language so anyone can find theirs.
export const locales: { id: Locale; name: string }[] = [
  { id: 'zh-CN', name: '简体中文' },
  { id: 'zh-TW', name: '繁體中文' },
  { id: 'en', name: 'English' },
  { id: 'ja', name: '日本語' },
  { id: 'fr', name: 'Français' },
  { id: 'de', name: 'Deutsch' },
]

const tables: Record<Locale, Record<string, string>> = { 'zh-CN': {}, 'zh-TW': zhTW, en, ja, fr, de }
const key = 'pare.lang'

function detect(): Locale {
  try {
    const saved = localStorage.getItem(key)
    if (saved && saved in tables) return saved as Locale
  } catch {}
  const lang = (navigator.language || 'zh-CN').toLowerCase()
  if (/^zh-(tw|hk|mo)|hant/.test(lang)) return 'zh-TW'
  if (lang.startsWith('zh')) return 'zh-CN'
  for (const id of ['ja', 'fr', 'de'] as const) if (lang.startsWith(id)) return id
  return 'en'
}

export const i18n = $state({ locale: detect() })

let notify: ((locale: Locale) => void) | undefined

// App registers this so the backend writes AI findings and its messages in the same language.
export function onLocaleChange(fn: (locale: Locale) => void) {
  notify = fn
  fn(i18n.locale)
}

export function setLocale(locale: Locale) {
  i18n.locale = locale
  document.documentElement.lang = locale
  try {
    localStorage.setItem(key, locale)
  } catch {}
  notify?.(locale)
}

export function initLocale() {
  document.documentElement.lang = i18n.locale
}

// t translates text and fills {name} placeholders. Reading i18n.locale here makes every
// template that calls t() update when the language changes.
export function t(text: string, params?: Record<string, string | number>): string {
  const message = tables[i18n.locale][text] || text
  if (!params) return message
  return message.replace(/\{(\w+)\}/g, (match, name) => (name in params ? String(params[name]) : match))
}

// numberLocale formats counts and dates in the interface language.
export function numberLocale(): string {
  return i18n.locale
}
