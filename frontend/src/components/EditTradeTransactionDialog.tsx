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
import { formatCurrency } from "@/lib/format";
import { todayDate } from "@/lib/dateLimits";
import type { UpdateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";
import type { InvestmentTransaction } from "@/api/types";

export type UpdateTransactionMutationVariables = {
  transactionId: string;
  payload: UpdateInvestmentTransactionPayload;
};

type Props<TResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  transaction: InvestmentTransaction;
  quantityUnit: string;
  mutation: UseMutationResult<TResult, unknown, UpdateTransactionMutationVariables>;
};

function deriveAmount(quantity: string, pricePerUnit: string): string | null {
  const q = Number(quantity);
  const p = Number(pricePerUnit);
  if (!quantity || !pricePerUnit || Number.isNaN(q) || Number.isNaN(p)) {
    return null;
  }
  return (q * p).toString();
}

export function EditTradeTransactionDialog<TResult>({
  open,
  onOpenChange,
  transaction,
  quantityUnit,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState({
    transaction_date: transaction.transaction_date.slice(0, 10),
    quantity: transaction.quantity ?? "",
    price_per_unit: transaction.price_per_unit ?? "",
    description: transaction.description ?? "",
  });

  const derivedAmount = deriveAmount(form.quantity, form.price_per_unit);
  const isBuy = transaction.transaction_type === "buy";

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (derivedAmount === null) return;
    mutation.mutate(
      {
        transactionId: transaction.id,
        payload: {
          transaction_date: form.transaction_date,
          currency: transaction.currency,
          description: form.description || null,
          amount: derivedAmount,
          quantity: form.quantity,
          price_per_unit: form.price_per_unit,
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
          <DialogTitle>
            {t(isBuy ? "investments:trade.editBuyTitle" : "investments:trade.editSellTitle")}
          </DialogTitle>
          <DialogDescription>{t("investments:trade.editDescription")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="edit_trade_date">{t("investments:trade.tradeDateLabel")}</Label>
            <Input
              id="edit_trade_date"
              type="date"
              required
              max={todayDate()}
              value={form.transaction_date}
              onChange={(e) => setForm({ ...form, transaction_date: e.target.value })}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_trade_quantity">
                {t("investments:trade.quantityLabel", { unit: quantityUnit })}
              </Label>
              <Input
                id="edit_trade_quantity"
                required
                inputMode="decimal"
                value={form.quantity}
                onChange={(e) => setForm({ ...form, quantity: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_trade_price">
                {t("investments:trade.pricePerUnitLabel", {
                  currency: transaction.currency,
                })}
              </Label>
              <Input
                id="edit_trade_price"
                required
                inputMode="decimal"
                value={form.price_per_unit}
                onChange={(e) => setForm({ ...form, price_per_unit: e.target.value })}
              />
            </div>
          </div>

          <div className="rounded-md bg-muted px-3 py-2 text-sm">
            <span className="text-muted-foreground">
              {t(isBuy ? "investments:trade.cashOutLabel" : "investments:trade.cashInLabel")}
            </span>{" "}
            <span className="font-medium">
              {derivedAmount !== null ? formatCurrency(derivedAmount, transaction.currency) : "—"}
            </span>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_trade_description">{t("common:fields.description")}</Label>
            <Input
              id="edit_trade_description"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending || derivedAmount === null}>
              {mutation.isPending ? t("common:actions.saving") : t("common:actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
