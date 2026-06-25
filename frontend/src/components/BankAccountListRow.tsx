import { useState } from "react";
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
import { StatusBadge } from "@/components/StatusBadge";
import { EditBankAccountDialog } from "@/components/EditBankAccountDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { useDeleteBankAccount } from "@/hooks/useBankAccounts";
import { formatCurrency, formatYearMonth } from "@/lib/format";
import { isActiveStatus } from "@/lib/lifecycle";
import { cn } from "@/lib/utils";
import type { BankAccountListItem } from "@/api/types";

type Props = {
  item: BankAccountListItem;
  // Resolved by the screen (nickname ?? display_name, or "Joint") so the row
  // doesn't re-fetch household members per instance.
  ownerLabel: string;
  onSelect: (id: string) => void;
};

export function BankAccountListRow({ item, ownerLabel, onSelect }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deleteMutation = useDeleteBankAccount();

  const terminated = !isActiveStatus(item.asset.status);

  // EditBankAccountDialog wants a {asset, details} BankAccount shape; the
  // list-item also carries latest_snapshot, which the dialog ignores.
  const accountForEdit = { asset: item.asset, details: item.details };

  function handleConfirmDelete() {
    deleteMutation.mutate(item.asset.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  return (
    <>
      <TableRow
        className={cn("cursor-pointer", terminated && "text-muted-foreground")}
        onClick={() => onSelect(item.asset.id)}
      >
        <TableCell>
          <div className={cn("font-medium", terminated && "font-normal")}>
            {item.asset.display_name}
          </div>
          <div className="text-xs text-muted-foreground">
            {t("assets:bankAccount.detailHeaderLine", {
              bankName: item.details.bank_name,
              accountNumber: item.details.account_number,
              accountType: t(
                `assets:bankAccount.accountTypes.${item.details.account_type}`,
              ),
            })}
          </div>
        </TableCell>
        <TableCell>{ownerLabel}</TableCell>
        <TableCell>
          <StatusBadge group="assets" status={item.asset.status} />
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {item.latest_snapshot ? (
            <>
              <div>
                {formatCurrency(
                  item.latest_snapshot.amount,
                  item.latest_snapshot.currency,
                )}
              </div>
              <div className="text-xs text-muted-foreground">
                {formatYearMonth(item.latest_snapshot.year_month)}
              </div>
            </>
          ) : (
            <span className="text-muted-foreground">{"—"}</span>
          )}
        </TableCell>
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("assets:bankAccount.rowActions")}
                onClick={(e) => e.stopPropagation()}
              >
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              onClick={(e) => e.stopPropagation()}
            >
              <DropdownMenuItem onClick={() => setEditOpen(true)}>
                {t("common:actions.edit")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => setDeleteOpen(true)}
                variant="destructive"
              >
                {t("common:delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      <EditBankAccountDialog
        key={accountForEdit.asset.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        account={accountForEdit}
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("assets:bankAccount.deleteTitle")}
        description={t("assets:bankAccount.deleteRowDescription", {
          name: item.asset.display_name,
        })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}
