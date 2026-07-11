import { useState } from "react";
import { CopyPlus, Plus } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { UseMutationResult } from "@tanstack/react-query";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { errorMessage } from "@/lib/errorMessage";
import { formatCurrency } from "@/lib/format";
import { thisYearMonth, carryoverSeed, monthStartDate, monthEndDateCapped } from "@/lib/dateLimits";
import { useSession } from "@/hooks/useSession";
import type { CarryoverDateMode } from "@/lib/dateLimits";
import type { CreateInvestmentSnapshotPayload } from "@/hooks/useInvestmentSnapshots";

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
      {carryover && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={startCarryover}
          data-testid="snapshot-carryover"
        >
          <CopyPlus className="mr-1 size-4" />
          {t("investments:quantityPriceSnapshot.carryoverTrigger")}
        </Button>
      )}
      <DialogTrigger asChild>
        <Button size="sm">
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
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="inv_year_month">{t("common:fields.month")}</Label>
              <Input
                id="inv_year_month"
                type="month"
                required
                max={thisYearMonth()}
                value={form.year_month}
                onChange={(e) => setForm({ ...form, year_month: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inv_as_of_date">{t("common:fields.statementDate")}</Label>
              <Input
                id="inv_as_of_date"
                type="date"
                min={monthStartDate(form.year_month)}
                max={monthEndDateCapped(form.year_month)}
                value={form.as_of_date}
                onChange={(e) => setForm({ ...form, as_of_date: e.target.value })}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="inv_quantity">
                {t("investments:quantityPriceSnapshot.quantityLabel")}
              </Label>
              <Input
                id="inv_quantity"
                required
                inputMode="decimal"
                value={form.quantity}
                onChange={(e) => setForm({ ...form, quantity: e.target.value })}
                placeholder={t("investments:quantityPriceSnapshot.quantityPlaceholder")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inv_price_per_unit">
                {t("investments:quantityPriceSnapshot.pricePerUnitLabel", {
                  currency,
                })}
              </Label>
              <Input
                id="inv_price_per_unit"
                required
                inputMode="decimal"
                value={form.price_per_unit}
                onChange={(e) => setForm({ ...form, price_per_unit: e.target.value })}
                placeholder={t("investments:quantityPriceSnapshot.pricePerUnitPlaceholder")}
              />
            </div>
          </div>

          {priceHint && <p className="text-xs text-muted-foreground">{priceHint}</p>}

          <div className="rounded-md bg-muted px-3 py-2 text-sm">
            <span className="text-muted-foreground">
              {t("investments:quantityPriceSnapshot.totalValueLabel")}
            </span>{" "}
            <span className="font-medium">
              {derivedAmount !== null ? formatCurrency(derivedAmount, currency) : "—"}
            </span>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="inv_snap_description">{t("common:fields.description")}</Label>
            <Input
              id="inv_snap_description"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder={t("investments:quantityPriceSnapshot.descriptionPlaceholder")}
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending || derivedAmount === null}>
              {mutation.isPending
                ? t("common:actions.saving")
                : t("investments:quantityPriceSnapshot.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
