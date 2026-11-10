import { useState } from "react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { MoveSnapshotDialog } from "@/components/dialogs/MoveSnapshotDialog";
import { snapshotGroupOf } from "@/lib/snapshotMove";
import { FormError, ValidatedForm } from "@/components/common/ValidatedForm";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

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

// year_month is shown read-only, not editable (it is immutable, #76). Moving a
// snapshot to another month is its own atomic act, Move… (#791), which archives
// it and records the same values in the target month.
export function EditSnapshotDialog<TResult>({
  open,
  onOpenChange,
  snapshot,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation("common");
  const [moveOpen, setMoveOpen] = useState(false);
  const movable = snapshotGroupOf(snapshot) !== null;
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
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("snapshot.editTitle")}</DialogTitle>
            <DialogDescription>{t("snapshot.editDescription")}</DialogDescription>
          </DialogHeader>
          <ValidatedForm onSubmit={submit} error={mutation.error} className="space-y-3">
            <SnapshotFormBody
              mode="edit"
              onMove={movable ? () => setMoveOpen(true) : undefined}
              currency={snapshot.currency}
              form={form}
              setForm={setForm}
            />

            <FormError />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                {t("cancel")}
              </Button>
              <Button type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? t("actions.saving") : t("actions.saveChanges")}
              </Button>
            </DialogFooter>
          </ValidatedForm>
        </DialogContent>
      </Dialog>
      <MoveSnapshotDialog
        open={moveOpen}
        onOpenChange={setMoveOpen}
        snapshot={snapshot}
        onMoved={() => onOpenChange(false)}
      />
    </>
  );
}
