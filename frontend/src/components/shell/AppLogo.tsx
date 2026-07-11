import { useTranslation } from "react-i18next";
import wordmarkDark from "@/assets/brand/wordmark-dark.svg";
import wordmarkLight from "@/assets/brand/wordmark-light.svg";
import { useTheme } from "@/theme/useTheme";

// The Balances wordmark (outlined IBM Plex Sans + the snapshot-scale glyph).
// Brand assets and the regeneration recipe live in docs/brand/logo.md.
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
