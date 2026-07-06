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
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

type ResetPreview = {
  email: string;
};

// ResetSetScreen is where an emailed reset link lands (ADR-0039/#282): the
// holder sets a new password, the credential is replaced, their other sessions
// are revoked, and a fresh session is minted — so re-running the session query
// flips App.tsx into the authed app. The token in the URL is the credential. A
// read-only preview validates the link without consuming it, so a reload is safe;
// a 409 means the link is used/expired/unknown and the form is replaced by a
// notice.
export function ResetSetScreen() {
  const { t } = useTranslation("common");
  const queryClient = useQueryClient();
  const [password, setPassword] = useState("");

  const token = new URLSearchParams(window.location.search).get("token") ?? "";

  const preview = useQuery<ResetPreview>({
    queryKey: ["reset-preview", token],
    queryFn: () => api<ResetPreview>(`/api/auth/local/reset?token=${encodeURIComponent(token)}`),
    enabled: token !== "",
    retry: false,
  });

  const reset = useMutation({
    mutationFn: () =>
      api("/api/auth/local/reset", {
        method: "POST",
        body: JSON.stringify({ token, password }),
      }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["session"] }),
  });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    reset.mutate();
  };

  const invalidLink =
    token === "" || (preview.error instanceof ApiError && preview.error.status === 409);

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm" data-testid="reset-set-card">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardTitle className="pt-2">{t("resetSet.title")}</CardTitle>
          {preview.data && <CardDescription>{t("resetSet.subtitle")}</CardDescription>}
        </CardHeader>

        <CardContent className="space-y-4">
          {invalidLink ? (
            <div className="space-y-3" data-testid="reset-invalid">
              <p className="text-sm text-muted-foreground">{t("resetSet.invalid")}</p>
              <Button asChild variant="outline" className="w-full">
                <a href="/forgot-password">{t("resetSet.requestNew")}</a>
              </Button>
            </div>
          ) : preview.isPending ? (
            <p className="text-sm text-muted-foreground">{t("working")}</p>
          ) : (
            <form onSubmit={onSubmit} className="space-y-3" data-testid="reset-set-form">
              <div className="space-y-1">
                <Label htmlFor="reset-set-email">{t("resetSet.emailLabel")}</Label>
                <Input
                  id="reset-set-email"
                  data-testid="reset-set-email"
                  type="email"
                  value={preview.data?.email ?? ""}
                  readOnly
                  disabled
                />
              </div>

              <div className="space-y-1">
                <Label htmlFor="reset-set-password">{t("resetSet.passwordLabel")}</Label>
                <Input
                  id="reset-set-password"
                  data-testid="reset-set-password"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">{t("signIn.local.passwordHint")}</p>
              </div>

              {reset.isError && (
                <p data-testid="reset-set-error" className="text-sm text-destructive">
                  {errorMessage(reset.error)}
                </p>
              )}

              <Button
                type="submit"
                className="w-full"
                data-testid="reset-set-submit"
                disabled={reset.isPending}
              >
                {reset.isPending ? t("working") : t("resetSet.submit")}
              </Button>
            </form>
          )}
        </CardContent>

        <CardFooter className="border-t pt-4">
          <AppInfo variant="split" />
        </CardFooter>
      </Card>
    </div>
  );
}
