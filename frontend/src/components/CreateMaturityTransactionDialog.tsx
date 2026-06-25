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
import type { Disposition, RolloverPolicy } from "@/api/types";

// Maturity shape (ADR-0009 §"Maturity transaction extension"). Records the
// principal + interest at maturity plus a disposition for each:
//   rolled_to_new — reinvested into a new instrument (a fresh row should
//     be created with the rolled-over amount; this dialog doesn't create
//     it automatically — see HANDOFF deferred items "duplicate matured TD")
//   cash_out      — paid out as cash (per ADR-0003 does NOT propagate to
//     bank-account snapshots; user sees it in the next bank statement)
//
// rolloverPolicy (when supplied — TD has it, Bond doesn't) drives the
// default dispositions. The user can override per event.
type Props<TResult> = {
  currency: string;
  rolloverPolicy?: RolloverPolicy;
  // The instrument's term, YYYY-MM-DD (issue #62). The Maturity event happened
  // at maturityDate, so it seeds the date field and — together with
  // placementDate — bounds it: the backend confines a TimeDeposit's Maturity to
  // [placement, maturity]. Both optional; a Bond passes only maturityDate (no
  // placement → no lower bound, and the backend leaves bonds unbounded).
  placementDate?: string;
  maturityDate?: string;
  mutation: UseMutationResult<
    TResult,
    unknown,
    CreateInvestmentTransactionPayload
  >;
};

// The Maturity event is dated at maturityDate, the day the deposit actually
// matured — not at data-entry time. Defaulting to "today" would stamp
// terminated_at and the close snapshot in the wrong month, and (post-#62) be
// rejected outright once maturityDate is in the past. Falls back to today when
// the term has no maturityDate (shouldn't happen) or it is still in the future.
function defaultMaturityDate(maturityDate?: string): string {
  const today = todayDate();
  return maturityDate && maturityDate <= today ? maturityDate : today;
}

// The latest selectable date: the term's maturity (the #62 hard upper bound),
// but never in the future (the existing transaction future-date guard).
function maxMaturityDate(maturityDate?: string): string {
  const today = todayDate();
  return maturityDate && maturityDate < today ? maturityDate : today;
}

function defaultsForPolicy(policy: RolloverPolicy | undefined): {
  principal: Disposition;
  interest: Disposition;
} {
  switch (policy) {
    case "auto_renew_with_interest":
      return { principal: "rolled_to_new", interest: "rolled_to_new" };
    case "auto_renew_principal":
      return { principal: "rolled_to_new", interest: "cash_out" };
    case "no_rollover":
    default:
      return { principal: "cash_out", interest: "cash_out" };
  }
}

function emptyForm(policy?: RolloverPolicy, maturityDate?: string) {
  const d = defaultsForPolicy(policy);
  return {
    transaction_date: defaultMaturityDate(maturityDate),
    principal_amount: "",
    interest_amount: "",
    principal_disposition: d.principal,
    interest_disposition: d.interest,
    description: "",
  };
}

export function CreateMaturityTransactionDialog<TResult>({
  currency,
  rolloverPolicy,
  placementDate,
  maturityDate,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(() =>
    emptyForm(rolloverPolicy, maturityDate),
  );

  const totalReceived = (() => {
    const p = Number(form.principal_amount);
    const i = Number(form.interest_amount);
    if (Number.isNaN(p) || Number.isNaN(i)) return null;
    return (p + i).toString();
  })();

  function close() {
    setOpen(false);
    setForm(emptyForm(rolloverPolicy, maturityDate));
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.principal_amount || !form.interest_amount) return;
    mutation.mutate(
      {
        transaction_type: "maturity",
        transaction_date: form.transaction_date,
        currency,
        description: form.description || null,
        amount: null,
        quantity: null,
        price_per_unit: null,
        principal_amount: form.principal_amount,
        interest_amount: form.interest_amount,
        principal_disposition: form.principal_disposition,
        interest_disposition: form.interest_disposition,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          <Plus className="mr-1 size-4" />
          {t("investments:maturityTxn.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("investments:maturityTxn.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:maturityTxn.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="mat_date">
              {t("investments:maturityTxn.maturityDateLabel")}
            </Label>
            <Input
              id="mat_date"
              type="date"
              required
              min={placementDate || undefined}
              max={maxMaturityDate(maturityDate)}
              value={form.transaction_date}
              onChange={(e) =>
                setForm({ ...form, transaction_date: e.target.value })
              }
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="mat_principal">
                {t("investments:maturityTxn.principalLabel", { currency })}
              </Label>
              <Input
                id="mat_principal"
                required
                inputMode="decimal"
                value={form.principal_amount}
                onChange={(e) =>
                  setForm({ ...form, principal_amount: e.target.value })
                }
                placeholder={t("investments:maturityTxn.principalPlaceholder")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="mat_interest">
                {t("investments:maturityTxn.interestLabel", { currency })}
              </Label>
              <Input
                id="mat_interest"
                required
                inputMode="decimal"
                value={form.interest_amount}
                onChange={(e) =>
                  setForm({ ...form, interest_amount: e.target.value })
                }
                placeholder={t("investments:maturityTxn.interestPlaceholder")}
              />
            </div>
          </div>

          <div className="rounded-md bg-muted px-3 py-2 text-sm">
            <span className="text-muted-foreground">
              {t("investments:maturityTxn.totalAtMaturityLabel")}
            </span>{" "}
            <span className="font-medium">
              {totalReceived !== null
                ? formatCurrency(totalReceived, currency)
                : "—"}
            </span>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="mat_principal_disp">
                {t("investments:maturityTxn.principalDispositionLabel")}
              </Label>
              <select
                id="mat_principal_disp"
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={form.principal_disposition}
                onChange={(e) =>
                  setForm({
                    ...form,
                    principal_disposition: e.target.value as Disposition,
                  })
                }
              >
                <option value="cash_out">
                  {t("investments:disposition.cashOut")}
                </option>
                <option value="rolled_to_new">
                  {t("investments:disposition.rolledToNew")}
                </option>
              </select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="mat_interest_disp">
                {t("investments:maturityTxn.interestDispositionLabel")}
              </Label>
              <select
                id="mat_interest_disp"
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={form.interest_disposition}
                onChange={(e) =>
                  setForm({
                    ...form,
                    interest_disposition: e.target.value as Disposition,
                  })
                }
              >
                <option value="cash_out">
                  {t("investments:disposition.cashOut")}
                </option>
                <option value="rolled_to_new">
                  {t("investments:disposition.rolledToNew")}
                </option>
              </select>
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="mat_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="mat_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              placeholder={t("investments:maturityTxn.descriptionPlaceholder")}
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
              disabled={
                mutation.isPending ||
                !form.principal_amount ||
                !form.interest_amount
              }
            >
              {mutation.isPending
                ? t("common:actions.saving")
                : t("investments:maturityTxn.recordMaturity")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
