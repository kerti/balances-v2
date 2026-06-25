// useTheme returns the active theme and a setter that persists to localStorage
// and reflects the choice onto <html> (via ThemeProvider). Settings calls
// setTheme() then PATCHes users.theme; AppLogo reads theme to pick its variant.
// Mirrors src/i18n/useLocale.ts for the theme axis.
import { useContext } from "react";
import { ThemeContext, type ThemeContextValue } from "@/theme";

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return ctx;
}
