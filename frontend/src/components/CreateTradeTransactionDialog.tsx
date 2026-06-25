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
import { formatCurrency } from "@/lib/format";
import { todayDate } from "@/lib/dateLimits";
import type { CreateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";

// Trade shape covers Buy + Sell. The type is fixed by the caller —
// each detail page wires a "+ Buy" and "+ Sell" button passing
// txnType: 'buy' | 'sell'.
type Props<TResult> = {
  currency: string;
  txnType: "buy" | "sell";
  quantityUnit: string; // "sh", "units", "g", etc.
  // Optional sub-field guidance under the price input. Gold passes
  // distinct buy/sell hints to spell out the bid/ask spread (issue #19);
  // other subtypes omit it.
  priceHint?: string;
  mutation: UseMutationResult<
    TResult,
    unknown,
    CreateInvestmentTransactionPayload
  >;
};

function emptyForm() {
  return {
    transaction_date: todayDate(),
    quantity: "",
    price_per_unit: "",
    description: "",
  };
}

function deriveAmount(quantity: string, pricePerUnit: string): string | null {
  const q = Number(quantity);
  const p = Number(pricePerUnit);
  if (!quantity || !pricePerUnit || Number.isNaN(q) || Number.isNaN(p)) {
    return null;
  }
  return (q * p).toString();
}

export function CreateTradeTransactionDialog<TResult>({
  currency,
  txnType,
  quantityUnit,
  priceHint,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const derivedAmount = deriveAmount(form.quantity, form.price_per_unit);
  const isBuy = txnType === "buy";

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
        transaction_type: txnType,
        transaction_date: form.transaction_date,
        currency,
        description: form.description || null,
        amount: derivedAmount,
        quantity: form.quantity,
        price_per_unit: form.price_per_unit,
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
        <Button size="sm" variant={isBuy ? "default" : "outline"}>
          <Plus className="mr-1 size-4" />
          {t(
            isBuy
              ? "investments:trade.buyTrigger"
              : "investments:trade.sellTrigger",
          )}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t(
              isBuy
                ? "investments:trade.buyTitle"
                : "investments:trade.sellTitle",
            )}
          </DialogTitle>
          <DialogDescription>
            {t(
              isBuy
                ? "investments:trade.buyDescription"
                : "investments:trade.sellDescription",
              { currency },
            )}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="trade_date">
              {t("investments:trade.tradeDateLabel")}
            </Label>
            <Input
              id="trade_date"
              type="date"
              required
              max={todayDate()}
              value={form.transaction_date}
              onChange={(e) =>
                setForm({ ...form, transaction_date: e.target.value })
              }
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="trade_quantity">
                {t("investments:trade.quantityLabel", { unit: quantityUnit })}
              </Label>
              <Input
                id="trade_quantity"
                required
                inputMode="decimal"
                value={form.quantity}
                onChange={(e) => setForm({ ...form, quantity: e.target.value })}
                placeholder={t("investments:trade.quantityPlaceholder")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="trade_price">
                {t("investments:trade.pricePerUnitLabel", { currency })}
              </Label>
              <Input
                id="trade_price"
                required
                inputMode="decimal"
                value={form.price_per_unit}
                onChange={(e) =>
                  setForm({ ...form, price_per_unit: e.target.value })
                }
                placeholder={t("investments:trade.pricePerUnitPlaceholder")}
              />
            </div>
          </div>

          {priceHint && (
            <p className="text-xs text-muted-foreground">{priceHint}</p>
          )}

          <div className="rounded-md bg-muted px-3 py-2 text-sm">
            <span className="text-muted-foreground">
              {t(
                isBuy
                  ? "investments:trade.cashOutLabel"
                  : "investments:trade.cashInLabel",
              )}
            </span>{" "}
            <span className="font-medium">
              {derivedAmount !== null
                ? formatCurrency(derivedAmount, currency)
                : "—"}
            </span>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="trade_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="trade_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              placeholder={t("investments:trade.descriptionPlaceholder")}
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
            <Button
              type="submit"
              disabled={mutation.isPending || derivedAmount === null}
            >
              {mutation.isPending
                ? t("common:actions.saving")
                : t(
                    isBuy
                      ? "investments:trade.recordBuy"
                      : "investments:trade.recordSell",
                  )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
