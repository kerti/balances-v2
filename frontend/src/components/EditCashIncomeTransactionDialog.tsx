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
import type { InvestmentTransaction } from "@/api/types";
import type { UpdateTransactionMutationVariables } from "@/components/EditTradeTransactionDialog";

type Props<TResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  transaction: InvestmentTransaction;
  mutation: UseMutationResult<TResult, unknown, UpdateTransactionMutationVariables>;
};

const TITLE_KEYS: Record<string, string> = {
  coupon: "investments:cashIncome.editCouponTitle",
  dividend: "investments:cashIncome.editDividendTitle",
  distribution: "investments:cashIncome.editDistributionTitle",
};

export function EditCashIncomeTransactionDialog<TResult>({
  open,
  onOpenChange,
  transaction,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [form, setForm] = useState({
    transaction_date: transaction.transaction_date.slice(0, 10),
    amount: transaction.amount ?? "",
    description: transaction.description ?? "",
  });

  const titleKey =
    TITLE_KEYS[transaction.transaction_type] ?? "investments:cashIncome.editIncomeTitle";

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.amount) return;
    mutation.mutate(
      {
        transactionId: transaction.id,
        payload: {
          transaction_date: form.transaction_date,
          currency: transaction.currency,
          description: form.description || null,
          amount: form.amount,
          quantity: null,
          price_per_unit: null,
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
          <DialogTitle>{t(titleKey)}</DialogTitle>
          <DialogDescription>{t("investments:cashIncome.editDescription")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_cash_date">{t("investments:cashIncome.paymentDateLabel")}</Label>
              <Input
                id="edit_cash_date"
                type="date"
                required
                max={todayDate()}
                value={form.transaction_date}
                onChange={(e) => setForm({ ...form, transaction_date: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_cash_amount">
                {t("investments:cashIncome.amountLabel", {
                  currency: transaction.currency,
                })}
              </Label>
              <Input
                id="edit_cash_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_cash_description">{t("common:fields.description")}</Label>
            <Input
              id="edit_cash_description"
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
            <Button type="submit" disabled={mutation.isPending || !form.amount}>
              {mutation.isPending ? t("common:actions.saving") : t("common:actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
