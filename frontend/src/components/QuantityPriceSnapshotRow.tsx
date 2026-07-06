import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { UseMutationResult } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TableCell, TableRow } from "@/components/ui/table";
import {
  EditQuantityPriceSnapshotDialog,
  type UpdateQuantityPriceSnapshotMutationVariables,
} from "@/components/EditQuantityPriceSnapshotDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { formatCurrency, formatYearMonth, formatDate } from "@/lib/format";

type QuantityPriceSnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  quantity: string | null;
  price_per_unit: string | null;
  as_of_date: string | null;
  description: string | null;
};

type Props<TUpdate, TDelete> = {
  snapshot: QuantityPriceSnapshotLike;
  // Unit label is subtype-specific ("sh" for stocks, "units" for mutual
  // funds, "g" for gold). Passed in so this row stays subtype-agnostic.
  quantityUnit: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateQuantityPriceSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

export function QuantityPriceSnapshotRow<TUpdate, TDelete>({
  snapshot,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation(["investments", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleConfirmDelete() {
    deleteMutation.mutate(snapshot.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  return (
    <>
      <TableRow>
        <TableCell>
          <div className="font-medium">{formatYearMonth(snapshot.year_month)}</div>
          {snapshot.as_of_date && (
            <div className="text-xs text-muted-foreground">
              {t("common:snapshot.statementPrefix", {
                date: formatDate(snapshot.as_of_date),
              })}
            </div>
          )}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {snapshot.quantity ? `${snapshot.quantity} ${quantityUnit}` : "—"}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {snapshot.price_per_unit
            ? formatCurrency(snapshot.price_per_unit, snapshot.currency)
            : "—"}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {formatCurrency(snapshot.amount, snapshot.currency)}
        </TableCell>
        <TableCell className="text-muted-foreground">{snapshot.description ?? "—"}</TableCell>
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={t("investments:snapshotRow.actions")}>
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setEditOpen(true)}>
                {t("common:actions.edit")}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setDeleteOpen(true)} variant="destructive">
                {t("common:delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

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
        description={t("investments:snapshotRow.deleteDescription", {
          month: formatYearMonth(snapshot.year_month),
        })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}
