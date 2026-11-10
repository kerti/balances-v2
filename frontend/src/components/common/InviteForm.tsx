import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { FormError, ValidatedForm } from "@/components/common/ValidatedForm";
import { SettingsControlRow } from "@/components/common/SettingsSurface";
import { api } from "@/api/client";
import { useCopyLink } from "@/hooks/useCopyLink";
import { MEMBERSHIP_KEY } from "@/hooks/useMembership";
import { formatDateTime } from "@/lib/format";

type InviteResp = {
  id: string;
  invited_email: string;
  expires_at: string;
  accept_url: string;
  email_sent: boolean;
};

export function InviteForm() {
  const { t } = useTranslation(["settings", "common"]);
  const qc = useQueryClient();
  const [email, setEmail] = useState("");
  const [result, setResult] = useState<InviteResp | null>(null);
  const { copied, copy, reset } = useCopyLink();

  const mutation = useMutation({
    mutationFn: (emailToInvite: string) =>
      api<InviteResp>("/api/invitations", {
        method: "POST",
        body: JSON.stringify({ email: emailToInvite }),
      }),
    onSuccess: (data) => {
      setResult(data);
      reset();
      setEmail("");
      void qc.invalidateQueries({ queryKey: MEMBERSHIP_KEY });
      // The invite + link are valid regardless; only nudge when the best-effort
      // email actually failed to send (email enabled but the send errored — e.g.
      // a misconfigured sender). With email disabled the backend reports
      // email_sent=true and the Copy invite link button below is the affordance.
      // See issue #212 / INV-NOTIFICATIONS-11.
      if (!data.email_sent) {
        toast.warning(t("invite.emailFailed"));
      }
    },
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("invite.title")}</CardTitle>
        <CardDescription>{t("invite.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {/* Send sits beside the field, not under it: the same
            label-above-control-with-a-flush-right-action shape the settings
            rows wear (#563), so Membership reads as part of the same screen.
            It is only ever disabled while sending (#824, ADR-0058 §5): an empty
            or malformed address is answered with a message under the field. */}
        <ValidatedForm onSubmit={() => mutation.mutate(email.trim())} error={mutation.error}>
          <SettingsControlRow
            action={
              <Button type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? t("common:sending") : t("invite.submit")}
              </Button>
            }
          >
            <Label htmlFor="email">{t("invite.emailLabel")}</Label>
            <Input
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t("invite.emailPlaceholder")}
            />
          </SettingsControlRow>
          <div className="mt-3">
            <FormError />
          </div>
        </ValidatedForm>

        {result && (
          <div
            className="mt-4 p-3 rounded-md bg-muted text-sm space-y-2"
            data-testid="invite-result"
          >
            <p className="font-medium">{t("invite.sentTo", { email: result.invited_email })}</p>
            <p className="text-muted-foreground">
              {t("invite.expires", { when: formatDateTime(result.expires_at) })}
            </p>
            {/* The link is a bearer secret, so it rides the clipboard and is never
                printed (#847). It exists only in this response: to get one again,
                revoke the invite and send a new one. */}
            <Button
              type="button"
              variant="outline"
              size="sm"
              data-testid="copy-invite-link"
              onClick={() => void copy(result.accept_url)}
            >
              {copied ? t("invite.copied") : t("invite.copyLink")}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
