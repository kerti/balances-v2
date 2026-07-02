import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/api/client";
import { useSession } from "@/hooks/useSession";
import { errorMessage } from "@/lib/errorMessage";
import { downloadBackup } from "@/lib/backup";
import { routes } from "@/lib/routes";

type EraseHouseholdResp = { erased: boolean };

// EraseCard is the founder's whole-household hard delete (ADR-0040/#300) — the
// GDPR right-to-erasure affordance. Founder-only (server-enforced; this card
// also hides itself for a peer member) and gated by typing the household's
// exact name back, checked server-side. There is nothing to preview (unlike
// RestoreCard's uploaded-file summary), so this is a single request, not a
// preview/commit pair — the household's real name is already on screen before
// the user types it.
//
// The commit clears the session cookie server-side rather than re-issuing one
// (there's no household left to sign back into), so on success this does a
// hard navigation to the dedicated post-erasure screen rather than the usual
// React Query cache dance.
export function EraseCard() {
  const { t } = useTranslation("settings");
  const { data: me } = useSession();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const [deleting, setDeleting] = useState(false);

  if (!me?.is_founder) return null;

  const householdName = me.household_display_name;
  const confirmed = input === householdName;

  const reset = () => {
    setOpen(false);
    setInput("");
  };

  const handleDelete = async () => {
    if (!confirmed) return;
    setDeleting(true);
    try {
      await api<EraseHouseholdResp>("/api/backup/erase", {
        method: "POST",
        body: JSON.stringify({ household_name: input }),
      });
      // The server already cleared the session cookie. A hard navigation (not
      // a SPA route change) is the cleanest way to land on the dedicated
      // post-erasure screen with a fully reset client state.
      window.location.assign(routes.erased);
    } catch (err) {
      setDeleting(false);
      toast.error(errorMessage(err, t("data.erase.failed")));
    }
  };

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="text-base text-destructive">
          {t("data.erase.title")}
        </CardTitle>
        <CardDescription>{t("data.erase.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {!open ? (
          <Button
            variant="destructive"
            onClick={() => setOpen(true)}
            data-testid="erase-open-button"
          >
            {t("data.erase.open")}
          </Button>
        ) : (
          <div className="space-y-4" data-testid="erase-confirm">
            <div
              className="rounded-md border border-destructive/50 bg-destructive/5 p-3 text-sm"
              data-testid="erase-export-nudge"
            >
              <p className="mb-2">{t("data.erase.exportFirst")}</p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={deleting}
                onClick={() => void downloadBackup("full")}
                data-testid="erase-export-now"
              >
                {t("data.erase.exportNow")}
              </Button>
            </div>

            <div className="space-y-1">
              <label htmlFor="erase-confirm-input" className="text-sm">
                {t("data.erase.confirmPrompt", { household: householdName })}
              </label>
              <Input
                id="erase-confirm-input"
                value={input}
                disabled={deleting}
                autoComplete="off"
                placeholder={t("data.erase.confirmPlaceholder")}
                onChange={(e) => setInput(e.target.value)}
                data-testid="erase-confirm-input"
              />
            </div>

            <div className="flex items-center gap-3">
              <Button
                variant="destructive"
                onClick={handleDelete}
                disabled={!confirmed || deleting}
                data-testid="erase-commit-button"
              >
                {deleting ? t("data.erase.deleting") : t("data.erase.commit")}
              </Button>
              <Button variant="ghost" onClick={reset} disabled={deleting}>
                {t("data.erase.cancel")}
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
