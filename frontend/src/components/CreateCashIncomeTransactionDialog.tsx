import { useState } from "react";
import { Plus } from "lucide-react";
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
import { todayDate } from "@/lib/dateLimits";
import type { CreateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";

// CashIncome shape covers Coupon (bond), Dividend (stock), Distribution
// (mutual fund). Cash received from the instrument; per ADR-0003 it does
// NOT propagate to bank-account snapshots (the user reads the resulting
// cash off the next bank statement).
type CashIncomeType = "coupon" | "dividend" | "distribution";

type Props<TResult> = {
  currency: string;
  txnType: CashIncomeType;
  mutation: UseMutationResult<
    TResult,
    unknown,
    CreateInvestmentTransactionPayload
  >;
};

function emptyForm() {
  return {
    transaction_date: todayDate(),
    amount: "",
    description: "",
  };
}

const TRIGGER_KEYS: Record<CashIncomeType, string> = {
  coupon: "investments:cashIncome.couponTrigger",
  dividend: "investments:cashIncome.dividendTrigger",
  distribution: "investments:cashIncome.distributionTrigger",
};
const TITLE_KEYS: Record<CashIncomeType, string> = {
  coupon: "investments:cashIncome.couponTitle",
  dividend: "investments:cashIncome.dividendTitle",
  distribution: "investments:cashIncome.distributionTitle",
};
const RECORD_KEYS: Record<CashIncomeType, string> = {
  coupon: "investments:cashIncome.recordCoupon",
  dividend: "investments:cashIncome.recordDividend",
  distribution: "investments:cashIncome.recordDistribution",
};

export function CreateCashIncomeTransactionDialog<TResult>({
  currency,
  txnType,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  function close() {
    setOpen(false);
    setForm(emptyForm());
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.amount) return;
    mutation.mutate(
      {
        transaction_type: txnType,
        transaction_date: form.transaction_date,
        currency,
        description: form.description || null,
        amount: form.amount,
        quantity: null,
        price_per_unit: null,
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
          {t(TRIGGER_KEYS[txnType])}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t(TITLE_KEYS[txnType])}</DialogTitle>
          <DialogDescription>
            {t("investments:cashIncome.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="cash_date">
                {t("investments:cashIncome.paymentDateLabel")}
              </Label>
              <Input
                id="cash_date"
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
              <Label htmlFor="cash_amount">
                {t("investments:cashIncome.amountLabel", { currency })}
              </Label>
              <Input
                id="cash_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
                placeholder={t("investments:cashIncome.amountPlaceholder")}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="cash_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="cash_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              placeholder={t("investments:cashIncome.descriptionPlaceholder")}
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending || !form.amount}>
              {mutation.isPending
                ? t("common:actions.saving")
                : t(RECORD_KEYS[txnType])}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
