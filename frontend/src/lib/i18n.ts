import type { enUS } from "@/locales/en-US";

/**
 * The locales the admin console can display.
 *
 * Message packs are code-split: bundling both put ~65 kB of translations in
 * the entry chunk, and a reader only ever needs one. `main.tsx` loads the
 * active pack before the first render, so nothing downstream has to cope with
 * a half-loaded locale.
 */
export type Locale = "zh-CN" | "en-US";

type Translations = typeof enUS;

const loaders: Record<Locale, () => Promise<Translations>> = {
  "zh-CN": () => import("@/locales/zh-CN").then((m) => m.zhCN),
  "en-US": () => import("@/locales/en-US").then((m) => m.enUS),
};

const translations: Partial<Record<Locale, Translations>> = {};
const inFlight = new Map<Locale, Promise<void>>();

const DEFAULT_LOCALE: Locale = "zh-CN";

let currentLocale: Locale = DEFAULT_LOCALE;

/** Narrows an untrusted value — a persisted preference, say — to a Locale. */
export function isLocale(value: string | null): value is Locale {
  return value === "zh-CN" || value === "en-US";
}

function detectLocale(): Locale {
  const savedLocale = localStorage.getItem("locale");
  if (isLocale(savedLocale)) {
    return savedLocale;
  }

  if (typeof navigator !== "undefined") {
    const languages = navigator.languages || [navigator.language];

    for (const lang of languages) {
      if (!lang) continue;

      const normalizedLang = lang.toLowerCase();

      if (normalizedLang.startsWith("zh")) {
        return "zh-CN";
      }

      if (normalizedLang.startsWith("en")) {
        return "en-US";
      }
    }
  }

  return DEFAULT_LOCALE;
}

/** The locale to start in: the stored preference, else the browser's. */
export function resolveInitialLocale(): Locale {
  return detectLocale();
}

/**
 * Fetches a locale's message pack. Resolves at once when it is already in
 * memory, and shares one request when several callers ask at the same time.
 */
export function loadLocale(locale: Locale): Promise<void> {
  if (translations[locale]) {
    return Promise.resolve();
  }

  const existing = inFlight.get(locale);
  if (existing) {
    return existing;
  }

  const pending = loaders[locale]()
    .then((table) => {
      translations[locale] = table;
    })
    .finally(() => {
      inFlight.delete(locale);
    });

  inFlight.set(locale, pending);
  return pending;
}

/**
 * Switches the active locale and remembers it. The caller must have awaited
 * {@link loadLocale} for it, or `t` will fall back to raw keys.
 */
export function setLocale(locale: Locale) {
  currentLocale = locale;
  localStorage.setItem("locale", locale);
}

export function getLocale(): Locale {
  return currentLocale;
}

export function t(key: string, params?: Record<string, unknown>): string {
  const table = translations[currentLocale];
  if (!table) {
    return key;
  }

  const keys = key.split(".");
  let value: unknown = table;

  for (const k of keys) {
    if (value && typeof value === "object" && k in value) {
      value = (value as Record<string, unknown>)[k];
    } else {
      return key;
    }
  }

  let result = (value as string) || key;

  // Substitute parameters when provided
  if (params) {
    result = result.replace(/\{(\w+)\}/g, (match, paramName) => {
      return params[paramName] !== undefined
        ? String(params[paramName])
        : match;
    });
  }

  return result;
}
