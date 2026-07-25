import { useState, type ReactNode } from "react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  EditQuantityPriceSnapshotDialog,
  type UpdateQuantityPriceSnapshotMutationVariables,
} from "@/components/dialogs/EditQuantityPriceSnapshotDialog";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { formatCurrency, formatYearMonth, formatDate } from "@/lib/format";

export type QuantityPriceSnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  quantity: string | null;
  price_per_unit: string | null;
  as_of_date: string | null;
  description: string | null;
};

// The shared qty×price snapshot row logic (ADR-0051 Phase B): edit/delete dialog
// state, the formatted text, and the two dialog nodes, single-sourced so the
// desktop `QuantityPriceSnapshotRow` and the mobile `QuantityPriceSnapshotCard`
// present the same values, open the same dialogs, and can never drift between
// renderers — the `useSnapshotRow` idiom applied to the qty×price shape (Stock /
// MutualFund / Gold). `quantityUnit` is subtype-specific ("sh"/"units"/"g"),
// passed in so this stays subtype-agnostic.
export function useQuantityPriceSnapshotRow<TUpdate, TDelete>(
  snapshot: QuantityPriceSnapshotLike,
  quantityUnit: string,
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateQuantityPriceSnapshotMutationVariables>,
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

  const monthText = formatYearMonth(snapshot.year_month);
  const amountText = formatCurrency(snapshot.amount, snapshot.currency);
  const quantityText = snapshot.quantity ? `${snapshot.quantity} ${quantityUnit}` : null;
  const priceText = snapshot.price_per_unit
    ? formatCurrency(snapshot.price_per_unit, snapshot.currency)
    : null;
  const statementText = snapshot.as_of_date
    ? t("common:snapshot.statementPrefix", { date: formatDate(snapshot.as_of_date) })
    : null;

  const dialogs: ReactNode = (
    <>
      <EditQuantityPriceSnapshotDialog
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
    quantityText,
    priceText,
    statementText,
    description: snapshot.description,
    onEdit: () => setEditOpen(true),
    onDelete: () => setDeleteOpen(true),
    dialogs,
  };
}
