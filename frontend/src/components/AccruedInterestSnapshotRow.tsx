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
  EditAccruedInterestSnapshotDialog,
  type UpdateAccruedInterestSnapshotMutationVariables,
} from "@/components/EditAccruedInterestSnapshotDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { formatCurrency, formatYearMonth, formatDate } from "@/lib/format";

type AccruedInterestSnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  accrued_interest: string | null;
  as_of_date: string | null;
  description: string | null;
};

type Props<TUpdate, TDelete> = {
  snapshot: AccruedInterestSnapshotLike;
  updateMutation: UseMutationResult<
    TUpdate,
    unknown,
    UpdateAccruedInterestSnapshotMutationVariables
  >;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// principal = amount − accrued. Label is "Principal" for now — for
// secondary-market bonds it's technically "clean value", but the
// simplification is fine pre-alpha (column header is the only place this
// would mislead; renaming is cheap later).
function principal(snapshot: AccruedInterestSnapshotLike): string | null {
  if (!snapshot.accrued_interest) return null;
  const a = Number(snapshot.amount);
  const i = Number(snapshot.accrued_interest);
  if (Number.isNaN(a) || Number.isNaN(i)) return null;
  return (a - i).toString();
}

export function AccruedInterestSnapshotRow<TUpdate, TDelete>({
  snapshot,
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

  const p = principal(snapshot);

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
          {p !== null ? formatCurrency(p, snapshot.currency) : "—"}
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {snapshot.accrued_interest
            ? formatCurrency(snapshot.accrued_interest, snapshot.currency)
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
