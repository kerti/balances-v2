import { useState } from "react";
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
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { errorMessage } from "@/lib/errorMessage";
import { todayDate } from "@/lib/dateLimits";
import { deriveFeeQuantity } from "@/lib/feeQuantity";
import type { InvestmentTransaction } from "@/api/types";
import type { UpdateTransactionMutationVariables } from "@/components/EditTradeTransactionDialog";

type Props<TResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  transaction: InvestmentTransaction;
  quantityUnit: string;
  mutation: UseMutationResult<
    TResult,
    unknown,
    UpdateTransactionMutationVariables
  >;
};

export function EditFeeTransactionDialog<TResult>({
  open,
  onOpenChange,
  transaction,
  quantityUnit,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState({
    transaction_date: transaction.transaction_date.slice(0, 10),
    amount: transaction.amount ?? "",
    quantity: transaction.quantity ?? "",
    price_per_unit: transaction.price_per_unit ?? "",
    description: transaction.description ?? "",
  });
  // Preserve a saved unit figure: only auto-derive (cash→quantity, Q12) when
  // this fee had no units to begin with.
  const [qtyTouched, setQtyTouched] = useState(!!transaction.quantity);

  const hasQty = !!form.quantity;
  const hasPrice = !!form.price_per_unit;
  const unitFeeIncomplete = hasQty !== hasPrice;
  const qtyAutoDerived = !qtyTouched && !!form.quantity;

  function patch(next: Partial<typeof form>) {
    setForm((prev) => {
      const merged = { ...prev, ...next };
      if (!qtyTouched && ("amount" in next || "price_per_unit" in next)) {
        merged.quantity =
          deriveFeeQuantity(merged.amount, merged.price_per_unit) ?? "";
      }
      return merged;
    });
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.amount || unitFeeIncomplete) return;
    mutation.mutate(
      {
        transactionId: transaction.id,
        payload: {
          transaction_date: form.transaction_date,
          currency: transaction.currency,
          description: form.description || null,
          amount: form.amount,
          quantity: form.quantity || null,
          price_per_unit: form.price_per_unit || null,
          principal_amount: null,
          interest_amount: null,
          principal_disposition: null,
          interest_disposition: null,
        },
      },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("investments:fee.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:fee.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_fee_date">
                {t("investments:fee.feeDateLabel")}
              </Label>
              <Input
                id="edit_fee_date"
                type="date"
                required
                max={todayDate()}
                value={form.transaction_date}
                onChange={(e) =>
                  setForm({ ...form, transaction_date: e.target.value })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_fee_amount">
                {t("investments:fee.cashAmountLabel", {
                  currency: transaction.currency,
                })}
              </Label>
              <Input
                id="edit_fee_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => patch({ amount: e.target.value })}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_fee_price">
                {t("investments:fee.conversionPriceLabel", {
                  currency: transaction.currency,
                })}
              </Label>
              <Input
                id="edit_fee_price"
                inputMode="decimal"
                value={form.price_per_unit}
                onChange={(e) => patch({ price_per_unit: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_fee_quantity">
                {t("investments:fee.unitsDeductedLabel", {
                  unit: quantityUnit,
                })}
              </Label>
              <Input
                id="edit_fee_quantity"
                inputMode="decimal"
                value={form.quantity}
                onChange={(e) => {
                  setQtyTouched(true);
                  setForm((prev) => ({ ...prev, quantity: e.target.value }));
                }}
              />
              {qtyAutoDerived && (
                <p className="text-xs text-muted-foreground">
                  {t("investments:fee.derivedHint")}
                </p>
              )}
            </div>
          </div>

          {unitFeeIncomplete && (
            <p className="text-xs text-amber-600">
              {t("investments:fee.incompleteHint")}
            </p>
          )}

          <div className="grid gap-2">
            <Label htmlFor="edit_fee_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="edit_fee_description"
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
              {t("common:cancel")}
            </Button>
            <Button
              type="submit"
              disabled={mutation.isPending || !form.amount || unitFeeIncomplete}
            >
              {mutation.isPending
                ? t("common:actions.saving")
                : t("common:actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
