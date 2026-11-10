import { useState } from "react";
import { CopyPlus, Plus } from "lucide-react";
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
import { thisYearMonth, carryoverSeed } from "@/lib/dateLimits";
import { useSession } from "@/hooks/useSession";
import type { CarryoverDateMode } from "@/lib/dateLimits";
import type { CreateInvestmentSnapshotPayload } from "@/hooks/useInvestmentSnapshots";
import { QuantityPriceSnapshotFormBody } from "@/components/dialogs/forms/QuantityPriceSnapshotForm";

type Props<TResult> = {
  currency: string;
  // Optional sub-field guidance under the price input. Gold passes a
  // "use the buyback price" hint (issue #19); stock/mutual-fund omit it.
  priceHint?: string;
  // Mutation is owned by the parent so the same dialog drives stocks,
  // mutual funds, and gold — each subtype's detail page wires its own
  // useCreateInvestmentSnapshot result in.
  mutation: UseMutationResult<TResult, unknown, CreateInvestmentSnapshotPayload>;
  // Latest snapshot's quantity + price, when one exists. Drives the "Copy
  // carryover" helper (issue #60): an unchanged month keeps the same factors,
  // so the derived total carries over too. Null hides the helper.
  carryover?: {
    quantity: string | null;
    price_per_unit: string | null;
    lastSnapshotMonth: string;
  } | null;
};

function emptyForm() {
  return {
    year_month: thisYearMonth(),
    quantity: "",
    price_per_unit: "",
    as_of_date: "",
    description: "",
  };
}

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

export function CreateQuantityPriceSnapshotDialog<TResult>({
  currency,
  priceHint,
  mutation,
  carryover,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const { data: me } = useSession();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const derivedAmount = deriveAmount(form.quantity, form.price_per_unit);

  // Seed the form from the last snapshot's factors and open the dialog. Month
  // resets to the current month; the statement date is seeded per the user's
  // carryover_date_mode preference (issue #105, default 'today').
  function startCarryover() {
    if (!carryover) return;
    const mode = (me?.carryover_date_mode ?? "today") as CarryoverDateMode;
    const seed = carryoverSeed(mode, carryover.lastSnapshotMonth);
    setForm({
      year_month: seed.yearMonth,
      quantity: carryover.quantity ?? "",
      price_per_unit: carryover.price_per_unit ?? "",
      as_of_date: seed.asOfDate,
      description: "",
    });
    setOpen(true);
  }

  function close() {
    setOpen(false);
    setForm(emptyForm());
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (derivedAmount === null) return;
    mutation.mutate(
      {
        year_month: form.year_month,
        amount: derivedAmount,
        currency,
        quantity: form.quantity,
        price_per_unit: form.price_per_unit,
        accrued_interest: null,
        as_of_date: form.as_of_date || null,
        description: form.description || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      {/* Copy carryover leads the create row as the promoted primary (#542):
          solid variant + large mobile tap target. New drops to a secondary
          outline unless there's no prior snapshot to copy, when it stands alone
          as the primary. Mirrors the amount-only `CreateSnapshotDialog`. */}
      {carryover && (
        <Button
          type="button"
          size="sm"
          onClick={startCarryover}
          className="h-11 md:h-8"
          data-testid="snapshot-carryover"
        >
          <CopyPlus className="mr-1 size-4" />
          {t("investments:quantityPriceSnapshot.carryoverTrigger")}
        </Button>
      )}
      <DialogTrigger asChild>
        <Button
          size="sm"
          variant={carryover ? "outline" : "default"}
          className={carryover ? undefined : "h-11 md:h-8"}
        >
          <Plus className="mr-1 size-4" />
          {t("investments:quantityPriceSnapshot.trigger")}
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
