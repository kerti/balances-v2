import type { UseMutationResult } from "@tanstack/react-query";
import { TableCell, TableRow } from "@/components/ui/table";
import { TransactionRowMenu } from "@/components/common/TransactionRowMenu";
import { useTransactionRow } from "@/components/common/useTransactionRow";
import type { InvestmentTransaction } from "@/api/types";
import type { UpdateTransactionMutationVariables } from "@/components/dialogs/EditTradeTransactionDialog";

type Props<TUpdate, TDelete> = {
  transaction: InvestmentTransaction;
  quantityUnit: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateTransactionMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// The desktop table row for an investment transaction. Shares its state,
// dialogs, cash-impact text and ⋮ menu with the mobile `TransactionCard` via
// `useTransactionRow`, and the same `transaction-row` / `transaction-amount`
// testids, so the two renderers stay in lockstep (ADR-0051 Phase B).
export function TransactionRow<TUpdate, TDelete>({
  transaction,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const {
    dateText,
    label,
    detail,
    impactText,
    impactColorClass,
    description,
    onEdit,
    onDelete,
    dialogs,
  } = useTransactionRow(transaction, quantityUnit, updateMutation, deleteMutation);

  return (
    <>
      <TableRow data-testid="transaction-row">
        <TableCell>
          <div className="font-medium">{dateText}</div>
        </TableCell>
        <TableCell>
          <div className="font-medium">{label}</div>
          {detail && <div className="text-xs text-muted-foreground">{detail}</div>}
        </TableCell>
        <TableCell
          className={`text-right tabular-nums ${impactColorClass}`}
          data-testid="transaction-amount"
        >
          {impactText}
        </TableCell>
        <TableCell className="text-muted-foreground">{description ?? "—"}</TableCell>
        <TableCell className="text-right">
          <TransactionRowMenu onEdit={onEdit} onDelete={onDelete} />
        </TableCell>
      </TableRow>

      {dialogs}
    </>
  );
}
