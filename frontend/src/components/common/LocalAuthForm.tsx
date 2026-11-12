import { useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
import { errorMessage } from "@/lib/errorMessage";
import { routes } from "@/lib/routes";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuthMethods } from "@/hooks/useAuthMethods";
import { useLocale } from "@/i18n/useLocale";

type Mode = "signin" | "register";

// LocalAuthForm renders the email + password identity provider (ADR-0039),
// shown only when the instance has AUTH_LOCAL_ENABLED (the sign-in screen gates
// on the methods endpoint). It toggles between signing in and self-registering
// the founder. Register routes through the onboarding gate exactly like Google,
// so on success it navigates to /onboarding rather than straight into the app.
export function LocalAuthForm() {
  const { t } = useTranslation("common");
  const { locale } = useLocale();
  // Emailed reset is only offered when the instance has both local auth and
  // outbound email (#282). With mail off the link is hidden — recovery there is
  // operator-assisted (#283/#284), not self-service.
  const { data: methods } = useAuthMethods();
  const showReset = methods ? methods.password_reset : false;
  // Closed founding (FOUNDING_DISABLED, #842): the Create account tab would only
  // dead-end at the onboarding gate (INV-AUTH-26), so it is not offered. Hidden
  // until methods resolves too, so the tab never flashes in then vanishes. Invitees
  // set their password from the invite link (ADR-0039), not from this form.
  const foundingOpen = methods ? methods.founding_open : false;
  const [chosenMode, setMode] = useState<Mode>("signin");
  const mode: Mode = foundingOpen ? chosenMode : "signin";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");

  // Public demo (ADR-0041, #217): pre-fill the shared demo login once methods
  // loads, so a visitor can just click Sign in. The caption below repeats the
  // same credentials in plain text as a fail-safe against the visitor's typing
  // clobbering these fields — there is no confidentiality cost, every visitor
  // shares this one identity regardless of whether the values are shown.
  //
  // Adjusted during render (React's documented pattern for syncing state from
  // a prop/query that resolves later) rather than in an effect, so it applies
  // before the first paint instead of flashing empty fields then refilling.
  const [demoPrefilled, setDemoPrefilled] = useState(false);
  if (!demoPrefilled && methods) {
    setDemoPrefilled(true);
    if (methods.demo_mode) {
      setEmail(methods.demo_email ?? "");
      setPassword(methods.demo_password ?? "");
    }
  }

  const login = useMutation({
    mutationFn: () =>
      api("/api/auth/local/login", {
        method: "POST",
        body: JSON.stringify({ email, password }),
      }),
    // A fresh session now exists; a full reload lands in the authenticated app.
    onSuccess: () => window.location.assign("/"),
  });

  const register = useMutation({
    mutationFn: () =>
      api("/api/auth/local/register", {
        method: "POST",
        body: JSON.stringify({
          email,
          password,
          display_name: displayName,
          locale,
        }),
      }),
    // No account yet — the handshake cookie is set; the gate commits the founder.
    onSuccess: () => window.location.assign(routes.onboarding),
  });

  const active = mode === "signin" ? login : register;

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    active.mutate();
  };

  const switchMode = (next: Mode) => {
    setMode(next);
    login.reset();
    register.reset();
  };

  return (
    <form onSubmit={onSubmit} className="space-y-3" data-testid="local-auth-form">
      {foundingOpen && (
        <div className="grid grid-cols-2 gap-1 rounded-md bg-muted p-1">
          <button
            type="button"
            data-testid="local-mode-signin"
            aria-pressed={mode === "signin"}
            onClick={() => switchMode("signin")}
            className={`rounded px-2 py-1 text-sm ${mode === "signin" ? "bg-background shadow-sm" : "text-muted-foreground"}`}
          >
            {t("signIn.local.signInTab")}
          </button>
          <button
            type="button"
            data-testid="local-mode-register"
            aria-pressed={mode === "register"}
            onClick={() => switchMode("register")}
            className={`rounded px-2 py-1 text-sm ${mode === "register" ? "bg-background shadow-sm" : "text-muted-foreground"}`}
          >
            {t("signIn.local.registerTab")}
          </button>
        </div>
      )}

      {/* Stable card height (#842): both tabs reserve the same space so switching
          doesn't move anything. In demo mode the Sign in tab's hint and the Create
          account tab's name field share one grid cell (the taller wins). Without
          the hint, Sign in reserves the name field's space at the bottom of the
          form instead (see the end), so no gap opens above Email. The password
          hint and the Forgot link likewise keep their line in both tabs. Reserved
          copies are invisible and inert: not seen, focused or announced. */}
      {(methods?.demo_mode || mode === "register") && (
        <div className="grid [&>*]:col-start-1 [&>*]:row-start-1">
          {methods?.demo_mode && (
            <p
              data-testid="local-demo-hint"
              className={`text-xs text-muted-foreground ${mode === "signin" ? "" : "invisible"}`}
              aria-hidden={mode === "signin" ? undefined : true}
            >
              {t("signIn.local.demoHint", {
                email: methods.demo_email,
                password: methods.demo_password,
              })}
            </p>
          )}
          {foundingOpen && (
            <div
              className={`space-y-1 ${mode === "register" ? "" : "invisible"}`}
              aria-hidden={mode === "register" ? undefined : true}
              inert={mode !== "register"}
            >
              <Label htmlFor="local-display-name">{t("signIn.local.displayName")}</Label>
              <Input
                id="local-display-name"
                data-testid="local-display-name"
                autoComplete="name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
              />
            </div>
          )}
        </div>
      )}

      <div className="space-y-1">
        <Label htmlFor="local-email">{t("signIn.local.email")}</Label>
        <Input
          id="local-email"
          data-testid="local-email"
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </div>

      <div className="space-y-1">
        <Label htmlFor="local-password">{t("signIn.local.password")}</Label>
        <Input
          id="local-password"
          data-testid="local-password"
          type="password"
          autoComplete={mode === "signin" ? "current-password" : "new-password"}
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {foundingOpen && (
          <p
            className={`text-xs text-muted-foreground ${mode === "register" ? "" : "invisible"}`}
            aria-hidden={mode === "register" ? undefined : true}
          >
            {t("signIn.local.passwordHint")}
          </p>
        )}
      </div>

      {active.isError && (
        <p data-testid="local-error" className="text-sm text-destructive">
          {errorMessage(active.error)}
        </p>
      )}

      <Button
        type="submit"
        className="w-full"
        data-testid="local-submit"
        disabled={active.isPending}
      >
        {active.isPending
          ? t("working")
          : mode === "signin"
            ? t("signIn.local.signInTab")
            : t("signIn.local.registerTab")}
      </Button>

      {showReset && (mode === "signin" || foundingOpen) && (
        <a
          href={routes.forgotPassword}
          data-testid="local-forgot-password"
          className={`block text-center text-xs text-muted-foreground underline-offset-4 hover:underline ${mode === "signin" ? "" : "invisible"}`}
          aria-hidden={mode === "signin" ? undefined : true}
          inert={mode !== "signin"}
        >
          {t("signIn.local.forgotPassword")}
        </a>
      )}

      {foundingOpen && mode === "signin" && !methods?.demo_mode && (
        <div className="invisible space-y-1" aria-hidden inert>
          <Label>{t("signIn.local.displayName")}</Label>
          <Input tabIndex={-1} readOnly />
        </div>
      )}
    </form>
  );
}
