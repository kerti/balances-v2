import type { ChangeEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { AppLogo } from "@/components/shell/AppLogo";
import { AppInfo } from "@/components/shell/AppInfo";
import { LocalAuthForm } from "@/components/common/LocalAuthForm";
import { useAuthMethods } from "@/hooks/useAuthMethods";
import { useLocale } from "@/i18n/useLocale";
import { SUPPORTED_LOCALES, type Locale } from "@/i18n";
import { Card, CardContent, CardDescription, CardFooter, CardHeader } from "@/components/ui/card";

// In-language display names, shown regardless of the active UI language so a
// visitor reading the wrong language can still find their option. Mirrors the
// Settings LanguageCard map; kept local so the two screens stay independent.
const LANGUAGE_LABELS: Record<Locale, string> = {
  "en-GB": "English",
  "id-ID": "Bahasa Indonesia",
};

export function SignInScreen() {
  const { t } = useTranslation("common");
  // Pre-auth picker is display-only (ADR-0035): switching the language updates
  // i18next + localStorage for the unauthenticated UI and is carried to the
  // backend via the start link's ?lng= so a brand-new account is seeded in the
  // chosen language. It never PATCHes an account — a returning user's saved
  // locale always wins after sign-in. The initial value is navigator-derived by
  // i18next's language detector, so the picker arrives pre-filled.
  const { locale, setLocale } = useLocale();

  // Which identity providers this instance offers (ADR-0039). Until it resolves
  // we render no provider affordance; on a fetch failure we fall back to showing
  // Google so a transient blip never strands a hosted user at a blank door.
  const { data: methods, isError: methodsError } = useAuthMethods();
  const showGoogle = methods ? methods.google : methodsError;
  const showLocal = methods ? methods.local : false;

  const onChange = (e: ChangeEvent<HTMLSelectElement>) => {
    void setLocale(e.target.value as Locale);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardDescription>{t("signIn.tagline")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Public-demo notice (ADR-0041, #346/CF-03): the demo is a shared,
              public, writable sandbox behind one login, so state the terms up
              front — shared account, no real data, nightly wipe, no warranty,
              and the maintainer-run out-of-cycle reset (requested via the
              project link in the footer). `restore-commit` stays live on demo
              (accept + document, ADR-0041) — this notice is the documented
              acceptance of the defacement-until-reset window. */}
          {methods?.demo_mode && (
            <div
              data-testid="signin-demo-notice"
              role="note"
              className="space-y-2 rounded-md border border-border bg-muted/50 p-3 text-xs text-muted-foreground"
            >
              <p className="font-medium text-foreground">{t("signIn.demoNotice.title")}</p>
              <ul className="list-disc space-y-1 pl-4">
                <li>{t("signIn.demoNotice.shared")}</li>
                <li>{t("signIn.demoNotice.noReal")}</li>
                <li>{t("signIn.demoNotice.wipe")}</li>
                <li>{t("signIn.demoNotice.warranty")}</li>
                <li>{t("signIn.demoNotice.reset")}</li>
              </ul>
            </div>
          )}
          <div className="space-y-1">
            <Label htmlFor="signin-language">{t("signIn.languageLabel")}</Label>
            <select
              id="signin-language"
              data-testid="signin-language-select"
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
              value={locale}
              onChange={onChange}
            >
              {SUPPORTED_LOCALES.map((l) => (
                <option key={l} value={l}>
                  {LANGUAGE_LABELS[l]}
                </option>
              ))}
            </select>
          </div>
          {showGoogle && (
            <Button asChild className="w-full min-h-11 md:min-h-0">
              <a
                href={`/api/auth/google/start?lng=${encodeURIComponent(locale)}`}
                data-testid="signin-google"
              >
                {t("signIn.withGoogle")}
              </a>
            </Button>
          )}

          {showGoogle && showLocal && (
            <div className="flex items-center gap-2" data-testid="signin-divider">
              <div className="h-px flex-1 bg-border" />
              <span className="text-xs uppercase text-muted-foreground">{t("signIn.or")}</span>
              <div className="h-px flex-1 bg-border" />
            </div>
          )}

          {showLocal && <LocalAuthForm />}
        </CardContent>
        {/* Same identity block as the sidebar footer (issue #123) so an
            unauthenticated visitor can still see the version, deploy target,
            and project/maintainer links. */}
        <CardFooter className="border-t pt-4">
          <AppInfo variant="split" />
        </CardFooter>
      </Card>
    </div>
  );
}
