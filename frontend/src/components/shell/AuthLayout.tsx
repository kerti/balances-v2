import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { AppLogo } from "@/components/shell/AppLogo";

// AuthLayout is the shared shell for the six shell-less pre-auth gate screens
// (sign-in, onboarding, invite-accept, reset request/set, household-erased). One
// responsive layout, no runtime renderer split for the brand chrome (ADR-0050
// amendment): on phones the screen's Card sits centred on the muted ground; from
// `md` up a bounded, page-centred two-column block splits a brand hero (left)
// from the Card (right), so outer margins stay even and the gap is a fixed
// `gap-10` rather than a half-screen void.
//
// The hero (logo + headline + tagline) is additive brand chrome carrying no
// asserted testid/ARIA/state, so it lives in a desktop-only `hidden md:flex`
// block while every interactive/asserted element stays single-instance in the
// Card `children`. The optional `aside` slot lets a screen drop one asserted
// element into the left column on desktop (the sign-in demo notice); the caller
// mounts it here *or* in the Card by breakpoint, never both.
export function AuthLayout({ children, aside }: { children: ReactNode; aside?: ReactNode }) {
  const { t } = useTranslation("common");
  return (
    <div className="flex min-h-screen items-center justify-center bg-muted p-6 md:p-10">
      <div className="w-full max-w-4xl md:grid md:grid-cols-2 md:items-center md:gap-10">
        <aside className="hidden md:flex md:flex-col md:gap-6">
          <div className="space-y-4">
            <AppLogo className="h-14 w-auto" />
            <h1 className="text-3xl font-semibold tracking-tight text-foreground">
              {t("authHero.headline")}
            </h1>
            <p className="text-base text-muted-foreground">{t("authHero.body")}</p>
          </div>
          {aside}
        </aside>
        <main className="flex justify-center">{children}</main>
      </div>
    </div>
  );
}
