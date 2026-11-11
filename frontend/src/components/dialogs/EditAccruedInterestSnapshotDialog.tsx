import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { UseMutationResult } from "@tanstack/react-query";
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
import type { UpdateInvestmentSnapshotPayload } from "@/hooks/useInvestmentSnapshots";

type AccruedInterestSnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  accrued_interest: string | null;
  as_of_date: string | null;
  description: string | null;
};
import { AccruedInterestSnapshotFormBody } from "@/components/dialogs/forms/AccruedInterestSnapshotForm";

export type UpdateAccruedInterestSnapshotMutationVariables = {
  snapshotId: string;
  payload: UpdateInvestmentSnapshotPayload;
};

type Props<TResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  snapshot: AccruedInterestSnapshotLike;
  mutation: UseMutationResult<TResult, unknown, UpdateAccruedInterestSnapshotMutationVariables>;
  // Time deposits pass their principal in force (#844); see
  // CreateAccruedInterestSnapshotDialog.
  principal?: string;
};

function derivePrincipal(amount: string, accrued: string): string | null {
  const a = Number(amount);
  const i = Number(accrued);
  if (!amount || !accrued || Number.isNaN(a) || Number.isNaN(i)) {
    return null;
  }
  return (a - i).toString();
}

export function EditAccruedInterestSnapshotDialog<TResult>({
  open,
  onOpenChange,
  snapshot,
  mutation,
  principal,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [moveOpen, setMoveOpen] = useState(false);
  const movable = snapshotGroupOf(snapshot) !== null;
  const [form, setForm] = useState({
    year_month: snapshot.year_month,
    amount: snapshot.amount,
    accrued_interest: snapshot.accrued_interest ?? "",
    as_of_date: snapshot.as_of_date ? snapshot.as_of_date.slice(0, 10) : "",
    description: snapshot.description ?? "",
  });

  const derivedPrincipal = derivePrincipal(form.amount, form.accrued_interest);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        snapshotId: snapshot.id,
        payload: {
          amount: form.amount,
          currency: snapshot.currency,
          quantity: null,
          price_per_unit: null,
          accrued_interest: principal !== undefined ? null : form.accrued_interest,
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
            <DialogTitle>{t("investments:accruedInterestSnapshot.editTitle")}</DialogTitle>
            <DialogDescription>
              {t(
                principal !== undefined
                  ? "investments:accruedInterestSnapshot.editDescriptionTimeDeposit"
                  : "investments:accruedInterestSnapshot.editDescription",
              )}
            </DialogDescription>
          </DialogHeader>
          <ValidatedForm onSubmit={submit} error={mutation.error} className="space-y-3">
            <AccruedInterestSnapshotFormBody
              mode="edit"
              onMove={movable ? () => setMoveOpen(true) : undefined}
              currency={snapshot.currency}
              form={form}
              setForm={setForm}
              derivedPrincipal={derivedPrincipal}
              principal={principal}
            />

            <FormError />

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                {t("common:cancel")}
              </Button>
              <Button type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? t("common:actions.saving") : t("common:actions.saveChanges")}
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
