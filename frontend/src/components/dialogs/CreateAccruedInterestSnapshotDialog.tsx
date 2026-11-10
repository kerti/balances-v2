import { useState } from "react";
import { CopyPlus, Plus } from "lucide-react";
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
import { thisYearMonth, carryoverSeed } from "@/lib/dateLimits";
import type { CarryoverDateMode } from "@/lib/dateLimits";
import { useSession } from "@/hooks/useSession";
import type { CreateInvestmentSnapshotPayload } from "@/hooks/useInvestmentSnapshots";
import type { CouponDisposition } from "@/api/types";
import { AccruedInterestSnapshotFormBody } from "@/components/dialogs/forms/AccruedInterestSnapshotForm";

type Props<TResult> = {
  currency: string;
  mutation: UseMutationResult<TResult, unknown, CreateInvestmentSnapshotPayload>;
  // Latest snapshot's total value + accrued + period, when one exists. Drives
  // the "Copy carryover" helper (issue #60); lastSnapshotMonth (the latest
  // snapshot's year_month) anchors the end_of_month_after_last_snapshot date
  // mode (issue #105). Null hides the helper.
  carryover?: {
    amount: string;
    accrued_interest: string | null;
    lastSnapshotMonth: string;
  } | null;
  // The bond's coupon disposition (#66). 'accrues' bonds carry coupon inside the
  // instrument, so the form starts the accrued field empty (force a real entry)
  // and swaps to the accrue-oriented hint; 'pays_out' (and time deposits, which
  // pass nothing) keep the historical accrued=0 default. Undefined ⇒ pays_out.
  couponDisposition?: CouponDisposition;
};

function emptyForm(couponDisposition?: CouponDisposition) {
  // accrued defaults to 0 for the common pays-out case (coupons land in the bank
  // account, no in-instrument accrual). An 'accrues' bond (#66) instead starts
  // empty so the user enters the real accrued figure rather than skipping the 0.
  return {
    year_month: thisYearMonth(),
    amount: "",
    accrued_interest: couponDisposition === "accrues" ? "" : "0",
    as_of_date: "",
    description: "",
  };
}

// `amount` is the dirty total value (already includes accrued); the user
// types it as it appears on the statement. The derived "of which principal"
// line below = amount − accrued. The backend's validateInvestmentSnapshotShape
// re-checks both fields are present for bond/time_deposit subtypes.
function derivePrincipal(amount: string, accrued: string): string | null {
  const a = Number(amount);
  const i = Number(accrued);
  if (!amount || !accrued || Number.isNaN(a) || Number.isNaN(i)) {
    return null;
  }
  return (a - i).toString();
}

export function CreateAccruedInterestSnapshotDialog<TResult>({
  currency,
  mutation,
  carryover,
  couponDisposition,
}: Props<TResult>) {
  const { t } = useTranslation(["investments", "common"]);
  const { data: me } = useSession();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(() => emptyForm(couponDisposition));

  const derivedPrincipal = derivePrincipal(form.amount, form.accrued_interest);

  // Seed the form from the last snapshot and open the dialog. Month resets to
  // the current month; the statement date is seeded per the user's
  // carryover_date_mode preference (issue #105, default 'today').
  function startCarryover() {
    if (!carryover) return;
    const mode = (me?.carryover_date_mode ?? "today") as CarryoverDateMode;
    const seed = carryoverSeed(mode, carryover.lastSnapshotMonth);
    setForm({
      year_month: seed.yearMonth,
      amount: carryover.amount,
      accrued_interest: carryover.accrued_interest ?? "0",
      as_of_date: seed.asOfDate,
      description: "",
    });
    setOpen(true);
  }

  function close() {
    setOpen(false);
    setForm(emptyForm(couponDisposition));
    mutation.reset();
  }

  // Re-seed the form from the *current* couponDisposition on each open. The
  // dialog stays mounted, so the lazy useState initializer can't reflect a
  // disposition the user changed after mount (e.g. editing the bond's
  // disposition on the detail screen); seeding here keeps the accrued default
  // (#66 pivot) in sync. The carryover path seeds its own form and bypasses this.
  function openFresh() {
    setForm(emptyForm(couponDisposition));
    setOpen(true);
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        year_month: form.year_month,
        amount: form.amount,
        currency,
        quantity: null,
        price_per_unit: null,
        accrued_interest: form.accrued_interest,
        as_of_date: form.as_of_date || null,
        description: form.description || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? openFresh() : close())}>
      {/* Copy carryover leads the create row as the promoted primary (#542):
          solid variant + large mobile tap target. New drops to a secondary
          outline unless there's no prior snapshot to copy, when it stands alone
          as the primary. Mirrors the amount-only `CreateSnapshotDialog`. */}
      {carryover && (
        <Button
          type="button"
          size="sm"
          onClick={startCarryover}
          className="h-11 md:h-8"
          data-testid="snapshot-carryover"
        >
          <CopyPlus className="mr-1 size-4" />
          {t("investments:accruedInterestSnapshot.carryoverTrigger")}
        </Button>
      )}
      <DialogTrigger asChild>
        <Button
          size="sm"
          variant={carryover ? "outline" : "default"}
          className={carryover ? undefined : "h-11 md:h-8"}
        >
          <Plus className="mr-1 size-4" />
          {t("investments:accruedInterestSnapshot.trigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("investments:accruedInterestSnapshot.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:accruedInterestSnapshot.createDescription", {
              currency,
            })}
          </DialogDescription>
        </DialogHeader>
        <ValidatedForm onSubmit={submit} error={mutation.error} className="space-y-3">
          <AccruedInterestSnapshotFormBody
            mode="create"
            currency={currency}
            form={form}
            setForm={setForm}
            derivedPrincipal={derivedPrincipal}
            couponDisposition={couponDisposition}
          />

          <FormError />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common:actions.saving")
                : t("investments:accruedInterestSnapshot.save")}
            </Button>
          </DialogFooter>
        </ValidatedForm>
      </DialogContent>
    </Dialog>
  );
}
