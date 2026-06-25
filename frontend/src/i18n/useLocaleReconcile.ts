// useLocaleReconcile syncs the UI language to the just-loaded user row. The
// saved user.locale is the single source of truth (ADR-0035): on sign-in the UI
// snaps to it and localStorage is primed to match. Mounted from AppShell once
// useSession resolves a user.
//
// The previous first-login navigator-flip — which PATCHed the user row to the
// browser language — was retired with the pre-auth language picker (ADR-0035,
// superseding that part of ADR-0026). navigator detection now only pre-fills the
// pre-auth picker (display-only); it never mutates an account. A brand-new
// account's locale is seeded server-side at birth from the picker's choice, so
// by the time the user row loads here the DB is already correct and there is
// nothing to negotiate.
import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { LOCALSTORAGE_KEY, SUPPORTED_LOCALES, type Locale } from "@/i18n";
import type { Me } from "@/hooks/useSession";

function isSupported(locale: string): locale is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(locale);
}

export function useLocaleReconcile(user: Me | null | undefined) {
  const { i18n } = useTranslation();
  // Fire once per mounted user; guard so a session refetch doesn't re-run.
  const reconciled = useRef(false);

  useEffect(() => {
    if (!user || reconciled.current) return;
    reconciled.current = true;

    const userLocale: Locale = isSupported(user.locale) ? user.locale : "en-GB";
    localStorage.setItem(LOCALSTORAGE_KEY, userLocale);
    if (i18n.language !== userLocale) {
      void i18n.changeLanguage(userLocale);
    }
    // i18n is a stable reference; user is the trigger.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user]);
}
