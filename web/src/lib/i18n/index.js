import { writable, derived } from 'svelte/store';
import en from './en.json';
import zh from './zh.json';

const translations = { en, zh };

export const locale = writable(localStorage.getItem('tw_locale') || 'zh');

locale.subscribe((val) => {
  localStorage.setItem('tw_locale', val);
});

export const t = derived(locale, ($locale) => {
  const dict = translations[$locale] || translations.zh;
  return function translate(key, params = {}) {
    const keys = key.split('.');
    let val = dict;
    for (const k of keys) {
      if (val && typeof val === 'object' && k in val) {
        val = val[k];
      } else {
        return key; // fallback to key
      }
    }
    if (typeof val !== 'string') return key;
    return val.replace(/\{(\w+)\}/g, (_, k) => (params[k] ?? `{${k}}`));
  };
});

export const locales = [
  { code: 'zh', label: '中文' },
  { code: 'en', label: 'EN' },
];
