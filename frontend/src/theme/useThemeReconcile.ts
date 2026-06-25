// useThemeReconcile syncs the UI theme to the just-loaded user row, and flips a
// first-login fresh signup to the OS preference when that disagrees with the DB
// default. Mounted from AppShell once useSession resolves a user. Mirrors
// useLocaleReconcile, with prefers-color-scheme standing in for navigator.language.
//
// Reconciliation logic:
//   1. If localStorage already holds a theme, the user has chosen before —
//      trust user.theme (set by the most recent Settings PATCH) and just sync
//      the UI to it. No OS-preference override.
//   2. If localStorage is empty AND prefers-color-scheme suggests a theme that
//      differs from user.theme, PATCH the new theme + apply it. This is the
//      first-login bias toward the device's appearance setting; it runs once
//      because step 1 fires forever after.
//   3. Otherwise (localStorage empty, OS agrees with user.theme) sync the UI to
//      user.theme and prime localStorage.
import { useEffect, useRef } from "react";
import { LOCALSTORAGE_KEY, isSupportedTheme, type Theme } from "@/theme";
import { useTheme } from "@/theme/useTheme";
import { useUpdateMe } from "@/hooks/useUpdateMe";
import type { Me } from "@/hooks/useSession";

function osPick(): Theme | null {
  if (typeof window.matchMedia !== "function") return null;
  if (window.matchMedia("(prefers-color-scheme: light)").matches)
    return "light";
  if (window.matchMedia("(prefers-color-scheme: dark)").matches) return "dark";
  return null;
}

export function useThemeReconcile(user: Me | null | undefined) {
  const { theme, setTheme } = useTheme();
  const updateMe = useUpdateMe();
  // Fire once per mounted user; guard so a session refetch doesn't trigger
  // another OS-preference flip.
  const reconciled = useRef(false);

  useEffect(() => {
    if (!user || reconciled.current) return;
    reconciled.current = true;

    const stored = localStorage.getItem(LOCALSTORAGE_KEY);
    const userTheme: Theme = isSupportedTheme(user.theme) ? user.theme : "dark";

    if (stored) {
      // Returning device: trust the server-side choice.
      if (theme !== userTheme) setTheme(userTheme);
      return;
    }

    // First-login on this device. Look at the OS preference before settling.
    const os = osPick();
    if (os && os !== userTheme) {
      setTheme(os);
      updateMe.mutate({ theme: os });
      return;
    }

    // OS agrees (or said nothing useful) — just prime localStorage via setTheme.
    setTheme(userTheme);
    // setTheme and updateMe are stable references; user is the trigger.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user]);
}
