import { useState } from "react";
import type { UseMutationResult } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
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
  EditSnapshotDialog,
  type UpdateSnapshotMutationVariables,
} from "@/components/EditSnapshotDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { formatCurrency, formatYearMonth, formatDate } from "@/lib/format";

type SnapshotLike = {
  id: string;
  year_month: string;
  amount: string;
  currency: string;
  as_of_date: string | null;
  description: string | null;
};

type Props<TUpdate, TDelete> = {
  snapshot: SnapshotLike;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateSnapshotMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

export function SnapshotRow<TUpdate, TDelete>({
  snapshot,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation("common");
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
              {t("snapshot.statementPrefix", {
                date: formatDate(snapshot.as_of_date),
              })}
            </div>
          )}
        </TableCell>
        <TableCell>{formatCurrency(snapshot.amount, snapshot.currency)}</TableCell>
        <TableCell className="text-muted-foreground">{snapshot.description ?? "—"}</TableCell>
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={t("snapshot.rowActions")}>
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => setEditOpen(true)}>
                {t("actions.edit")}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setDeleteOpen(true)} variant="destructive">
                {t("delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

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
        description={t("snapshot.deleteDescription", {
          month: formatYearMonth(snapshot.year_month),
        })}
        confirmLabel={t("delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}
