import { useTranslation } from "react-i18next";
import wordmarkDark from "@/assets/brand/wordmark-dark.svg";
import wordmarkLight from "@/assets/brand/wordmark-light.svg";
import { useTheme } from "@/theme/useTheme";

// The Balances wordmark — outlined Plus Jakarta Sans, the word alone (ADR-0054).
// Brand assets and the regeneration recipe live in docs/brand/logo.md; run
// `make brand` to regenerate, which also refreshes the copies under src/assets.
//
// Wordmark-only, no mark beside it: the identity now lives *in* the word (the
// tall tapered brass `l` is the fulcrum post), so pairing it with the mark would
// state the same idea twice. The mark stands alone where there is no room for
// text — favicon, app icon, email header, PDF report.
//
// Outlined to <path> rather than set as live text, so the logo carries no font
// dependency and renders identically everywhere. That matters more than usual
// here: the app's UI face is Geist, not Plus Jakarta Sans, so a live-text
// wordmark would depend on a webfont the app otherwise never loads.
//
// The variant follows the active theme (issue #33): the dark wordmark on the
// dark palette, the light wordmark on the light one. Always rendered inside the
// ThemeProvider (mounted in main.tsx), so useTheme() is safe in every placement
// — including the pre-auth SignInScreen.
//
// `className` controls sizing per placement: the default `h-7 w-auto` suits the
// inline spots (mobile top bar, sign-in card); the sidebar passes `w-full h-auto`
// so the wordmark spans the sidebar width.
export function AppLogo({ className = "h-7 w-auto" }: { className?: string }) {
  const { t } = useTranslation("common");
  const { theme } = useTheme();
  const src = theme === "dark" ? wordmarkDark : wordmarkLight;
  return <img src={src} alt={t("brand")} className={className} />;
}
