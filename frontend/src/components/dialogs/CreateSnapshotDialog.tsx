import { useState } from "react";
import { CopyPlus, Plus } from "lucide-react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
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
import { thisYearMonth, carryoverSeed } from "@/lib/dateLimits";
import { useSession } from "@/hooks/useSession";
import type { CarryoverDateMode } from "@/lib/dateLimits";
import type { RevaluationSuggestion } from "@/lib/revaluation";
import { SnapshotFormBody } from "@/components/dialogs/forms/SnapshotForm";

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
      {/* Copy carryover is the promoted primary of the create row (#542): the
          common month-close is "same as last month", so it leads with the solid
          variant and the large mobile tap target (`h-11 md:h-8`). New drops to a
          secondary outline — except on the first-ever snapshot, when there's no
          carryover and New stands alone as the primary. */}
      {carryover && (
        <Button
          type="button"
          size="sm"
          onClick={startCarryover}
          className="h-11 md:h-8"
          data-testid="snapshot-carryover"
        >
          <CopyPlus className="mr-1 size-4" />
          {t("snapshot.carryoverTrigger")}
        </Button>
      )}
      <DialogTrigger asChild>
        <Button
          size="sm"
          variant={carryover ? "outline" : "default"}
          className={carryover ? undefined : "h-11 md:h-8"}
        >
          <Plus className="mr-1 size-4" />
          {t("snapshot.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("snapshot.createTitle")}</DialogTitle>
          <DialogDescription>{t("snapshot.createDescription", { currency })}</DialogDescription>
        </DialogHeader>
        <ValidatedForm onSubmit={submit} error={mutation.error} className="space-y-3">
          <SnapshotFormBody
            mode="create"
            currency={currency}
            form={form}
            setForm={setForm}
            suggest={suggest}
          />

          <FormError />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("actions.saving") : t("snapshot.save")}
            </Button>
          </DialogFooter>
        </ValidatedForm>
      </DialogContent>
    </Dialog>
  );
}
