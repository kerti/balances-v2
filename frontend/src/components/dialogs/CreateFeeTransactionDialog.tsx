import { useState } from "react";
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
import { Input } from "@/components/ui/input";
import { NumericInput } from "@/components/common/NumericInput";
import {
  PRICE_DECIMALS,
  QUANTITY_DECIMALS,
  QUANTITY_INTEGER_DIGITS,
  currencyDecimals,
} from "@/lib/format";
import { Label } from "@/components/ui/label";
import { todayDate } from "@/lib/dateLimits";
import { deriveFeeQuantity } from "@/lib/feeQuantity";
import type { CreateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";

// Fee shape: cash amount required; quantity + price_per_unit optional but
// must be set together (for instruments where the manager settles a fee
// by removing units, typical for gold and some mutual funds). CONTEXT.md
// + ADR-0003: "snapshot quantity should reconcile to Σ(Buys.qty) −
// Σ(Sells.qty) − Σ(Fees.qty_deducted)".
type Props<TResult> = {
  currency: string;
  quantityUnit: string;
  mutation: UseMutationResult<TResult, unknown, CreateInvestmentTransactionPayload>;
};

function emptyForm() {
  return {
    transaction_date: todayDate(),
    amount: "",
    quantity: "",
    price_per_unit: "",
    description: "",
  };
}

export function CreateFeeTransactionDialog<TResult>({
  currency,
  quantityUnit,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  // Once the user types into the units field we stop auto-deriving it, so their
  // figure is never clobbered (cash→quantity helper, Q12).
  const [qtyTouched, setQtyTouched] = useState(false);

  // Unit fee: both qty + price are filled; neither = pure cash fee.
  const hasQty = !!form.quantity;
  const hasPrice = !!form.price_per_unit;
  const unitFeeIncomplete = hasQty !== hasPrice;
  const qtyAutoDerived = !qtyTouched && !!form.quantity;

  // Patch the form, re-deriving units from cash ÷ price unless the user has
  // taken over the units field. Computing in the change handler (not a useEffect
  // on a mutation-bearing form) sidesteps the deps-array render loop.
  function patch(next: Partial<ReturnType<typeof emptyForm>>) {
    setForm((prev) => {
      const merged = { ...prev, ...next };
      if (!qtyTouched && ("amount" in next || "price_per_unit" in next)) {
        merged.quantity = deriveFeeQuantity(merged.amount, merged.price_per_unit) ?? "";
      }
      return merged;
    });
  }

  function close() {
    setOpen(false);
    setForm(emptyForm());
    setQtyTouched(false);
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.amount || unitFeeIncomplete) return;
    mutation.mutate(
      {
        transaction_type: "fee",
        transaction_date: form.transaction_date,
        currency,
        description: form.description || null,
        amount: form.amount,
        quantity: form.quantity || null,
        price_per_unit: form.price_per_unit || null,
        principal_amount: null,
        interest_amount: null,
        principal_disposition: null,
        interest_disposition: null,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          <Plus className="mr-1 size-4" />
          {t("investments:fee.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("investments:fee.createTitle")}</DialogTitle>
          <DialogDescription>{t("investments:fee.createDescription")}</DialogDescription>
        </DialogHeader>
        <ValidatedForm
          onSubmit={submit}
          error={mutation.error}
          validate={() =>
            unitFeeIncomplete
              ? { [hasQty ? "fee_price" : "fee_quantity"]: t("common:validation.required") }
              : {}
          }
          className="space-y-3"
        >
          <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
            <div className="grid gap-2">
              <Label htmlFor="fee_date">{t("investments:fee.feeDateLabel")}</Label>
              <Input
                id="fee_date"
                type="date"
                required
                max={todayDate()}
                value={form.transaction_date}
                onChange={(e) => setForm({ ...form, transaction_date: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="fee_amount">
                {t("investments:fee.cashAmountLabel", { currency })}
              </Label>
              <NumericInput
                id="fee_amount"
                required
                decimals={currencyDecimals(currency)}
                value={form.amount}
                onChange={(v) => patch({ amount: v })}
                placeholder={t("investments:fee.cashAmountPlaceholder")}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
            <div className="grid gap-2">
              <Label htmlFor="fee_price">
                {t("investments:fee.conversionPriceLabel", { currency })}
              </Label>
              <NumericInput
                id="fee_price"
                decimals={PRICE_DECIMALS}
                value={form.price_per_unit}
                onChange={(v) => patch({ price_per_unit: v })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="fee_quantity">
                {t("investments:fee.unitsDeductedLabel", {
                  unit: quantityUnit,
                })}
              </Label>
              <NumericInput
                id="fee_quantity"
                decimals={QUANTITY_DECIMALS}
                maxIntegerDigits={QUANTITY_INTEGER_DIGITS}
                value={form.quantity}
                onChange={(v) => {
                  setQtyTouched(true);
                  setForm((prev) => ({ ...prev, quantity: v }));
                }}
              />
              {qtyAutoDerived && (
                <p className="text-xs text-muted-foreground">{t("investments:fee.derivedHint")}</p>
              )}
            </div>
          </div>

          {unitFeeIncomplete && (
            <p className="text-xs text-warning">{t("investments:fee.incompleteHint")}</p>
          )}

          <div className="grid gap-2">
            <Label htmlFor="fee_description">{t("common:fields.note")}</Label>
            <Input
              id="fee_description"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder={t("investments:fee.descriptionPlaceholder")}
            />
          </div>

          <FormError />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("common:actions.saving") : t("investments:fee.recordFee")}
            </Button>
          </DialogFooter>
        </ValidatedForm>
      </DialogContent>
    </Dialog>
  );
}
