import { useState, type ReactNode } from "react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  EditSnapshotDialog,
  type UpdateSnapshotMutationVariables,
} from "@/components/dialogs/EditSnapshotDialog";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { formatCurrency, formatYearMonth, formatDate } from "@/lib/format";

export type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

// The shared amount-only snapshot row logic (ADR-0051 Phase B): edit/delete
// dialog state, the formatted text, and the two dialog nodes, single-sourced so
// the desktop `SnapshotRow` and the mobile `SnapshotCard` present the same
// values, open the same dialogs, and can never drift between renderers — the
// `useIncomeRow` idiom applied to snapshots.
export function useSnapshotRow<TUpdate, TDelete>(
  snapshot: SnapshotLike,
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateSnapshotMutationVariables>,
  deleteMutation: UseMutationResult<TDelete, unknown, string>,
) {
  const { t } = useTranslation("common");
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleConfirmDelete() {
    deleteMutation.mutate(snapshot.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  const monthText = formatYearMonth(snapshot.year_month);
  const amountText = formatCurrency(snapshot.amount, snapshot.currency);
  const statementText = snapshot.as_of_date
    ? t("snapshot.statementPrefix", { date: formatDate(snapshot.as_of_date) })
    : null;

  const dialogs: ReactNode = (
    <>
      <EditSnapshotDialog
        key={snapshot.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        snapshot={snapshot}
        mutation={updateMutation}
      />
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("snapshot.deleteTitle")}
        description={t("snapshot.deleteDescription", { month: monthText })}
        confirmLabel={t("delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );

  return {
    monthText,
    amountText,
    statementText,
    description: snapshot.description,
    onEdit: () => setEditOpen(true),
    onDelete: () => setDeleteOpen(true),
    dialogs,
  };
}
