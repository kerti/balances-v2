// Returns the active locale as a BCP47 string and a setter that both calls
// i18next and mirrors the choice into localStorage. Settings calls setLocale()
// after the users.locale PATCH succeeds; other code reads the locale to drive
// format helpers (lib/format.ts).
import { useTranslation } from "react-i18next";
import { LOCALSTORAGE_KEY, SUPPORTED_LOCALES, type Locale } from "./index";

function normalise(raw: string | undefined): Locale {
  if (!raw) return "en-GB";
  if ((SUPPORTED_LOCALES as readonly string[]).includes(raw)) {
    return raw as Locale;
  }
  // navigator.language or a stale localStorage value may return a 2-letter
  // 'en' / 'id'; project the base back onto a supported BCP47 form.
  const base = raw.split("-")[0];
  if (base === "id") return "id-ID";
  return "en-GB";
}

export function useLocale(): {
  locale: Locale;
  setLocale: (next: Locale) => Promise<void>;
} {
  const { i18n } = useTranslation();
  const locale = normalise(i18n.language);
  const setLocale = async (next: Locale) => {
    localStorage.setItem(LOCALSTORAGE_KEY, next);
    await i18n.changeLanguage(next);
  };
  return { locale, setLocale };
}
