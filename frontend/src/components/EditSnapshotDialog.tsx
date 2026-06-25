import { useState } from "react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { errorMessage } from "@/lib/errorMessage";
import { monthStartDate, monthEndDateCapped } from "@/lib/dateLimits";
import { formatYearMonth } from "@/lib/format";

// Generic snapshot shape — only the fields the edit form needs.
type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

export type UpdateSnapshotPayload = {
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

export type UpdateSnapshotMutationVariables = {
  snapshotId: string;
  payload: UpdateSnapshotPayload;
};

type Props<TResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  snapshot: SnapshotLike;
  // Owned by the parent so this dialog works for any position group.
  mutation: UseMutationResult<
    TResult,
    unknown,
    UpdateSnapshotMutationVariables
  >;
};

// year_month is shown read-only, not editable: changing it would mean creating
// a different month's snapshot, which conflicts with the (position_id,
// year_month) unique constraint. To "move" a snapshot to a different month, the
// user deletes it (row menu) and records a new one — see snapshot.wrongMonthHint.
export function EditSnapshotDialog<TResult>({
  open,
  onOpenChange,
  snapshot,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation("common");
  const [form, setForm] = useState({
    amount: snapshot.amount,
    as_of_date: snapshot.as_of_date ? snapshot.as_of_date.slice(0, 10) : "",
    description: snapshot.description ?? "",
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        snapshotId: snapshot.id,
        payload: {
          amount: form.amount,
          currency: snapshot.currency,
          as_of_date: form.as_of_date || null,
          description: form.description || null,
        },
      },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("snapshot.editTitle")}</DialogTitle>
          <DialogDescription>{t("snapshot.editDescription")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="edit_year_month">{t("fields.month")}</Label>
            <Input
              id="edit_year_month"
              data-testid="snapshot-month-locked"
              disabled
              value={formatYearMonth(snapshot.year_month)}
            />
            <p className="text-xs text-muted-foreground">
              {t("snapshot.wrongMonthHint")}
            </p>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_amount">
              {t("fields.amountIn", { currency: snapshot.currency })}
            </Label>
            <Input
              id="edit_amount"
              required
              inputMode="decimal"
              value={form.amount}
              onChange={(e) => setForm({ ...form, amount: e.target.value })}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_as_of_date">{t("fields.statementDate")}</Label>
            <Input
              id="edit_as_of_date"
              type="date"
              min={monthStartDate(snapshot.year_month)}
              max={monthEndDateCapped(snapshot.year_month)}
              value={form.as_of_date}
              onChange={(e) => setForm({ ...form, as_of_date: e.target.value })}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_snap_description">
              {t("fields.description")}
            </Label>
            <Input
              id="edit_snap_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("actions.saving")
                : t("actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
