import { useState } from "react";
import { CopyPlus, Plus } from "lucide-react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
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
import {
  thisYearMonth,
  carryoverSeed,
  monthStartDate,
  monthEndDateCapped,
} from "@/lib/dateLimits";
import { useSession } from "@/hooks/useSession";
import type { CarryoverDateMode } from "@/lib/dateLimits";
import type { RevaluationSuggestion } from "@/lib/revaluation";
import { formatCurrency, formatYearMonth, roundToCurrency } from "@/lib/format";

export type CreateSnapshotPayload = {
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

type Props<TResult> = {
  currency: string;
  // Mutation is owned by the parent so the same dialog can drive snapshot
  // creation for any position group (asset/liability/receivable).
  mutation: UseMutationResult<TResult, unknown, CreateSnapshotPayload>;
  // Optional revaluation helper (property + vehicle, Q8a / ADR-0008). The
  // parent encapsulates the signed annual rate + snapshot history and hands
  // the dialog a function it calls each render with the picked month; returns
  // null when no suggestion applies. The Apply button is the only writer —
  // typing the amount manually is never overridden.
  suggest?: (yearMonth: string) => RevaluationSuggestion | null;
  // Latest snapshot's amount + period, when one exists. Drives the "Copy
  // carryover" helper (issue #60): formalises an unchanged month by pre-filling
  // the amount and seeding the as-of date per the user's carryover_date_mode
  // preference (issue #105). lastSnapshotMonth (the latest snapshot's
  // year_month) anchors the end_of_month_after_last_snapshot mode. Null hides
  // the helper.
  carryover?: { amount: string; lastSnapshotMonth: string } | null;
};

export function CreateSnapshotDialog<TResult>({
  currency,
  mutation,
  suggest,
  carryover,
}: Props<TResult>) {
  const { t } = useTranslation("common");
  const { data: me } = useSession();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({
    year_month: thisYearMonth(),
    amount: "",
    as_of_date: "",
    description: "",
  });

  // Seed the form from the last snapshot and open the dialog. The month resets
  // to the current month; the statement date is seeded per the user's
  // carryover_date_mode preference (issue #105, default 'today'). The user
  // edits the month if the carryover belongs to an earlier period.
  function startCarryover() {
    if (!carryover) return;
    const mode = (me?.carryover_date_mode ?? "today") as CarryoverDateMode;
    const seed = carryoverSeed(mode, carryover.lastSnapshotMonth);
    setForm({
      year_month: seed.yearMonth,
      amount: carryover.amount,
      as_of_date: seed.asOfDate,
      description: "",
    });
    setOpen(true);
  }

  function close() {
    setOpen(false);
    setForm({
      year_month: thisYearMonth(),
      amount: "",
      as_of_date: "",
      description: "",
    });
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        year_month: form.year_month,
        amount: form.amount,
        currency,
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
          {t("snapshot.carryoverTrigger")}
        </Button>
      )}
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-1 size-4" />
          {t("snapshot.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("snapshot.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("snapshot.createDescription", { currency })}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="year_month">{t("fields.month")}</Label>
              <Input
                id="year_month"
                type="month"
                required
                max={thisYearMonth()}
                value={form.year_month}
                onChange={(e) =>
                  setForm({ ...form, year_month: e.target.value })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="as_of_date">{t("fields.statementDate")}</Label>
              <Input
                id="as_of_date"
                type="date"
                min={monthStartDate(form.year_month)}
                max={monthEndDateCapped(form.year_month)}
                value={form.as_of_date}
                onChange={(e) =>
                  setForm({ ...form, as_of_date: e.target.value })
                }
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="amount">{t("fields.amountIn", { currency })}</Label>
            <Input
              id="amount"
              required
              inputMode="decimal"
              value={form.amount}
              onChange={(e) => setForm({ ...form, amount: e.target.value })}
              placeholder={t("snapshot.amountPlaceholder")}
            />
            {(() => {
              const s = suggest?.(form.year_month);
              if (!s) return null;
              // Sign-aware copy: positive rate reads as "appreciation" (real
              // "+" prefix in EN, "apresiasi" verb in ID); negative rate
              // reads as "depreciation" (real minus "−" U+2212 in EN,
              // "penyusutan" verb in ID). Two keys keep the verb-form
              // localisable rather than dropping a raw glyph into ID copy.
              const magnitude = Math.abs(s.annualRatePct);
              const hintKey =
                s.annualRatePct > 0
                  ? "snapshot.revaluationHintAppreciate"
                  : "snapshot.revaluationHintDepreciate";
              return (
                <div
                  className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
                  data-testid="revaluation-hint"
                >
                  <span>
                    {t(hintKey, {
                      amount: formatCurrency(s.amount, currency),
                      magnitude,
                      months: s.monthsElapsed,
                      anchor: formatYearMonth(
                        s.anchorYearMonth + "-01T00:00:00Z",
                      ),
                    })}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2"
                    onClick={() =>
                      setForm({
                        ...form,
                        // Round to the currency's display precision (0 dp for
                        // IDR/JPY/KRW/VND, 2 dp elsewhere) so the input shows
                        // a clean figure rather than the helper's raw 4dp.
                        amount: roundToCurrency(s.amount, currency),
                      })
                    }
                    data-testid="revaluation-apply"
                  >
                    {t("snapshot.revaluationApply")}
                  </Button>
                </div>
              );
            })()}
          </div>

          <div className="grid gap-2">
            <Label htmlFor="description">{t("fields.description")}</Label>
            <Input
              id="description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              placeholder={t("snapshot.descriptionPlaceholder")}
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("actions.saving") : t("snapshot.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
