import { useState } from "react";
import { useTranslation } from "react-i18next";
import { CopyPlus } from "lucide-react";
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
import { FieldError, useFieldError } from "@/components/ui/field-error";
import { NumericInput } from "@/components/common/NumericInput";
import { useCreateIncomeBulk } from "@/hooks/useIncome";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { copyCandidates, previousMonth, type CopyCandidate } from "@/lib/incomeCopy";
import { currencyDecimals, formatDate, formatYearMonth } from "@/lib/format";
import { ownershipLabel } from "@/lib/ownership";
import { cn } from "@/lib/utils";
import type { Income } from "@/api/types";

type Props = {
  /** Every income row the screen holds; the dialog picks the rows it needs. */
  income: Income[];
  /** The selected month, "YYYY-MM". Rows are copied from the month before. */
  month: string;
  /** Mobile toolbar: the short trigger label. */
  compactTrigger?: boolean;
};

type RowState = { ticked: boolean; amount: string };

// Midday UTC so a viewer's timezone never shifts the month name.
const monthLabel = (ym: string) => formatYearMonth(`${ym}-15T12:00:00Z`);

function TickError() {
  const { message } = useFieldError("copy_rows");
  return message ? <FieldError id="copy_rows">{message}</FieldError> : null;
}

// Review form (#846): one row per Regular event of the previous month, dated
// into the selected month, amount editable, ticked unless this month already
// has that source. Mounted only while the dialog is open, so its state starts
// fresh from the candidates each time — no effect needed.
function CopyForm({
  candidates,
  month,
  onDone,
}: {
  candidates: CopyCandidate[];
  month: string;
  onDone: () => void;
}) {
  const { t } = useTranslation(["income", "common"]);
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  const mutation = useCreateIncomeBulk();
  const [rows, setRows] = useState<RowState[]>(() =>
    candidates.map((c) => ({ ticked: !c.alreadyThere, amount: c.source.amount })),
  );

  function patch(i: number, next: Partial<RowState>) {
    setRows((prev) => prev.map((r, j) => (j === i ? { ...r, ...next } : r)));
  }

  const tickedCount = rows.filter((r) => r.ticked).length;

  function validate() {
    return tickedCount === 0 ? { copy_rows: t("income:copy.noneTicked") } : {};
  }

  function submit() {
    const payload = candidates.flatMap((c, i) =>
      rows[i].ticked
        ? [
            {
              date: c.date,
              amount: rows[i].amount,
              currency: c.source.currency,
              category: c.source.category,
              description: c.source.description,
              ownership_type: c.source.ownership_type,
              sole_owner_user_id: c.source.sole_owner_user_id,
              regularity: c.source.regularity,
            },
          ]
        : [],
    );
    mutation.mutate(payload, { onSuccess: onDone });
  }

  return (
    <ValidatedForm
      onSubmit={submit}
      validate={validate}
      error={mutation.error}
      className="space-y-3"
    >
      <div id="copy_rows" tabIndex={-1} className="space-y-2 outline-none">
        {candidates.map((c, i) => {
          const r = rows[i];
          const label = t(`income:categories.${c.source.category}`);
          const owner = ownershipLabel(
            c.source.ownership_type,
            c.source.sole_owner_user_id,
            members,
            currentUser,
          );
          return (
            <div
              key={c.source.id}
              className="grid gap-2 rounded-lg border p-3"
              data-testid="copy-income-row"
            >
              <label className="flex min-h-11 items-start gap-2 md:min-h-0">
                <input
                  id={`copy_tick_${i}`}
                  type="checkbox"
                  className="mt-1 size-4"
                  checked={r.ticked}
                  onChange={(e) => patch(i, { ticked: e.target.checked })}
                  data-testid={`copy-income-tick-${i}`}
                />
                <span className="min-w-0">
                  <span className="block font-medium">
                    {label}
                    {c.source.description ? ` · ${c.source.description}` : ""}
                  </span>
                  <span className="block text-xs text-muted-foreground">
                    {owner} · {formatDate(`${c.date}T12:00:00Z`)}
                  </span>
                </span>
              </label>
              {c.alreadyThere && (
                <p className="text-xs text-muted-foreground" data-testid={`copy-income-flag-${i}`}>
                  {t("income:copy.alreadyThere", { month: monthLabel(month) })}
                </p>
              )}
              <div className={cn("grid grid-cols-[1fr_auto] items-center gap-2")}>
                <NumericInput
                  id={`copy_amount_${i}`}
                  aria-label={t("income:fields.amount")}
                  required={r.ticked}
                  disabled={!r.ticked}
                  decimals={currencyDecimals(c.source.currency)}
                  value={r.amount}
                  onChange={(v) => patch(i, { amount: v })}
                  data-testid={`copy-income-amount-${i}`}
                />
                <span className="text-sm text-muted-foreground">{c.source.currency}</span>
              </div>
            </div>
          );
        })}
      </div>
      <TickError />
      <FormError />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onDone}>
          {t("common:cancel")}
        </Button>
        <Button type="submit" disabled={mutation.isPending} data-testid="copy-income-save">
          {mutation.isPending
            ? t("income:submit.saving")
            : t("income:copy.save", { count: tickedCount })}
        </Button>
      </DialogFooter>
    </ValidatedForm>
  );
}

// "Copy last month's regular income". Hidden (renders nothing) when the month
// before has no Regular events: the header action only appears when it can do
// something, and the dialog it opens always has rows.
export function CopyRegularIncomeDialog({ income, month, compactTrigger = false }: Props) {
  const { t } = useTranslation(["income"]);
  const [open, setOpen] = useState(false);
  const candidates = copyCandidates(income, month);
  if (candidates.length === 0) return null;
  const from = previousMonth(month);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" data-testid="copy-income-trigger">
          <CopyPlus className="mr-1 size-4" />
          {t(compactTrigger ? "income:copy.triggerShort" : "income:copy.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("income:copy.title")}</DialogTitle>
          <DialogDescription>
            {t("income:copy.description", { from: monthLabel(from), to: monthLabel(month) })}
          </DialogDescription>
        </DialogHeader>
        <CopyForm candidates={candidates} month={month} onDone={() => setOpen(false)} />
      </DialogContent>
    </Dialog>
  );
}
