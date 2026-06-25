import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AppLogo } from "@/components/AppLogo";
import { AppInfo } from "@/components/AppInfo";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

// onboardingInvite mirrors the backend's joinable-Household row: one per
// distinct Household the verified email has a pending invitation to (ADR-0038).
type OnboardingInvite = {
  invitation_id: string;
  household_id: string;
  household_name: string;
  inviter_name: string;
  hint: boolean;
};

type OnboardingOptions = {
  email: string;
  display_name: string;
  suggested_household_name: string;
  invitations: OnboardingInvite[];
};

// OnboardingScreen is the post-auth gate (ADR-0038), rendered by App.tsx for a
// visitor holding an onboarding handshake cookie but no session — the account
// does not exist until a choice commits here. It resolves the invite-vs-found
// decision by the *verified email*, not the clicked link: it lists pending
// invitations (one row per Household, the clicked link pre-highlighted) plus a
// "start your own" path. Founding while invitations exist asks for explicit
// confirmation, since one-household-per-person is irreversible (ADR-0017). A
// missing/expired handshake answers 401 from /options, surfaced as a "sign in
// again" prompt.
export function OnboardingScreen() {
  const { t } = useTranslation("onboarding");
  const queryClient = useQueryClient();
  // `null` means "untouched" — the field shows the server's suggestion until
  // the user types. Derived rather than seeded via an effect to avoid a
  // cascading setState-in-effect; an empty value falls back server-side.
  const [override, setOverride] = useState<string | null>(null);
  const [confirmFound, setConfirmFound] = useState(false);
  const [showFounder, setShowFounder] = useState(false);
  const [staleNotice, setStaleNotice] = useState(false);

  const options = useQuery<OnboardingOptions>({
    queryKey: ["onboarding-options"],
    queryFn: () => api<OnboardingOptions>("/api/onboarding/options"),
    retry: false,
  });

  const invitations = options.data?.invitations ?? [];
  const hasInvites = invitations.length > 0;
  const householdName =
    override ?? options.data?.suggested_household_name ?? "";

  // On success the commit set the real session cookie; re-running the session
  // query flips App.tsx over to the authed router, landing on the dashboard.
  const onCommitted = () =>
    void queryClient.invalidateQueries({ queryKey: ["session"] });

  const found = useMutation({
    mutationFn: () =>
      api("/api/onboarding/choice", {
        method: "POST",
        body: JSON.stringify({ found: true, display_name: householdName }),
      }),
    onSuccess: onCommitted,
  });

  const join = useMutation({
    mutationFn: (invitationId: string) =>
      api("/api/onboarding/choice", {
        method: "POST",
        body: JSON.stringify({ join: true, invitation_id: invitationId }),
      }),
    onSuccess: onCommitted,
    onError: (err) => {
      // 409 = the invitation went stale between the gate's read and this
      // commit (used/expired). Refresh the options and tell the user, rather
      // than surfacing it as a hard failure.
      if (err instanceof ApiError && err.status === 409) {
        setStaleNotice(true);
        void queryClient.invalidateQueries({
          queryKey: ["onboarding-options"],
        });
      }
    },
  });

  const expired =
    options.error instanceof ApiError && options.error.status === 401;

  // Founder form is shown directly when there are no invitations; when there
  // are, it appears only after the explicit "start your own instead" confirm.
  const founderView = !hasInvites || showFounder;

  const confirmDescription =
    invitations.length === 1
      ? t("confirmFound.descriptionOne", {
          household: invitations[0].household_name,
        })
      : t("confirmFound.descriptionMany");

  return (
    <div className="min-h-screen flex items-center justify-center bg-muted p-6">
      <Card className="w-full max-w-sm" data-testid="onboarding-card">
        <CardHeader>
          <AppLogo className="w-full h-auto" />
          <CardTitle className="pt-2">
            {hasInvites && !showFounder ? t("invited.title") : t("title")}
          </CardTitle>
          <CardDescription>
            {hasInvites && !showFounder ? t("invited.subtitle") : t("subtitle")}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {expired ? (
            <div className="space-y-3" data-testid="onboarding-expired">
              <p className="text-sm text-muted-foreground">{t("expired")}</p>
              <Button asChild variant="outline" className="w-full">
                <a href="/">{t("signInAgain")}</a>
              </Button>
            </div>
          ) : (
            <>
              {hasInvites && !showFounder && (
                <div className="space-y-3" data-testid="onboarding-invites">
                  {staleNotice && (
                    <p
                      className="text-sm text-muted-foreground"
                      role="status"
                      data-testid="onboarding-stale-notice"
                    >
                      {t("invited.staleInvite")}
                    </p>
                  )}
                  {invitations.map((inv) => (
                    <button
                      key={inv.invitation_id}
                      type="button"
                      data-testid="onboarding-join-row"
                      data-hint={inv.hint ? "true" : undefined}
                      disabled={join.isPending}
                      onClick={() => {
                        setStaleNotice(false);
                        join.mutate(inv.invitation_id);
                      }}
                      className={`w-full rounded-md border px-3 py-2 text-left transition-colors hover:bg-accent disabled:opacity-50 ${
                        inv.hint
                          ? "border-primary ring-1 ring-primary"
                          : "border-input"
                      }`}
                    >
                      <span className="block text-sm font-medium">
                        {join.isPending && join.variables === inv.invitation_id
                          ? t("invited.joining")
                          : t("invited.join", {
                              household: inv.household_name,
                            })}
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        {t("invited.invitedBy", { inviter: inv.inviter_name })}
                      </span>
                    </button>
                  ))}
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full"
                    data-testid="onboarding-start-own"
                    disabled={join.isPending}
                    onClick={() => setConfirmFound(true)}
                  >
                    {t("invited.startOwnInstead")}
                  </Button>
                </div>
              )}

              {founderView && (
                <form
                  className="space-y-4"
                  onSubmit={(e) => {
                    e.preventDefault();
                    found.mutate();
                  }}
                >
                  <div className="space-y-1">
                    <p className="text-sm font-medium">{t("founder.title")}</p>
                    <p className="text-sm text-muted-foreground">
                      {t("founder.description")}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <Label htmlFor="onboarding-household-name">
                      {t("founder.nameLabel")}
                    </Label>
                    <Input
                      id="onboarding-household-name"
                      data-testid="onboarding-household-name"
                      value={householdName}
                      placeholder={t("founder.namePlaceholder")}
                      onChange={(e) => setOverride(e.target.value)}
                      disabled={options.isPending || found.isPending}
                    />
                  </div>
                  {found.isError && (
                    <p className="text-sm text-destructive" role="alert">
                      {t("error")}
                    </p>
                  )}
                  <Button
                    type="submit"
                    className="w-full"
                    data-testid="onboarding-found-submit"
                    disabled={options.isPending || found.isPending}
                  >
                    {found.isPending
                      ? t("founder.submitting")
                      : t("founder.submit")}
                  </Button>
                </form>
              )}
            </>
          )}
        </CardContent>

        <CardFooter className="border-t pt-4">
          <AppInfo variant="split" />
        </CardFooter>
      </Card>

      <ConfirmDialog
        open={confirmFound}
        onOpenChange={setConfirmFound}
        title={t("confirmFound.title")}
        description={confirmDescription}
        confirmLabel={t("confirmFound.confirm")}
        destructive
        onConfirm={() => {
          setConfirmFound(false);
          setShowFounder(true);
        }}
      />
    </div>
  );
}
