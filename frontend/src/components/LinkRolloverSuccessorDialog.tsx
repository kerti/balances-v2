import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  useTimeDeposits,
  useLinkRolloverSuccessor,
} from "@/hooks/useInvestments";
import { errorMessage } from "@/lib/errorMessage";

type Props = {
  // The matured source deposit whose funds rolled over. The picked successor
  // gets stamped rolled_from = sourceId, clearing this position's callout.
  sourceId: string;
};

export function LinkRolloverSuccessorDialog({ sourceId }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [open, setOpen] = useState(false);
  const [selectedId, setSelectedId] = useState("");
  const { data: deposits } = useTimeDeposits();
  const mutation = useLinkRolloverSuccessor(sourceId);

  // Eligible successors: any other time deposit that isn't already the
  // successor of some deposit. The backend re-checks (self-link, cycles,
  // double-link); this just keeps the obvious non-options out of the picker.
  const candidates = useMemo(
    () =>
      (deposits ?? [])
        .filter(
          (d) =>
            d.investment.id !== sourceId &&
            !d.investment.rolled_from_investment_id,
        )
        .sort((a, b) =>
          a.investment.display_name.localeCompare(b.investment.display_name),
        ),
    [deposits, sourceId],
  );

  function close() {
    setOpen(false);
    setSelectedId("");
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedId) return;
    mutation.mutate(selectedId, { onSuccess: close });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" data-testid="rollover-link-trigger">
          <Link2 className="mr-1 size-4" />
          {t("investments:timeDeposit.rollover.calloutLinkAction")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t("investments:timeDeposit.rollover.linkTitle")}
          </DialogTitle>
          <DialogDescription>
            {t("investments:timeDeposit.rollover.linkDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="grid gap-2">
            <Label htmlFor="rollover_successor_select">
              {t("investments:timeDeposit.rollover.linkLabel")}
            </Label>
            {candidates.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                {t("investments:timeDeposit.rollover.linkEmpty")}
              </p>
            ) : (
              <select
                id="rollover_successor_select"
                data-testid="rollover-successor-select"
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={selectedId}
                onChange={(e) => setSelectedId(e.target.value)}
              >
                <option value="" disabled>
                  {t("investments:timeDeposit.rollover.linkPlaceholder")}
                </option>
                {candidates.map((d) => (
                  <option key={d.investment.id} value={d.investment.id}>
                    {d.investment.display_name}
                  </option>
                ))}
              </select>
            )}
          </div>

          {mutation.error && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button
              type="submit"
              disabled={!selectedId || mutation.isPending}
              data-testid="rollover-link-submit"
            >
              {mutation.isPending
                ? t("common:actions.saving")
                : t("investments:timeDeposit.rollover.linkSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
