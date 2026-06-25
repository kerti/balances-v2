import { useCallback, useState, type ReactNode } from "react";
import {
  LOCALSTORAGE_KEY,
  ThemeContext,
  applyTheme,
  resolveBootTheme,
  type Theme,
} from "@/theme";

// ThemeProvider holds the active theme and is the single writer of the choice:
// setTheme persists to localStorage, reflects the `dark` class onto <html>, and
// updates state so consumers (AppLogo, the Settings card) re-render. The PATCH
// to users.theme is issued by the caller (Settings) / the reconcile hook, not
// here — same split as the locale flow, where useLocale owns localStorage +
// i18next and Settings owns the PATCH.
//
// Initial state comes from resolveBootTheme() so it matches what the inline
// boot script in index.html already painted; no flash on hydrate.
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(resolveBootTheme);

  const setTheme = useCallback((next: Theme) => {
    localStorage.setItem(LOCALSTORAGE_KEY, next);
    applyTheme(next);
    setThemeState(next);
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}
