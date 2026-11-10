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
import { errorMessage } from "@/lib/errorMessage";

// Generic snapshot shape — only the fields the edit form needs.
type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};
import { SnapshotFormBody } from "@/components/dialogs/forms/SnapshotForm";

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
  mutation: UseMutationResult<TResult, unknown, UpdateSnapshotMutationVariables>;
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
    year_month: snapshot.year_month,
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
          <SnapshotFormBody
            mode="edit"
            currency={snapshot.currency}
            form={form}
            setForm={setForm}
          />

          {mutation.isError && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("actions.saving") : t("actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
