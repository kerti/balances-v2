// Theme entry point. Mirrors src/i18n for the UI-theme axis: users.theme is the
// server-side source of truth (PATCH /api/me), localStorage is the
// before-first-paint cache, and prefers-color-scheme is the first-login
// fallback. The active theme toggles the `dark` class on <html>; index.css
// ships both palettes (:root = light, .dark = dark).
//
// Unlike locale (driven by i18next's own change events), theme has no library
// emitting changes, so a small React context (ThemeProvider/useTheme) holds the
// active value and re-renders consumers like AppLogo.
import { createContext } from "react";

export const SUPPORTED_THEMES = ["light", "dark"] as const;
export type Theme = (typeof SUPPORTED_THEMES)[number];
export const LOCALSTORAGE_KEY = "balances.theme";

export function isSupportedTheme(value: string): value is Theme {
  return (SUPPORTED_THEMES as readonly string[]).includes(value);
}

// applyTheme reflects the choice onto <html>: the `dark` class drives the CSS
// custom-property palette in index.css. Kept here so the boot script in
// index.html and the React layer agree on the exact mechanism.
export function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

// resolveBootTheme is the same precedence the inline boot script in index.html
// uses: an explicit stored choice, then the OS preference, then dark (the
// app's historical default). The React provider initialises from this so its
// state matches whatever the boot script already painted — no flash, no
// flip-on-hydrate.
export function resolveBootTheme(): Theme {
  const stored = localStorage.getItem(LOCALSTORAGE_KEY);
  if (stored && isSupportedTheme(stored)) return stored;
  if (
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-color-scheme: light)").matches
  ) {
    return "light";
  }
  return "dark";
}

export type ThemeContextValue = {
  theme: Theme;
  setTheme: (next: Theme) => void;
};

export const ThemeContext = createContext<ThemeContextValue | null>(null);
