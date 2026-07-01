import { useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/api/client";
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

// ResetRequestScreen is the "forgot password" form (ADR-0039/#282), reached from
// the sign-in screen. Submitting always lands on the same generic confirmation —
// the backend returns an identical 204 whether or not the email maps to a local
// account, so the UI must not reveal it either (no enumeration). The only error
// worth surfacing is the soft rate-limit (429); everything else resolves to the
// "check your email" notice.
export function ResetRequestScreen() {
  const { t } = useTranslation("common");
  const [email, setEmail] = useState("");

  const request = useMutation({
    mutationFn: () =>
      api("/api/auth/local/reset/request", {
        method: "POST",
        body: JSON.stringify({ email }),
      }),
  });

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    request.mutate();
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm" data-testid="reset-request-card">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardTitle className="pt-2">{t("resetRequest.title")}</CardTitle>
          <CardDescription>{t("resetRequest.subtitle")}</CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {request.isSuccess ? (
            <div className="space-y-3" data-testid="reset-request-sent">
              <p className="text-sm text-muted-foreground">
                {t("resetRequest.sent")}
              </p>
              <Button asChild variant="outline" className="w-full">
                <a href="/" data-testid="reset-request-back">
                  {t("resetRequest.backToSignIn")}
                </a>
              </Button>
            </div>
          ) : (
            <form
              onSubmit={onSubmit}
              className="space-y-3"
              data-testid="reset-request-form"
            >
              <div className="space-y-1">
                <Label htmlFor="reset-request-email">
                  {t("resetRequest.emailLabel")}
                </Label>
                <Input
                  id="reset-request-email"
                  data-testid="reset-request-email"
                  type="email"
                  autoComplete="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>

              {request.isError && (
                <p
                  data-testid="reset-request-error"
                  className="text-sm text-destructive"
                >
                  {errorMessage(request.error)}
                </p>
              )}

              <Button
                type="submit"
                className="w-full"
                data-testid="reset-request-submit"
                disabled={request.isPending}
              >
                {request.isPending ? t("working") : t("resetRequest.submit")}
              </Button>

              <Button asChild variant="link" className="w-full">
                <a href="/">{t("resetRequest.backToSignIn")}</a>
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
