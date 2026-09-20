import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark" | "system";

/** Kept in sync with `SettingsPage` and `App` — one key, one source of truth. */
export const THEME_STORAGE_KEY = "theme";

export function readStoredTheme(): Theme {
  const saved = localStorage.getItem(THEME_STORAGE_KEY);
  return saved === "dark" || saved === "system" || saved === "light"
    ? saved
    : "light";
}

function systemPrefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

/**
 * Resolves a theme into the `dark`/`light` class pair the token layer keys
 * off, and keeps `color-scheme` honest so native controls (scrollbars, form
 * widgets, the captcha canvas) match instead of flashing white.
 */
export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  const isDark =
    theme === "dark" || (theme === "system" && systemPrefersDark());

  root.classList.toggle("dark", isDark);
  root.classList.toggle("light", !isDark);
  root.style.colorScheme = isDark ? "dark" : "light";
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(readStoredTheme);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  // While on "system", follow the OS live instead of only at first paint.
  useEffect(() => {
    if (theme !== "system") return;
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("system");
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, [theme]);

  // Both setters persist and paint immediately rather than waiting on an
  // effect: more than one component can own a `useTheme()` instance, and each
  // must see the change the same way.
  const setTheme = useCallback((next: Theme) => {
    localStorage.setItem(THEME_STORAGE_KEY, next);
    applyTheme(next);
    setThemeState(next);
  }, []);

  /** Flips between light and dark; a system default resolves to its opposite. */
  const toggleTheme = useCallback(() => {
    const current = readStoredTheme();
    const next =
      current === "dark" || (current === "system" && systemPrefersDark())
        ? "light"
        : "dark";
    localStorage.setItem(THEME_STORAGE_KEY, next);
    applyTheme(next);
    setThemeState(next);
  }, []);

  return { theme, setTheme, toggleTheme };
}
