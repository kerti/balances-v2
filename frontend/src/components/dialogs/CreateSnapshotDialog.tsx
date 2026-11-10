import { useState } from "react";
import { useRecordOnArrival } from "@/lib/recordOnArrival";
import { Plus } from "lucide-react";
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
import { formatCurrency } from "@/lib/format";
import { asOfInputValue, formatRecordMonth } from "@/lib/recordSnapshot";
import { useRecordSnapshotForm } from "@/hooks/useRecordSnapshotForm";
import type { RevaluationSuggestion } from "@/lib/revaluation";
import { SnapshotFormBody, type SnapshotFormState } from "@/components/dialogs/forms/SnapshotForm";
import type { UpdateSnapshotMutationVariables } from "@/components/dialogs/EditSnapshotDialog";

export type CreateSnapshotPayload = {
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

// The live snapshots the dialog needs: to find the next missing month, to spot a
// month that already has a value, and to offer "Same as last month".
type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  as_of_date: string | null;
  description: string | null;
};

type Props<TCreated, TUpdated> = {
  currency: string;
  // Live snapshots of this position, newest first.
  snapshots: SnapshotLike[] | undefined;
  // Mutations are owned by the parent so the same dialog can drive snapshot
  // recording for any amount-only group (asset/liability/receivable). Create
  // writes an empty month; update replaces a month that already has a value
  // (ADR-0058: the earlier value is archived, the toast offers Undo).
  createMutation: UseMutationResult<TCreated, unknown, CreateSnapshotPayload>;
  updateMutation: UseMutationResult<TUpdated, unknown, UpdateSnapshotMutationVariables>;
  // Optional revaluation helper (property + vehicle, Q8a / ADR-0008). The
  // parent encapsulates the signed annual rate + snapshot history and hands
  // the dialog a function it calls each render with the picked month; returns
  // null when no suggestion applies. The Apply button is the only writer —
  // typing the amount manually is never overridden.
  suggest?: (yearMonth: string) => RevaluationSuggestion | null;
};

const emptyForm = (yearMonth: string): SnapshotFormState => ({
  year_month: yearMonth,
  amount: "",
  as_of_date: "",
  description: "",
});

const formFrom = (s: SnapshotLike, yearMonth: string): SnapshotFormState => ({
  year_month: yearMonth,
  amount: s.amount,
  as_of_date: asOfInputValue(s.as_of_date),
  description: s.description ?? "",
});

// "Record snapshot" for the amount-only shape (ADR-0046 amendment, #788). One
// button, opening on the month after the latest snapshot. A month that already
// has a value switches the dialog to replacing it, with a notice, instead of
// failing with SNAPSHOT_MONTH_EXISTS.
export function CreateSnapshotDialog<TCreated, TUpdated>({
  currency,
  snapshots,
  createMutation,
  updateMutation,
  suggest,
}: Props<TCreated, TUpdated>) {
  const { t } = useTranslation("common");
  // Open straight away when a create flow just landed on this item (#843).
  const [open, setOpen] = useState(useRecordOnArrival());
  const { form, setForm, existing, last, pickMonth, reset } = useRecordSnapshotForm({
    snapshots,
    emptyForm,
    formFrom,
  });
  const mutation = existing ? updateMutation : createMutation;

  function begin() {
    reset();
    setOpen(true);
  }

  function close() {
    setOpen(false);
    createMutation.reset();
    updateMutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const common = {
      currency,
      as_of_date: form.as_of_date || null,
      description: form.description || null,
    };
    if (existing) {
      updateMutation.mutate(
        { snapshotId: existing.id, payload: { amount: form.amount, ...common } },
        { onSuccess: close },
      );
      return;
    }
    createMutation.mutate(
      { year_month: form.year_month, amount: form.amount, ...common },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? begin() : close())}>
      {/* The single entry point to recording a value (#788): the primary of the
          create row, with the large mobile tap target (`h-11 md:h-8`). */}
      <DialogTrigger asChild>
        <Button size="sm" className="h-11 md:h-8" data-testid="snapshot-record">
          <Plus className="mr-1 size-4" />
          {t("snapshot.recordTrigger")}
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
            onMonthChange={pickMonth}
            replacing={existing ? formatCurrency(existing.amount, currency) : undefined}
            lastRecorded={
              last
                ? {
                    value: formatCurrency(last.amount, currency),
                    month: formatRecordMonth(last.year_month),
                    onApply: () => setForm({ ...form, amount: last.amount }),
                  }
                : undefined
            }
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
