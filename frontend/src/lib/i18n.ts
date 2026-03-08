import { zhCN } from '@/locales/zh-CN';
import { enUS } from '@/locales/en-US';

type Translations = typeof zhCN;

const translations: Record<string, Translations> = {
  'zh-CN': zhCN,
  'en-US': enUS,
};

let currentLocale = 'zh-CN';

export function setLocale(locale: string) {
  if (locale === 'zh-CN' || locale === 'en-US') {
    currentLocale = locale;
    localStorage.setItem('locale', locale);
  }
}

export function getLocale(): string {
  return currentLocale;
}

export function t(key: string): string {
  const keys = key.split('.');
  let value: unknown = translations[currentLocale];
  
  for (const k of keys) {
    if (value && typeof value === 'object' && k in value) {
      value = (value as Record<string, unknown>)[k];
    } else {
      return key;
    }
  }
  
  return (value as string) || key;
}

// 初始化：从 localStorage 读取语言设置
const savedLocale = localStorage.getItem('locale');
if (savedLocale === 'zh-CN' || savedLocale === 'en-US') {
  currentLocale = savedLocale;
} else if (navigator.language.startsWith('zh')) {
  currentLocale = 'zh-CN';
}
