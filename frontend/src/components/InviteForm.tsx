import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api } from "@/api/client";
import { errorMessage } from "@/lib/errorMessage";
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
  const [email, setEmail] = useState("");
  const [result, setResult] = useState<InviteResp | null>(null);
  const [copied, setCopied] = useState(false);

  const mutation = useMutation({
    mutationFn: (emailToInvite: string) =>
      api<InviteResp>("/api/invitations", {
        method: "POST",
        body: JSON.stringify({ email: emailToInvite }),
      }),
    onSuccess: (data) => {
      setResult(data);
      setCopied(false);
      setEmail("");
      // The invite + link are valid regardless; only nudge when the best-effort
      // email actually failed to send (email enabled but the send errored — e.g.
      // a misconfigured sender). With email disabled the backend reports
      // email_sent=true and the always-visible link panel below is the
      // affordance. See issue #212 / INV-NOTIFICATIONS-11.
      if (!data.email_sent) {
        toast.warning(t("invite.emailFailed"));
      }
    },
  });

  // The accept link is the only mail with a hard dependency, so it's always
  // surfaced for manual sharing — the fallback when EMAIL_ENABLED=false and the
  // invite email never goes out (ADR-0037). Best-effort: a denied clipboard
  // permission leaves the URL visible to copy by hand.
  async function copyAcceptUrl(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("invite.title")}</CardTitle>
        <CardDescription>{t("invite.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            mutation.mutate(email);
          }}
          className="flex flex-col gap-3"
        >
          <div className="grid gap-2">
            <Label htmlFor="email">{t("invite.emailLabel")}</Label>
            <Input
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t("invite.emailPlaceholder")}
            />
          </div>
          <Button type="submit" disabled={mutation.isPending || !email}>
            {mutation.isPending ? t("common:sending") : t("invite.submit")}
          </Button>
        </form>

        {mutation.error && (
          <p className="mt-3 text-sm text-destructive">
            {errorMessage(mutation.error)}
          </p>
        )}

        {result && (
          <div className="mt-4 p-3 rounded-md bg-muted text-sm space-y-2">
            <p className="font-medium">
              {t("invite.sentTo", { email: result.invited_email })}
            </p>
            <p className="text-muted-foreground">
              {t("invite.expires", { when: formatDateTime(result.expires_at) })}
            </p>
            <p className="text-muted-foreground break-all">
              {t("invite.acceptUrl")}{" "}
              <code className="text-xs" data-testid="invite-accept-url">
                {result.accept_url}
              </code>
            </p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              data-testid="copy-invite-link"
              onClick={() => copyAcceptUrl(result.accept_url)}
            >
              {copied ? t("invite.copied") : t("invite.copyLink")}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
