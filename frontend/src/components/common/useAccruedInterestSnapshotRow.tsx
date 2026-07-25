import { useState, type ReactNode } from "react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  EditAccruedInterestSnapshotDialog,
  type UpdateAccruedInterestSnapshotMutationVariables,
} from "@/components/dialogs/EditAccruedInterestSnapshotDialog";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { formatCurrency, formatYearMonth, formatDate } from "@/lib/format";

export type AccruedInterestSnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  accrued_interest: string | null;
  as_of_date: string | null;
  description: string | null;
};

// principal = amount − accrued. Label is "Principal" for now — for
// secondary-market bonds it's technically "clean value", but the
// simplification is fine pre-alpha (the header is the only place this would
// mislead; renaming is cheap later).
function principal(snapshot: AccruedInterestSnapshotLike): string | null {
  if (!snapshot.accrued_interest) return null;
  const a = Number(snapshot.amount);
  const i = Number(snapshot.accrued_interest);
  if (Number.isNaN(a) || Number.isNaN(i)) return null;
  return (a - i).toString();
}

// The shared accrued snapshot row logic (ADR-0051 Phase B): edit/delete dialog
// state, the formatted text, and the two dialog nodes, single-sourced so the
// desktop `AccruedInterestSnapshotRow` and the mobile
// `AccruedInterestSnapshotCard` present the same values, open the same dialogs,
// and can never drift between renderers — the `useQuantityPriceSnapshotRow`
// idiom applied to the accrued shape (Bond / TimeDeposit).
export function useAccruedInterestSnapshotRow<TUpdate, TDelete>(
  snapshot: AccruedInterestSnapshotLike,
  updateMutation: UseMutationResult<
    TUpdate,
    unknown,
    UpdateAccruedInterestSnapshotMutationVariables
  >,
  deleteMutation: UseMutationResult<TDelete, unknown, string>,
) {
  const { t } = useTranslation(["investments", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleConfirmDelete() {
    deleteMutation.mutate(snapshot.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  const p = principal(snapshot);

  const monthText = formatYearMonth(snapshot.year_month);
  const amountText = formatCurrency(snapshot.amount, snapshot.currency);
  const principalText = p !== null ? formatCurrency(p, snapshot.currency) : null;
  const accruedText = snapshot.accrued_interest
    ? formatCurrency(snapshot.accrued_interest, snapshot.currency)
    : null;
  const statementText = snapshot.as_of_date
    ? t("common:snapshot.statementPrefix", { date: formatDate(snapshot.as_of_date) })
    : null;

  const dialogs: ReactNode = (
    <>
      <EditAccruedInterestSnapshotDialog
        key={snapshot.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        snapshot={snapshot}
        mutation={updateMutation}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("investments:snapshotRow.deleteTitle")}
        description={t("investments:snapshotRow.deleteDescription", { month: monthText })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );

  return {
    monthText,
    amountText,
    principalText,
    accruedText,
    statementText,
    description: snapshot.description,
    onEdit: () => setEditOpen(true),
    onDelete: () => setDeleteOpen(true),
    dialogs,
  };
}
