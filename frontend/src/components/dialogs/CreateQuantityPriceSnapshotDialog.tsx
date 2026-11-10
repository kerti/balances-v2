import { useState } from "react";
import { useRecordOnArrival } from "@/lib/recordOnArrival";
import { Plus } from "lucide-react";
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
  DialogTrigger,
} from "@/components/ui/dialog";
import { formatCurrency } from "@/lib/format";
import { asOfInputValue, formatRecordMonth } from "@/lib/recordSnapshot";
import { useRecordSnapshotForm } from "@/hooks/useRecordSnapshotForm";
import type { CreateInvestmentSnapshotPayload } from "@/hooks/useInvestmentSnapshots";
import {
  QuantityPriceSnapshotFormBody,
  type QuantityPriceSnapshotFormState,
} from "@/components/dialogs/forms/QuantityPriceSnapshotForm";
import type { UpdateQuantityPriceSnapshotMutationVariables } from "@/components/dialogs/EditQuantityPriceSnapshotDialog";

// The live snapshots the dialog needs (see CreateSnapshotDialog).
type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  quantity: string | null;
  price_per_unit: string | null;
  as_of_date: string | null;
  description: string | null;
};

type Props<TCreated, TUpdated> = {
  currency: string;
  // Optional sub-field guidance under the price input. Gold passes a
  // "use the buyback price" hint (issue #19); stock/mutual-fund omit it.
  priceHint?: string;
  // Live snapshots of this position, newest first.
  snapshots: SnapshotLike[] | undefined;
  // Mutations are owned by the parent so the same dialog drives stocks,
  // mutual funds, and gold. Create writes an empty month; update replaces a
  // month that already has a value (ADR-0058: archived, Undo in the toast).
  createMutation: UseMutationResult<TCreated, unknown, CreateInvestmentSnapshotPayload>;
  updateMutation: UseMutationResult<
    TUpdated,
    unknown,
    UpdateQuantityPriceSnapshotMutationVariables
  >;
};

const emptyForm = (yearMonth: string): QuantityPriceSnapshotFormState => ({
  year_month: yearMonth,
  quantity: "",
  price_per_unit: "",
  as_of_date: "",
  description: "",
});

const formFrom = (s: SnapshotLike, yearMonth: string): QuantityPriceSnapshotFormState => ({
  year_month: yearMonth,
  quantity: s.quantity ?? "",
  price_per_unit: s.price_per_unit ?? "",
  as_of_date: asOfInputValue(s.as_of_date),
  description: s.description ?? "",
});

// amount = quantity × price_per_unit. The backend re-validates and stores
// amount alongside the two factors, so the UI sends both. Computed in
// JS with Number — household scale is fine; precision-sensitive arithmetic
// stays on the backend (decimal.Decimal).
function deriveAmount(quantity: string, pricePerUnit: string): string | null {
  const q = Number(quantity);
  const p = Number(pricePerUnit);
  if (!quantity || !pricePerUnit || Number.isNaN(q) || Number.isNaN(p)) {
    return null;
  }
  return (q * p).toString();
}

// "Record snapshot" for the quantity × price shape (stock, mutual fund, gold).
// See CreateSnapshotDialog for the replace-mode behaviour (#788).
export function CreateQuantityPriceSnapshotDialog<TCreated, TUpdated>({
  currency,
  priceHint,
  snapshots,
  createMutation,
  updateMutation,
}: Props<TCreated, TUpdated>) {
  const { t } = useTranslation(["investments", "common"]);
  // Open straight away when a create flow just landed on this item (#843).
  const [open, setOpen] = useState(useRecordOnArrival());
  const { form, setForm, existing, last, pickMonth, reset } = useRecordSnapshotForm({
    snapshots,
    emptyForm,
    formFrom,
  });
  const mutation = existing ? updateMutation : createMutation;

  const derivedAmount = deriveAmount(form.quantity, form.price_per_unit);

  function begin() {
    reset();
    setOpen(true);
  }

  function close() {
    setOpen(false);
    createMutation.reset();
    updateMutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (derivedAmount === null) return;
    const common = {
      amount: derivedAmount,
      currency,
      quantity: form.quantity,
      price_per_unit: form.price_per_unit,
      accrued_interest: null,
      as_of_date: form.as_of_date || null,
      description: form.description || null,
    };
    if (existing) {
      updateMutation.mutate({ snapshotId: existing.id, payload: common }, { onSuccess: close });
      return;
    }
    createMutation.mutate({ year_month: form.year_month, ...common }, { onSuccess: close });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? begin() : close())}>
      {/* The single entry point to recording a value (#788); mirrors the
          amount-only `CreateSnapshotDialog`. */}
      <DialogTrigger asChild>
        <Button size="sm" className="h-11 md:h-8" data-testid="snapshot-record">
          <Plus className="mr-1 size-4" />
          {t("common:snapshot.recordTrigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("investments:quantityPriceSnapshot.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:quantityPriceSnapshot.createDescription", {
              currency,
            })}
          </DialogDescription>
        </DialogHeader>
        <ValidatedForm onSubmit={submit} error={mutation.error} className="space-y-3">
          <QuantityPriceSnapshotFormBody
            mode="create"
            currency={currency}
            form={form}
            setForm={setForm}
            derivedAmount={derivedAmount}
            priceHint={priceHint}
            onMonthChange={pickMonth}
            replacing={existing ? formatCurrency(existing.amount, currency) : undefined}
            lastRecorded={
              last
                ? {
                    value: formatCurrency(last.amount, currency),
                    month: formatRecordMonth(last.year_month),
                    onApply: () =>
                      setForm({
                        ...form,
                        quantity: last.quantity ?? "",
                        price_per_unit: last.price_per_unit ?? "",
                      }),
                  }
                : undefined
            }
          />

          <FormError />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common:actions.saving")
                : t("investments:quantityPriceSnapshot.save")}
            </Button>
          </DialogFooter>
        </ValidatedForm>
      </DialogContent>
    </Dialog>
  );
}
