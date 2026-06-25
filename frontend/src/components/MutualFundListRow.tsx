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
import { EditMutualFundDialog } from "@/components/EditMutualFundDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { useDeleteMutualFund } from "@/hooks/useInvestments";
import { formatCurrency, formatYearMonth } from "@/lib/format";
import { StatusBadge } from "@/components/StatusBadge";
import { isActiveStatus } from "@/lib/lifecycle";
import { cn } from "@/lib/utils";
import { RiskProfileBadge } from "@/components/RiskProfileBadge";
import { TransactionActivityCell } from "@/components/TransactionActivityCell";
import type { MutualFundListItem } from "@/api/types";

type Props = {
  item: MutualFundListItem;
  onSelect: (id: string) => void;
};

export function MutualFundListRow({ item, onSelect }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deleteMutation = useDeleteMutualFund();

  const terminated = !isActiveStatus(item.investment.status);

  function handleConfirmDelete() {
    deleteMutation.mutate(item.investment.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  return (
    <>
      <TableRow
        className={cn("cursor-pointer", terminated && "text-muted-foreground")}
        onClick={() => onSelect(item.investment.id)}
      >
        <TableCell>
          <div className={cn("font-medium", terminated && "font-normal")}>
            {item.investment.display_name}
          </div>
          <div className="mt-0.5 flex items-center gap-2">
            <RiskProfileBadge profile={item.investment.risk_profile} compact />
            <span
              className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground"
              data-testid="mf-fund-type"
            >
              {t(
                `investments:mutualFund.fundType.short.${item.details.fund_type}`,
              )}
            </span>
          </div>
          {item.investment.description && (
            <div className="text-xs text-muted-foreground">
              {item.investment.description}
            </div>
          )}
        </TableCell>
        <TableCell>
          <div className="font-mono text-sm">{item.details.fund_code}</div>
          {item.details.fund_manager && (
            <div className="text-xs text-muted-foreground">
              {item.details.fund_manager}
            </div>
          )}
        </TableCell>
        <TableCell>
          <StatusBadge group="investments" status={item.investment.status} />
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
            <span className="text-muted-foreground">—</span>
          )}
        </TableCell>
        <TransactionActivityCell
          count={item.transaction_count}
          lastDate={item.last_transaction_date}
        />
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("investments:mutualFund.rowActions")}
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

      <EditMutualFundDialog
        key={item.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        mutualFund={item}
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("investments:mutualFund.deleteTitle")}
        description={t("investments:mutualFund.deleteRowDescription", {
          name: item.investment.display_name,
        })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}
