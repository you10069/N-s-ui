import { createI18n } from 'vue-i18n'
import en from './en'
import zhcn from './zhcn'

export type AppLocale = 'en' | 'zhHans'

export const normalizeLocale = (value: string | null | undefined): AppLocale =>
  value === 'zhHans' ? 'zhHans' : 'en'

const initialLocale = normalizeLocale(localStorage.getItem('locale'))
localStorage.setItem('locale', initialLocale)

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'en',
  messages: {
    en,
    zhHans: zhcn,
  },
})

export const languages = [
  { title: '简体中文', value: 'zhHans' },
  { title: 'English', value: 'en' },
]
