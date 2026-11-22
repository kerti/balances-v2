import { useState } from "react";
import { focusFirstValue } from "@/lib/recordSnapshot";
import { useRecordOnArrival } from "@/lib/recordOnArrival";
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
} from "@/components/ui/dialog";
import { subtractDecimals } from "@/lib/decimal";
import { formatCurrency } from "@/lib/format";
import { swingAsk, type SwingAsk } from "@/lib/valueSwing";
import { SwingConfirmDialog } from "@/components/dialogs/SwingConfirmDialog";
import { asOfInputValue, formatRecordMonth } from "@/lib/recordSnapshot";
import { useOnRecordOpen, useOnRecordRequest } from "@/lib/recordSnapshotRequest";
import { useRecordSnapshotForm } from "@/hooks/useRecordSnapshotForm";
import type { CreateInvestmentSnapshotPayload } from "@/hooks/useInvestmentSnapshots";
import type { CouponDisposition } from "@/api/types";
import {
  AccruedInterestSnapshotFormBody,
  type AccruedInterestSnapshotFormState,
} from "@/components/dialogs/forms/AccruedInterestSnapshotForm";
import type { UpdateAccruedInterestSnapshotMutationVariables } from "@/components/dialogs/EditAccruedInterestSnapshotDialog";

// The live snapshots the dialog needs (see CreateSnapshotDialog).
type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  accrued_interest: string | null;
  as_of_date: string | null;
  description: string | null;
};

type Props<TCreated, TUpdated> = {
  currency: string;
  // Live snapshots of this position, newest first.
  snapshots: SnapshotLike[] | undefined;
  // Create writes an empty month; update replaces a month that already has a
  // value (ADR-0058: archived, Undo in the toast).
  createMutation: UseMutationResult<TCreated, unknown, CreateInvestmentSnapshotPayload>;
  updateMutation: UseMutationResult<
    TUpdated,
    unknown,
    UpdateAccruedInterestSnapshotMutationVariables
  >;
  // The bond's coupon disposition (#66). 'accrues' bonds carry coupon inside the
  // instrument, so the form starts the accrued field empty (force a real entry)
  // and swaps to the accrue-oriented hint; 'pays_out' keeps the historical
  // accrued=0 default. Undefined ⇒ pays_out. Bonds only — see `principal`.
  couponDisposition?: CouponDisposition;
  // Time deposits pass their principal in force (#844): the form then takes one
  // Value, shows "of which interest" live, and sends no accrued_interest — the
  // server derives it. Bonds pass nothing and keep the two-figure form.
  principal?: string;
};

// `amount` is the dirty total value (already includes accrued); the user
// types it as it appears on the statement, and what is typed is what is sent.
// The "of which principal" line below is display only: amount − accrued.

// "Record snapshot" for the accrued-interest shape (bond, time deposit). See
// CreateSnapshotDialog for the replace-mode behaviour (#788).
export function CreateAccruedInterestSnapshotDialog<TCreated, TUpdated>({
  currency,
  snapshots,
  createMutation,
  updateMutation,
  couponDisposition,
  principal,
}: Props<TCreated, TUpdated>) {
  const { t } = useTranslation(["investments", "common"]);
  // Open straight away when a create flow just landed on this item (#843).
  const [open, setOpen] = useState(useRecordOnArrival());

  // accrued defaults to 0 for the common pays-out case (coupons land in the bank
  // account, no in-instrument accrual). An 'accrues' bond (#66) instead starts
  // empty so the user enters the real accrued figure rather than skipping the 0.
  // Read from the *current* couponDisposition each time the dialog opens, since
  // the dialog stays mounted while the bond's disposition may be edited.
  const emptyForm = (yearMonth: string): AccruedInterestSnapshotFormState => ({
    year_month: yearMonth,
    amount: "",
    accrued_interest: couponDisposition === "accrues" ? "" : "0",
    as_of_date: "",
    description: "",
  });
  const formFrom = (s: SnapshotLike, yearMonth: string): AccruedInterestSnapshotFormState => ({
    year_month: yearMonth,
    amount: s.amount,
    accrued_interest: s.accrued_interest ?? "",
    as_of_date: asOfInputValue(s.as_of_date),
    description: s.description ?? "",
  });

  const { form, setForm, existing, last, prior, pickMonth, reset } = useRecordSnapshotForm({
    snapshots,
    emptyForm,
    formFrom,
  });
  const mutation = existing ? updateMutation : createMutation;
  // A total value 10x or a tenth of the last one waits for a yes (#933).
  const [ask, setAsk] = useState<SwingAsk | null>(null);

  const derivedPrincipal = subtractDecimals(form.amount, form.accrued_interest);

  function begin() {
    reset();
    setOpen(true);
  }

  // The detail header's Record snapshot button (#935) asks this dialog to open.
  useOnRecordOpen(begin);

  // "Update {Mon} holding →" (#830) after a bond trade: this shape has no
  // quantity or price to carry over, so it just opens on the default month.
  useOnRecordRequest(begin);

  function close() {
    setOpen(false);
    setAsk(null);
    createMutation.reset();
    updateMutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const question = swingAsk(prior, form.amount, currency);
    if (question) setAsk(question);
    else save();
  }

  function save() {
    const common = {
      amount: form.amount,
      currency,
      quantity: null,
      price_per_unit: null,
      accrued_interest: principal !== undefined ? null : form.accrued_interest,
      as_of_date: form.as_of_date || null,
      description: form.description || null,
    };
    if (existing) {
      updateMutation.mutate({ snapshotId: existing.id, payload: common }, { onSuccess: close });
      return;
    }
    createMutation.mutate({ year_month: form.year_month, ...common }, { onSuccess: close });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? begin() : close())}>
      <DialogContent onOpenAutoFocus={focusFirstValue}>
        <DialogHeader>
          <DialogTitle>{t("investments:accruedInterestSnapshot.createTitle")}</DialogTitle>
          <DialogDescription>
            {t(
              principal !== undefined
                ? "investments:accruedInterestSnapshot.createDescriptionTimeDeposit"
                : "investments:accruedInterestSnapshot.createDescription",
              { currency },
            )}
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
            principal={principal}
            onMonthChange={pickMonth}
            replacing={existing ? formatCurrency(existing.amount, currency) : undefined}
            lastRecorded={
              last
                ? {
                    value: formatCurrency(last.amount, currency),
                    month: formatRecordMonth(last.year_month),
                    onApply: () =>
                      setForm({
                        ...form,
                        amount: last.amount,
                        accrued_interest: last.accrued_interest ?? form.accrued_interest,
                      }),
                  }
                : undefined
            }
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
        <SwingConfirmDialog
          ask={ask}
          onConfirm={() => {
            setAsk(null);
            save();
          }}
          onCancel={() => setAsk(null)}
        />
      </DialogContent>
    </Dialog>
  );
}
