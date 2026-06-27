import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "@/api/client";
import { errorMessage } from "@/lib/errorMessage";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AppLogo } from "@/components/AppLogo";
import { AppInfo } from "@/components/AppInfo";
import { useAuthMethods } from "@/hooks/useAuthMethods";
import { useLocale } from "@/i18n/useLocale";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

type InvitePreview = {
  invited_email: string;
  household_name: string;
};

// InviteAcceptScreen is where a local invite link lands (ADR-0039/#281): an
// invitee with no Google account sets a password and the account is created
// bound to the invited email — possession of the single-use link IS the email
// proof, so there is no second onboarding gate. Rendered by App.tsx for a
// visitor holding only the URL token (no session, no handshake). When the
// instance also offers Google, a "continue with Google" option is shown beside
// the form so a Google-capable invitee can take that path instead.
export function InviteAcceptScreen() {
  const { t } = useTranslation("common");
  const { locale } = useLocale();
  const queryClient = useQueryClient();
  const [password, setPassword] = useState("");

  // The token is the credential; it rides the URL the invite email linked to.
  const token = new URLSearchParams(window.location.search).get("token") ?? "";

  const { data: methods } = useAuthMethods();
  const showGoogle = methods ? methods.google : false;

  // Read-only resolve — never consumes the link, so a reload is safe. A 409
  // means the link is used/expired/unknown; the form is replaced by a notice.
  const preview = useQuery<InvitePreview>({
    queryKey: ["invite-preview", token],
    queryFn: () =>
      api<InvitePreview>(
        `/api/auth/local/invite?token=${encodeURIComponent(token)}`,
      ),
    enabled: token !== "",
    retry: false,
  });

  const accept = useMutation({
    mutationFn: () =>
      api("/api/auth/local/invite/accept", {
        method: "POST",
        body: JSON.stringify({ token, password, locale }),
      }),
    // The session cookie now exists; re-running the session query flips App.tsx
    // over to the authed router, landing the new member straight in the app.
    onSuccess: () =>
      void queryClient.invalidateQueries({ queryKey: ["session"] }),
  });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    accept.mutate();
  };

  const invalidLink =
    token === "" ||
    (preview.error instanceof ApiError && preview.error.status === 409);

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm" data-testid="invite-accept-card">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardTitle className="pt-2">{t("invite.title")}</CardTitle>
          {preview.data && (
            <CardDescription>
              {t("invite.subtitle", {
                household: preview.data.household_name,
              })}
            </CardDescription>
          )}
        </CardHeader>

        <CardContent className="space-y-4">
          {invalidLink ? (
            <div className="space-y-3" data-testid="invite-invalid">
              <p className="text-sm text-muted-foreground">
                {t("invite.invalid")}
              </p>
              <Button asChild variant="outline" className="w-full">
                <a href="/">{t("invite.goToSignIn")}</a>
              </Button>
            </div>
          ) : preview.isPending ? (
            <p className="text-sm text-muted-foreground">{t("working")}</p>
          ) : (
            <>
              <form
                onSubmit={onSubmit}
                className="space-y-3"
                data-testid="invite-accept-form"
              >
                <div className="space-y-1">
                  <Label htmlFor="invite-email">{t("invite.emailLabel")}</Label>
                  <Input
                    id="invite-email"
                    data-testid="invite-email"
                    type="email"
                    value={preview.data?.invited_email ?? ""}
                    readOnly
                    disabled
                  />
                </div>

                <div className="space-y-1">
                  <Label htmlFor="invite-password">
                    {t("invite.passwordLabel")}
                  </Label>
                  <Input
                    id="invite-password"
                    data-testid="invite-password"
                    type="password"
                    autoComplete="new-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    {t("signIn.local.passwordHint")}
                  </p>
                </div>

                {accept.isError && (
                  <p
                    data-testid="invite-error"
                    className="text-sm text-destructive"
                  >
                    {errorMessage(accept.error)}
                  </p>
                )}

                <Button
                  type="submit"
                  className="w-full"
                  data-testid="invite-submit"
                  disabled={accept.isPending}
                >
                  {accept.isPending ? t("working") : t("invite.submit")}
                </Button>
              </form>

              {showGoogle && (
                <>
                  <div
                    className="flex items-center gap-2"
                    data-testid="invite-divider"
                  >
                    <div className="h-px flex-1 bg-border" />
                    <span className="text-xs uppercase text-muted-foreground">
                      {t("signIn.or")}
                    </span>
                    <div className="h-px flex-1 bg-border" />
                  </div>
                  <Button asChild variant="outline" className="w-full">
                    <a
                      href={`/api/auth/google/start?invite=${encodeURIComponent(token)}&lng=${encodeURIComponent(locale)}`}
                      data-testid="invite-google"
                    >
                      {t("signIn.withGoogle")}
                    </a>
                  </Button>
                </>
              )}
            </>
          )}
        </CardContent>

        <CardFooter className="border-t pt-4">
          <AppInfo variant="split" />
        </CardFooter>
      </Card>
    </div>
  );
}
