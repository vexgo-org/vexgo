import { createContext, useContext, useState, useCallback } from "react";
import type { ReactNode } from "react";
import {
  getLocale,
  isLocale,
  loadLocale,
  setLocale as setLocaleUtil,
  t as translate,
  type Locale,
} from "./i18n";

interface I18nContextType {
  locale: Locale;
  /**
   * Switches locale, fetching the message pack first so labels never render as
   * raw keys. Returns a promise the caller may ignore from a click handler.
   */
  setLocale: (locale: Locale) => Promise<void>;
  t: (key: string, params?: Record<string, unknown>) => string;
}

const I18nContext = createContext<I18nContextType | undefined>(undefined);

interface I18nProviderProps {
  children: ReactNode;
}

export function I18nProvider({ children }: I18nProviderProps) {
  // The entry point loads and applies the starting locale before the first
  // render, so this only reads back the choice it already made.
  const [locale, setLocaleState] = useState<Locale>(() => {
    const savedLocale = localStorage.getItem("locale");
    return isLocale(savedLocale) ? savedLocale : getLocale();
  });

  const setLocale = useCallback(async (newLocale: Locale) => {
    await loadLocale(newLocale);
    setLocaleUtil(newLocale);
    setLocaleState(newLocale);
  }, []);

  const t = useCallback(
    (key: string, params?: Record<string, unknown>): string => {
      return translate(key, params);
    },
    [],
  );

  return (
    <I18nContext.Provider value={{ locale, setLocale, t }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useTranslation() {
  const context = useContext(I18nContext);
  if (context === undefined) {
    throw new Error("useTranslation must be used within an I18nProvider");
  }
  return context;
}
