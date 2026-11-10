import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { UseMutationResult } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
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

type InvestmentSnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  quantity: string | null;
  price_per_unit: string | null;
  as_of_date: string | null;
  description: string | null;
};
import { QuantityPriceSnapshotFormBody } from "@/components/dialogs/forms/QuantityPriceSnapshotForm";

export type UpdateQuantityPriceSnapshotMutationVariables = {
  snapshotId: string;
  payload: UpdateInvestmentSnapshotPayload;
};

type Props<TResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  snapshot: InvestmentSnapshotLike;
  // Mutation owned by the parent so the dialog works for any
  // M4.3a investment subtype (stock / mutual_fund / gold).
  mutation: UseMutationResult<TResult, unknown, UpdateQuantityPriceSnapshotMutationVariables>;
};

function deriveAmount(quantity: string, pricePerUnit: string): string | null {
  const q = Number(quantity);
  const p = Number(pricePerUnit);
  if (!quantity || !pricePerUnit || Number.isNaN(q) || Number.isNaN(p)) {
    return null;
  }
  return (q * p).toString();
}

// year_month is not editable here (same constraint as the amount-shape edit
// dialog — would break the (investment_id, year_month) unique index).
export function EditQuantityPriceSnapshotDialog<TResult>({
  open,
  onOpenChange,
  snapshot,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState({
    year_month: snapshot.year_month,
    quantity: snapshot.quantity ?? "",
    price_per_unit: snapshot.price_per_unit ?? "",
    as_of_date: snapshot.as_of_date ? snapshot.as_of_date.slice(0, 10) : "",
    description: snapshot.description ?? "",
  });

  const derivedAmount = deriveAmount(form.quantity, form.price_per_unit);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (derivedAmount === null) return;
    mutation.mutate(
      {
        snapshotId: snapshot.id,
        payload: {
          amount: derivedAmount,
          currency: snapshot.currency,
          quantity: form.quantity,
          price_per_unit: form.price_per_unit,
          accrued_interest: null,
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
          <DialogTitle>{t("investments:quantityPriceSnapshot.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:quantityPriceSnapshot.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <ValidatedForm onSubmit={submit} error={mutation.error} className="space-y-3">
          <QuantityPriceSnapshotFormBody
            mode="edit"
            currency={snapshot.currency}
            form={form}
            setForm={setForm}
            derivedAmount={derivedAmount}
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
  );
}
