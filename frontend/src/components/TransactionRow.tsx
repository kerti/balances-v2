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
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { EditTradeTransactionDialog } from "@/components/EditTradeTransactionDialog";
import { EditCashIncomeTransactionDialog } from "@/components/EditCashIncomeTransactionDialog";
import { EditFeeTransactionDialog } from "@/components/EditFeeTransactionDialog";
import { EditMaturityTransactionDialog } from "@/components/EditMaturityTransactionDialog";
import { formatCurrency, formatDate } from "@/lib/format";
import type { InvestmentTransaction } from "@/api/types";
import type { UpdateTransactionMutationVariables } from "@/components/EditTradeTransactionDialog";

// Sign convention for the Cash impact column:
//   Buy, Fee     → negative (cash out)
//   Sell, Coupon, Dividend, Distribution → positive (cash in)
//   Maturity     → positive total (principal + interest), unless both
//                  rolled (then 0 / "rolled")
type Direction = "in" | "out" | "mixed";

function impactDirection(t: InvestmentTransaction): Direction {
  switch (t.transaction_type) {
    case "buy":
    case "fee":
      return "out";
    case "sell":
    case "coupon":
    case "dividend":
    case "distribution":
      return "in";
    case "maturity": {
      const bothRolled =
        t.principal_disposition === "rolled_to_new" && t.interest_disposition === "rolled_to_new";
      return bothRolled ? "mixed" : "in";
    }
  }
}

function impactAmount(t: InvestmentTransaction): string | null {
  if (t.transaction_type === "maturity") {
    if (!t.principal_amount || !t.interest_amount) return null;
    const p = Number(t.principal_amount);
    const i = Number(t.interest_amount);
    if (Number.isNaN(p) || Number.isNaN(i)) return null;
    // For cash-out portions only, show only the cash-out sum. Rolled
    // portions are tracked in the detail line below.
    const principalCash = t.principal_disposition === "cash_out" ? p : 0;
    const interestCash = t.interest_disposition === "cash_out" ? i : 0;
    return (principalCash + interestCash).toString();
  }
  return t.amount;
}

function impactColor(dir: Direction): string {
  if (dir === "in") return "text-emerald-600";
  if (dir === "out") return "text-destructive";
  return "text-muted-foreground";
}

type Props<TUpdate, TDelete> = {
  transaction: InvestmentTransaction;
  quantityUnit: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateTransactionMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

export function TransactionRow<TUpdate, TDelete>({
  transaction,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const { t } = useTranslation(["investments", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const dir = impactDirection(transaction);
  const impact = impactAmount(transaction);
  const label = t(`investments:transactionType.${transaction.transaction_type}`);

  function detailLine(): string {
    switch (transaction.transaction_type) {
      case "buy":
      case "sell":
        if (!transaction.quantity || !transaction.price_per_unit) return "";
        return t("investments:transactionRow.tradeDetail", {
          quantity: transaction.quantity,
          unit: quantityUnit,
          price: formatCurrency(transaction.price_per_unit, transaction.currency),
        });
      case "fee":
        if (transaction.quantity && transaction.price_per_unit) {
          return t("investments:transactionRow.feeDetail", {
            quantity: transaction.quantity,
            unit: quantityUnit,
            price: formatCurrency(transaction.price_per_unit, transaction.currency),
          });
        }
        return "";
      case "maturity": {
        const parts: string[] = [];
        if (transaction.principal_amount) {
          const disp = t(
            transaction.principal_disposition === "rolled_to_new"
              ? "investments:disposition.rolledShort"
              : "investments:disposition.cashShort",
          );
          parts.push(
            t("investments:transactionRow.maturityPrincipalDetail", {
              amount: formatCurrency(transaction.principal_amount, transaction.currency),
              disp,
            }),
          );
        }
        if (transaction.interest_amount) {
          const disp = t(
            transaction.interest_disposition === "rolled_to_new"
              ? "investments:disposition.rolledShort"
              : "investments:disposition.cashShort",
          );
          parts.push(
            t("investments:transactionRow.maturityInterestDetail", {
              amount: formatCurrency(transaction.interest_amount, transaction.currency),
              disp,
            }),
          );
        }
        return parts.join(" · ");
      }
      default:
        return "";
    }
  }

  const detail = detailLine();

  function handleConfirmDelete() {
    deleteMutation.mutate(transaction.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  function impactText(): string {
    if (impact === null) return "—";
    if (dir === "mixed" && Number(impact) === 0) {
      return t("investments:transactionRow.rolledImpact");
    }
    const sign = dir === "out" ? "−" : dir === "in" ? "+" : "";
    return `${sign}${formatCurrency(impact, transaction.currency)}`;
  }

  return (
    <>
      <TableRow>
        <TableCell>
          <div className="font-medium">{formatDate(transaction.transaction_date)}</div>
        </TableCell>
        <TableCell>
          <div className="font-medium">{label}</div>
          {detail && <div className="text-xs text-muted-foreground">{detail}</div>}
        </TableCell>
        <TableCell className={`text-right tabular-nums ${impactColor(dir)}`}>
          {impactText()}
        </TableCell>
        <TableCell className="text-muted-foreground">{transaction.description ?? "—"}</TableCell>
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t("investments:transactionRow.actions")}
              >
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

      {/* Edit dialogs are shape-conditional; one of them is mounted at a
          time. Mounting per-shape keeps each dialog's form state initialized
          from the right fields. */}
      {(transaction.transaction_type === "buy" || transaction.transaction_type === "sell") && (
        <EditTradeTransactionDialog
          key={transaction.id}
          open={editOpen}
          onOpenChange={setEditOpen}
          transaction={transaction}
          quantityUnit={quantityUnit}
          mutation={updateMutation}
        />
      )}
      {(transaction.transaction_type === "coupon" ||
        transaction.transaction_type === "dividend" ||
        transaction.transaction_type === "distribution") && (
        <EditCashIncomeTransactionDialog
          key={transaction.id}
          open={editOpen}
          onOpenChange={setEditOpen}
          transaction={transaction}
          mutation={updateMutation}
        />
      )}
      {transaction.transaction_type === "fee" && (
        <EditFeeTransactionDialog
          key={transaction.id}
          open={editOpen}
          onOpenChange={setEditOpen}
          transaction={transaction}
          quantityUnit={quantityUnit}
          mutation={updateMutation}
        />
      )}
      {transaction.transaction_type === "maturity" && (
        <EditMaturityTransactionDialog
          key={transaction.id}
          open={editOpen}
          onOpenChange={setEditOpen}
          transaction={transaction}
          mutation={updateMutation}
        />
      )}

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("investments:transactionRow.deleteTitle")}
        description={t("investments:transactionRow.deleteDescription", {
          label: label.toLowerCase(),
          date: formatDate(transaction.transaction_date),
        })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );
}
