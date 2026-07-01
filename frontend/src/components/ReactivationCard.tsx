import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api } from "@/api/client";
import { useSession } from "@/hooks/useSession";
import { useAuthMethods } from "@/hooks/useAuthMethods";
import { errorMessage } from "@/lib/errorMessage";
import { formatDateTime } from "@/lib/format";

type DormantMember = {
  id: string;
  display_name: string;
  email: string;
};

type ReactivateResp = {
  email: string;
  set_password_url: string;
  expires_at: string;
};

// ReactivationCard is the founder's no-mail recovery affordance (ADR-0039/#283):
// it lists the household's dormant members (restored-from-backup local members
// with no credential yet) and mints a one-time set-password link for one, shown
// once to relay out-of-band — the same copy-link shape as InviteForm. Rendered
// only for the founder on a local-auth instance; the backend independently
// enforces both scopes on the routes.
export function ReactivationCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: me } = useSession();
  const { data: methods } = useAuthMethods();

  const dormant = useQuery({
    queryKey: ["dormant-members"],
    queryFn: () => api<DormantMember[]>("/api/auth/local/reactivation/members"),
    // Only the founder on a local instance ever reaches this query.
    enabled: Boolean(me?.is_founder && methods?.local),
    staleTime: 60_000,
  });

  const [result, setResult] = useState<ReactivateResp | null>(null);
  const [copied, setCopied] = useState(false);

  const mutation = useMutation({
    mutationFn: (userId: string) =>
      api<ReactivateResp>("/api/auth/local/reactivation", {
        method: "POST",
        body: JSON.stringify({ user_id: userId }),
      }),
    onSuccess: (data) => {
      setResult(data);
      setCopied(false);
      // The member is no longer dormant once they set a password, but the link is
      // still pending until then — keep the list fresh so a completed reactivation
      // drops off on the next load.
      void dormant.refetch();
    },
  });

  // Founder-only, local-auth-only, and only worth showing when there is actually
  // someone to reactivate — for a normally-running household the list is empty and
  // this card stays hidden rather than adding clutter.
  if (!me?.is_founder || !methods?.local) return null;
  if (!dormant.data || dormant.data.length === 0) return null;

  async function copyLink(url: string) {
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
        <CardTitle>{t("reactivation.title")}</CardTitle>
        <CardDescription>{t("reactivation.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <ul className="flex flex-col gap-2" data-testid="dormant-members">
          {dormant.data.map((m) => (
            <li
              key={m.id}
              className="flex items-center justify-between gap-3"
              data-testid={`dormant-member-${m.email}`}
            >
              <span className="text-sm">
                <span className="font-medium">{m.display_name}</span>{" "}
                <span className="text-muted-foreground break-all">
                  {m.email}
                </span>
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                data-testid={`reactivate-${m.email}`}
                disabled={mutation.isPending}
                onClick={() => mutation.mutate(m.id)}
              >
                {t("reactivation.reactivate")}
              </Button>
            </li>
          ))}
        </ul>

        {mutation.error && (
          <p className="mt-3 text-sm text-destructive">
            {errorMessage(mutation.error)}
          </p>
        )}

        {result && (
          <div className="mt-4 p-3 rounded-md bg-muted text-sm space-y-2">
            <p className="font-medium">
              {t("reactivation.linkFor", { email: result.email })}
            </p>
            <p className="text-muted-foreground">
              {t("reactivation.expires", {
                when: formatDateTime(result.expires_at),
              })}
            </p>
            <p className="text-muted-foreground break-all">
              {t("reactivation.linkLabel")}{" "}
              <code className="text-xs" data-testid="reactivation-link">
                {result.set_password_url}
              </code>
            </p>
            <p className="text-muted-foreground">{t("reactivation.hint")}</p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              data-testid="copy-reactivation-link"
              onClick={() => copyLink(result.set_password_url)}
            >
              {copied ? t("reactivation.copied") : t("reactivation.copyLink")}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
